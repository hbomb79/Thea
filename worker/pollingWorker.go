package worker

import (
	"log"

	"gitlab.com/hbomb79/TPA/enum"
)

type PollingWorker struct {
	TaskFn func(*PollingWorker) error
	baseWorker
}

// NewPollingWorkers will create a certain 'amount' of PollingWorkers inside the WorkerPool provided
// This func will construct a new PollingWorker for each 'amount', and will
// set the TaskFn and WakeupChan to the values provided.
// The taskFn given should not use a goroutine - the worker will
// automatically take care of this inside it's Start() method
func NewPollingWorkers(pool *WorkerPool, amount int, taskFn func(*PollingWorker) error, wakupChan chan int) {
	log.Printf("Creating %v PollingWorkers\n", amount)
	for i := 0; i < amount; i++ {
		log.Printf("Creating new PollingWorker")
		p := &PollingWorker{
			taskFn,
			baseWorker{
				wakupChan,
				enum.Idle,
				enum.Import,
				nil,
			},
		}

		pool.PushWorker(p)
	}
}

// PollingWorker Start() will enter an infinite loop
// that will listen on the WakeupChan,
// if the WakeupChan is sent data - the task for this
// worker will be executed. However, If the WakeupChan is
// closed, the worker will exit
func (poller *PollingWorker) Start() error {
	log.Printf("Started a PollingWorker!")

workLoop:
	for {
		_, ok := <-poller.WakeupChan()
		// Hm, we're the only one that should be broadcasting
		// on this channel - check that it hasn't been closed.
		// If it's still open, just ignore the message - if it's
		// closed, then break the loop and exit the worker
		if !ok {
			log.Printf("Notify channel has closed - PollingWorker is quitting\n")
			break workLoop
		}

		// Tick...
		log.Printf("PollingWorker tick.. running\n")
		poller.TaskFn(poller)
	}

	return nil
}

func (poller *PollingWorker) StatusInfo() string {
	switch poller.Status() {
	case enum.Idle:
		return "Idle"
	case enum.Working:
		return "Polling directory"
	default:
		return "Unknown Status"
	}
}
