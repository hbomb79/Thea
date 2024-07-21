package internal

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/pkg/logger"
)

const (
	DebounceDuration time.Duration = time.Second * 2
	MaxTimerDuration time.Duration = time.Second * 5

	RapidEventDebounceDuration time.Duration = time.Millisecond * 500
	RapidEventMaxTimerDuration time.Duration = time.Second * 2
)

type (
	broadcastHandler func(uuid.UUID) error

	broadcaster interface {
		BroadcastTranscodeUpdate(id uuid.UUID) error
		BroadcastTaskProgressUpdate(id uuid.UUID) error
		BroadcastWorkflowUpdate(id uuid.UUID) error
		BroadcastMediaUpdate(id uuid.UUID) error
		BroadcastIngestUpdate(id uuid.UUID) error
	}

	eventKey struct {
		ev event.Event
		id uuid.UUID
	}

	activityService struct {
		*sync.Mutex
		broadcaster
		eventBus       event.EventHandler
		debounceTimers map[eventKey]*time.Timer
		maxTimers      map[eventKey]*time.Timer
	}
)

func newActivityService(broadcaster broadcaster, event event.EventHandler) *activityService {
	return &activityService{
		Mutex:          &sync.Mutex{},
		broadcaster:    broadcaster,
		eventBus:       event,
		debounceTimers: make(map[eventKey]*time.Timer),
		maxTimers:      make(map[eventKey]*time.Timer),
	}
}

func (service *activityService) Run(ctx context.Context) error {
	channelBufferSize := 100
	messageChan := make(chan event.HandlerEvent, channelBufferSize)
	service.eventBus.RegisterHandlerChannel(messageChan,
		event.IngestUpdateEvent, event.IngestCompleteEvent, event.TranscodeUpdateEvent,
		event.TranscodeTaskProgressEvent, event.TranscodeCompleteEvent, event.WorkflowUpdateEvent,
		event.DownloadUpdateEvent, event.DownloadCompleteEvent, event.DownloadProgressEvent,
		event.NewMediaEvent, event.DeleteMediaEvent,
	)

	log.Emit(logger.NEW, "Activity service started\n")
	for {
		select {
		case ev := <-messageChan:
			if err := service.handleEvent(ev); err != nil {
				log.Emit(logger.ERROR, "Handling of event %v failed: %v\n", ev, err)
			}
		case <-ctx.Done():
			log.Emit(logger.STOP, "Activity service closed\n")
			return nil
		}
	}
}

func (service *activityService) handleEvent(ev event.HandlerEvent) error {
	resourceID, ok := ev.Payload.(uuid.UUID)
	if !ok {
		return errors.New("illegal payload (expected UUID)")
	}

	resourceKey := eventKey{id: resourceID, ev: ev.Event}

	//exhaustive:enforce
	switch ev.Event {
	case event.IngestUpdateEvent:
		fallthrough
	case event.IngestCompleteEvent:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastIngestUpdate)
	case event.TranscodeUpdateEvent:
		fallthrough
	case event.TranscodeCompleteEvent:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastTranscodeUpdate)
	case event.TranscodeTaskProgressEvent:
		service.scheduleRapidEventBroadcast(resourceKey, service.BroadcastTaskProgressUpdate)
	case event.WorkflowUpdateEvent:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastWorkflowUpdate)
	case event.NewMediaEvent:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastMediaUpdate)
	case event.DeleteMediaEvent:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastMediaUpdate)
	case event.DownloadUpdateEvent:
		fallthrough
	case event.DownloadCompleteEvent:
		fallthrough
	case event.DownloadProgressEvent:
		return nil
	default:
		return errors.New("unknown event type")
	}

	return nil
}

func (service *activityService) scheduleEventBroadcast(resourceKey eventKey, handler broadcastHandler) {
	service._scheduleEventBroadcast(resourceKey, handler, DebounceDuration, MaxTimerDuration)
}

func (service *activityService) scheduleRapidEventBroadcast(resourceKey eventKey, handler broadcastHandler) {
	service._scheduleEventBroadcast(resourceKey, handler, RapidEventDebounceDuration, RapidEventMaxTimerDuration)
}

func (service *activityService) _scheduleEventBroadcast(resourceKey eventKey, handler broadcastHandler, debounceTime time.Duration, maxTime time.Duration) {
	service.Lock()
	defer service.Unlock()

	broadcaster := func() { service.broadcast(resourceKey, handler) }

	// Cancel and re-set a debounce timer
	if t, ok := service.debounceTimers[resourceKey]; ok {
		t.Stop()
	}
	service.debounceTimers[resourceKey] = time.AfterFunc(debounceTime, broadcaster)

	// Set a max timer if not already set
	if _, ok := service.maxTimers[resourceKey]; !ok {
		service.maxTimers[resourceKey] = time.AfterFunc(maxTime, broadcaster)
	}
}

func (service *activityService) broadcast(resourceKey eventKey, handler broadcastHandler) {
	service.Lock()
	defer service.Unlock()

	if t, ok := service.debounceTimers[resourceKey]; ok {
		t.Stop()
		delete(service.debounceTimers, resourceKey)
	}

	if t, ok := service.maxTimers[resourceKey]; ok {
		t.Stop()
		delete(service.maxTimers, resourceKey)
	}

	if err := handler(resourceKey.id); err != nil {
		log.Errorf("Failed to broadcast event %v due to error: %s\n", resourceKey, err)
	}
}
