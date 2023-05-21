package internal

import (
	"sync"

	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/queue"
)

type UpdateManager interface {
	NotifyItemUpdate(int)
	NotifyFfmpegUpdate(int)
	NotifyQueueUpdate()
	NotifyProfileUpdate()
	SubmitUpdates()
	EventHandler() event.EventHandler
}

type processorUpdateType = int

const (
	ITEM_UPDATE processorUpdateType = iota
	QUEUE_UPDATE
	PROFILE_UPDATE
	FFMPEG_UPDATE
)

type Update struct {
	UpdateType processorUpdateType
	Payload    any
}

type itemUpdate struct {
	QueueItem    *queue.Item `json:"item"`
	ItemPosition int         `json:"item_position"`
	ItemId       int         `json:"item_id"`
}

type ffmpegUpdate struct {
	ItemId    int                     `json:"item_id"`
	Instances []ffmpeg.FfmpegInstance `json:"ffmpeg_instances"`
}

type UpdateManagerSubmitFn func(*Update)

type updateManager struct {
	thea                 Thea
	submitFn             UpdateManagerSubmitFn
	pendingItemUpdates   map[int]bool
	pendingFfmpegUpdates map[int]bool
	eventCoordinator     event.EventCoordinator
	mutex                sync.Mutex
}

func (mgr *updateManager) NotifyItemUpdate(itemID int) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	mgr.pendingItemUpdates[itemID] = true
	mgr.eventCoordinator.Dispatch(event.ITEM_UPDATE_EVENT, itemID)
}

func (mgr *updateManager) NotifyProfileUpdate() {
	mgr.submitFn(&Update{PROFILE_UPDATE, nil})
	mgr.eventCoordinator.Dispatch(event.PROFILE_UPDATE_EVENT, nil)
}

func (mgr *updateManager) NotifyQueueUpdate() {
	mgr.submitFn(&Update{QUEUE_UPDATE, nil})
	mgr.eventCoordinator.Dispatch(event.QUEUE_UPDATE_EVENT, nil)
}

func (mgr *updateManager) NotifyFfmpegUpdate(itemID int) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	mgr.pendingFfmpegUpdates[itemID] = true
	// mgr.eventCoordinator.Dispatch(event.ITEM_FFMPEG_UPDATE_EVENT, itemID)
}

func (mgr *updateManager) SubmitUpdates() {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	if len(mgr.pendingItemUpdates) > 0 {
		// Something has changed, wakeup sleeping workers to detect if any work can be done.
		mgr.thea.workerPool().WakeupWorkers()
	}

	for itemID := range mgr.pendingItemUpdates {
		queueItem, idx := mgr.thea.queue().FindById(itemID)
		var payload *itemUpdate
		if queueItem == nil || idx < 0 {
			payload = &itemUpdate{nil, -1, itemID}
		} else {
			payload = &itemUpdate{queueItem, idx, itemID}
		}

		mgr.submitFn(&Update{
			UpdateType: ITEM_UPDATE,
			Payload:    payload,
		})

		delete(mgr.pendingItemUpdates, itemID)
	}

	for itemID := range mgr.pendingFfmpegUpdates {
		instances := mgr.thea.GetFfmpegInstancesForItem(itemID)

		mgr.submitFn(&Update{
			UpdateType: FFMPEG_UPDATE,
			Payload:    &ffmpegUpdate{itemID, instances},
		})

		delete(mgr.pendingFfmpegUpdates, itemID)
	}
}

func (mgr *updateManager) EventHandler() event.EventHandler { return mgr.eventCoordinator }

func NewUpdateManager(submitFn UpdateManagerSubmitFn, thea Thea) UpdateManager {
	return &updateManager{
		submitFn:             submitFn,
		thea:                 thea,
		pendingItemUpdates:   make(map[int]bool),
		pendingFfmpegUpdates: make(map[int]bool),
		mutex:                sync.Mutex{},
		eventCoordinator:     event.NewEventHandler(),
	}
}
