package worker

import (
	"fmt"
	"log"
)

// Each stage represents a certain stage in the pipeline
type PipelineStage int

// When a QueueItem is initially added, it should be of stage Import,
// each time a worker works on the task it should increment it's
// Stage (Title->Omdb->etc..) and set it's Status to 'Pending'
// to allow a worker to pick the item from the Queue
// TODO Really.. worker should have no concept of pipeline stages
// as it's only relevant in this package. We could further de-couple
// this codebase by waking up workers based on their label, rather
// than the worker.PipelineStage enum
const (
	Import PipelineStage = iota
	Title
	Omdb
	Format
	Database
	Finish
)

type WorkerWakeupChan chan int
type WorkerStatus int

type WorkerTaskMeta interface {
	Execute(*Worker) error
}

const (
	Sleeping WorkerStatus = iota
	Working
	Finished
)

type Worker struct {
	label         string
	task          WorkerTaskMeta
	wakeupChan    WorkerWakeupChan
	currentStatus WorkerStatus
	pipelineStage PipelineStage
}

func NewWorker(label string, task WorkerTaskMeta, pipelineStage PipelineStage, wakeupChan chan int) *Worker {
	return &Worker{
		label,
		task,
		wakeupChan,
		Sleeping,
		pipelineStage,
	}
}

func (worker *Worker) Start() {
	fmt.Printf("[Worker] Starting worker for stage %v with label %v\n", worker.pipelineStage, worker.label)
	worker.currentStatus = Working
	if err := worker.task.Execute(worker); err != nil {
		log.Panicf("[Error] Worker for stage %v with label %v has reported an error(%T): %v\n", worker.pipelineStage, worker.label, err, err.Error())
	}

	worker.currentStatus = Finished
	fmt.Printf("[Worker] Worker for stage %v with label %v has stopped\n", worker.pipelineStage, worker.label)
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

// Close() closes the Worker by closing the WakeChan.
// Note that this does not interupt currently running
// goroutines. TODO implement a way to forcefully
// close goroutines.
func (worker *Worker) Close() error {
	close(worker.wakeupChan)
	return nil
}

// sleep puts a worker to sleep until it's wakeupChan is
// signalled from another goroutine. Returns a boolean that
// is 'false' if the wakeup channel was closed - indicating
// the worker should quit.
func (worker *Worker) Sleep() (isAlive bool) {
	worker.currentStatus = Sleeping

	if _, isAlive = <-worker.wakeupChan; isAlive {
		worker.currentStatus = Working
	} else {
		log.Printf("Wakup channel for worker '%v' has been closed - worker is exiting\n", worker.label)
		worker.currentStatus = Finished
	}

	return isAlive
}
