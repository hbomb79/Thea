package queue

import (
	"errors"
	"fmt"
	"sync"

	"github.com/hbomb79/Thea/pkg/cache"
)

type QueueManager interface {
	Items() *[]*Item
	Retrieve(string) *Item
	Contains(string) bool
	Push(*Item) error
	Remove(*Item) error
	Pick(ItemStage) *Item
	AdvanceStage(*Item)
	Filter(ItemFn)
	ForEach(ItemFn)
	FindById(id int) (*Item, int)
	Reorder([]int) error
	Reload()
}

// processorQueue is the Queue of items to be processed by this
// processor
type processorQueue struct {
	items  []*Item
	lastId int
	cache  *cache.Cache
	sync.Mutex
}

// NewProcessorQueue returns a pointer to a newly-created ProcessorQueue
// with a slice of QueueItems and the persistent file-system cache
// already populated.
func NewProcessorQueue(cachePath string) QueueManager {
	return &processorQueue{
		items:  make([]*Item, 0),
		lastId: 0,
		cache:  cache.New(cachePath),
	}
}

func (queue *processorQueue) Reload() {
	queue.cache.Load()
}

// Retrieve will search the Queue for a QueueItem with a path that matches
// the one provided. If one is found, a pointer to the item is returned; otherwise
// nil is returned.
func (queue *processorQueue) Retrieve(path string) *Item {
	for _, item := range queue.items {
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

// Items returns all the items inside of this queue
func (queue *processorQueue) Items() *[]*Item {
	return &queue.items
}

// Push accepts a QueueItem pointer and will push (append) it to
// the Queue. This method also sets the 'Id' of the QueueItem
// automatically (queue.lastId)
func (queue *processorQueue) Push(item *Item) error {
	queue.Lock()
	defer queue.Unlock()

	if queue.Contains(item.Path) || queue.cache.HasItem(item.Path) {
		return fmt.Errorf("item (%s) is either already in queue, or marked as complete in cache", item.Path)
	}

	item.ItemID = queue.lastId
	queue.items = append(queue.items, item)
	queue.lastId++

	return nil
}

func (queue *processorQueue) Remove(item *Item) error {
	queue.Lock()
	defer queue.Unlock()

	item, idx := queue.FindById(item.ItemID)
	if item == nil || idx < 0 {
		return errors.New("cannot remove: does not exist in queue")
	}

	if item.Status == Cancelled {
		queue.cache.PushItem(item.Path, "cancelled")
	}

	queue.items = append(queue.items[:idx], queue.items[idx+1:len(queue.items)]...)
	return nil
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *processorQueue) Pick(stage ItemStage) *Item {
	queue.Lock()
	defer queue.Unlock()

	for _, item := range queue.items {
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
func (queue *processorQueue) AdvanceStage(item *Item) {
	queue.Lock()
	defer queue.Unlock()

	if item.Stage == Finish {
		item.SetStatus(Completed)
	} else if item.Stage == Database {
		item.SetStage(Finish)
		item.SetStatus(Completed)

		// Add this item to the cache to indicate it's complete
		queue.cache.PushItem(item.Path, "completed")
	} else {
		item.SetStage(item.Stage + 1)
		item.SetStatus(Pending)
	}
}

type ItemFn func(QueueManager, int, *Item) bool

// Filter runs the provided callback for every item inside the queue. If the callback
// returns true, the item is retained. Otherwise, if the callback returns false, the item
// is ejected from the queue.
func (queue *processorQueue) Filter(cb ItemFn) {
	queue.Lock()
	defer queue.Unlock()

	newItems := make([]*Item, 0)
	for key, item := range queue.items {
		if cb(queue, key, item) {
			newItems = append(newItems, item)
		}
	}

	queue.items = newItems
}

// ForEach iterates over each item in the ProcessorQueue and executes
// the provided callback (cb) once per item, passinng a pointer to the
// queue, the index of the item, and a pointer to the QueueItem to the callback
// each time (see type itemFn). If the callback at any point returns 'True', the
// loop is broken. A 'false' return from the callback has no impact.
func (queue *processorQueue) ForEach(cb ItemFn) {
	for key, item := range queue.items {
		if cb(queue, key, item) {
			break
		}
	}
}

// FindById iterates over the queue searching for a QueueItem with an ID that matches
// the int 'id' provided to this method. If found, a pointer to this QueueItem, and the index
// of the QueueItem in the queue is returned. If not found, nil and -1 is returned
func (queue *processorQueue) FindById(id int) (*Item, int) {
	for idx, item := range queue.items {
		if item.ItemID == id {
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

	queueLength := len(queue.items)
	if len(indexOrder) != queueLength {
		return errors.New("indexOrder provided must be equal in length to the queue")
	}

	newQueue := make([]*Item, queueLength)
	for k, v := range indexOrder {
		if item, idx := queue.FindById(v); item != nil && idx > -1 {
			newQueue[k] = item
			continue
		}

		return fmt.Errorf("indexOrder key %v specifies item ID %v, which does not exist", k, v)
	}

	queue.items = newQueue
	return nil
}
