package worker

import "gitlab.com/hbomb79/TPA/enum"

type Worker interface {
	Start() error
	Close() error
	Status() enum.WorkerStatus
	StatusInfo() string
	Stage() enum.PipelineStage
	WakeupChan() enum.WorkerWakeupChan
	CurrentItem() *enum.QueueItem
}

// baseWorker is a worker with no
// specific functionality - only for
// use with struct embedding
type baseWorker struct {
	wakeupChan    enum.WorkerWakeupChan
	currentStatus enum.WorkerStatus
	workerStage   enum.PipelineStage
	currentItem   *enum.QueueItem
}

// Stage method returns the current status of this worker,
// can be overidden by higher-level struct to embed
// custom functionality
func (baseWorker *baseWorker) Status() enum.WorkerStatus {
	return baseWorker.currentStatus
}

// Stage method returns the stage of this worker,
// can be overidden by higher-level struct to embed
// custom functionality
func (baseWorker *baseWorker) Stage() enum.PipelineStage {
	return baseWorker.workerStage
}

func (baseWorker *baseWorker) WakeupChan() enum.WorkerWakeupChan {
	return baseWorker.wakeupChan
}

// Closes the Worker by closing the NotifyChan,
func (baseWorker *baseWorker) Close() error {
	close(baseWorker.wakeupChan)
	return nil
}

func (baseWorker *baseWorker) CurrentItem() *enum.QueueItem {
	if baseWorker.Status() != enum.Working {
		return nil
	}

	return baseWorker.currentItem
}
