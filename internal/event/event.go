// A collection of event names and common methods used to handle the events, typically
// redirecting the handling to a service method or other method via the `Handler` interface.
package event

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Activity")

// Events emitted by various parts of Thea that should be handled by another, silo'd part
// of Theas' architecture.
// Each silo/service of Thea's architecture listens for a specific event, which indicates
// an item is ready for processing by that service
type (
	Event         string
	Payload       any
	HandlerMethod func(Event, Payload)

	HandlerChannel chan HandlerEvent
	HandlerEvent   struct {
		Event   Event
		Payload Payload
	}

	EventDispatcher interface {
		Dispatch(Event, Payload)
	}

	EventHandler interface {
		RegisterAsyncHandlerFunction(Event, HandlerMethod)
		RegisterHandlerFunction(Event, HandlerMethod)
		RegisterHandlerChannel(HandlerChannel, ...Event)
	}

	EventCoordinator interface {
		EventDispatcher
		EventHandler
	}

	eventHandler struct {
		fnHandlers   map[Event][]handlerMethod
		chanHandlers map[Event][]HandlerChannel
	}

	handlerMethod struct {
		handle HandlerMethod
		async  bool
	}
)

const (
	INGEST_UPDATE   Event = "ingest:update"
	INGEST_COMPLETE Event = "ingest:complete"

	TRANSCODE_UPDATE        Event = "transcode:task:update"
	TRANSCODE_COMPLETE      Event = "transcode:task:complete"
	TRANSCODE_TASK_PROGRESS Event = "transcode:task:update:progress"

	WORKFLOW_UPDATE Event = "workflow:update"

	DOWNLOAD_UPDATE   Event = "download:update"
	DOWNLOAD_COMPLETE Event = "download:complete"
	DOWNLOAD_PROGRESS Event = "download:update:progress"
)

func New() EventCoordinator {
	return &eventHandler{
		fnHandlers:   make(map[Event][]handlerMethod),
		chanHandlers: make(map[Event][]HandlerChannel),
	}
}

// RegisterHandlerChannel takes an event type and a channel and will send Event messages on
// the channel any time a Dispatch for the provided event occurs.
// This method can be used multiple times for different events on the same channel.
//
// If the channel is BLOCKED when the event bus attempts to send the message on the handler channel,
// then the thread dispatching the event will also be BLOCKED. It is recomended to buffer the handler channels
// appropiately to avoid dispatcher-side blocking.
func (handler *eventHandler) RegisterHandlerChannel(handle HandlerChannel, events ...Event) {
	for _, event := range events {
		handler.chanHandlers[event] = append(handler.chanHandlers[event], handle)
	}
}

// RegisterHandler takes an event type and a handler method which will be stored
// and called with the payload for the event whenever it is provided to the 'Handle' method.
// The handle provided should be guaranteed to return quickly, else other threads calling
// Dispatch on this event bus will be blocked.
func (handler *eventHandler) RegisterHandlerFunction(event Event, handle HandlerMethod) {
	handler.registerHandlerMethod(event, handlerMethod{handle, false})
}

// RegisterAsyncHandlerFunction accepts a TheaEvent and a HandlerMethod which will be stored and
// called inside of a goroutine when the event is handled.
// The speed at which this handle runs is not important to the event bus, unlike RegisterHandlerFunction.
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
// Note that this method WILL block if a synchronous handler function is blocking, or if channel
// handlers are blocked.
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
	var payloadTypeName string
	if t := reflect.TypeOf(payload); t != nil {
		payloadTypeName = t.Name()
	} else {
		payloadTypeName = "Nil"
	}

	switch event {
	case INGEST_UPDATE:
		fallthrough
	case INGEST_COMPLETE:
		fallthrough
	case TRANSCODE_COMPLETE:
		fallthrough
	case TRANSCODE_TASK_PROGRESS:
		fallthrough
	case WORKFLOW_UPDATE:
		fallthrough
	case DOWNLOAD_UPDATE:
		fallthrough
	case DOWNLOAD_COMPLETE:
		fallthrough
	case DOWNLOAD_PROGRESS:
		fallthrough
	case TRANSCODE_UPDATE:
		if _, ok := payload.(uuid.UUID); !ok {
			return fmt.Errorf("illegal payload (type %s) for %s event. Expected uuid.UUID payload", payloadTypeName, event)
		}

		return nil
	}

	return errors.New("event type not recognized for validation")
}
