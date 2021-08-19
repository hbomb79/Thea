package api

import (
	"errors"
	"fmt"

	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
)

type WsGateway struct {
	proc *processor.Processor
}

func NewWsGateway(proc *processor.Processor) *WsGateway {
	return &WsGateway{proc: proc}
}

// ** Websocket API Methods ** //
func (wsGateway *WsGateway) WsQueueIndex(hub *ws.SocketHub, message *ws.SocketMessage) error {
	data, err := sheriffApiMarshal(wsGateway.proc.Queue, "api")
	if err != nil {
		return err
	}

	// Queue a reply to this message by setting the target of the
	// next message to the origin of the current one.
	// Also set the ID to match any provided by the client
	// so they can pair this reply with the source request.
	hub.Send(&ws.SocketMessage{
		Title:  "COMMAND_SUCCESS",
		Body:   map[string]interface{}{"payload": data, "command": message},
		Type:   ws.Response,
		Id:     message.Id,
		Target: message.Origin,
	})

	return nil
}

func (wsGateway *WsGateway) WsQueueDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	v, ok := message.Body["id"].(float64)
	if !ok {
		return errors.New("failed to vaidate arguments - ID provided is not an integer")
	}

	queueItem, idx := wsGateway.proc.Queue.FindById(int(v))
	if queueItem == nil || idx < 0 {
		return errors.New("failed to get queue details - item with matching ID not found")
	}

	hub.Send(&ws.SocketMessage{
		Title:  "COMMAND_SUCCESS",
		Body:   map[string]interface{}{"payload": queueItem, "command": message},
		Id:     message.Id,
		Target: message.Origin,
		Type:   ws.Response,
	})
	return nil
}

func (wsGateway *WsGateway) WsQueuePromote(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to promote queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	queueItem, idx := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil || idx < 0 {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	}

	err := wsGateway.proc.Queue.PromoteItem(queueItem)
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, err.Error()))
	}

	hub.Send(&ws.SocketMessage{
		Title:  "COMMAND_SUCCESS",
		Body:   map[string]interface{}{"payload": queueItem, "command": message},
		Id:     message.Id,
		Target: message.Origin,
		Type:   ws.Response,
	})

	wsGateway.proc.UpdateChan <- queueItem.Id
	return nil
}

func (wsGateway *WsGateway) WsTroubleDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to get trouble details for queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	queueItem, idx := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil || idx < 0 {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	} else if queueItem.Trouble == nil {
		return errors.New(fmt.Sprintf(ERR_FMT, "item has no trouble"))
	}

	trouble := struct {
		Message           string                `json:"message"`
		Type              processor.TroubleType `json:"type"`
		ExpectedArgs      map[string]string     `json:"expectedArgs"`
		AdditionalPayload interface{}           `json:"additionalPayload"`
		ItemId            int                   `json:"itemId"`
	}{
		queueItem.Trouble.Error(),
		queueItem.Trouble.Type(),
		queueItem.Trouble.Args(),
		queueItem.Trouble.Payload(),
		queueItem.Id,
	}

	hub.Send(&ws.SocketMessage{
		Title:  "COMMAND_SUCCESS",
		Body:   map[string]interface{}{"payload": trouble, "command": message},
		Id:     message.Id,
		Target: message.Origin,
		Type:   ws.Response,
	})

	return nil
}

func (wsGateway *WsGateway) WsTroubleResolve(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to resolve trouble for queue item %v - %v"

	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	if item, idx := wsGateway.proc.Queue.FindById(int(idArg.(float64))); item != nil && idx >= 0 {
		if err := item.Trouble.Resolve(message.Body); err != nil {
			return errors.New(fmt.Sprintf(ERR_FMT, idArg, err.Error()))
		}

		wsGateway.proc.WorkerPool.WakeupWorkers(item.Stage)
		hub.Send(&ws.SocketMessage{
			Title:  "COMMAND_SUCCESS",
			Id:     message.Id,
			Target: message.Origin,
			Type:   ws.Response,
			Body: map[string]interface{}{
				"command": message,
			},
		})

		return nil
	}

	return errors.New(fmt.Sprintf(ERR_FMT, idArg, "item could not be found"))
}
