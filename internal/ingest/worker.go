package ingest

import "github.com/hbomb79/Thea/pkg/worker"

// ExecuteTask is the worker function for the IngestService, which is called
// by the services WorkerPool.
// This function will claim the first IDLE item it finds and attempt to ingest it.
// If the ingestion fails with an IngestTrouble, then it will be set on
// the item and it's state set to TROUBLED.
func (service *ingestService) ExecuteTask(w worker.Worker) (bool, error) {
	item := service.claimIdleItem()
	if item == nil {
		return false, nil
	}

	if err := item.ingest(); err != nil {
		if trbl, ok := err.(IngestTrouble); ok {
			item.trouble = &trbl
			item.state = TROUBLED
		} else {
			return false, err
		}
	}

	return true, nil
}

// claimIdleItem will try and find an IDLE item in the ingest service,
// and set it's state to 'INGESTING' to prevent another
// worker from claiming it once the mutex lock is released.
//
// Note: This function takes ownership of the mutex, and releases it when returning
func (service *ingestService) claimIdleItem() *IngestItem {
	service.Lock()
	defer service.Unlock()

	for _, item := range service.items {
		if item.state == IDLE {
			item.state = INGESTING
			return item
		}
	}

	return nil
}

func (service *ingestService) wakeupWorkerPool() {
	service.workerPool.WakeupWorkers()
}
