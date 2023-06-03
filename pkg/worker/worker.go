package worker

import "github.com/hbomb79/Thea/pkg/logger"

var workerLogger = logger.Get("Worker")

type WorkerWakeupChan chan int
type WorkerStatus int

type WorkerTaskMeta interface {
	Execute(Worker) error
}

const (
	Sleeping WorkerStatus = iota
	Working
	Finished
)

type Worker interface {
	Start()
	Status() WorkerStatus
	Stage() int
	WakeupChan() WorkerWakeupChan
	Label() string
	Sleep() bool
	Close()
}

type taskWorker struct {
	label         string
	task          WorkerTaskMeta
	wakeupChan    WorkerWakeupChan
	currentStatus WorkerStatus
	stage         int
}

func NewWorker(label string, task WorkerTaskMeta, pipelineStage int) *taskWorker {
	return &taskWorker{
		label,
		task,
		make(WorkerWakeupChan),
		Sleeping,
		pipelineStage,
	}
}

func (worker *taskWorker) Start() {
	workerLogger.Emit(logger.NEW, "Starting worker for stage %v with label %v\n", worker.stage, worker.label)
	worker.currentStatus = Working
	if err := worker.task.Execute(worker); err != nil {
		workerLogger.Emit(logger.ERROR, "Worker for stage %v with label %v has reported an error(%T): %v\n", worker.stage, worker.label, err, err.Error())
	}

	worker.currentStatus = Finished
	workerLogger.Emit(logger.STOP, "Worker for stage %v with label %v has stopped\n", worker.stage, worker.label)
}

// Status returns the current status of this worker
func (worker *taskWorker) Status() WorkerStatus {
	return worker.currentStatus
}

// Stage returns the stage of this worker
func (worker *taskWorker) Stage() int {
	return worker.stage
}

func (worker *taskWorker) WakeupChan() WorkerWakeupChan {
	return worker.wakeupChan
}

// Close closes the Worker by closing the WakeChan.
// Note that this does not interupt currently running
// goroutines.
func (worker *taskWorker) Close() {
	close(worker.wakeupChan)
}

// Label returns the label for this worker
func (worker *taskWorker) Label() string {
	return worker.label
}

// Sleep puts a worker to sleep until it's wakeupChan is
// signalled from another goroutine. Returns a boolean that
// is 'false' if the wakeup channel was closed - indicating
// the worker should quit.
func (worker *taskWorker) Sleep() (isAlive bool) {
	worker.currentStatus = Sleeping

	if _, isAlive = <-worker.wakeupChan; isAlive {
		worker.currentStatus = Working
	} else {
		workerLogger.Emit(logger.STOP, "Wakeup channel for worker '%v' has been closed - worker is exiting\n", worker.label)
		worker.currentStatus = Finished
	}

	return isAlive
}
