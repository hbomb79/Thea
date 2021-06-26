package processor

import (
	"errors"
	"log"
)

func (p *Processor) pollingWorkerTask(_ *Worker) error {
	if notify, err := p.PollInputSource(); err != nil {
		return errors.New("cannot PollImportSource inside of PollingWorker: " + err.Error())
	} else if notify > 0 {
		log.Printf("Notiying Title workers of new items in queue! (New items: %v)\n", notify)
		p.WorkerPool.NotifyWorkers(Title)
	}
	return nil
}

func (p *Processor) titleWorkerTask(w *Worker) error {
	item := w.CurrentItem()
	if item == nil {
		return errors.New("cannot process title of item 'nil' - no current item attached to this worker\n")
	}

	old := item.Name
	item.Name = p.FormatTitle(item.Name)

	log.Printf("TitleWorker performing format of title string. %v -> %v\nWaking up OMDB workers\n", old, item.Name)
	p.WorkerPool.NotifyWorkers(OMDB)
	return nil
}
