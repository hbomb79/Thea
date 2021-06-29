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
			p.WorkerPool.WakupWorkers(Title)
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
			queueItem := p.Queue.Pick(w.pipelineStage)
			if queueItem == nil {
				// No item, break inner loop and sleep
				break workLoop
			}

			// Do our work..
			if v, err := p.FormatTitle(queueItem); err != nil {
				if _, ok := err.(TitleFormatError); ok {
					// We caught an error, but it's a recoverable error - raise a trouble
					// sitation for this queue item to request user interaction to resolve it
					p.Queue.RaiseTrouble(queueItem, &Trouble{err.Error(), Error, nil})
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
				p.WorkerPool.WakupWorkers(Omdb)
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}

// networkWorkerTask will pick an item from the queue that
// needs some stats found from OMDB. Stats include the genre,
// rating, runtime, etc. This worker will attempt to find the
// item at OMDB, and if it fails it will try to refine the
// title until it can't anymore - in which case the Queue item
// will have a trouble state raised.
func (p *Processor) networkWorkerTask(w *Worker) error {
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := p.Queue.Pick(w.pipelineStage)
			if queueItem == nil {
				break workLoop
			}

			// Do our work..

		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}
