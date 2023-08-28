package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/pkg/worker"
	"github.com/rjeczalik/notify"
)

var log = logger.Get("IngestServ")

type (
	scraper interface {
		ScrapeFileForMediaInfo(path string) (*media.FileMediaMetadata, error)
	}

	searcher interface {
		SearchForSeries(*media.FileMediaMetadata) (*tmdb.Series, error)
		SearchForMovie(*media.FileMediaMetadata) (*tmdb.Movie, error)
		GetSeason(string, int) (*tmdb.Season, error)
		GetSeries(string) (*tmdb.Series, error)
		GetEpisode(string, int, int) (*tmdb.Episode, error)
	}

	DataStore interface {
		GetAllMediaSourcePaths() ([]string, error)
		GetSeasonWithTmdbId(string) (*media.Season, error)
		GetSeriesWithTmdbId(string) (*media.Series, error)
		GetEpisodeWithTmdbId(string) (*media.Episode, error)

		SaveSeries(*media.Series) error
		SaveSeason(*media.Season) error
		SaveEpisode(*media.Episode, *media.Season, *media.Series) error
		SaveMovie(*media.Movie) error
	}

	// ingestService is responsible for managing the automatic detection
	// and ingestion of files from the servers file system. The detected
	// files should be:
	// - Checked against a blacklist to ensure they should be processed
	// - Run through a metadata scraper to find out as much information as possible
	// - Searched for in TMDB using the information we scraped
	// - Added to Thea's database, along with any related data
	ingestService struct {
		*sync.Mutex
		scraper   scraper
		searcher  searcher
		dataStore DataStore
		eventBus  event.EventCoordinator

		config           Config
		items            []*IngestItem
		importHoldTimers map[uuid.UUID]*time.Timer
		workerPool       worker.WorkerPool
	}
)

