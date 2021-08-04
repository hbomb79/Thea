package processor

import (
	"errors"
	"io/fs"
	"sync"

	"github.com/hbomb79/TPA/worker"
)

// ProcessorQueue is the Queue of items to be processed by this
// processor
type ProcessorQueue struct {
	Items  []*QueueItem `json:"items" groups:"api"`
	lastId int
	cache  *queueCache
	sync.Mutex
}

// HandleFile will take the provided file and if it's not
// currently inside the queue, it will be inserted in to the queue.
// If it is in the queue, the entry is skipped - this is because
// this method is usually called as a result of polling the
// input directory many times a day for new files.
func (queue *ProcessorQueue) HandleFile(path string, fileInfo fs.FileInfo) *QueueItem {
	queue.Lock()
	defer queue.Unlock()

	isInQueue := func(path string) bool {
		for _, v := range queue.Items {
			if v.Path == path {
				return true
			}
		}

		return false
	}

	if !isInQueue(path) {
		item := &QueueItem{
			Id:     queue.lastId,
			Name:   fileInfo.Name(),
			Path:   path,
			Status: Pending,
			Stage:  worker.Title,
		}

		queue.Items = append(queue.Items, item)
		queue.lastId++

		return item
	}

	return nil
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *ProcessorQueue) Pick(stage worker.PipelineStage) *QueueItem {
	queue.Lock()
	defer queue.Unlock()

	for _, item := range queue.Items {
		if item.Stage == stage && item.Status == Pending {
			item.Status = Processing
			return item
		}
	}

	return nil
}

// AdvanceStage will take the QueueItem this method is attached to,
// and set it's stage to the next stage and reset it's status to Pending
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *ProcessorQueue) AdvanceStage(item *QueueItem) {
	queue.Lock()
	defer queue.Unlock()

	if item.Stage == worker.Finish {
		item.Status = Completed
	} else if item.Stage == worker.Format {
		item.Stage = worker.Finish
		item.Status = Completed
	} else {
		item.Stage++
		item.Status = Pending
	}
}

// PromoteItem accepts a QueueItem and will restructure the processor
// queue items to mean that the item provided is the first QueueItem in
// the slice. Returns an error if the queue item provided is not found
// inside the queue slice.
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *ProcessorQueue) PromoteItem(item *QueueItem) error {
	queue.Lock()
	defer queue.Unlock()

	// Restructures the slice by taking the items before and
	// after the index given, and appending them together
	// before appending the result to a new slice containing
	// only the item referenced by the index given.
	promote := func(source []*QueueItem, index int) []*QueueItem {
		if index == 0 {
			return source
		} else if index == len(source)-1 {
			return append([]*QueueItem{source[index]}, source[:len(source)-1]...)
		}

		out := append([]*QueueItem{source[index]}, source[:index]...)
		return append(out, source[index+1:]...)
	}

	// Search for the item and promote it if/when found
	for position := 0; position <= len(queue.Items); position++ {
		if queue.Items[position] == item {
			queue.Items = promote(queue.Items, position)

			return nil
		}
	}

	// Not found, return error
	return errors.New("cannot promote: item does not exist inside this queue")
}

func (queue *ProcessorQueue) FindById(id int) *QueueItem {
	for _, item := range queue.Items {
		if item.Id == id {
			return item
		}
	}

	return nil
}
