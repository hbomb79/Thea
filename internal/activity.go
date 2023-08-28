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
	DEBOUNCE_DURATION  time.Duration = time.Second * 1
	MAX_TIMER_DURATION time.Duration = time.Second * 5
)

type (
	broadcastHandler func(uuid.UUID) error

	broadcaster interface {
		BroadcastTaskUpdate(uuid.UUID) error
		BroadcastTaskProgressUpdate(uuid.UUID) error
		BroadcastWorkflowUpdate(uuid.UUID) error
		BroadcastMediaUpdate(uuid.UUID) error
		BroadcastIngestUpdate(uuid.UUID) error
	}

	activityManager struct {
		*sync.Mutex
		broadcaster
		eventBus       event.EventHandler
		debounceTimers map[uuid.UUID]*time.Timer
		maxTimers      map[uuid.UUID]*time.Timer
	}
)

func newActivityManager(broadcaster broadcaster, event event.EventHandler) *activityManager {
	return &activityManager{
		Mutex:          &sync.Mutex{},
		broadcaster:    broadcaster,
		eventBus:       event,
		debounceTimers: make(map[uuid.UUID]*time.Timer),
		maxTimers:      make(map[uuid.UUID]*time.Timer),
	}
}

func (service *activityManager) Run(ctx context.Context) error {
	messageChan := make(chan event.HandlerEvent, 10)
	service.eventBus.RegisterHandlerChannel(messageChan,
		event.INGEST_UPDATE, event.INGEST_COMPLETE, event.TRANSCODE_UPDATE,
		event.TRANSCODE_TASK_PROGRESS, event.TRANSCODE_COMPLETE, event.WORKFLOW_UPDATE,
		event.DOWNLOAD_UPDATE, event.DOWNLOAD_COMPLETE, event.DOWNLOAD_PROGRESS)

	for {
		select {
		case ev := <-messageChan:
			if err := service.handleEvent(ev); err != nil {
				log.Emit(logger.ERROR, "Handling of event %v failed: %v\n", ev, err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (service *activityManager) handleEvent(ev event.HandlerEvent) error {
	resourceID, ok := ev.Payload.(uuid.UUID)
	if !ok {
		return errors.New("illegal payload (expected UUID)")
	}

	switch ev.Event {
	case event.INGEST_UPDATE:
		fallthrough
	case event.INGEST_COMPLETE:
		service.scheduleEventBroadcast(resourceID, service.BroadcastIngestUpdate)
	case event.TRANSCODE_UPDATE:
		fallthrough
	case event.TRANSCODE_COMPLETE:
		service.scheduleEventBroadcast(resourceID, service.BroadcastTaskUpdate)
	case event.TRANSCODE_TASK_PROGRESS:
		service.scheduleEventBroadcast(resourceID, service.BroadcastTaskProgressUpdate)
	case event.WORKFLOW_UPDATE:
		service.scheduleEventBroadcast(resourceID, service.BroadcastWorkflowUpdate)
	case event.DOWNLOAD_UPDATE:
		fallthrough
	// case event.DOWNLOAD_COMPLETE:
	// 	service.scheduleEventBroadcast(resourceID, service.BroadcastDownloadUpdate)
	// case event.DOWNLOAD_PROGRESS:
	// 	service.scheduleEventBroadcast(resourceID, service.BroadcastDownloadProgressUpdate)
	default:
		return errors.New("unknown event type")
	}

	return nil
}

func (service *activityManager) scheduleEventBroadcast(resourceID uuid.UUID, handler broadcastHandler) {
	service.Lock()
	defer service.Unlock()

	broadcaster := func() { service.broadcast(resourceID, handler) }

	// Cancel and re-set a debounce timer
	if t, ok := service.debounceTimers[resourceID]; ok {
		t.Stop()
	}
	service.debounceTimers[resourceID] = time.AfterFunc(DEBOUNCE_DURATION, broadcaster)

	// Set a max timer if not already set
	if _, ok := service.maxTimers[resourceID]; !ok {
		time.AfterFunc(MAX_TIMER_DURATION, broadcaster)
	}
}

func (service *activityManager) broadcast(resourceID uuid.UUID, handler broadcastHandler) {
	service.Lock()
	defer service.Unlock()

	if t, ok := service.debounceTimers[resourceID]; ok {
		t.Stop()
		delete(service.debounceTimers, resourceID)
	}

	if t, ok := service.maxTimers[resourceID]; ok {
		t.Stop()
		delete(service.maxTimers, resourceID)
	}

	handler(resourceID)
}
