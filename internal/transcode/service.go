package transcode

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/pkg/logger"
)

var (
	log = logger.Get("TranscodeServ")

	ErrTaskNotFound = errors.New("no task found")
)

type (
	DataStore interface {
		SaveTranscode(task *TranscodeTask) error
		GetAllWorkflows() []*workflow.Workflow
		GetMedia(mediaID uuid.UUID) *media.Container
		GetTarget(targetID uuid.UUID) *ffmpeg.Target
		GetForMediaAndTarget(mediaID uuid.UUID, targetID uuid.UUID) (*Transcode, error)
	}

	// transcodeService is Thea's solution to pre-transcoding of user media.
	// It is responsible for some key aspects of Thea:
	//   - Transcoding workflows for newly ingested media
	//   - Manual transcode requests for ingested media
	//   - Live-tracking and reporting of ongoing transcodes over the event bus
	// 	 - Persistence of completed transcodes to the transcode store
	transcodeService struct {
		*sync.Mutex
		taskWg          *sync.WaitGroup
		config          *Config
		tasks           []*TranscodeTask
		consumedThreads int

		eventBus  event.EventCoordinator
		dataStore DataStore

		queueChange chan bool
		taskChange  chan uuid.UUID
	}
)

// New creates a new transcodeService, injecting all required stores. Error is returned
// in the configuration provided is not valid (e.g., ffmpeg path is wrong).
func New(config Config, eventBus event.EventCoordinator, dataStore DataStore) (*transcodeService, error) {
	// Check for output path dir, create if not found

	// Ensure ffmpeg/ffprobe available at the bin path provided

	// Ensure maximum thread consumption is reasonable (>2)

	return &transcodeService{
		Mutex:       &sync.Mutex{},
		taskWg:      &sync.WaitGroup{},
		config:      &config,
		tasks:       make([]*TranscodeTask, 0),
		eventBus:    eventBus,
		dataStore:   dataStore,
		queueChange: make(chan bool, 128),
		taskChange:  make(chan uuid.UUID, 128),
	}, nil
}

// Run is the main entry point for this service. This method will block
// until the provided context is cancelled.
// Note: when context is cancelled this method will not immediately return as it
// will wait for it's running transcode tasks to cancel.
func (service *transcodeService) Run(ctx context.Context) error {
	eventChannel := make(event.HandlerChannel, 100)
	service.eventBus.RegisterHandlerChannel(eventChannel, event.NewMediaEvent, event.DeleteMediaEvent)

	for {
		select {
		case <-service.queueChange:
			service.startWaitingTasks(ctx)
		case taskID := <-service.taskChange:
			service.handleTaskUpdate(taskID)
		case message := <-eventChannel:
			//exhaustive:ignore
			switch message.Event {
			case event.NewMediaEvent:
				if mediaID, ok := message.Payload.(uuid.UUID); ok {
					log.Emit(logger.DEBUG, "newly ingested media with ID %s detected\n", mediaID)
					service.createWorkflowTasksForMedia(mediaID)
				} else {
					log.Emit(logger.ERROR, "failed to extract UUID from %s event (payload %#v)\n", message.Event, message.Payload)
				}
			case event.DeleteMediaEvent:
				if mediaID, ok := message.Payload.(uuid.UUID); ok {
					log.Emit(logger.DEBUG, "media with ID %s deleted, cancelling any ongoing transcodes\n", mediaID)
					service.CancelTasksForMedia(mediaID)
				} else {
					log.Emit(logger.ERROR, "failed to extract UUID from %s event (payload %#v)\n", message.Event, message.Payload)
				}
			}
		case <-ctx.Done():
			log.Emit(logger.STOP, "Shutting down (context cancelled). Waiting for transcode tasks to cancel.\n")
			service.taskWg.Wait()
			return nil
		}
	}
}

// AllTasks returns the array/slice of the transcode task pointers.
func (service *transcodeService) AllTasks() []*TranscodeTask { return service.tasks }

// Task looks through all the tasks known to this service and returns the one with
// a matching ID, if it can be found. If no such task exists, nil is returned.
func (service *transcodeService) Task(id uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.ID() == id {
			return t
		}
	}

	return nil
}

// ActiveTasksForMedia returns all the tasks which are running against the given media ID.
func (service *transcodeService) ActiveTasksForMedia(mediaID uuid.UUID) []*TranscodeTask {
	tasks := make([]*TranscodeTask, 0)
	for _, t := range service.tasks {
		if t.media.ID() == mediaID {
			tasks = append(tasks, t)
		}
	}

	return tasks
}

// CancelTasksForMedia finds and cancels any active transcodes for the media ID provided.
// This function acquires the service mutex to ensure no tasks for this media are added
// while this process is occurring.
func (service *transcodeService) CancelTasksForMedia(mediaID uuid.UUID) {
	service.Lock()
	defer service.Unlock()

	toDelete := make([]uuid.UUID, 0)
	for _, t := range service.tasks {
		if t.Media().ID() == mediaID {
			toDelete = append(toDelete, t.ID())
		}
	}

	log.Debugf("Cancelling all tasks for media %s (tasks: %v)\n", mediaID, toDelete)
	for _, id := range toDelete {
		if err := service.CancelTask(id); err != nil {
			log.Warnf("Cancellation of task %s failed with error: %s\n", id, err)
		}
	}
}

