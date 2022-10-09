package internal

import (
	"fmt"

	"github.com/hbomb79/TPA/internal/ffmpeg"
	"github.com/hbomb79/TPA/internal/queue"
)

// QueueService is responsible for exposing methods for reading or mutating
// the state of the TPA queue.
type QueueService interface {
	GetAllItems() *[]*queue.QueueItem
	GetItem(int) (*queue.QueueItem, error)
	ReorderQueue([]int) error
	PromoteItem(int) error
	CancelItem(int) error
	PauseItem(int) error
	ResumeItem(int) error
	AdvanceItem(*queue.QueueItem)
}

type queueService struct {
	tpa TPA
}

// GetAllItems returns all QueueItems currently managed by the queue service
func (service *queueService) GetAllItems() *[]*queue.QueueItem {
	return service.tpa.queue().Items()
}

// GetItem returns the QueueItem with the matching ID, if found
func (service *queueService) GetItem(itemID int) (*queue.QueueItem, error) {
	item, position := service.tpa.queue().FindById(itemID)
	if position == -1 || item == nil {
		return nil, fmt.Errorf("failed to GetItem(%d) -> No item with this ID exists", itemID)
	}
	return item, nil
}

// ReorderList accepts a list of IDs representing the desired ordering,
// and will reorder the internal data to match.
func (service *queueService) ReorderQueue(newOrder []int) error {
	if err := service.tpa.queue().Reorder(newOrder); err != nil {
		return fmt.Errorf("failed to ReorderList(%v) -> %s", newOrder, err.Error())
	}

	return nil
}

// PromoteItem reorders the queue (via ReorderQueue) so that the provided
// ID is at index 0
func (service *queueService) PromoteItem(itemID int) error {
	item, idx := service.tpa.queue().FindById(itemID)
	if item == nil || idx == -1 {
		return fmt.Errorf("failed to PromoteItem(%d) -> No item with this ID exists", itemID)
	} else if idx == 0 {
		return nil
	}

	newOrder := make([]int, 0)
	for _, item := range *service.GetAllItems() {
		newOrder = append(newOrder, item.ItemID)
	}

	if idx == len(newOrder)-1 {
		newOrder = append([]int{newOrder[idx]}, newOrder[:len(newOrder)-1]...)
	} else {
		extracted := append([]int{newOrder[idx]}, newOrder[:idx]...)
		newOrder = append(extracted, newOrder[idx+1:]...)
	}

	if err := service.tpa.queue().Reorder(newOrder); err != nil {
		return fmt.Errorf("failed to PromoteItem(%d) -> %s", itemID, err.Error())
	}

	return nil
}

// CancelItem will cancel the item with the ID provided if it can be found
func (service *queueService) CancelItem(itemID int) error {
	item, pos := service.tpa.queue().FindById(itemID)
	if item == nil || pos == -1 {
		return fmt.Errorf("failed to CancelItem(%d) -> No item with this ID exists", itemID)
	}

	// Notify the queue item that it's been cancelled
	item.Cancel()

	// Cancel any/all ffmpeg instances for this item
	for _, instance := range service.tpa.ffmpeg().GetInstancesForItem(itemID) {
		instance.Stop()
	}

	return nil
}

// PauseItem will pause a specified item if it can be found, and will
// also pause any associatted Ffmpeg instances.
func (service *queueService) PauseItem(itemID int) error {
	item, pos := service.tpa.queue().FindById(itemID)
	if item == nil || pos == -1 {
		return fmt.Errorf("failed to PauseItem(%d) -> No item with this ID exists", itemID)
	}

	item.SetPaused(true)

	instances := service.tpa.ffmpeg().GetInstancesForItem(itemID)
	for _, v := range instances {
		v.SetPaused(true)
	}

	return nil
}

// ResumeItem will resume an items progress by "unpausing" it. If all Ffmpeg Instances are
// paused at the time, they will also be resumed
func (service *queueService) ResumeItem(itemID int) error {
	item, pos := service.tpa.queue().FindById(itemID)
	if item == nil || pos == -1 {
		return fmt.Errorf("failed to ResumeItem(%d) -> No item with this ID exists", itemID)
	} else if item.Status != queue.Paused {
		return fmt.Errorf("failed to ResumeItem(%d) -> Item is not paused", itemID)
	}

	item.SetPaused(false)
	// If all ffmpeg instances were paused then we can somewhat safely assume that unpausing
	// the item means we should unpause all instances too
	instances := service.tpa.ffmpeg().GetInstancesForItem(itemID)
	areAllPaused := func() bool {
		for _, v := range instances {
			if v.Status() != ffmpeg.PAUSED {
				return false
			}
		}
		return true
	}

	if areAllPaused() {
		// Unpause all
		for _, v := range instances {
			v.SetPaused(false)
		}
	}

	return nil
}

func (service *queueService) AdvanceItem(item *queue.QueueItem) {
	service.tpa.queue().AdvanceStage(item)
}

func NewQueueApi(tpa TPA) QueueService {
	return &queueService{
		tpa: tpa,
	}
}
