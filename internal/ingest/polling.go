package ingest

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
)

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

	sourcePaths := media.GetAllSourcePaths()
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
