package worker

import (
	"log"

	"gitlab.com/hbomb79/TPA/enum"
)

type TitleWorker struct {
	TaskFn        func(*TitleWorker) error
	WakeupChan    chan int
	workerStage   enum.PipelineStage
	currentStatus enum.WorkerStatus
}

// NewTitleWorkers will create a certain 'amount' of TitleWorkers inside the WorkerPool provided
// This func will construct a new TitleWorker for each 'amount', and will
// set the TaskFn and WakeupChan to the values provided.
// The taskFn given should not use a goroutine - the worker will
// automatically take care of this inside it's Start() method
func NewTitleWorkers(pool *WorkerPool, amount int, taskFn func(*TitleWorker) error, wakupChan chan int) {
	log.Printf("Creating %v TitleWorkers\n", amount)
	for i := 0; i < amount; i++ {
		log.Printf("Creating new TitleWorker")
		p := &TitleWorker{
			taskFn,
			wakupChan,
			enum.Title,
			enum.Idle,
		}

		pool.PushWorker(p)
	}
}

// TitleWorker Start() will enter an infinite loop
// that will listen on the NotifyChan and WakeupChan,
// if the WakeupChan is selected - the task for this
// worker will be executed
// Once the task is executed, we'll check again to see if
// work can be done - if not, we will go to sleep
// and wait for WakeupChan to tell us more work is available
// rather than just time.Sleeping for an arbritrary amount of time
func (titleWorker *TitleWorker) Start() error {
	log.Printf("Started a TitleWorker!")

workLoop:
	for {
		_, ok := <-titleWorker.WakeupChan
		if !ok {
			log.Printf("Wakeup channel has closed - TitleWorker is quitting\n")
			break workLoop
		}

		// Tick...
		log.Printf("TitleWorker tick.. running\n")
		titleWorker.TaskFn(titleWorker)
	}

	return nil
}

func (titleWorker *TitleWorker) Status() enum.WorkerStatus {
	return titleWorker.currentStatus
}

func (titleWorker *TitleWorker) Stage() enum.PipelineStage {
	return titleWorker.workerStage
}

// Closes the TitleWorker by closing the WakeupChan we're
// infinitely listening to
func (titleWorker *TitleWorker) Close() error {
	close(titleWorker.WakeupChan)
	return nil
}
