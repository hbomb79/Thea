// A collection of event names and common methods used to handle the events, typically
// redirecting the handling to a service method or other method via the `Handler` interface.
package activity

import (
	"fmt"
	"reflect"

	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Log.GetLogger("Event")

// Events emitted by various parts of Thea that should be handled by another, silo'd part
// of Theas' architecture.
// Each silo/service of Thea's architecture listens for a specific event, which indicates
// an item is ready for processing by that service
type TheaEvent string

type TheaPayload any

const (
	// Server is shutting down
	THEA_SHUTDOWN_EVENT  TheaEvent = "thea:shutdown"
	PROFILE_UPDATE_EVENT TheaEvent = "thea:profile:update"

	// A QueueItem has been updated, this includes any changes to it's state, trouble changes, or including trouble updates.
	ITEM_UPDATE_EVENT        TheaEvent = "item:update"
	ITEM_FFMPEG_UPDATE_EVENT TheaEvent = "item:ffmpeg:update"
	QUEUE_UPDATE_EVENT       TheaEvent = "queue:update"
)

type HandlerMethod func(TheaEvent, TheaPayload)

type HandlerChannel chan HandlerEvent
type HandlerEvent struct {
	Event   TheaEvent
	Payload TheaPayload
}

type EventDispatcher interface {
	Dispatch(TheaEvent, TheaPayload)
}

type EventHandler interface {
	RegisterAsyncHandlerFunction(TheaEvent, HandlerMethod)
	RegisterHandlerFunction(TheaEvent, HandlerMethod)
	RegisterHandlerChannel(TheaEvent, HandlerChannel)
}

type EventCoordinator interface {
	EventDispatcher
	EventHandler
}

type eventHandler struct {
	fnHandlers   map[TheaEvent][]handlerMethod
	chanHandlers map[TheaEvent][]HandlerChannel
}

type handlerMethod struct {
	handle HandlerMethod
	async  bool
}

func NewEventHandler() EventCoordinator {
	return &eventHandler{
		fnHandlers:   make(map[TheaEvent][]handlerMethod),
		chanHandlers: make(map[TheaEvent][]HandlerChannel),
	}
}

func (handler *eventHandler) RegisterHandlerChannel(event TheaEvent, handle HandlerChannel) {
	handler.chanHandlers[event] = append(handler.chanHandlers[event], handle)
}

// RegisterHandler takes an event type and a handler method which will be stored
// and called with the payload for the event whenever it is provided to the 'Handle' method.
func (handler *eventHandler) RegisterHandlerFunction(event TheaEvent, handle HandlerMethod) {
	handler.registerHandlerMethod(event, handlerMethod{handle, false})
}

func (handler *eventHandler) RegisterAsyncHandlerFunction(event TheaEvent, handle HandlerMethod) {
	handler.registerHandlerMethod(event, handlerMethod{handle, true})
}

func (handler *eventHandler) registerHandlerMethod(event TheaEvent, handle handlerMethod) {
	handler.fnHandlers[event] = append(handler.fnHandlers[event], handle)
}

// Handle takes an event type and a payload and dispatches the payload to the handler specified
// for the event type provided.
func (handler *eventHandler) Dispatch(event TheaEvent, payload TheaPayload) {
	if err := handler.validatePayload(event, payload); err != nil {
		log.Emit(logger.FATAL, "Dispatch for event %v FAILED validation: %v", event, err)
		return
	}

	if handles, ok := handler.fnHandlers[event]; ok {
		for _, handle := range handles {
			if handle.async {
				go handle.handle(event, payload)
			} else {
				handle.handle(event, payload)
			}
		}
	}

	if handles, ok := handler.chanHandlers[event]; ok {
		payload := HandlerEvent{event, payload}
		for _, handle := range handles {
			handle <- payload
		}
	}
}

func (handler *eventHandler) validatePayload(event TheaEvent, payload TheaPayload) error {
	log.Emit(logger.VERBOSE, "Validating payload %#v for event %v\n", payload, event)

	var payloadTypeName string
	if t := reflect.TypeOf(payload); t != nil {
		payloadTypeName = t.Name()
	} else {
		payloadTypeName = "Nil"
	}

	switch event {
	case QUEUE_UPDATE_EVENT:
		fallthrough
	case THEA_SHUTDOWN_EVENT:
		if payload != nil {
			return fmt.Errorf("event does not accept any payload, found %v", payloadTypeName)
		}

		return nil
	case ITEM_UPDATE_EVENT:
		fallthrough
	case ITEM_FFMPEG_UPDATE_EVENT:
		_, ok := payload.(int)
		if !ok {
			return fmt.Errorf("ITEM events require int representing QueueItem ID, found %v", payloadTypeName)
		}

		return nil
	}

	return fmt.Errorf("TheaEvent type not recognized for validation")
}
