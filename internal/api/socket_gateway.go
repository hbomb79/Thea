package api

import (
	"errors"
	"fmt"

	"github.com/hbomb79/TPA/internal"
	"github.com/hbomb79/TPA/internal/profile"
	"github.com/hbomb79/TPA/pkg/socket"
)

type WsGateway struct {
	tpa internal.TPA
}

func NewWsGateway(tpa internal.TPA) *WsGateway {
	return &WsGateway{tpa: tpa}
}

// ** Websocket API Methods ** //
func (wsGateway *WsGateway) WsQueueIndex(hub *socket.SocketHub, message *socket.SocketMessage) error {
	data, err := sheriffApiMarshal(wsGateway.tpa.GetAllItems(), "api")
	if err != nil {
		return err
	}

	// Queue a reply to this message by setting the target of the
	// next message to the origin of the current one.
	// Also set the ID to match any provided by the client
	// so they can pair this reply with the source request.
	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": data}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsQueueDetails(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	v, ok := message.Body["id"].(float64)
	if !ok {
		return errors.New("failed to vaidate arguments - ID provided is not an integer")
	}

	queueItem, err := wsGateway.tpa.GetItem(int(v))
	if err != nil {
		return err
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsQueueReorder(hub *socket.SocketHub, message *socket.SocketMessage) error {
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

	if err := wsGateway.tpa.ReorderQueue(newOrder); err != nil {
		return fmt.Errorf(ERR_FMT, err.Error())
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	wsGateway.tpa.NotifyQueueUpdate()

	return nil
}

func (wsGateway *WsGateway) WsItemPromote(hub *socket.SocketHub, message *socket.SocketMessage) error {
	const ERR_FMT = "failed to promote queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	id := int(message.Body["id"].(float64))
	err := wsGateway.tpa.PromoteItem(id)
	if err != nil {
		return fmt.Errorf(ERR_FMT, err.Error())
	}

	wsGateway.tpa.NotifyItemUpdate(id)
	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": id}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsItemPause(hub *socket.SocketHub, message *socket.SocketMessage) error {
	const ERR_FMT = "failed to pause queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := int(message.Body["id"].(float64))
	if err := wsGateway.tpa.PauseItem(idArg); err != nil {
		return err
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": idArg}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsItemCancel(hub *socket.SocketHub, message *socket.SocketMessage) error {
	const ERR_FMT = "failed to cancel queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := int(message.Body["id"].(float64))
	err := wsGateway.tpa.CancelItem(idArg)
	if err != nil {
		return fmt.Errorf(ERR_FMT, err.Error())
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": idArg}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsTroubleDetails(hub *socket.SocketHub, message *socket.SocketMessage) error {
	const ERR_FMT = "failed to get trouble details for queue item - %v"
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	idArg := message.Body["id"]
	queueItem, err := wsGateway.tpa.GetItem(int(idArg.(float64)))
	if err != nil {
		return err
	} else if queueItem.Trouble == nil {
		return fmt.Errorf(ERR_FMT, "item has no trouble")
	}

	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": queueItem.Trouble}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsTroubleResolve(hub *socket.SocketHub, message *socket.SocketMessage) error {
	const ERR_FMT = "failed to resolve trouble for queue item %v - %v"

	// Validate QueueItem ID is present
	if err := message.ValidateArguments(map[string]string{"id": "number"}); err != nil {
		return err
	}

	// Optional paramater for instance tag allows the client to resolve troubles embedded inside of ffmpeg instances
	instanceTag, isEmbed := message.Body["instanceTag"]

	idArg := message.Body["id"]
	if item, err := wsGateway.tpa.GetItem(int(idArg.(float64))); err == nil {
		if isEmbed {
			for _, i := range wsGateway.tpa.GetFfmpegInstancesForItem(item.ItemID) {
				if i.ProfileTag() == instanceTag {
					if err := i.ResolveTrouble(message.Body); err != nil {
						return fmt.Errorf("failed to resolve embedded ffmpeg trouble for queue item %v - %v", idArg, err.Error())
					}

					break
				}
			}
		} else {
			if item.Trouble != nil {
				if err := item.Trouble.Resolve(message.Body); err != nil {
					return fmt.Errorf(ERR_FMT, idArg, err.Error())
				}
			} else {
				return fmt.Errorf(ERR_FMT, idArg, "item has no trouble")
			}
		}

		hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
		return nil
	}

	return fmt.Errorf(ERR_FMT, idArg, "item could not be found")
}

func (wsGateway *WsGateway) WsProfileIndex(hub *socket.SocketHub, message *socket.SocketMessage) error {
	hub.Send(message.FormReply("COMMAND_SUCCESS", map[string]interface{}{"payload": wsGateway.tpa.GetAllProfiles()}, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsProfileCreate(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"tag": "string"}); err != nil {
		return err
	}

	p := profile.NewProfile(message.Body["tag"].(string))
	if err := wsGateway.tpa.CreateProfile(p); err != nil {
		return err
	}

	wsGateway.tpa.NotifyProfileUpdate()
	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsProfileRemove(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"tag": "string"}); err != nil {
		return err
	}

	if err := wsGateway.tpa.DeleteProfileByTag(message.Body["tag"].(string)); err != nil {
		return err
	}

	wsGateway.tpa.NotifyProfileUpdate()
	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsProfileMove(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"tag": "string", "desiredIndex": "int"}); err != nil {
		return err
	}

	tag := message.Body["tag"].(string)
	desiredIndex := int(message.Body["desiredIndex"].(float64))

	if err := wsGateway.tpa.MoveProfile(tag, desiredIndex); err != nil {
		return err
	}

	wsGateway.tpa.NotifyProfileUpdate()
	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsProfileSetMatchConditions(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"profileTag": "string"}); err != nil {
		return err
	}

	profile := wsGateway.tpa.GetProfileByTag(message.Body["profileTag"].(string))
	if profile == nil {
		return fmt.Errorf("cannot set match conditions for profile because tag '%v' is invalid", message.Body["profileTag"])
	}

	matchConditions, ok := message.Body["matchConditions"]
	if !ok {
		return fmt.Errorf("cannot set match conditions on profile '%v' because matchConditions key is missing from payload", message.Body["profileTag"])
	}

	err := profile.SetMatchConditions(matchConditions)
	if err != nil {
		return fmt.Errorf("cannot set match conditions on profile '%v': %v", message.Body["profileTag"], err.Error())
	}

	wsGateway.tpa.NotifyProfileUpdate()
	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	return nil
}

func (wsGateway *WsGateway) WsProfileUpdateCommand(hub *socket.SocketHub, message *socket.SocketMessage) error {
	if err := message.ValidateArguments(map[string]string{"profileTag": "string"}); err != nil {
		return err
	}

	profile := wsGateway.tpa.GetProfileByTag(message.Body["profileTag"].(string))
	if profile == nil {
		return fmt.Errorf("cannot update target command for profile because tag '%v' is invalid", message.Body["profileTag"])
	}

	command, ok := message.Body["command"]
	if !ok {
		return fmt.Errorf("cannot update target command on profile '%v' because command key is missing from payload", message.Body["profileTag"])
	}

	err := profile.SetCommand(command)
	if err != nil {
		return fmt.Errorf("cannot update target command on profile '%v': %v", message.Body["profileTag"], err.Error())
	}

	wsGateway.tpa.NotifyProfileUpdate()
	hub.Send(message.FormReply("COMMAND_SUCCESS", nil, socket.Response))
	return nil
}
