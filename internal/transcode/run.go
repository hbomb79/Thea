package transcode

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/activity"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("TranscodeServ")

type MediaStore interface {
	GetMedia(uuid.UUID) *media.Container
}

type FfmpegStore interface {
	GetWorkflows() []*workflow.Workflow
	GetTarget(uuid.UUID) *ffmpeg.Target
}

// TranscodeService is Thea's solution to pre-transcoding of user media.
// It is responsible for some key aspects of Thea:
//   - Transcoding workflows for newly ingested media
//   - Manual transcode requests for ingested media
//   - Live-tracking and reporting of ongoing transcodes over the event bus
type TranscodeService struct {
	*sync.Mutex
	taskWg          *sync.WaitGroup
	config          *Config
	tasks           []*TranscodeTask
	consumedThreads int

	eventBus activity.EventHandler

	mediaStore  MediaStore
	ffmpegStore FfmpegStore

	queueChange chan bool
	taskChange  chan uuid.UUID
}

// New creates a new TranscodeService using the Config provided.
func New(config Config) (*TranscodeService, error) { return nil, nil }

// Start is the main entry point for this service. This method will block
// until the provided context is cancelled.
// Note: when context is cancelled this method will not immediately return as it
// will wait for it's running transcode tasks to cancel.
func (service *TranscodeService) Start(ctx context.Context) {
	eventChannel := make(activity.HandlerChannel, 2)
	service.eventBus.RegisterHandlerChannel(activity.INGEST_MEDIA_COMPLETE, eventChannel)

	for {
		select {
		case <-service.queueChange:
			service.startWaitingTasks(ctx)
		case taskId := <-service.taskChange:
			service.handleTaskUpdate(taskId)
		case event := <-eventChannel:
			ev := event.Event
			if ev != activity.INGEST_MEDIA_COMPLETE {
				log.Emit(logger.WARNING, "received unknown event %s\n", ev)
				continue
			}

			if mediaId, ok := event.Payload.(uuid.UUID); ok {
				log.Emit(logger.DEBUG, "newly ingested media with ID %s detected\n", mediaId)
				service.createWorkflowTasksForMedia(mediaId)
			} else {
				log.Emit(logger.ERROR, "failed to extract UUID from %s event (payload %#v)\n", ev, event.Payload)
			}
		case <-ctx.Done():
			log.Emit(logger.STOP, "Shutting down (context cancelled). Waiting for transcode tasks to cancel.\n")
			service.taskWg.Wait()
			return
		}
	}

}

// AllTasks returns the array/slice of the transcode task pointers.
func (service *TranscodeService) AllTasks() []*TranscodeTask { return service.tasks }

// Task looks through all the tasks known to this service and returns the one with
// a matching ID, if it can be found. If no such task exists, nil is returned.
func (service *TranscodeService) Task(id uuid.UUID) *TranscodeTask {
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
func (service *TranscodeService) TaskForMediaAndTarget(mediaId uuid.UUID, targetId uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.media.Id() == mediaId && t.target.Id() == targetId {
			return t
		}
	}

	return nil
}

// NewTask fetches the media and target corresponding to the IDs provided and attempts to spawn
// a task using the result.
// If the media/target fail to be retrieved, or if a transcode task for the
// media+target already exists, an error is returned.
func (service *TranscodeService) NewTask(mediaId uuid.UUID, targetId uuid.UUID) error {
	media := service.mediaStore.GetMedia(mediaId)
	if media == nil {
		return fmt.Errorf("media %s not found", mediaId)
	}

	target := service.ffmpegStore.GetTarget(targetId)
	if target == nil {
		return fmt.Errorf("target %s not found", targetId)
	}

	return service.spawnFfmpegTarget(media, target)
}

// CancelTask will find the transcode task with the ID provided and cancel it. If the task
// is not in a cancellable state, it will simply be removed from the service.
func (service *TranscodeService) CancelTask(id uuid.UUID) {
	task := service.Task(id)
	isBeingMonitored := task.Status() == WORKING

	task.Cancel()
	if !isBeingMonitored {
		// Manually remove from the queue because the task was not being
		// monitored by the service at the time of it's cancellation.
		service.removeTaskFromQueue(id)
	}
}

// startWaitingTasks finds any transcode items that are waiting to be started will be started, and any that are
// finished will be removed from the transcoders. The starting of FFmpeg tasks will be subject to
// the maximum thread usage defined in the services configuration.
func (service *TranscodeService) startWaitingTasks(ctx context.Context) {
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
			log.Emit(logger.DEBUG, "Thread requirements of task %s (%s) exceed remaining budget (%s), instance spawning complete\n", task, requiredBudget, availableBudget)
			return
		}

		service.consumedThreads += requiredBudget
		service.taskWg.Add(1)
		go func(taskToStart *TranscodeTask, wg *sync.WaitGroup, budget int) {
			defer wg.Done()

			updateHandler := func(_ *ffmpeg.Progress) {
				service.taskChange <- taskToStart.id
			}

			service.taskChange <- taskToStart.id
			taskToStart.Run(ctx, updateHandler)

			service.Lock()
			defer service.Unlock()
			service.consumedThreads -= budget
		}(task, service.taskWg, requiredBudget)
	}
}

// handleTaskUpdate is the handler for any task updates in this service.
// Any dead tasks are removed from the queue.
func (service *TranscodeService) handleTaskUpdate(taskId uuid.UUID) {
	task := service.Task(taskId)
	if task == nil {
		return
	}

	if task.status == CANCELLED || task.status == COMPLETE {
		service.removeTaskFromQueue(task.id)
	}
}

// createWorkflowTasksForMedia takes a media ID, and queries the Ffmpeg Store for a workflow
// matching the media provided. The first workflow to be found as eligible will see the associatted
// tasks be created, managed and monitored by this service.
func (service *TranscodeService) createWorkflowTasksForMedia(mediaId uuid.UUID) {
	media := service.mediaStore.GetMedia(mediaId)
	workflows := service.ffmpegStore.GetWorkflows()

	for _, workflow := range workflows {
		if workflow.IsMediaEligible(media) {
			for _, target := range workflow.Targets {
				if err := service.spawnFfmpegTarget(media, target); err != nil {
					log.Emit(logger.ERROR, "failed to spawn ffmpeg target %s for media %s: %s\n", target, media, err.Error())
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
// An error is returned if a task for this media+target already exists.
// Note: This function does not START the transcoding, it only creates the task and adds it to the
// processing queue.
func (service *TranscodeService) spawnFfmpegTarget(m *media.Container, target *ffmpeg.Target) error {
	service.Lock()
	defer service.Unlock()

	if existing := service.TaskForMediaAndTarget(m.Id(), target.Id()); existing != nil {
		return fmt.Errorf("task for media %s and target %s already exists", m.Id(), target.Id())
	}

	newTask := NewTranscodeTask(service.config.OutputPath, m, target)
	service.tasks = append(service.tasks, newTask)

	service.queueChange <- true
	return nil
}

// removeTaskFromQueue will look for and remove the task with the ID provided
// from the services queue.
// NOTE: The task will NOT be cancelled as part of removal
func (service *TranscodeService) removeTaskFromQueue(taskId uuid.UUID) {
	for i, v := range service.tasks {
		if v.id == taskId {
			service.tasks = append(service.tasks[:i], service.tasks[i+1:]...)
			service.queueChange <- true

			return
		}
	}
}
