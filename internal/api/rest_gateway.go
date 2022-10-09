package api

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/internal"
)

type HttpGateway struct {
	tpa internal.TPA
}

func NewHttpGateway(tpa internal.TPA) *HttpGateway {
	return &HttpGateway{tpa: tpa}
}

// ** HTTP API Methods ** //

// httpQueueIndex returns the current processor queue with some information
// omitted. Full information for each item can be found via HttpQueueGet
func (httpGateway *HttpGateway) HttpQueueIndex(w http.ResponseWriter, r *http.Request) {
	data, err := sheriffApiMarshal(httpGateway.tpa.GetAllItems(), "api")
	if err != nil {
		JsonMessage(w, err.Error(), http.StatusInternalServerError)

		return
	}

	JsonMarshal(w, data)
}

// httpQueueGet returns full details for a queue item at the index {id} inside the queue
func (httpGateway *HttpGateway) HttpQueueGet(w http.ResponseWriter, r *http.Request) {
	stringId := mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem, err := httpGateway.tpa.GetItem(id)
	if err != nil {
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
	stringId := mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	if httpGateway.tpa.PromoteItem(id) != nil {
		JsonMessage(w, "Failed to promote QueueItem #"+stringId+": "+err.Error(), http.StatusInternalServerError)
	} else {
		JsonMessage(w, "Queue item promoted successfully", http.StatusOK)
	}
}
