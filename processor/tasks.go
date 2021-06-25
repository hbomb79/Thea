package processor

import (
	"errors"

	"gitlab.com/hbomb79/TPA/worker"
)

func (p *Processor) pollingWorkerTask(_ *worker.PollingWorker) error {
	if notify, err := p.PollInputSource(); err != nil {
		return errors.New("cannot PollImportSource inside of PollingWorker: " + err.Error())
	} else if notify {
		p.WorkerPool.IterWorkers(func(w worker.Worker) {
			if v, ok := w.(*worker.TitleWorker); ok {
				// For each worker that is reponsible for the next stage,
				// send an int on their wakeup channel
				v.WakeupChan <- 1
			}
		})
	}
	return nil
}

func (p *Processor) titleWorkerTask(_ *worker.TitleWorker) error {
	return nil
}
