package processor

import (
	"errors"
	"fmt"
)

// pollingWorkerTask is a WorkerTask that is responsible
// for polling the import directory for new items to
// add to the Queue
func (p *Processor) pollingWorkerTask(w *Worker) error {
	for {
		// Wait for wakeup tick
		if isAlive := w.sleep(); !isAlive {
			return nil
		}

		// Do work
		if notify, err := p.PollInputSource(); err != nil {
			return errors.New(fmt.Sprintf("cannot PollImportSource inside of worker '%v' - %v", w.label, err.Error()))
		} else if notify > 0 {
			p.WorkerPool.NotifyWorkers(Title)
		}
	}
}

// titleWorkerTask is a WorkerTask that will
// pick a new item from the queue that needs it's
// title formatted to remove superfluous information.
func (p *Processor) titleWorkerTask(w *Worker) error {
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
			if v, err := p.FormatTitle(queueItem); err != nil {
				if _, ok := err.(TitleFormatError); ok {
					// We caught an error, but it's a recoverable error - raise a trouble
					// sitation for this queue item to request user interaction to resolve it
					p.Queue.RaiseTrouble(queueItem, &Trouble{"Failed to format title", Error, nil})
					continue
				} else {
					// Unknown error
					return err
				}
			} else {
				// Formatting success
				queueItem.Name = v
				// Release the QueueItem by advancing it to the next pipeline stage
				p.Queue.AdvanceStage(queueItem)

				// Wakeup any pipeline workers that are sleeping
				p.WorkerPool.NotifyWorkers(OMDB)
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}
