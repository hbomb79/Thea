package processor

import (
	"fmt"
	"io/fs"
	"sync"
)

// QueueItemStatus represents whether or not the
// QueueItem is currently being worked on, or if
// it's waiting for a worker to pick it up
// and begin working on the task
type QueueItemStatus int

// If a task is Pending, it's waiting for a worker
// ... if processing, it's currently being worked on.
// When a stage in the pipeline is finished with the task,
// it should set the Stage to the next stage, and set the
// Status to pending - except for Format stage, which should
// mark it as completed
const (
	Pending QueueItemStatus = iota
	Processing
	Completed
)

type QueueItem struct {
	Path       string
	Name       string
	Status     QueueItemStatus
	Stage      PipelineStage
	StatusLine string
}

type QueuePickError struct {
	pickStage  PipelineStage
	pickStatus QueueItemStatus
}

func (e *QueuePickError) Error() string {
	return fmt.Sprintf("cannot find item with stage %v and status %v\n", e.pickStage, e.pickStatus)
}

type QueueAssignError struct {
	itemName      string
	currentStage  PipelineStage
	currentStatus QueueItemStatus
}

func (e *QueueAssignError) Error() string {
	return fmt.Sprintf("cannot assign item to worker - item (%v) already assigned! Stage: %vneovim and status %v\n", e.itemName, e.currentStage, e.currentStatus)
}

type ProcessorQueue struct {
	Items []QueueItem
	sync.Mutex
}

// HandleFile will take the provided file and if it's not
// currently inside the queue, it will be inserted in to the queue.
// If it is in the queue, the entry is skipped - this is because
// this method is usually called as a result of polling the
// input directory many times a day for new files.
func (queue *ProcessorQueue) HandleFile(path string, fileInfo fs.FileInfo) bool {
	queue.Lock()
	defer queue.Unlock()

	if !queue.isInQueue(path) {
		queue.Items = append(queue.Items, QueueItem{
			Name:   fileInfo.Name(),
			Path:   path,
			Status: Pending,
			Stage:  Title,
		})

		return true
	}

	return false
}

// isInQueue will return true if the queue contains a QueueItem
// with a path field matching the path provided to this method
// Note: callers responsiblity to ensure the queues Mutex is
// already locked before use - otherwise the queue contents
// may mutate while iterating through it
func (queue *ProcessorQueue) isInQueue(path string) bool {
	for _, v := range queue.Items {
		if v.Path == path {
			return true
		}
	}

	return false
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *ProcessorQueue) Pick(stage PipelineStage, status QueueItemStatus) (*QueueItem, error) {
	queue.Lock()
	defer queue.Unlock()

	for _, item := range queue.Items {
		if item.Stage == stage && item.Status == status {
			return &item, nil
		}
	}

	return nil, &QueuePickError{stage, status}
}

// AssignItem will assign the provided item by setting it's
// status to Processing - will return an error if the QueueItem
// already has status of Processing.
// It is the workers responsiblity to perform the work and then
// advance the stage (see queue.AdvanceStage) - freeing the QueueItem
// for work by another worker
func (queue *ProcessorQueue) AssignItem(item *QueueItem) error {
	queue.Lock()
	defer queue.Unlock()

	if item.Status == Processing {
		return &QueueAssignError{item.Name, item.Stage, item.Status}
	}

	item.Status = Processing
	return nil
}

// AdvanceStage will take the QueueItem this method is attached to,
// and set it's stage to the next stage and reset it's status to Pending
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *ProcessorQueue) AdvanceStage(item *QueueItem) {
	queue.Lock()
	defer queue.Unlock()

	if item.Stage == Finish {
		item.Status = Completed
	} else if item.Stage == Format {
		item.Stage = Finish
		item.Status = Completed
	} else {
		item.Stage++
		item.Status = Pending
	}
}
