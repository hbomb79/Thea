package transcode

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("TranscodeServ")

type MediaStore interface {
	GetMedia(uuid.UUID) *media.Container
}

type FfmpegStore interface {
	GetWorkflows() []TranscodeWorkflow
	GetTarget(uuid.UUID) interface{}
}

// TranscodeService ... TODO
type TranscodeService struct {
	*sync.Mutex
	config          *Config
	tasks           []*TranscodeTask
	consumedThreads int

	mediaStore  MediaStore
	ffmpegStore FfmpegStore

	queueChange chan bool
	taskChange  chan uuid.UUID
}

// New creates a new TranscodeService using the Config provided.
func New(config Config) (*TranscodeService, error) { return nil, nil }

func (service *TranscodeService) Start(ctx context.Context) {
	for {
		select {
		case <-service.queueChange:
			service.startNewTasks(ctx)
		case taskId := <-service.taskChange:
			service.handleTaskUpdate(taskId)
		}
	}
}

// startNewTasks finds any transcode items that are waiting to be started will be started, and any that are
// finished will be removed from the transcoders. The starting of FFmpeg tasks will be subject to
// the maximum thread usage defined in the services configuration.
func (service *TranscodeService) startNewTasks(ctx context.Context) {
	service.Lock()
	defer service.Unlock()

	tasks := service.AllTasks()
	for _, task := range *tasks {
		if task.Status() != WAITING {
			log.Emit(logger.DEBUG, "Transcode task %v is not status=WAITING... skipping\n", task)
			continue
		}

		requiredBudget := task.Target().RequiredThreads()
		availableBudget := service.config.MaximumThreadConsumption - service.consumedThreads
		if requiredBudget > availableBudget {
			log.Emit(logger.DEBUG, "Thread requirements of task %v (%v) exceed remaining budget (%v), instance spawning complete\n", task, requiredBudget, availableBudget)
			return
		}

		service.consumedThreads += task.Target().RequiredThreads()
		go func(taskToStart *TranscodeTask) {
			updateHandler := func(progress *ffmpeg.Progress) {
				service.taskChange <- taskToStart.id
			}

			taskToStart.Run(ctx, updateHandler)
			service.taskChange <- taskToStart.id

			service.Lock()
			defer service.Unlock()
			service.consumedThreads -= taskToStart.Target().RequiredThreads()
		}(task)
	}
}

// handleTaskUpdate is the handler for any task updates in this service.
// It will clean up any that are cancelled (this will see partially transcoded
// media removed). Additionally, any successfully completed tasks are removed
// from the queue also.
func (service *TranscodeService) handleTaskUpdate(taskId uuid.UUID) {
	task := service.Task(taskId)
	if task == nil {
		return
	}

	// Emit task update event.
	// TODO

	if task.status == CANCELLED {
		// Cleanup
		service.cleanupCancelledTask(task)
	} else if task.status != COMPLETE {
		return
	}

	service.removeTaskFromQueue(task.id)
}

func (service *TranscodeService) cleanupCancelledTask(task *TranscodeTask) {}

// CheckAndCreateWorkflowTasks takes a media ID, and queries the Ffmpeg Store for a workflow
// matching the media provided. The first workflow to be found as eligible will see the associatted
// tasks be created, managed and monitored by this service.
func (service *TranscodeService) CheckAndCreateWorkflowTasks(mediaId uuid.UUID) error {
	media := service.mediaStore.GetMedia(mediaId)
	workflows := service.ffmpegStore.GetWorkflows()

	for _, workflow := range workflows {
		if workflow.IsMediaEligible(media) {
			for _, target := range *workflow.Targets() {
				service.spawnFfmpegTarget(media, target)
			}

			return nil
		}
	}

	// TODO: Maybe we log a notification or something about not being able to find an eligible
	//		 workflow? I could see that being very useful.
	return nil
}

func (service *TranscodeService) NewTask(mediaId uuid.UUID, targetId uuid.UUID) error {
	return nil
}

func (service *TranscodeService) spawnFfmpegTarget(m *media.Container, target *ffmpeg.Target) error {
	service.queueChange <- true
	return nil
}

func (service *TranscodeService) removeTaskFromQueue(taskId uuid.UUID) {
	for i, v := range service.tasks {
		if v.id == taskId {
			service.tasks = append(service.tasks[:i], service.tasks[i+1:]...)
			service.queueChange <- true

			return
		}
	}
}

func (service *TranscodeService) CancelTask(id uuid.UUID) {
	task := service.Task(id)
	if !task.Cancel() {
		service.removeTaskFromQueue(id)
	}
}

func (service *TranscodeService) Task(id uuid.UUID) *TranscodeTask {
	for _, t := range service.tasks {
		if t.Id() == id {
			return t
		}
	}

	return nil
}

func (service *TranscodeService) AllTasks() *[]*TranscodeTask {
	return &service.tasks
}
