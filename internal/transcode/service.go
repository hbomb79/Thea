package transcode

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("TranscodeServ")

type (
	DataStore interface {
		SaveTranscode(*TranscodeTask) error
		GetAllWorkflows() []*workflow.Workflow
		GetMedia(uuid.UUID) *media.Container
		GetTarget(uuid.UUID) *ffmpeg.Target
		GetForMediaAndTarget(uuid.UUID, uuid.UUID) (*Transcode, error)
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
// in the configuration provided is not valid (e.g., ffmpeg path is wrong)
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
		queueChange: make(chan bool, 2),
		taskChange:  make(chan uuid.UUID),
	}, nil
}

// Run is the main entry point for this service. This method will block
// until the provided context is cancelled.
// Note: when context is cancelled this method will not immediately return as it
// will wait for it's running transcode tasks to cancel.
func (service *transcodeService) Run(ctx context.Context) error {
	eventChannel := make(event.HandlerChannel, 2)
	service.eventBus.RegisterHandlerChannel(eventChannel, event.NEW_MEDIA)

	for {
		select {
		case <-service.queueChange:
			service.startWaitingTasks(ctx)
		case taskId := <-service.taskChange:
			service.handleTaskUpdate(taskId)
		case message := <-eventChannel:
			ev := message.Event
			if ev != event.NEW_MEDIA {
				log.Emit(logger.WARNING, "received unknown event %s\n", ev)
				continue
			}

			if mediaId, ok := message.Payload.(uuid.UUID); ok {
				log.Emit(logger.DEBUG, "newly ingested media with ID %s detected\n", mediaId)
				service.createWorkflowTasksForMedia(mediaId)
			} else {
				log.Emit(logger.ERROR, "failed to extract UUID from %s event (payload %#v)\n", ev, message.Payload)
			}
		case <-ctx.Done():
			log.Emit(logger.STOP, "Shutting down (context cancelled). Waiting for transcode tasks to cancel.\n")
			service.taskWg.Wait()
			return nil
		}
	}
}

// RegisterEventCoordinator allows the consumer/manager of this service to
// inject an event bus that we can use to send/receive messages with other
// parts of the system.
func (service *transcodeService) RegisterEventCoordinator(ev event.EventCoordinator) {
	service.eventBus = ev
}

// AllTasks returns the array/slice of the transcode task pointers.
func (service *transcodeService) AllTasks() []*TranscodeTask { return service.tasks }

// Task looks through all the tasks known to this service and returns the one with
// a matching ID, if it can be found. If no such task exists, nil is returned.
func (service *transcodeService) Task(id uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.Id() == id {
			return t
		}
	}

	return nil
}

// TaskForMediaAndTarget searches through all the tasks in this service and looks for one
// which was created for the media and target matching the IDs provided. If no such task exists
// then nil is returned.
func (service *transcodeService) ActiveTaskForMediaAndTarget(mediaId uuid.UUID, targetId uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.media.Id() == mediaId && t.target.ID == targetId {
			return t
		}
	}

	return nil
}

// NewTask fetches the media and target corresponding to the IDs provided and attempts to spawn
// a task using the result.
// If the media/target fail to be retrieved, or if a transcode task for the
// media+target already exists, an error is returned.
func (service *transcodeService) NewTask(mediaId uuid.UUID, targetId uuid.UUID) error {
	media := service.dataStore.GetMedia(mediaId)
	if media == nil {
		return fmt.Errorf("media %s not found", mediaId)
	}

	target := service.dataStore.GetTarget(targetId)
	if target == nil {
		return fmt.Errorf("target %s not found", targetId)
	}

	return service.spawnFfmpegTarget(media, target)
}