// TaskForMediaAndTarget searches through all the tasks in this service and looks for one
// which was created for the media and target matching the IDs provided. If no such task exists
// then nil is returned.
func (service *transcodeService) ActiveTaskForMediaAndTarget(mediaID uuid.UUID, targetID uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.media.ID() == mediaID && t.target.ID == targetID {
			return t
		}
	}

	return nil
}

// NewTask fetches the media and target corresponding to the IDs provided and attempts to spawn
// a task using the result.
// If the media/target fail to be retrieved, or if a transcode task for the
// media+target already exists, an error is returned.
func (service *transcodeService) NewTask(mediaID uuid.UUID, targetID uuid.UUID) error {
	media := service.dataStore.GetMedia(mediaID)
	if media == nil {
		return fmt.Errorf("media %s not found", mediaID)
	}

	target := service.dataStore.GetTarget(targetID)
	if target == nil {
		return fmt.Errorf("target %s not found", targetID)
	}

	return service.spawnFfmpegTarget(media, target)
}

// CancelTask will find the transcode task with the ID provided and cancel it. If the task
// is not in a cancellable state, it will simply be removed from the service.
func (service *transcodeService) CancelTask(id uuid.UUID) error {
	task := service.Task(id)
	if task == nil {
		return ErrTaskNotFound
	}

	if err := task.cancel(); err != nil {
		// This error usually indicates the task is not the right state to be cancelled, however
		// we should still proceed with removing it from the queue
		log.Warnf("failed to cancel task %s command: %s", task, err)
	}

	isBeingMonitored := task.Status() == WORKING || task.Status() == SUSPENDED
	if !isBeingMonitored {
		// Manually remove from the queue because the task was not being
		// monitored by the service at the time of it's cancellation.
		service.removeTaskFromQueue(id)
	}

	log.Emit(logger.STOP, "Cancelled %s\n", task)
	return nil
}

// PauseTask searches the services for the task with the ID provided and suspends
// the underlying ffmpeg command. If the task cannot be found, ErrTaskNotFound is returned.
// If the task is not capable of being suspended (e.g. it's already suspended), then an
// error describing the problem will be returned.
func (service *transcodeService) PauseTask(id uuid.UUID) error {
	task := service.Task(id)
	if task == nil {
		return ErrTaskNotFound
	}

	if err := task.pause(); err != nil {
		return err
	}

	log.Infof("Paused %s\n", task)
	service.taskChange <- id
	return nil
}

// ResumeTake searches the services for the task with the ID provided and attempts to resume
// the underlying ffmpeg command. If the task cannot be found, ErrTaskNotFound is returned.
// If the task is not capable of being resumed (e.g. it's not already suspended), then an
// error describing the problem will be returned.
func (service *transcodeService) ResumeTask(id uuid.UUID) error {
	task := service.Task(id)
	if task == nil {
		return ErrTaskNotFound
	}

	if err := task.resume(); err != nil {
		return err
	}

	log.Infof("Resumed %s\n", task)
	service.taskChange <- id
	return nil
}

// startWaitingTasks finds any transcode items that are waiting to be started will be started, and any that are
// finished will be removed from the transcoders. The starting of FFmpeg tasks will be subject to
// the maximum thread usage defined in the services configuration.
func (service *transcodeService) startWaitingTasks(ctx context.Context) {
	service.Lock()
	defer service.Unlock()

	if service.consumedThreads == service.config.MaximumThreadConsumption {
		return
	}

	for _, task := range service.tasks {
		if task.Status() != WAITING {
			continue
		}

		requiredBudget := task.Target().RequiredThreads()
		availableBudget := service.config.MaximumThreadConsumption - service.consumedThreads
		if requiredBudget > availableBudget {
			log.Emit(logger.DEBUG, "Thread requirements of task %s (%d) exceed remaining budget (%d), instance spawning complete\n", task, requiredBudget, availableBudget)
			return
		}

		service.consumedThreads += requiredBudget
		service.taskWg.Add(1)
		go func(taskToStart *TranscodeTask, wg *sync.WaitGroup, threadCost int) {
			defer wg.Done()

			updateHandler := func(prog *ffmpeg.Progress) {
				taskToStart.lastProgress = prog
				service.eventBus.Dispatch(event.TranscodeTaskProgressEvent, taskToStart.ID())
			}

			taskToStart.status = WORKING
			service.taskChange <- taskToStart.id
			log.Emit(logger.DEBUG, "Starting task %s, consuming %d threads\n", taskToStart, threadCost)
			if err := taskToStart.Run(ctx, updateHandler); err != nil {
				log.Emit(logger.WARNING, "Task %s has concluded with error: %v\n", taskToStart, err)
			} else {
				log.Emit(logger.DEBUG, "Task %s has concluded nominally\n", taskToStart)
			}

			// Submit a non-blocking update to ensure completed/cancelled tasks are correctly dealt with
			// If the service is shutting down, then the above task will be automatically cancelled
			// AND the thread responsible for draining this channel is no longer listening, so send these
			// messages non-blocking
			// TODO: consider being a bit smarter about this (e.g. only send non-blocking on cancellation, or
			// perhaps avoid sending altogether if the service is closing).
			select {
			case service.taskChange <- taskToStart.id:
			default:
				log.Emit(logger.WARNING, "Failed to notify service of task change... this could be because the service is shutting down\n")
			}

			service.Lock()
			defer service.Unlock()
			service.consumedThreads -= threadCost
			log.Emit(logger.DEBUG, "Task %s has released %d threads\n", taskToStart.ID(), threadCost)
		}(task, service.taskWg, requiredBudget)
	}
}

