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
	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": data}, ws.Response))
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

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem}, ws.Response))
	return nil
}

func (wsGateway *WsGateway) WsQueueReorder(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to reorder queue - %v"
	index := message.Body["index"]
	if index == nil {
		return fmt.Errorf(ERR_FMT, "required 'index' array is missing")
	}

	orderArray, ok := index.([]interface{})
	if !ok {
		return fmt.Errorf(ERR_FMT, "'index' key is malformed, must be a JSON array of integers")
	}

	newOrder := make([]int, len(orderArray))
	for k, v := range orderArray {
		tmp, ok := v.(float64)
		if !ok {
			return fmt.Errorf(ERR_FMT, fmt.Sprintf("'index' array contains illegal value at key %v (val: %v) - can only be integers", k, v))
		}

		newOrder[k] = int(tmp)
	}

	if err := wsGateway.proc.Queue.Reorder(newOrder); err != nil {
		return fmt.Errorf(ERR_FMT, err.Error())
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, ws.Response))
	wsGateway.proc.UpdateChan <- -1

	return nil
}

func (wsGateway *WsGateway) WsItemPromote(hub *ws.SocketHub, message *ws.SocketMessage) error {
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

	wsGateway.proc.UpdateChan <- queueItem.Id
	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem}, ws.Response))
	return nil
}

func (wsGateway *WsGateway) WsItemPause(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to pause queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	queueItem, idx := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil || idx < 0 {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	}

	err := queueItem.Pause()
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, err.Error()))
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem}, ws.Response))
	return nil
}

func (wsGateway *WsGateway) WsItemCancel(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to cancel queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	queueItem, idx := wsGateway.proc.Queue.FindById(int(idArg.(float64)))
	if queueItem == nil || idx < 0 {
		return errors.New(fmt.Sprintf(ERR_FMT, "item with matching ID not found"))
	}

	err := queueItem.Cancel()
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, err.Error()))
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem}, ws.Response))
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

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem.Trouble}, ws.Response))
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
