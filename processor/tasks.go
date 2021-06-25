package processor

import (
	"errors"
	"log"

	"gitlab.com/hbomb79/TPA/enum"
	"gitlab.com/hbomb79/TPA/worker"
)

func (p *Processor) pollingWorkerTask(_ *worker.PollingWorker) error {
	if notify, err := p.PollInputSource(); err != nil {
		return errors.New("cannot PollImportSource inside of PollingWorker: " + err.Error())
	} else if notify > 0 {
		log.Printf("Notiying Title workers of new items in queue! (New items: %v)\n", notify)
		p.WorkerPool.NotifyWorkers(enum.Title)
	}
	return nil
}

func (p *Processor) titleWorkerTask(_ *worker.TitleWorker) error {
	return nil
}
