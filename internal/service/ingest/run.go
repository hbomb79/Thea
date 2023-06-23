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
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/worker"
	"github.com/rjeczalik/notify"
)

type Scraper interface {
	ScrapeFileForMediaInfo(path string) (*media.FileMediaMetadata, error)
}

type MediaSearcher interface {
	SearchForEpisode(*media.FileMediaMetadata) (*media.Episode, error)
	SearchForMovie(*media.FileMediaMetadata) (*media.Movie, error)
	GetSeason(string, int) (*media.Season, error)
	GetSeries(string) (*media.Series, error)
}

type MediaStore interface {
	GetAllSourcePaths() []string
}

// Config contains configuration options that allow
// customization of how Thea detects files to auto-ingest.
type Config struct {
	// The IngestService uses a directory watcher, but a
	// 'force' sync can be performed on a regular interval
	// to protect against the watcher failing.
	ForceSyncSeconds int

	// The path to the directory the service should monitor
	// for new files
	IngestPath string

	// An array of regular expressions that can be used to RESTRICT
	// the files processed by this service. If any expression match
	// the name of the file, it is ignored.
	Blacklist []string

	// When a new file is detected, it's likely to be an in-progress
	// download using an external software. As we cannot KNOW when the
	// download is complete, we instead wait for the 'modtime' of
	// the item to be at least this long in the past before processing
	RequiredModTimeAgeSeconds int

	// Controls the number of workers that can perform ingestions. Reducing
	// to 1 means one ingestion at a time.
	// Caution should be taken to not increase this value too high, as ingestion
	// involves talking to external APIs which may impose rate limits
	IngestionParallelism int
}

func (config *Config) RequiredModTimeAgeDuration() time.Duration {
	return time.Duration(config.RequiredModTimeAgeSeconds) * time.Second
}

// IngestService is responsible for managing the automatic detection
// and ingestion of files from the servers file system. The detected
// files should be:
// - Checked against a blacklist to ensure they should be processed
// - Run through a metadata scraper to find out as much information as possible
// - Searched for in TMDB using the information we scraped
// - Added to Thea's database, along with any related data
type IngestService struct {
	*sync.Mutex
	Scraper
	Searcher MediaSearcher
	MediaStore

	config           Config
	items            []*IngestItem
	importHoldTimers map[uuid.UUID]*time.Timer
	workerPool       worker.WorkerPool
}

// New creates a new IngestService, using the provided config for
// subsequent calls to 'Start'.
//
// The configs 'IngestPath' is validated to be an existing directory.
// If the directory is missing it will be created, if the path
// provided points to an existing FILE, an error is returned.
func New(config Config) (*IngestService, error) {
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

	service := &IngestService{
		Mutex:            &sync.Mutex{},
		Scraper:          &media.MetadataScraper{},
		Searcher:         tmdb.NewSearcher(tmdb.Config{}),
		MediaStore:       &media.MediaStore{},
		config:           config,
		items:            make([]*IngestItem, 0),
		importHoldTimers: make(map[uuid.UUID]*time.Timer),
		workerPool:       *worker.NewWorkerPool(),
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
func (service *IngestService) Run(ctx context.Context) {
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

// ExecuteTask is the worker function for the IngestService, which is called
// by the services WorkerPool.
// This function will claim the first IDLE item it finds and attempt to ingest it.
// If the ingestion fails with an IngestTrouble, then it will be set on
// the item and it's state set to TROUBLED.
func (service *IngestService) ExecuteTask(w worker.Worker) (bool, error) {
	item := service.claimIdleItem()
	if item == nil {
		return false, nil
	}

	if err := item.ingest(); err != nil {
		if trbl, ok := err.(IngestItemTrouble); ok {
			item.trouble = &trbl
			item.state = TROUBLED
		} else {
			return false, err
		}
	}

	return true, nil
}

// DiscoverNewFiles will scan the host file system at the path
// configured and check for items that need to be ingested (as
// in no database row for these items already exist, and
// no current item in this service represents this path).
// Any paths found that match with any configured blacklists will
// be ignored.
//
// Note: This function will take ownership of the mutex, and releases it when returning
func (service *IngestService) DiscoverNewFiles() {
	service.Lock()
	defer service.Unlock()

	sourcePaths := service.GetAllSourcePaths()
	sourcePathsLookup := make(map[string]bool, len(sourcePaths))
	for _, path := range sourcePaths {
		sourcePathsLookup[path] = true
	}
	for _, item := range service.items {
		sourcePathsLookup[item.path] = true
	}

	newItems, err := recursivelyWalkFileSystem(service.config.IngestPath, sourcePathsLookup)
	if err != nil {
		// Ah! TODO
		panic(err.Error())
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
			id:    itemID,
			path:  itemPath,
			state: itemState,
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
func (service *IngestService) RemoveItem(itemID uuid.UUID) error {
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
func (service *IngestService) Item(itemID uuid.UUID) *IngestItem {
	for _, item := range service.items {
		if item.id == itemID {
			return item
		}
	}

	return nil
}

// AllItems returns a pointer to the array containing all
// the IngestItems being processed by this service.
func (service *IngestService) AllItems() *[]*IngestItem {
	return &service.items
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
func (service *IngestService) evaluateItemHold(id uuid.UUID) {
	service.Lock()
	defer service.Unlock()

	item := service.Item(id)
	if item == nil || item.state != IMPORT_HOLD {
		return
	}

	timeDiff, err := service.Item(id).modtimeDiff()
	if err != nil {
		// Item's source file has gone away!
		service.RemoveItem(id)
		return
	}

	thresholdModTime := service.config.RequiredModTimeAgeDuration()
	if *timeDiff > thresholdModTime {
		service.scheduleImportHoldTimer(id, *timeDiff-thresholdModTime)
		return
	}

	item.state = IDLE
	service.wakeupWorkerPool()
}

// scheduleImportHoldTimer will call evaluateItemHold for the item provided
// after the delay duration specified has elapsed. Any existing import hold timer
// for the item specified will be *cancelled* before the new timer is created.
func (service *IngestService) scheduleImportHoldTimer(id uuid.UUID, delay time.Duration) {
	service.clearImportHoldTimer(id)
	service.importHoldTimers[id] = time.AfterFunc(delay, func() {
		service.evaluateItemHold(id)
	})
}

// clearImportHoldTimer cancels and deletes the import hold timer associatted
// with the item ID specified.
func (service *IngestService) clearImportHoldTimer(id uuid.UUID) {
	if timer, ok := service.importHoldTimers[id]; ok {
		timer.Stop()
		delete(service.importHoldTimers, id)
	}
}

// clearAllImportHoldTimers cancels and deletes the import hold timers for
// all items.
func (service *IngestService) clearAllImportHoldTimers() {
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
func (service *IngestService) claimIdleItem() *IngestItem {
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

func (service *IngestService) wakeupWorkerPool() {
	service.workerPool.WakeupWorkers()
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
		return nil, fmt.Errorf("failed to walk file system: %s", err.Error())
	}

	return foundItems, nil
}
