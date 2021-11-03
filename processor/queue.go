package processor

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hbomb79/TPA/cache"
	"github.com/hbomb79/TPA/worker"
)

// processorQueue is the Queue of items to be processed by this
// processor
type processorQueue struct {
	Items  []*QueueItem `json:"items" groups:"api"`
	lastId int
	cache  *cache.Cache
	sync.Mutex
}

// NewProcessorQueue returns a pointer to a newly-created processorQueue
// with a slice of QueueItems and the persistent file-system cache
// already populated.
func NewProcessorQueue(cachePath string) *processorQueue {
	return &processorQueue{
		Items:  make([]*QueueItem, 0),
		lastId: 0,
		cache:  cache.New(cachePath),
	}
}

// Retrieve will search the Queue for a QueueItem with a path that matches
// the one provided. If one is found, a pointer to the item is returned; otherwise
// nil is returned.
func (queue *processorQueue) Retrieve(path string) *QueueItem {
	for _, item := range queue.Items {
		if item.Path == path {
			return item
		}
	}

	return nil
}

// Contains will return true if a QueueItem exists inside of this Queue that
// has a matching path to the one provided; false otherwise
func (queue *processorQueue) Contains(path string) bool {
	if item := queue.Retrieve(path); item != nil {
		return true
	}

	return false
}

// Push accepts a QueueItem pointer and will push (append) it to
// the Queue. This method also sets the 'Id' of the QueueItem
// automatically (queue.lastId)
func (queue *processorQueue) Push(item *QueueItem) error {
	queue.Lock()
	defer queue.Unlock()

	if queue.Contains(item.Path) || queue.cache.HasItem(item.Path) {
		return errors.New(fmt.Sprintf("item (%s) is either already in queue, or marked as complete in cache", item.Path))
	}

	item.Id = queue.lastId
	queue.Items = append(queue.Items, item)
	queue.lastId++

	return nil
}

func (queue *processorQueue) Remove(item *QueueItem) error {
	queue.Lock()
	defer queue.Unlock()

	item, idx := queue.FindById(item.Id)
	if item == nil || idx < 0 {
		return errors.New("cannot remove: does not exist in queue")
	}

	queue.Items = append(queue.Items[:idx], queue.Items[idx+1:len(queue.Items)]...)
	return nil
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *processorQueue) Pick(stage worker.PipelineStage) *QueueItem {
	queue.Lock()
	defer queue.Unlock()

	for _, item := range queue.Items {
		if item.Stage == stage && item.Status == Pending {
			item.SetStatus(Processing)

			return item
		}
	}

	return nil
}

// AdvanceStage will take the QueueItem this method is attached to, reset it's trouble state,
// and set it's stage to the next stage and reset it's status to Pending
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *processorQueue) AdvanceStage(item *QueueItem) {
	queue.Lock()
	defer queue.Unlock()

	if item.Stage == worker.Finish {
		item.SetStatus(Completed)
	} else if item.Stage == worker.Format {
		item.SetStage(worker.Finish)
		item.SetStatus(Completed)

		// Add this item to the cache to indicate it's complete
		queue.cache.PushItem(item.Path, "completed")
	} else {
		item.SetStage(item.Stage + 1)
		item.SetStatus(Pending)
	}
}

// PromoteItem accepts a QueueItem and will restructure the processor
// queue items to mean that the item provided is the first QueueItem in
// the slice. Returns an error if the queue item provided is not found
// inside the queue slice.
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *processorQueue) PromoteItem(item *QueueItem) error {
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

type ItemFn func(*processorQueue, int, *QueueItem) bool

// Filter runs the provided callback for every item inside the queue. If the callback
// returns true, the item is retained. Otherwise, if the callback returns false, the item
// is ejected from the queue.
func (queue *processorQueue) Filter(cb ItemFn) {
	queue.Lock()
	defer queue.Unlock()

	newItems := make([]*QueueItem, 0)
	for key, item := range queue.Items {
		if cb(queue, key, item) {
			newItems = append(newItems, item)
		}
	}

	queue.Items = newItems
}

// ForEach iterates over each item in the processorQueue and executes
// the provided callback (cb) once per item, passinng a pointer to the
// queue, the index of the item, and a pointer to the QueueItem to the callback
// each time (see type itemFn). If the callback at any point returns 'True', the
// loop is broken. A 'false' return from the callback has no impact.
func (queue *processorQueue) ForEach(cb ItemFn) {
	for key, item := range queue.Items {
		if cb(queue, key, item) {
			break
		}
	}
}

// FindById iterates over the queue searching for a QueueItem with an ID that matches
// the int 'id' provided to this method. If found, a pointer to this QueueItem, and the index
// of the QueueItem in the queue is returned. If not found, nil and -1 is returned
func (queue *processorQueue) FindById(id int) (*QueueItem, int) {
	for idx, item := range queue.Items {
		if item.Id == id {
			return item, idx
		}
	}

	return nil, -1
}

// Reorder accepts an array/slice of integers where each value corresponds to the
// ID of a QueueItem. The queue is reordered so that the queue matches the order provided
// by this array. An error is returned if the indexOrder is not the same length, or if the
// indexOrder references an item ID that does no exist.
func (queue *processorQueue) Reorder(indexOrder []int) error {
	queue.Lock()
	defer queue.Unlock()

	queueLength := len(queue.Items)
	if len(indexOrder) != queueLength {
		return errors.New("indexOrder provided must be equal in length to the queue")
	}

	newQueue := make([]*QueueItem, queueLength)
	for k, v := range indexOrder {
		// TODO Please optimize this... calling FindById on each iteration is very expensive.
		if item, idx := queue.FindById(v); item != nil && idx > -1 {
			newQueue[k] = item
			continue
		}

		return fmt.Errorf("indexOrder key %v specifies item ID %v, which does not exist!", k, v)
	}

	queue.Items = newQueue
	return nil
}
