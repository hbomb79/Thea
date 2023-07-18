package worker

import (
	"fmt"

	"github.com/hbomb79/Thea/pkg/logger"
)

var workerLogger = logger.Get("Worker")

type WorkerWakeupChan chan int
type WorkerStatus int

// WorkerTask is the function containing the task that this
// worker should execute. It is called repeatedly until 'true'
// is returned (indicating the worker should Sleep as there is
// no more work to do), OR an error is returned (indicating
// the worker should close)
type WorkerTask func(Worker) (bool, error)

const (
	SLEEPING WorkerStatus = iota
	ALIVE
	KILLING
	DEAD
)

type Worker interface {
	Start()
	Status() WorkerStatus
	WakeupChan() WorkerWakeupChan
	Label() string
	Sleep() bool
	Close()
}

type taskWorker struct {
	label         string
	task          WorkerTask
	wakeupChan    WorkerWakeupChan
	currentStatus WorkerStatus
}

func NewWorker(label string, task WorkerTask) *taskWorker {
	return &taskWorker{
		label,
		task,
		make(WorkerWakeupChan),
		SLEEPING,
	}
}

func (worker *taskWorker) Start() {
	workerLogger.Emit(logger.NEW, "Starting worker with label %v\n", worker.label)
	worker.currentStatus = ALIVE

	for {
		if worker.currentStatus != ALIVE {
			// Stop the task being executed if we're killing
			// the worker
			break
		}

		shouldSleep, err := worker.task(worker)
		workerLogger.Emit(logger.VERBOSE, "%s task complete. Should sleep: %v. Has error: %v\n", worker, shouldSleep, err)
		if err != nil {
			workerLogger.Emit(logger.ERROR, "%s has reported an error(%T): %v\n", worker, err, err.Error())
			break
		}

		if shouldSleep && worker.currentStatus == ALIVE {
			if !worker.Sleep() {
				// Worker was sleeping, but is being killed now
				break
			}
		}
	}

	worker.currentStatus = DEAD
	workerLogger.Emit(logger.STOP, "Worker %s has stopped\n", worker.label)
}

// Status returns the current status of this worker
func (worker *taskWorker) Status() WorkerStatus {
	return worker.currentStatus
}

// WakeupChan returns the channel that should be used to
// awken this worker from a SLEEPING state.
func (worker *taskWorker) WakeupChan() WorkerWakeupChan {
	return worker.wakeupChan
}

// Close closes the Worker by closing the WakeChan.
// Note that this does not interupt currently running
// goroutines.
func (worker *taskWorker) Close() {
	worker.currentStatus = KILLING
	close(worker.wakeupChan)
}

// Label returns the label for this worker
func (worker *taskWorker) Label() string {
	return worker.label
}

// Sleep puts a worker to sleep until it's wakeupChan is
// signalled from another goroutine. Returns a boolean that
// is 'false' if the wakeup channel was closed - indicating
// the worker is being killed.
func (worker *taskWorker) Sleep() (isAlive bool) {
	worker.currentStatus = SLEEPING

	if _, isAlive = <-worker.wakeupChan; isAlive {
		worker.currentStatus = ALIVE
	} else {
		workerLogger.Emit(logger.STOP, "Wakeup channel for worker '%v' has been closed - worker is exiting\n", worker.label)
		worker.currentStatus = DEAD
	}

	return isAlive
}

func (worker *taskWorker) String() string {
	return fmt.Sprintf("Worker{label=%s state=%d}", worker.label, worker.currentStatus)
}
