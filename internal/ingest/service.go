package ingest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/pkg/worker"
	"github.com/rjeczalik/notify"
)

// ingestService is responsible for managing the automatic detection
// and ingestion of files from the servers file system. The detected
// files should be:
// - Checked against a blacklist to ensure they should be processed
// - Run through a metadata scraper to find out as much information as possible
// - Searched for in TMDB using the information we scraped
// - Added to Thea's database, along with any related data
type ingestService struct {
	config           IngestConfig
	items            []*IngestItem
	importHoldTimers map[uuid.UUID]*time.Timer
	workerPool       worker.WorkerPool
	*sync.Mutex
}

// New creates a new IngestService, using the provided config for
// subsequent calls to 'Start'.
//
// The configs 'IngestPath' is validated to be an existing directory.
// If the directory is missing it will be created, if the path
// provided points to an existing FILE, an error is returned.
func New(config IngestConfig) (*ingestService, error) {
	// Ensure config ingest path is a valid directory, create it
	// if it's missing.
	if info, err := os.Stat(config.IngestPath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("ingestion path '%s' is not a directory", config.IngestPath)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(config.IngestPath, os.ModeDir)
	} else {
		return nil, fmt.Errorf("ingestion path '%s' could not be accessed: %s", config.IngestPath, err.Error())
	}

	service := &ingestService{
		config:           config,
		items:            make([]*IngestItem, 0),
		importHoldTimers: make(map[uuid.UUID]*time.Timer),
		workerPool:       *worker.NewWorkerPool(),
		Mutex:            &sync.Mutex{},
	}

	for i := 0; i < config.IngestionParallelism; i++ {
		label := fmt.Sprintf("ingest-worker-%d", i)
		worker := worker.NewWorker(label, service)

		service.workerPool.PushWorker(worker)
	}

	return service, nil
}

// Start is the main entry point of this service. It's responsible
// for listening to the OS file system and responding to change events,
// as well as regularly polling the file system irrespective of the
// watcher (if the configuration used when creating the service
// has enabled this).
// To kill the service, the calling code should cancel the context
// provided.
func (service *ingestService) Start(ctx context.Context) {
	fsNotifyChannel := make(chan notify.EventInfo)
	forceIngestChannel := time.NewTicker(time.Second * time.Duration(service.config.ForceSyncSeconds)).C

	defer service.clearAllImportHoldTimers()

	service.DiscoverNewFiles()

	for {
		select {
		case <-fsNotifyChannel:
			service.DiscoverNewFiles()
		case <-forceIngestChannel:
			service.DiscoverNewFiles()
		case <-ctx.Done():
			return
		}
	}
}

// RemoveItem looks for an item with the ID provided in the services
// state, and removes it if it's found.
// This method *fails* if the item is currently 'INGESTING' as interrupting
// the ingestion is not possible.
// This method does not error if the itemID does not exist.
//
// Note: This function takes ownership of the mutex and releases it on return
func (service *ingestService) RemoveItem(itemID uuid.UUID) error {
	service.Lock()
	defer service.Unlock()

	for k, v := range service.items {
		if v.id == itemID {
			// Remove item from service
			if v.state == INGESTING {
				return fmt.Errorf("cannot remove item %v as a worker is currently ingesting it", itemID)
			}

			service.items = append(service.items[:k], service.items[k+1:]...)
		}
	}

	return nil
}

// Item accepts the ID of an ingest item and attempts to find it
// in the services queue. If it cannot be found, nil is returned.
func (service *ingestService) Item(itemID uuid.UUID) *IngestItem {
	for _, item := range service.items {
		if item.id == itemID {
			return item
		}
	}

	return nil
}

// AllItems returns a pointer to the array containing all
// the IngestItems being processed by this service.
func (service *ingestService) AllItems() *[]*IngestItem {
	return &service.items
}
