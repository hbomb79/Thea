package processor

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/api"
	"github.com/liip/sheriff"
)

// ProcessorQueue is the Queue of items to be processed by this
// processor
type ProcessorQueue struct {
	Items []*QueueItem `groups:"api"`
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
		queue.Items = append(queue.Items, &QueueItem{
			Name:   fileInfo.Name(),
			Path:   path,
			Status: Pending,
			Stage:  Title,
		})

		return true
	}

	return false
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *ProcessorQueue) Pick(stage PipelineStage) *QueueItem {
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

// SheriffApiMarshal is a method of QueueItem that will marshal
// the QueueItem that marshals the struct using Sheriff to remove
// items from the struct that aren't exposed to the API (i.e. removes
// struct fields that lack the `groups:"api"` tag)
func (queue *ProcessorQueue) SheriffApiMarshal() (interface{}, error) {
	o := &sheriff.Options{Groups: []string{"api"}}

	data, err := sheriff.Marshal(o, queue)
	if err != nil {
		return nil, err
	}

	return data, nil
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

// apiQueueIndex returns the current processor queue
func (queue *ProcessorQueue) ApiQueueIndex(w http.ResponseWriter, r *http.Request) {
	data, err := queue.SheriffApiMarshal()
	if err != nil {
		api.JsonMessage(w, err.Error(), http.StatusInternalServerError)

		return
	}

	api.JsonMarshal(w, data)
}

// apiQueueGet returns full details for a queue item at the index {id} inside the queue
func (queue *ProcessorQueue) ApiQueueGet(w http.ResponseWriter, r *http.Request) {
	stringId := mux.Vars(r)["id"]
	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	if len(queue.Items) <= id {
		api.JsonMessage(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
		return
	}

	api.JsonMarshal(w, queue.Items[id])
}

// apiQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func (queue *ProcessorQueue) ApiQueueUpdate(w http.ResponseWriter, r *http.Request) {
	stringId := mux.Vars(r)["id"]
	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	if len(queue.Items) <= id {
		api.JsonMessage(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
	} else if queue.PromoteItem(queue.Items[id]) != nil {
		api.JsonMessage(w, "Failed to promote QueueItem #"+stringId+": "+err.Error(), http.StatusInternalServerError)
	} else {
		api.JsonMessage(w, "Queue item promoted successfully", http.StatusOK)
	}
}
