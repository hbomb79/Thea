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
		Title:     "COMMAND_SUCCESS",
		Arguments: map[string]interface{}{"payload": data, "command": message},
		Type:      ws.Response,
		Id:        message.Id,
		Target:    message.Origin,
	})

	return nil
}

func (wsGateway *WsGateway) WsQueueDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	v, ok := message.Arguments["id"].(float64)
	if !ok {
		return errors.New("failed to vaidate arguments - ID provided is not an integer")
	}

	queueItem := wsGateway.proc.Queue.FindById(int(v))
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

func (wsGateway *WsGateway) WsQueuePromote(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to promote queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Arguments["id"]
	queueItem := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	}

	err := wsGateway.proc.Queue.PromoteItem(queueItem)
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

func (wsGateway *WsGateway) WsTroubleDetails(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to get trouble details for queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Arguments["id"]
	queueItem := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	} else if queueItem.Trouble == nil {
		return errors.New(fmt.Sprintf(ERR_FMT, "item has no trouble"))
	}

	trouble := struct {
		Type              processor.TroubleType `json:"type"`
		ExpectedArgs      map[string]string     `json:"expectedArgs"`
		processor.Trouble `json:"trouble"`
	}{
		queueItem.Trouble.Type(),
		queueItem.Trouble.Args(),
		queueItem.Trouble,
	}

	fmt.Printf("[Payload] Encoding a payload for %#v\n", queueItem.Trouble)
	hub.Send(&ws.SocketMessage{
		Title:     "COMMAND_SUCCESS",
		Arguments: map[string]interface{}{"payload": trouble, "command": message},
		Id:        message.Id,
		Target:    message.Origin,
		Type:      ws.Response,
	})

	return nil
}

func (wsGateway *WsGateway) WsTroubleResolve(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to resolve trouble for queue item %v - %v"

	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Arguments["id"]
	if item := wsGateway.proc.Queue.FindById(int(idArg.(float64))); item != nil {
		if err := item.Trouble.Resolve(message.Arguments); err != nil {
			return errors.New(fmt.Sprintf(ERR_FMT, idArg, err.Error()))
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

	return errors.New(fmt.Sprintf(ERR_FMT, idArg, "item could not be found"))
}
