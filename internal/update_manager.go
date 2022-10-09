package internal

import "github.com/hbomb79/TPA/internal/queue"

type UpdateManager interface {
	NotifyItemUpdate(int)
	NotifyQueueUpdate()
	NotifyProfileUpdate()
	SubmitUpdates()
}

type processorUpdateType = int

const (
	ITEM_UPDATE processorUpdateType = iota
	QUEUE_UPDATE
	PROFILE_UPDATE
)

type Update struct {
	UpdateType   processorUpdateType
	QueueItem    *queue.QueueItem
	ItemPosition int
	ItemId       int
}

type UpdateManagerSubmitFn func(*Update)

type updateManager struct {
	tpa            TPA
	submitFn       UpdateManagerSubmitFn
	pendingUpdates map[int]bool
}

func (mgr *updateManager) NotifyItemUpdate(itemID int) {
	mgr.pendingUpdates[itemID] = true
}

func (mgr *updateManager) NotifyProfileUpdate() {
	mgr.submitFn(&Update{PROFILE_UPDATE, nil, -1, -1})
}

func (mgr *updateManager) NotifyQueueUpdate() {
	mgr.submitFn(&Update{QUEUE_UPDATE, nil, -1, -1})
}

func (mgr *updateManager) SubmitUpdates() {
	for itemID := range mgr.pendingUpdates {
		queueItem, idx := mgr.tpa.queue().FindById(itemID)
		if queueItem == nil || idx < 0 {
			mgr.submitFn(&Update{
				UpdateType:   ITEM_UPDATE,
				QueueItem:    nil,
				ItemPosition: -1,
				ItemId:       itemID})
		} else {
			mgr.submitFn(&Update{
				UpdateType:   ITEM_UPDATE,
				QueueItem:    queueItem,
				ItemPosition: idx,
				ItemId:       itemID,
			})
		}

		delete(mgr.pendingUpdates, itemID)
	}
}

func NewUpdateManager(submitFn UpdateManagerSubmitFn, tpa TPA) UpdateManager {
	return &updateManager{
		submitFn: submitFn,
		tpa:      tpa,
	}
}
