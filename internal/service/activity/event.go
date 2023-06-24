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
type Event string

type Payload any

const (
	// Server is shutting down
	THEA_SHUTDOWN_EVENT  Event = "thea:shutdown"
	PROFILE_UPDATE_EVENT Event = "thea:profile:update"

	// A QueueItem has been updated, this includes any changes to it's state, trouble changes, or including trouble updates.
	ITEM_UPDATE_EVENT        Event = "item:update"
	ITEM_FFMPEG_UPDATE_EVENT Event = "item:ffmpeg:update"
	QUEUE_UPDATE_EVENT       Event = "queue:update"
)

type HandlerMethod func(Event, Payload)

type HandlerChannel chan HandlerEvent
type HandlerEvent struct {
	Event   Event
	Payload Payload
}

type EventDispatcher interface {
	Dispatch(Event, Payload)
}

type EventHandler interface {
	RegisterAsyncHandlerFunction(Event, HandlerMethod)
	RegisterHandlerFunction(Event, HandlerMethod)
	RegisterHandlerChannel(Event, HandlerChannel)
}

type EventCoordinator interface {
	EventDispatcher
	EventHandler
}

type eventHandler struct {
	fnHandlers   map[Event][]handlerMethod
	chanHandlers map[Event][]HandlerChannel
}

type handlerMethod struct {
	handle HandlerMethod
	async  bool
}

func NewEventHandler() EventCoordinator {
	return &eventHandler{
		fnHandlers:   make(map[Event][]handlerMethod),
		chanHandlers: make(map[Event][]HandlerChannel),
	}
}

func (handler *eventHandler) RegisterHandlerChannel(event Event, handle HandlerChannel) {
	handler.chanHandlers[event] = append(handler.chanHandlers[event], handle)
}

// RegisterHandler takes an event type and a handler method which will be stored
// and called with the payload for the event whenever it is provided to the 'Handle' method.
func (handler *eventHandler) RegisterHandlerFunction(event Event, handle HandlerMethod) {
	handler.registerHandlerMethod(event, handlerMethod{handle, false})
}

// RegisterAsyncHandlerFunction accepts a TheaEvent and a HandlerMethod which will be stored and
// called inside of a goroutine when the event is handled.
func (handler *eventHandler) RegisterAsyncHandlerFunction(event Event, handle HandlerMethod) {
	handler.registerHandlerMethod(event, handlerMethod{handle, true})
}

// registerHandlerMethod is the internal implementation for both RegisterHandlerFunction and
// RegisterAsyncHandlerFunction.
func (handler *eventHandler) registerHandlerMethod(event Event, handle handlerMethod) {
	handler.fnHandlers[event] = append(handler.fnHandlers[event], handle)
}

// Handle takes an event type and a payload and dispatches the payload to the handler specified
// for the event type provided.
func (handler *eventHandler) Dispatch(event Event, payload Payload) {
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

// validatePayload ensures that the payload provided is valid for the event specified. An error
// will be returned if the payload is not valid, and the event should not be sent to the registered
// handlers in this case.
func (handler *eventHandler) validatePayload(event Event, payload Payload) error {
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