// handleTaskUpdate is the handler for any task updates in this service.
// Any dead tasks are removed from the queue. Completed tasks are committed
// to the database before being removed from the queue.
func (service *transcodeService) handleTaskUpdate(taskID uuid.UUID) {
	task := service.Task(taskID)
	if task == nil {
		return
	}

	if task.status == COMPLETE {
		if err := service.dataStore.SaveTranscode(task); err != nil {
			// TODO: implement a retry logic here because otherwise this transcode is lost
			log.Errorf("failed to save transcode %s due to error: %v\n", task, err)
		} else {
			service.eventBus.Dispatch(event.TranscodeCompleteEvent, taskID)
			service.removeTaskFromQueue(task.id)

			return
		}
	}

	if task.status == CANCELLED {
		service.removeTaskFromQueue(task.id)
	}

	service.eventBus.Dispatch(event.TranscodeUpdateEvent, taskID)
}

// createWorkflowTasksForMedia takes a media ID, and queries the Ffmpeg Store for a workflow
// matching the media provided. The first workflow to be found as eligible will see the associatted
// tasks be created, managed and monitored by this service.
func (service *transcodeService) createWorkflowTasksForMedia(mediaID uuid.UUID) {
	media := service.dataStore.GetMedia(mediaID)
	workflows := service.dataStore.GetAllWorkflows()

	for _, workflow := range workflows {
		if workflow.IsMediaEligible(media) {
			for _, target := range workflow.Targets {
				if err := service.spawnFfmpegTarget(media, target); err != nil {
					log.Emit(logger.ERROR, "failed to spawn ffmpeg target %s for media %s: %v\n", target, media.ID(), err)
				}
			}

			log.Emit(logger.NEW, "Media %s met the conditions of workflow %v... Automated transcodes queued\n", mediaID, workflow)
			return
		}
	}

	// TODO: Maybe we create some sort of a notification or something about not being able to find an eligible
	//		 workflow? I could see that being useful.
	log.Emit(logger.DEBUG, "Media %s did not meet the conditions of any known workflows. No automated transcoding will occur\n", mediaID)
}

// spawnFfmpegTarget will create a new transcode task assigned to the media and target provided,
// and add the task to the services queue in an 'IDLE' state.
// An error is returned if a task for this media+target already exists, whether completed (in DB) or active
// Note: This function does not START the transcoding, it only creates the task and adds it to the
// processing queue.
func (service *transcodeService) spawnFfmpegTarget(m *media.Container, target *ffmpeg.Target) error {
	service.Lock()
	defer service.Unlock()

	if existing := service.ActiveTaskForMediaAndTarget(m.ID(), target.ID); existing != nil {
		return fmt.Errorf("an active task for media %s and target %s already exists", m.ID(), target.ID)
	}

	if existing, _ := service.dataStore.GetForMediaAndTarget(m.ID(), target.ID); existing != nil {
		return fmt.Errorf("a completed task for media %s and target %s already exists", m.ID(), target.ID)
	}

	newTask, err := NewTranscodeTask(m, target, ffmpeg.Config{
		FfmpegBinPath:       service.config.FfmpegBinaryPath,
		FfprobeBinPath:      service.config.FfprobeBinaryPath,
		OutputBaseDirectory: service.config.OutputPath,
	})
	if err != nil {
		return fmt.Errorf("failed to create new transcode task: %w", err)
	}

	service.tasks = append(service.tasks, newTask)
	service.queueChange <- true
	return nil
}

// removeTaskFromQueue will look for and remove the task with the ID provided
// from the services queue.
// NOTE: The task will NOT be cancelled as part of removal.
func (service *transcodeService) removeTaskFromQueue(taskID uuid.UUID) {
	for i, v := range service.tasks {
		if v.id == taskID {
			service.tasks = append(service.tasks[:i], service.tasks[i+1:]...)
			service.queueChange <- true

			return
		}
	}
}
