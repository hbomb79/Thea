package processor

import (
	"errors"
	"fmt"
	"log"
)

func (p *Processor) pollingWorkerTask(w *Worker) error {
	for {
		// Wait for wakeup tick
		if _, ok := <-w.WakeupChan(); !ok {
			log.Printf("Worker %v - wakeup channel has been closed. Exiting\n", w.label)
			break
		}

		// Poll directory
		if notify, err := p.PollInputSource(); err != nil {
			return errors.New(fmt.Sprintf("cannot PollImportSource inside of worker '%v' - %v", w.label, err.Error()))
		} else if notify > 0 {
			log.Printf("Notiying Title workers of new items in queue! (New items: %v)\n", notify)
			p.WorkerPool.NotifyWorkers(Title)
		}
	}

	return nil
}

func (p *Processor) titleWorkerTask(w *Worker) error {
mainLoop:
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := p.Queue.Pick(w.pipelineStage, Pending)
			if queueItem == nil {
				break workLoop
			}

			// Try assign queue item to us
			if err := p.Queue.AssignItem(queueItem); err != nil {
				if _, ok := err.(QueueAssignError); ok {
					// Hm, another worker may have beaten us to this task. Oh well.. try find another
					continue workLoop
				}

				// Another type of error... unexpected so we'll return it from the task
				return err
			}

			// Bingo, we got the item assigned to us (marked as processing so no other
			// worker will be able to enter this critical section with the same QueueItem)
			// Do our work..
			queueItem.Name = p.FormatTitle(queueItem.Name)

			// Release the QueueItem by advancing it to the next pipeline stage
			p.Queue.AdvanceStage(queueItem)

			// Wakeup any pipeline workers that are sleeping
			p.WorkerPool.NotifyWorkers(OMDB)
		}

		// If no work, wait for wakeup
		if _, ok := <-w.WakeupChan(); !ok {
			log.Printf("Worker %v - wakeup channel has been closed. Exiting\n", w.label)
			break mainLoop
		}
	}

	return nil
}
