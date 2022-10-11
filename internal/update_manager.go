package internal

import (
	"github.com/asaskevich/EventBus"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/queue"
)

type UpdateManager interface {
	NotifyItemUpdate(int)
	NotifyQueueUpdate()
	NotifyProfileUpdate()
	NotifyFfmpegUpdate(int, ffmpeg.FfmpegInstance)
	SubmitUpdates()
	EventBus() EventBus.Bus
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
	QueueItem    *queue.QueueItem
	ItemPosition int
	ItemId       int
}

type UpdateManagerSubmitFn func(*Update)

type updateManager struct {
	thea                 Thea
	submitFn             UpdateManagerSubmitFn
	pendingItemUpdates   map[int]bool
	pendingFfmpegUpdates map[int]bool
	eventBus             EventBus.Bus
}

func (mgr *updateManager) NotifyItemUpdate(itemID int) {
	mgr.pendingItemUpdates[itemID] = true
	mgr.eventBus.Publish("update:item")
}

func (mgr *updateManager) NotifyProfileUpdate() {
	mgr.submitFn(&Update{PROFILE_UPDATE, nil})
	mgr.eventBus.Publish("update:profile")

}

func (mgr *updateManager) NotifyQueueUpdate() {
	mgr.submitFn(&Update{QUEUE_UPDATE, nil})
	mgr.eventBus.Publish("update:queue")
}

func (mgr *updateManager) NotifyFfmpegUpdate(itemID int, instance ffmpeg.FfmpegInstance) {
	mgr.pendingFfmpegUpdates[itemID] = true
	mgr.eventBus.Publish("update:ffmpeg")
}

func (mgr *updateManager) SubmitUpdates() {
	if len(mgr.pendingItemUpdates) > 0 {
		// Something has changed, wakeup sleeping workers to detect if any work can be done.
		mgr.thea.workerPool().WakeupWorkers()
	}

	for itemID := range mgr.pendingItemUpdates {
		queueItem, idx := mgr.thea.queue().FindById(itemID)
		if queueItem == nil || idx < 0 {
			mgr.submitFn(&Update{
				UpdateType: ITEM_UPDATE,
				Payload:    &itemUpdate{nil, -1, itemID}})
		} else {
			mgr.submitFn(&Update{
				UpdateType: ITEM_UPDATE,
				Payload:    &itemUpdate{queueItem, idx, itemID},
			})
		}

		delete(mgr.pendingItemUpdates, itemID)
	}

	for itemID := range mgr.pendingFfmpegUpdates {
		instances := mgr.thea.GetFfmpegInstancesForItem(itemID)
		details := make([]ffmpeg.InstanceProgress, len(instances))
		for _, v := range instances {
			// Generate details
			details = append(details, *mgr.thea.ffmpeg().GetLastKnownProgressForInstance(v.Id()))
		}

		mgr.submitFn(&Update{
			UpdateType: FFMPEG_UPDATE,
			Payload:    details,
		})
	}
}

func (mgr *updateManager) EventBus() EventBus.Bus { return mgr.eventBus }

func NewUpdateManager(submitFn UpdateManagerSubmitFn, thea Thea) UpdateManager {
	return &updateManager{
		submitFn:           submitFn,
		thea:               thea,
		pendingItemUpdates: make(map[int]bool),
		eventBus:           EventBus.New(),
	}
}