// CancelTask will find the transcode task with the ID provided and cancel it. If the task
// is not in a cancellable state, it will simply be removed from the service.
func (service *transcodeService) CancelTask(id uuid.UUID) error {
	task := service.Task(id)
	isBeingMonitored := task.Status() == WORKING

	if task == nil {
		return fmt.Errorf("no task with ID %s", id)
	}

	if err := task.Cancel(); err != nil {
		return fmt.Errorf("failed to cancel task %s: %w", task, err)
	}

	if !isBeingMonitored {
		// Manually remove from the queue because the task was not being
		// monitored by the service at the time of it's cancellation.
		service.removeTaskFromQueue(id)
	}

	log.Emit(logger.STOP, "Task %s cancelled and cleaned up\n", task)
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
				service.taskChange <- taskToStart.id
				fmt.Printf("\rTask %s transcode progress: %d%%", taskToStart.Id(), int(prog.Progress))
			}

			taskToStart.status = WORKING
			log.Emit(logger.DEBUG, "Starting task %s, consuming %d threads\n", taskToStart, threadCost)
			if err := taskToStart.Run(ctx, updateHandler); err != nil {
				log.Emit(logger.WARNING, "Task %s has concluded with error: %v\n", taskToStart, err)
			} else {
				log.Emit(logger.DEBUG, "Task %s has concluded nominally\n", taskToStart)
			}

			// Submit an update to ensure completed/cancelled tasks are correctly dealt with
			service.taskChange <- taskToStart.id

			service.Lock()
			defer service.Unlock()
			service.consumedThreads -= threadCost
		}(task, service.taskWg, requiredBudget)
	}
}

// handleTaskUpdate is the handler for any task updates in this service.
// Any dead tasks are removed from the queue.
func (service *transcodeService) handleTaskUpdate(taskId uuid.UUID) {
	task := service.Task(taskId)
	if task == nil {
		return
	}

	if task.status == COMPLETE {
		if err := service.dataStore.SaveTranscode(task); err != nil {
			// TODO: implement a retry logic here because otherwise this transcode is lost
			log.Errorf("failed to save transcode %s due to error: %v\n", task, err)
		} else {
			service.eventBus.Dispatch(event.TRANSCODE_COMPLETE, taskId)
		}
	}

	if task.status == CANCELLED || task.status == COMPLETE {
		service.removeTaskFromQueue(task.id)
		return
	}

	service.eventBus.Dispatch(event.TRANSCODE_UPDATE, taskId)
}

// createWorkflowTasksForMedia takes a media ID, and queries the Ffmpeg Store for a workflow
// matching the media provided. The first workflow to be found as eligible will see the associatted
// tasks be created, managed and monitored by this service.
func (service *transcodeService) createWorkflowTasksForMedia(mediaId uuid.UUID) {
	media := service.dataStore.GetMedia(mediaId)
	workflows := service.dataStore.GetAllWorkflows()

	for _, workflow := range workflows {
		if workflow.IsMediaEligible(media) {
			for _, target := range workflow.Targets {
				if err := service.spawnFfmpegTarget(media, target); err != nil {
					log.Emit(logger.ERROR, "failed to spawn ffmpeg target %s for media %s: %v\n", target, media.Id(), err)
				}
			}

			return
		}
	}

	// TODO: Maybe we log a notification or something about not being able to find an eligible
	//		 workflow? I could see that being useful.
}

// spawnFfmpegTarget will create a new transcode task assigned to the media and target provided,
// and add the task to the services queue in an 'IDLE' state.
// An error is returned if a task for this media+target already exists, whether completed (in DB) or active
// Note: This function does not START the transcoding, it only creates the task and adds it to the
// processing queue.
func (service *transcodeService) spawnFfmpegTarget(m *media.Container, target *ffmpeg.Target) error {
	service.Lock()
	defer service.Unlock()

	if existing := service.ActiveTaskForMediaAndTarget(m.Id(), target.ID); existing != nil {
		return fmt.Errorf("an active task for media %s and target %s already exists", m.Id(), target.ID)
	}

	if existing, _ := service.dataStore.GetForMediaAndTarget(m.Id(), target.ID); existing != nil {
		return fmt.Errorf("a completed task for media %s and target %s already exists", m.Id(), target.ID)
	}

	newTask, err := NewTranscodeTask(service.config.OutputPath, m, target, ffmpeg.Config{
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
// NOTE: The task will NOT be cancelled as part of removal
func (service *transcodeService) removeTaskFromQueue(taskId uuid.UUID) {
	for i, v := range service.tasks {
		if v.id == taskId {
			service.tasks = append(service.tasks[:i], service.tasks[i+1:]...)
			service.queueChange <- true

			return
		}
	}
}
