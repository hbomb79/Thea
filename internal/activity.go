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
	DEBOUNCE_DURATION  time.Duration = time.Second * 2
	MAX_TIMER_DURATION time.Duration = time.Second * 5

	RAPID_EVENT_DEBOUNCE_DURATION  time.Duration = time.Millisecond * 500
	RAPID_EVENT_MAX_TIMER_DURATION time.Duration = time.Second * 2
)

type (
	broadcastHandler func(uuid.UUID) error

	broadcaster interface {
		BroadcastTranscodeUpdate(uuid.UUID) error
		BroadcastTaskProgressUpdate(uuid.UUID) error
		BroadcastWorkflowUpdate(uuid.UUID) error
		BroadcastMediaUpdate(uuid.UUID) error
		BroadcastIngestUpdate(uuid.UUID) error
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
	messageChan := make(chan event.HandlerEvent, 100)
	service.eventBus.RegisterHandlerChannel(messageChan,
		event.INGEST_UPDATE, event.INGEST_COMPLETE, event.TRANSCODE_UPDATE,
		event.TRANSCODE_TASK_PROGRESS, event.TRANSCODE_COMPLETE, event.WORKFLOW_UPDATE,
		event.DOWNLOAD_UPDATE, event.DOWNLOAD_COMPLETE, event.DOWNLOAD_PROGRESS)

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

	switch ev.Event {
	case event.INGEST_UPDATE:
		fallthrough
	case event.INGEST_COMPLETE:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastIngestUpdate)
	case event.TRANSCODE_UPDATE:
		fallthrough
	case event.TRANSCODE_COMPLETE:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastTranscodeUpdate)
	case event.TRANSCODE_TASK_PROGRESS:
		service.scheduleRapidEventBroadcast(resourceKey, service.BroadcastTaskProgressUpdate)
	case event.WORKFLOW_UPDATE:
		service.scheduleEventBroadcast(resourceKey, service.BroadcastWorkflowUpdate)
	case event.DOWNLOAD_UPDATE:
		fallthrough
	// case event.DOWNLOAD_COMPLETE:
	// 	service.scheduleEventBroadcast(resourceKey, service.BroadcastDownloadUpdate)
	// case event.DOWNLOAD_PROGRESS:
	// 	service.scheduleEventBroadcast(resourceKey, service.BroadcastDownloadProgressUpdate)
	default:
		return errors.New("unknown event type")
	}

	return nil
}

func (service *activityService) scheduleEventBroadcast(resourceKey eventKey, handler broadcastHandler) {
	service._scheduleEventBroadcast(resourceKey, handler, DEBOUNCE_DURATION, MAX_TIMER_DURATION)
}

func (service *activityService) scheduleRapidEventBroadcast(resourceKey eventKey, handler broadcastHandler) {
	service._scheduleEventBroadcast(resourceKey, handler, RAPID_EVENT_DEBOUNCE_DURATION, RAPID_EVENT_MAX_TIMER_DURATION)
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

	handler(resourceKey.id)
}