// New creates a new IngestService, using the provided config for
// subsequent calls to 'Start'.
//
// The configs 'IngestPath' is validated to be an existing directory.
// If the directory is missing it will be created, if the path
// provided points to an existing FILE, an error is returned.
func New(config Config, searcher searcher, scraper scraper, store DataStore, eventBus event.EventCoordinator) (*ingestService, error) {
	// Ensure config ingest path is a valid directory, create it
	// if it's missing.
	if info, err := os.Stat(config.IngestPath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("ingestion path '%s' is not a directory", config.IngestPath)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(config.IngestPath, os.ModeDir|os.ModePerm)
	} else {
		return nil, fmt.Errorf("ingestion path '%s' could not be accessed: %w", config.IngestPath, err)
	}

	service := &ingestService{
		Mutex:            &sync.Mutex{},
		scraper:          scraper,
		searcher:         searcher,
		dataStore:        store,
		config:           config,
		items:            make([]*IngestItem, 0),
		importHoldTimers: make(map[uuid.UUID]*time.Timer),
		workerPool:       *worker.NewWorkerPool(),
		eventBus:         eventBus,
	}

	for i := 0; i < config.IngestionParallelism; i++ {
		label := fmt.Sprintf("ingest-worker-%d", i)
		worker := worker.NewWorker(label, service.PerformItemIngest)

		if err := service.workerPool.PushWorker(worker); err != nil {
			return nil, fmt.Errorf("failed to push worker to pool: %w", err)
		}
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
func (service *ingestService) Run(ctx context.Context) error {
	fsNotifyChannel := make(chan notify.EventInfo)
	forceIngestChannel := time.NewTicker(time.Second * time.Duration(service.config.ForceSyncSeconds)).C

	defer service.clearAllImportHoldTimers()

	if err := service.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to construct worker pool: %w", err)
	}
	defer service.workerPool.Close()

	ev := make(event.HandlerChannel, 10)
	service.eventBus.RegisterHandlerChannel(ev, event.INGEST_COMPLETE)

	service.DiscoverNewFiles()

	for {
		select {
		case <-fsNotifyChannel:
			service.DiscoverNewFiles()
		case <-forceIngestChannel:
			service.DiscoverNewFiles()
		case message := <-ev:
			ev := message.Event
			if ev != event.INGEST_COMPLETE {
				log.Emit(logger.WARNING, "received unknown event %s\n", ev)
				continue
			}

			if injestID, ok := message.Payload.(uuid.UUID); ok {
				log.Emit(logger.DEBUG, "ingest with ID %s has completed - removing\n", injestID)
				service.RemoveIngest(injestID)
			} else {
				log.Emit(logger.ERROR, "failed to extract UUID from %s event (payload %#v)\n", ev, message.Payload)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// PerformItemIngest is the worker function for the IngestService, which is called
// by the services WorkerPool.
// This function will claim the first IDLE item it finds and attempt to ingest it.
// If the ingestion fails with an IngestTrouble, then it will be set on
// the item and it's state set to TROUBLED.
func (service *ingestService) PerformItemIngest(w worker.Worker) (bool, error) {
	item := service.claimIdleItem()
	if item == nil {
		return true, nil
	}

	log.Emit(logger.DEBUG, "Item %s claimed by worker %s for ingestion\n", item, w)
	time.Sleep(time.Second * 1)
	if err := item.ingest(service.eventBus, service.scraper, service.searcher, service.dataStore); err != nil {
		if trbl, ok := err.(IngestItemTrouble); ok {
			item.Trouble = &trbl
			item.State = TROUBLED

			log.Emit(logger.ERROR, "Ingestion of item %s failed due to error %s - Trouble (%s) raised!\n", item, trbl.Error(), trbl.Type)
		} else {
			return false, err
		}
	} else {
		log.Emit(logger.SUCCESS, "Ingestion of item %s complete!\n", item)
		service.eventBus.Dispatch(event.INGEST_COMPLETE, item.Id)
	}

	return false, nil
}

// DiscoverNewFiles will scan the host file system at the path
// configured and check for items that need to be ingested (as
// in no database row for these items already exist, and
// no current item in this service represents this path).
// Any paths found that match with any configured blacklists will
// be ignored.
//
// Note: This function will take ownership of the mutex, and releases it when returning
func (service *ingestService) DiscoverNewFiles() {
	service.Lock()
	defer service.Unlock()

	sourcePaths, err := service.dataStore.GetAllMediaSourcePaths()
	if err != nil {
		log.Fatalf("could not query DB for existing source paths: %v", err)
		return
	}

	sourcePathsLookup := make(map[string]bool, len(sourcePaths))
	for _, path := range sourcePaths {
		sourcePathsLookup[path] = true
	}
	for _, item := range service.items {
		sourcePathsLookup[item.Path] = true
	}

	newItems, err := recursivelyWalkFileSystem(service.config.IngestPath, sourcePathsLookup)
	if err != nil {
		log.Emit(logger.FATAL, "file system polling failed: %v\n", err)
		return
	}

	minModtimeAge := service.config.RequiredModTimeAgeDuration()
	dirty := false
	for itemPath, itemInfo := range newItems {
		itemID := uuid.New()
		timeDiff := time.Since(itemInfo.ModTime())

		itemState := IMPORT_HOLD
		if timeDiff > minModtimeAge {
			dirty = true
			itemState = IDLE
		}

		ingestItem := &IngestItem{
			Id:    itemID,
			Path:  itemPath,
			State: itemState,
		}

		service.items = append(service.items, ingestItem)
		if itemState == IMPORT_HOLD {
			service.scheduleImportHoldTimer(itemID, timeDiff-minModtimeAge)
		}
	}

	if dirty {
		service.wakeupWorkerPool()
	}
}

// RemoveItem looks for an item with the ID provided in the services
// state, and removes it if it's found.
// This method *fails* if the item is currently 'INGESTING' as interrupting
// the ingestion is not possible.
// This method does not error if the itemID does not exist.
//
// Note: This function takes ownership of the mutex and releases it on return
func (service *ingestService) RemoveIngest(itemID uuid.UUID) error {
	service.Lock()
	defer service.Unlock()

	for k, v := range service.items {
		if v.Id == itemID {
			// Remove item from service
			if v.State == INGESTING {
				return fmt.Errorf("cannot remove item %v as a worker is currently ingesting it", itemID)
			}

			service.items = append(service.items[:k], service.items[k+1:]...)
		}
	}

	return nil
}

// Item accepts the ID of an ingest item and attempts to find it
// in the services queue. If it cannot be found, nil is returned.
func (service *ingestService) GetIngest(itemID uuid.UUID) *IngestItem {
	for _, item := range service.items {
		if item.Id == itemID {
			return item
		}
	}

	return nil
}

func (service *ingestService) ResolveTroubledIngest(itemID uuid.UUID, method string, context map[string]string) error {
	item := service.GetIngest(itemID)
	if item == nil {
		return errors.New("ingest with ID provided does not exist")
	}

	return item.ResolveTrouble(method, context)
}

// AllItems returns a pointer to the array containing all
// the IngestItems being processed by this service.
func (service *ingestService) GetAllIngests() []*IngestItem {
	return service.items
}

// evaluateItemHold accepts the ID of an item that is on IMPORT_HOLD,
// and checks it's modtime to see if the item can be moved on to
// the 'IDLE' state.
// If the item with the ID provided no longer exists, the method is a NO-OP.
// If the item exists, but it's source file no longer exists, the item is removed
// from the services state.
// If the item exists and it's source still does not meet modtime requirements, then
// then a new timer will be scheduled to re-evaluate the item hold.
//
// Note: this function takes ownership of the mutex, and releases it when returning
func (service *ingestService) evaluateItemHold(id uuid.UUID) {
	service.Lock()
	defer service.Unlock()

	item := service.GetIngest(id)
	if item == nil || item.State != IMPORT_HOLD {
		return
	}

	timeDiff, err := item.modtimeDiff()
	if err != nil {
		// Item's source file has gone away!
		service.RemoveIngest(id)
		return
	}

	thresholdModTime := service.config.RequiredModTimeAgeDuration()
	if *timeDiff > thresholdModTime {
		service.scheduleImportHoldTimer(id, *timeDiff-thresholdModTime)
		return
	}

	item.State = IDLE
	service.wakeupWorkerPool()
}

// scheduleImportHoldTimer will call evaluateItemHold for the item provided
// after the delay duration specified has elapsed. Any existing import hold timer
// for the item specified will be *cancelled* before the new timer is created.
func (service *ingestService) scheduleImportHoldTimer(id uuid.UUID, delay time.Duration) {
	service.clearImportHoldTimer(id)
	service.importHoldTimers[id] = time.AfterFunc(delay, func() {
		service.evaluateItemHold(id)
	})
}

// clearImportHoldTimer cancels and deletes the import hold timer associatted
// with the item ID specified.
func (service *ingestService) clearImportHoldTimer(id uuid.UUID) {
	if timer, ok := service.importHoldTimers[id]; ok {
		timer.Stop()
		delete(service.importHoldTimers, id)
	}
}

// clearAllImportHoldTimers cancels and deletes the import hold timers for
// all items.
func (service *ingestService) clearAllImportHoldTimers() {
	for key, timer := range service.importHoldTimers {
		timer.Stop()
		delete(service.importHoldTimers, key)
	}
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
		if item.State == IDLE {
			item.State = INGESTING
			return item
		}
	}

	return nil
}

func (service *ingestService) wakeupWorkerPool() {
	if err := service.workerPool.WakeupWorkers(); err != nil {
		log.Warnf("failed to wakeup workers in pool: %v\n", err)
	}
}

// recursivelyWalkFileSystem will walk the file system, starting at the directory provided,
// and construct a map of all the files inside (including any inside of nested directories).
// Files whose paths are included in the 'known' map will NOT be included in the result.
// The key of the returned map is the path, and the value contains the FileInfo
func recursivelyWalkFileSystem(rootDirPath string, known map[string]bool) (map[string]fs.FileInfo, error) {
	foundItems := make(map[string]fs.FileInfo, 0)
	err := filepath.WalkDir(rootDirPath, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dir.IsDir() {
			fileInfo, err := dir.Info()
			if err != nil {
				return err
			}

			if _, ok := known[path]; !ok {
				foundItems[path] = fileInfo
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk file system: %w", err)
	}

	return foundItems, nil
}
