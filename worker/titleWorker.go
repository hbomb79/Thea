package worker

import (
	"log"

	"gitlab.com/hbomb79/TPA/enum"
)

type TitleWorker struct {
	TaskFn func(*TitleWorker) error
	baseWorker
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
			baseWorker{
				wakupChan,
				enum.Idle,
				enum.Title,
			},
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

func (titleWorker *TitleWorker) StatusInfo() string {
	switch titleWorker.Status() {
	case enum.Idle:
		return "Idle"
	case enum.Working:
		return "Renaming"
	default:
		return "Unknown Status"
	}
}
