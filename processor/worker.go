package processor

import (
	"log"
)

type WorkerTask func(*Worker) error
type WorkerWakeupChan chan int

type WorkerStatus int

const (
	Idle WorkerStatus = iota
	Working
)

type Worker struct {
	label         string
	task          WorkerTask
	wakeupChan    WorkerWakeupChan
	currentStatus WorkerStatus
	pipelineStage PipelineStage
	currentItem   *QueueItem
}

func NewWorker(label string, task WorkerTask, wakeupChannel WorkerWakeupChan, pipelineStage PipelineStage) *Worker {
	return &Worker{
		label,
		task,
		wakeupChannel,
		Idle,
		pipelineStage,
		nil,
	}
}

func (worker *Worker) Start() {
	log.Printf("Starting worker for stage %v with label %v\n", worker.pipelineStage, worker.label)
	worker.task(worker)
	log.Printf("Worker for stage %v with label %v has stopped\n", worker.pipelineStage, worker.label)
}

// Stage method returns the current status of this worker,
// can be overidden by higher-level struct to embed
// custom functionality
func (worker *Worker) Status() WorkerStatus {
	return worker.currentStatus
}

// Stage method returns the stage of this worker,
// can be overidden by higher-level struct to embed
// custom functionality
func (worker *Worker) Stage() PipelineStage {
	return worker.pipelineStage
}

func (worker *Worker) WakeupChan() WorkerWakeupChan {
	return worker.wakeupChan
}

// Closes the Worker by closing the NotifyChan,
func (worker *Worker) Close() error {
	close(worker.wakeupChan)
	return nil
}

func (worker *Worker) CurrentItem() *QueueItem {
	if worker.Status() != Working {
		return nil
	}

	return worker.currentItem
}
