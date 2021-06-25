package worker

import (
	"log"
	"time"
)

type PollingWorker struct {
	TaskFn     func(*PollingWorker) error
	NotifyChan chan int
	WakeupChan <-chan time.Time
}

// NewPollingWorkers will create a certain 'amount' of PollingWorkers inside the WorkerPool provided
// This func will construct a new PollingWorker for each 'amount', and will
// set the TaskFn, WakeupChan and NotifyChan to the values provided.
// The taskFn given should not use a goroutine - the worker will
// automatically take care of this inside it's Start() method
func NewPollingWorkers(pool *WorkerPool, amount int, taskFn func(*PollingWorker) error, wakupChan <-chan time.Time, notifyChan WorkerNotifyChan) {
	for i := 0; i < amount; i++ {
		log.Printf("Constructing new PollingWorker")
		p := &PollingWorker{
			taskFn,
			notifyChan,
			wakupChan,
		}

		pool.PushWorker(p)
	}
}

// PollingWorker Start() will enter an infinite loop
// that will listen on the NotifyChan and WakeupChan,
// if the WakeupChan is selected - the task for this
// worker will be executed
// If the NotifyChan is selected on, and it's because
// the channel was closed, the worker loop is broken
// and the function returns - this will stop the worker
// and call Done() on the waitgroup in the WorkerPool
// responsible for this worker
func (poller *PollingWorker) Start() error {
	log.Printf("Started a PollingWorker!")

workLoop:
	for {
		select {
		case _, ok := <-poller.NotifyChan:
			// Hm, we're the only one that should be broadcasting
			// on this channel - check that it hasn't been closed.
			// If it's still open, just ignore the message - if it's
			// closed, then break the loop and exit the worker
			if !ok {
				break workLoop
			}
		case <-poller.WakeupChan:
			// Tick...
			poller.TaskFn(poller)
		}
	}

	return nil
}

// Closes the PollingWorker by closing the NotifyChan,
// as we're the sender for this channel - closing it
// means we can no longer notify other workers
// that we've completed work - thus the worker
// will exit
func (poller *PollingWorker) Close() error {
	close(poller.NotifyChan)
	return nil
}
