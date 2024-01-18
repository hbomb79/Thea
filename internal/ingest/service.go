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
		SearchForSeries(metadata *media.FileMediaMetadata) (string, error)
		SearchForMovie(metadata *media.FileMediaMetadata) (string, error)
		GetSeason(seriesID string, seasonNumber int) (*tmdb.Season, error)
		GetSeries(seriesID string) (*tmdb.Series, error)
		GetEpisode(seriesID string, seasonNumber int, episodeNumber int) (*tmdb.Episode, error)
		GetMovie(movieID string) (*tmdb.Movie, error)
	}

	DataStore interface {
		GetAllMediaSourcePaths() ([]string, error)
		GetSeasonWithTmdbID(seasonID string) (*media.Season, error)
		GetSeriesWithTmdbID(seriesID string) (*media.Series, error)
		GetEpisodeWithTmdbID(episodeID string) (*media.Episode, error)

		SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error
		SaveMovie(movie *media.Movie) error
	}

	// ingestService is responsible for managing the automatic detection
	// and ingestion of files from the servers file system. The detected
	// files should be:
	// - Checked against a blacklist to ensure they should be processed
	// - Run through a metadata scraper to find out as much information as possible
	// - Searched for in TMDB using the information we scraped
	// - Added to Thea's database, along with any related data.
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
	ingestionPath := config.GetIngestPath()
	if info, err := os.Stat(ingestionPath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("ingestion path '%s' is not a directory", ingestionPath)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(ingestionPath, os.ModeDir|os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	} else {
		return nil, fmt.Errorf("ingestion path '%s' could not be accessed: %w", ingestionPath, err)
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

	handlerChannelSize := 100
	ev := make(event.HandlerChannel, handlerChannelSize)
	service.eventBus.RegisterHandlerChannel(ev, event.IngestCompleteEvent)

	service.DiscoverNewFiles()

	for {
		select {
		case <-fsNotifyChannel:
			service.DiscoverNewFiles()
		case <-forceIngestChannel:
			service.DiscoverNewFiles()
		case message := <-ev:
			ev := message.Event
			if ev != event.IngestCompleteEvent {
				log.Emit(logger.WARNING, "received unknown event %s\n", ev)
				continue
			}

			if injestID, ok := message.Payload.(uuid.UUID); ok {
				log.Emit(logger.DEBUG, "ingest with ID %s has completed - removing\n", injestID)
				if err := service.RemoveIngest(injestID); err != nil {
					log.Errorf("Unable to remove ingest (id: %s): %s\n", injestID, err)
				}
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
	service.eventBus.Dispatch(event.IngestUpdateEvent, item.ID)

	if err := item.ingest(service.eventBus, service.scraper, service.searcher, service.dataStore); err != nil {
		service.eventBus.Dispatch(event.IngestUpdateEvent, item.ID)
		//nolint
		if trbl, ok := err.(Trouble); ok {
			item.Trouble = &trbl
			item.State = Troubled

			log.Emit(logger.ERROR, "Ingestion of item %s failed, raising trouble {message='%s' type=%s}\n", item, item.Trouble, item.Trouble.Type())
		} else {
			log.Emit(logger.FATAL, "Ingestion of item %s returned an unexpected error (%#v) (not a trouble)! Worker will crash\n", item, err)
			return false, err
		}
	} else {
		log.Emit(logger.SUCCESS, "Ingestion of item %s complete!\n", item)
		item.State = Complete
		service.eventBus.Dispatch(event.IngestCompleteEvent, item.ID)
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
// Note: This function will take ownership of the mutex, and releases it when returning.
func (service *ingestService) DiscoverNewFiles() {
	service.Lock()
	defer service.Unlock()

	sourcePaths, err := service.dataStore.GetAllMediaSourcePaths()
	if err != nil {
		log.Fatalf("Could not query DB for existing source paths: %v\n", err) //nolint
		return
	}

	sourcePathsLookup := make(map[string]bool, len(sourcePaths))
	for _, path := range sourcePaths {
		sourcePathsLookup[path] = true
	}
	for _, item := range service.items {
		sourcePathsLookup[item.Path] = true
	}

	newItems, err := recursivelyWalkFileSystem(service.config.GetIngestPath(), sourcePathsLookup)
	if err != nil {
		log.Emit(logger.FATAL, "file system polling failed: %v\n", err)
		return
	}

	minModtimeAge := service.config.RequiredModTimeAgeDuration()
	dirty := false
	for itemPath, itemInfo := range newItems {
		itemID := uuid.New()
		timeDiff := time.Since(itemInfo.ModTime())

		itemState := ImportHold
		if timeDiff > minModtimeAge {
			dirty = true
			itemState = Idle
		}

		ingestItem := &IngestItem{
			ID:    itemID,
			Path:  itemPath,
			State: itemState,
		}

		service.items = append(service.items, ingestItem)
		if itemState == ImportHold {
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
// Note: This function takes ownership of the mutex and releases it on return.
func (service *ingestService) RemoveIngest(itemID uuid.UUID) error {
	service.Lock()
	defer service.Unlock()

	return service.removeIngest(itemID)
}

func (service *ingestService) removeIngest(itemID uuid.UUID) error {
	for k, v := range service.items {
		if v.ID == itemID {
			// Remove item from service
			if v.State == Ingesting {
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
		if item.ID == itemID {
			return item
		}
	}

	return nil
}

func (service *ingestService) ResolveTroubledIngest(itemID uuid.UUID, method ResolutionType, context map[string]string) error {
	service.Lock()
	defer service.Unlock()

	item := service.GetIngest(itemID)
	if item == nil {
		return ErrIngestNotFound
	}

	if item.Trouble == nil || item.State != Troubled {
		return ErrNoTrouble
	}

	res, err := item.Trouble.GenerateResolution(method, context)
	if res == nil || err != nil {
		return fmt.Errorf("failed to resolve with method %v: %w", method, err)
	}

	switch v := res.(type) {
	case *AbortResolution:
		if err := service.removeIngest(item.ID); err != nil {
			return err
		}
	case *RetryResolution:
		item.State = Idle
		item.Trouble = nil
		// An item has been updated, so we need to inform the service to check for work to be done
		service.wakeupWorkerPool()
	case *TmdbIDResolution:
		item.State = Idle
		item.Trouble = nil
		item.OverrideTmdbID = &v.tmdbID
		// An item has been updated, so we need to inform the service to check for work to be done
		service.wakeupWorkerPool()
	default:
		return fmt.Errorf("trouble resolution type of %T was not expected. This is likely a bug/should be unreachable", res)
	}

	service.eventBus.Dispatch(event.IngestUpdateEvent, item.ID)
	return nil
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
// a new timer will be scheduled to re-evaluate the item hold.
//
// Note: this function takes ownership of the mutex, and releases it when returning.
func (service *ingestService) evaluateItemHold(id uuid.UUID) {
	service.Lock()
	defer service.Unlock()

	item := service.GetIngest(id)
	if item == nil || item.State != ImportHold {
		return
	}

	timeDiff, err := item.modtimeDiff()
	if err != nil {
		// Item's source file has gone away!
		_ = service.RemoveIngest(id)
		return
	}

	thresholdModTime := service.config.RequiredModTimeAgeDuration()
	if *timeDiff > thresholdModTime {
		service.scheduleImportHoldTimer(id, *timeDiff-thresholdModTime)
		return
	}

	item.State = Idle
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
// Note: This function takes ownership of the mutex, and releases it when returning.
func (service *ingestService) claimIdleItem() *IngestItem {
	service.Lock()
	defer service.Unlock()

	for _, item := range service.items {
		if item.State == Idle {
			item.State = Ingesting
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
// The key of the returned map is the path, and the value contains the FileInfo.
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
