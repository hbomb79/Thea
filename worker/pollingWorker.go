package worker

import (
	"log"
	"time"
)

type PollingWorker struct {
	TickInterval int
	TaskFn       func() error
}

func (worker *PollingWorker) Start(inboundChannel chan interface{}, outboundChannel chan interface{}) error {
	tickInterval := time.Duration(worker.TickInterval * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}

	pollTicker := time.NewTicker(tickInterval)
	pollRoutine := func(ticker <-chan time.Time) {
		<-ticker
		if err := worker.TaskFn(); err != nil {
			log.Panicf("PollingWorker task error - %v\n", err.Error())
		}
	}

	// Start the goroutine, passing the receive-only channel
	// so the goroutine can wait on ticks on that chan
	go pollRoutine(pollTicker.C)
	return nil
}

func (worker *PollingWorker) Status() WorkerStatus {
	return nil
}
