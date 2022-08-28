package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/internal"
)

type HttpGateway struct {
	queue internal.ProcessorQueue
}

func NewHttpGateway(queue internal.ProcessorQueue) *HttpGateway {
	return &HttpGateway{queue: queue}
}

// ** HTTP API Methods ** //

// httpQueueIndex returns the current processor queue with some information
// omitted. Full information for each item can be found via HttpQueueGet
func (httpGateway *HttpGateway) HttpQueueIndex(w http.ResponseWriter, r *http.Request) {
	data, err := sheriffApiMarshal(httpGateway.queue, "api")
	if err != nil {
		JsonMessage(w, err.Error(), http.StatusInternalServerError)

		return
	}

	JsonMarshal(w, data)
}

// httpQueueGet returns full details for a queue item at the index {id} inside the queue
func (httpGateway *HttpGateway) HttpQueueGet(w http.ResponseWriter, r *http.Request) {
	queue, stringId := httpGateway.queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem, idx := queue.FindById(id)
	if queueItem == nil || idx < 0 {
		JsonMessage(w, "QueueItem ID '"+stringId+"' cannot be found", http.StatusBadRequest)
		return
	}

	JsonMarshal(w, queueItem)
}

// httpQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func (httpGateway *HttpGateway) HttpQueueUpdate(w http.ResponseWriter, r *http.Request) {
	queue, stringId := httpGateway.queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem, idx := queue.FindById(id)
	if queueItem == nil || idx < 0 {
		JsonMessage(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
	} else if queue.PromoteItem(queueItem) != nil {
		JsonMessage(w, "Failed to promote QueueItem #"+stringId+": "+err.Error(), http.StatusInternalServerError)
	} else {
		JsonMessage(w, "Queue item promoted successfully", http.StatusOK)
	}
}
