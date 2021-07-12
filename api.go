package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
)

type ApiNegotiator struct {
	Tpa *TPA
}

func (negotiator *ApiNegotiator) OnProcessorUpdate(update *processor.ProcessorUpdate) {
	negotiator.Tpa.socketHub.Send(&ws.SocketMessage{
		Title: "UPDATE",
		Arguments: map[string]interface{}{
			"context": update.Context,
		},
		Type: ws.Update,
	})
}

func (negotiator *ApiNegotiator) wsQueueIndex(hub *ws.SocketHub, message *ws.SocketMessage) error {
	data, err := sheriffApiMarshal(negotiator.Tpa.proc.Queue, []string{"api"})
	if err != nil {
		return err
	}

	// Queue a reply to this message by setting the target of the
	// next message to the origin of the current one.
	// Also set the ID to match any provided by the client
	// so they can pair this reply with the source request.
	hub.Send(&ws.SocketMessage{
		Title:     "COMMAND_SUCCESS",
		Arguments: map[string]interface{}{"payload": data, "command": message},
		Type:      ws.Response,
		Id:        message.Id,
		Target:    message.Origin,
	})

	return nil
}

func (negotiator *ApiNegotiator) wsQueueDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	v, ok := message.Arguments["id"].(float64)
	if !ok {
		return errors.New("failed to vaidate arguments - ID provided is not an integer")
	}

	queueItem := negotiator.Tpa.proc.Queue.FindById(int(v))
	if queueItem == nil {
		return errors.New("failed to get queue details - item with matching ID not found")
	}

	hub.Send(&ws.SocketMessage{
		Title:     "COMMAND_SUCCESS",
		Arguments: map[string]interface{}{"payload": queueItem, "command": message},
		Id:        message.Id,
		Target:    message.Origin,
		Type:      ws.Response,
	})
	return nil
}

func (negotiator *ApiNegotiator) wsQueuePromote(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to promote queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	v, ok := message.Arguments["id"].(float64)
	if !ok {
		return errors.New("failed to vaidate arguments - ID provided is not an integer")
	}

	queueItem := negotiator.Tpa.proc.Queue.FindById(int(v))
	if queueItem == nil {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	}

	err := negotiator.Tpa.proc.Queue.PromoteItem(queueItem)
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, err.Error()))
	}

	hub.Send(&ws.SocketMessage{
		Title:     "COMMAND_SUCCESS",
		Arguments: map[string]interface{}{"payload": queueItem, "command": message},
		Id:        message.Id,
		Target:    message.Origin,
		Type:      ws.Response,
	})

	return nil
}

func (negotiator *ApiNegotiator) wsTroubleDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	return nil
}

func (negotiator *ApiNegotiator) wsTroubleResolve(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to resolve trouble for queue item %v - %v"

	stringId, ok := message.Arguments["id"]
	if !ok {
		return errors.New(fmt.Sprintf(ERR_FMT, "?", "no 'id' argument provided"))
	}

	queueItemId, err := strconv.Atoi(fmt.Sprintf("%v", stringId))
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, stringId, err.Error()))
	}

	if item := negotiator.Tpa.proc.Queue.FindById(queueItemId); item != nil {
		if err = item.Trouble.Resolve(message.Arguments); err != nil {
			return errors.New(fmt.Sprintf(ERR_FMT, stringId, err.Error()))
		}

		hub.Send(&ws.SocketMessage{
			Title:  "COMMAND_SUCCESS",
			Id:     message.Id,
			Target: message.Origin,
			Type:   ws.Response,
			Arguments: map[string]interface{}{
				"command": message,
			},
		})

		return nil
	}

	return errors.New(fmt.Sprintf(ERR_FMT, stringId, "item could not be found"))
}

// HttpQueueIndex returns the current processor queue with some information
// omitted. Full information for each item can be found via HttpQueueGet
func (negotiator *ApiNegotiator) HttpQueueIndex(w http.ResponseWriter, r *http.Request) {
	data, err := sheriffApiMarshal(negotiator.Tpa.proc.Queue, []string{"api"})
	if err != nil {
		api.JsonMessage(w, err.Error(), http.StatusInternalServerError)

		return
	}

	api.JsonMarshal(w, data)
}

// HttpQueueGet returns full details for a queue item at the index {id} inside the queue
func (negotiator *ApiNegotiator) HttpQueueGet(w http.ResponseWriter, r *http.Request) {
	queue, stringId := negotiator.Tpa.proc.Queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem := queue.FindById(id)
	if queueItem == nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' cannot be found", http.StatusBadRequest)
		return
	}

	api.JsonMarshal(w, queueItem)
}

// HttpQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func (negotiator *ApiNegotiator) HttpQueueUpdate(w http.ResponseWriter, r *http.Request) {
	queue, stringId := negotiator.Tpa.proc.Queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem := queue.FindById(id)
	if queueItem == nil {
		api.JsonMessage(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
	} else if queue.PromoteItem(queueItem) != nil {
		api.JsonMessage(w, "Failed to promote QueueItem #"+stringId+": "+err.Error(), http.StatusInternalServerError)
	} else {
		api.JsonMessage(w, "Queue item promoted successfully", http.StatusOK)
	}
}
