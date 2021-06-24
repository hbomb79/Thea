package processor

import (
	"io/fs"
)

type QueueItemStatus int

const (
	Pending QueueItemStatus = iota
	Querying
	Queued
	Formatting
	Finished
)

type ProcessorQueue []QueueItem
type QueueItem struct {
	Path   string
	Name   string
	Status QueueItemStatus
}

// HandleFile will take the provided file and if it's not
// currently inside the queue, it will be inserted in to the queue.
// If it is in the queue, the entry is skipped - this is because
// this method is usually called as a result of polling the
// input directory many times a day for new files.
func (queue *ProcessorQueue) HandleFile(path string, fileInfo fs.FileInfo) {
	if !queue.isInQueue(path) {
		*queue = append(*queue, QueueItem{
			Name:   fileInfo.Name(),
			Path:   path,
			Status: Pending,
		})
	}
}

func (queue *ProcessorQueue) isInQueue(path string) bool {
	for _, v := range *queue {
		if v.Path == path {
			return true
		}
	}

	return false
}
