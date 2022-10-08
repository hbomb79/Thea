package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"

	"github.com/hbomb79/TPA/internal/dockerService"
	"github.com/hbomb79/TPA/pkg/logger"
	"gorm.io/gorm"
)

var itemLogger = logger.Get("QueueItem")

func init() {
	dockerService.DB.RegisterModel(&ExportedItem{}, &ExportDetail{}, &Series{}, &Genre{})
}

// Each stage represents a certain stage in the pipeline
type QueueItemStage int

// When a QueueItem is initially added, it should be of stage Import,
// each time a worker works on the task it should increment it's
// Stage (Title->Omdb->etc..) and set it's Status to 'Pending'
// to allow a worker to pick the item from the Queue
// TODO Really.. worker should have no concept of pipeline stages
// as it's only relevant in this package. We could further de-couple
// this codebase by waking up workers based on their label, rather
// than the worker.PipelineStage enum
const (
	Import QueueItemStage = iota
	Title
	Omdb
	Format
	Database
	Finish
)

// The current status of this QueueItem
type QueueItemStatus int

const (
	// Inidicates that this item is waiting to be worked on. It's currently idle
	Pending QueueItemStatus = iota

	// TPA is making progress on this item
	Processing

	// This item has been completed
	Completed

	// Inidcates that work on this item has HALTED due to an error. The user
	// should inspect the 'Trouble' parameter to resolve this issue
	NeedsResolving

	// The user has marked this item for cancellation, but TPA is performing
	// a non-interuptable task. Once TPA is complete, it will notice this
	// Status and move the item to 'Cancelled'
	Cancelling

	// This item has been cancelled and should be removed from the processor queue
	Cancelled

	// When Paused, TPA (both workers and the commander) will ignore this item entirely
	Paused

	// Indicates to the user that this item is experiencing a problem *but* it's not
	// interfering with all progress. The user should still work to solve this problem
	// as a 'NeedsAttention' status may be increased to 'NeedsResolving' if TPA detects
	// that work can no longer be made while the problem remains
	NeedsAttention
)

// QueueItem contains all the information needed to fully
// encapsulate the state of each item in the formatting queue.
// This includes information found from the file-system, OMDB,
// and the current processing status/stage
type QueueItem struct {
	gorm.Model
	ItemID     int             `json:"id" groups:"api" gorm:"-"`
	Path       string          `json:"path"`
	Name       string          `json:"name" groups:"api"`
	Status     QueueItemStatus `json:"status" groups:"api" gorm:"-"`
	Stage      QueueItemStage  `json:"stage" groups:"api" gorm:"-"`
	TitleInfo  *TitleInfo      `json:"title_info"`
	OmdbInfo   *OmdbInfo       `json:"omdb_info"`
	Trouble    Trouble         `json:"trouble" gorm:"-"`
	ProfileTag string          `json:"profile_tag" gorm:"-"`
	tpa        TPA             `json:"-" gorm:"-"`
}

func NewQueueItem(info fs.FileInfo, path string, tpa TPA) *QueueItem {
	return &QueueItem{
		Name:   info.Name(),
		Path:   path,
		Status: Pending,
		Stage:  Import,
		tpa:    tpa,
	}
}

func (item *QueueItem) SetStage(stage QueueItemStage) {
	item.Stage = stage

	item.NotifyUpdate()
}

func (item *QueueItem) SetStatus(status QueueItemStatus) {
	if item.Status == status {
		return
	}

	item.Status = status
	if item.Status == Cancelled {
		// Item has been cancelled and has wrapped up what it was doing
		// Remove this item from the queue, mark it in the queue cache
		// so we don't re-ingest it later
		item.tpa.Queue().Remove(item)
	}

	item.NotifyUpdate()
}

// SetTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *QueueItem) SetTrouble(trouble Trouble) {
	defer item.NotifyUpdate()

	itemLogger.Emit(logger.WARNING, "Raising trouble %T for QueueItem %s\n", trouble, item)
	item.Trouble = trouble

	// If the item is cancelled/cancelling, we don't want to override that status
	// with 'NeedsResolving'.
	if item.Status != Cancelling && item.Status != Cancelled {
		item.SetStatus(NeedsResolving)
	}
}

// ClearTrouble is used to remove the trouble state from
// this item and notify the procesor of this change
func (item *QueueItem) ClearTrouble() {
	if item.Trouble == nil {
		return
	}

	item.Trouble = nil
	item.NotifyUpdate()
}

// FormatTitle accepts a string (title) and reformats it
// based on text-filtering configuration provided by
// the user
func (item *QueueItem) FormatTitle() error {
	normaliserMatcher := regexp.MustCompile(`(?i)[\.\s]`)
	seasonMatcher := regexp.MustCompile(`(?i)^(.*?)\_?s(\d+)\_?e(\d+)\_*((?:20|19)\d{2})?`)
	movieMatcher := regexp.MustCompile(`(?i)^(.+?)\_*((?:20|19)\d{2})`)
	resolutionMatcher := regexp.MustCompile(`(?i)(\d{3,4}p)|(\dk)`)

	title := normaliserMatcher.ReplaceAllString(item.Name, "_")
	resolution := resolutionMatcher.FindString(item.Name)

	// Search for season info and optional year information
	if seasonGroups := seasonMatcher.FindStringSubmatch(title); len(seasonGroups) >= 1 {
		item.TitleInfo = &TitleInfo{
			Episodic:   true,
			Title:      seasonGroups[1],
			Season:     convertToInt(seasonGroups[2]),
			Episode:    convertToInt(seasonGroups[3]),
			Year:       convertToInt(seasonGroups[4]),
			Resolution: resolution,
		}

		return nil
	}

	// Try find if it's a movie instead
	if movieGroups := movieMatcher.FindStringSubmatch(title); len(movieGroups) >= 1 {
		item.TitleInfo = &TitleInfo{
			Episodic:   false,
			Season:     -1,
			Episode:    -1,
			Title:      movieGroups[1],
			Year:       convertToInt(movieGroups[2]),
			Resolution: resolution,
		}

		return nil
	}

	// Didn't match either case; return error so that trouble
	// can be raised by the worker.
	return TitleFormatError{item, "Failed to match RegExp!"}
}

// ValidateProfileSuitable accepts a profile and will check it's match conditions, and potentially
// other criteria, to asertain if the profile should be used when transcoding this items content
// via the FFmpeg commander.
func (item *QueueItem) ValidateProfileSuitable(pr Profile) bool {
	matchConds := pr.MatchConditions()

	// Check that this item matches the the conditions specified by the profile. If there
	// are no conditions, we assume this profile has none and will return true
	if len(matchConds) == 0 {
		return true
	}

	currentEval := true
	for _, condition := range matchConds {
		var v interface{}

		switch condition.Key {
		case TITLE:
			v = item.TitleInfo.Title
		case RESOLUTION:
			v = item.TitleInfo.Resolution
		case EPISODE_NUMBER:
			if item.TitleInfo.Episodic && item.TitleInfo.Episode != -1 {
				v = item.TitleInfo.Episode
			} else {
				v = nil
			}
		case SEASON_NUMBER:
			if item.TitleInfo.Episodic && item.TitleInfo.Season != -1 {
				v = item.TitleInfo.Season
			} else {
				v = nil
			}
		case SOURCE_EXTENSION:
			v = item.Path
		case SOURCE_NAME:
			v = item.Name
		case SOURCE_PATH:
			v = item.Path
		}

		isMatch, err := condition.IsMatch(v)
		if err != nil {
			itemLogger.Emit(logger.ERROR, "FAILED to validate if profile (%s) match condition (%v) is suitable for item %s because: %v\n", pr, condition, item, err.Error())
		}

		if currentEval {
			currentEval = isMatch
		}

		if condition.Modifier == OR {
			// End of this block
			if currentEval {
				return true
			} else {
				currentEval = true
			}
		}
	}

	return currentEval
}

// Cancel will cancel an item that is currently pending by setting it's status to cancelled.
// If the item is currently in progress, it's command context will be cancelled, and it's
// status will be set to Cancelling. Once the running task finishes, the items
// state will be updated to Cancelled.
func (item *QueueItem) Cancel() error {
	switch item.Status {
	case Cancelled:
	case Cancelling:
		return errors.New("cannot cancel item because it's already cancelled")
	case Pending:
	case NeedsResolving:
		item.SetStatus(Cancelled)
	case Completed:
		return errors.New("cannot cancel item as it's already completed")
	case Processing:
		item.SetStatus(Cancelling)
	}

	return nil
}

func (item *QueueItem) CommitToDatabase() error {
	db := dockerService.DB.GetInstance()

	// Compose optional/nil-able fields of the export
	var episodeNumber *int = nil
	var seasonNumber *int = nil
	var series *Series = nil
	if item.TitleInfo.Episodic {
		if item.TitleInfo.Episode > -1 {
			episodeNumber = &item.TitleInfo.Episode
		}

		if item.TitleInfo.Season > -1 {
			seasonNumber = &item.TitleInfo.Season
		}

		series = &Series{
			Name: item.OmdbInfo.Title,
		}
	}

	// Construct exports based on the completed ffmpeg instances
	exports := make([]*ExportDetail, 0)
	for _, instance := range item.tpa.Ffmpeg().GetInstancesForItem(item.ItemID) {
		exports = append(exports, &ExportDetail{
			Name: instance.ProfileTag(),
			Path: instance.GetOutputPath(),
		})
	}

	// Compose our export item
	export := &ExportedItem{
		Name:          item.OmdbInfo.Title, //TODO Potentially we want to find titles of episodes (if episodic) via OMDB? Would require altering the title parser too
		Description:   item.OmdbInfo.Description,
		Runtime:       item.OmdbInfo.Runtime, //TODO Perhaps we can derive this from the actual exported file (all exports should have similar length... right?)
		ReleaseYear:   item.OmdbInfo.ReleaseYear,
		Image:         item.OmdbInfo.PosterUrl,
		Genres:        item.OmdbInfo.Genre.ToGenreList(),
		Exports:       exports,
		EpisodeNumber: episodeNumber,
		SeasonNumber:  seasonNumber,
		Series:        series,
	}

	if err := db.Debug().Save(export).Error; err != nil {
		return fmt.Errorf("failed to commit item %s to database: %s", item, err.Error())
	}

	return nil
}

func (item *QueueItem) SetPaused(paused bool) error {
	if (paused && item.Status == Paused) ||
		(!paused && item.Status != Paused) {
		return nil
	}

	// (Un)Pause any ffmpeg instances for this item
	for _, ffmpegInstance := range item.tpa.Ffmpeg().GetInstancesForItem(item.ItemID) {
		ffmpegInstance.SetPaused(paused)
	}

	if paused {
		item.Status = Paused
	} else {
		item.Status = Pending
	}

	return nil
}

func (item *QueueItem) NotifyUpdate() {
	item.tpa.Updates().NotifyItemUpdate(item.ItemID)
}

func (item *QueueItem) String() string {
	return fmt.Sprintf("{%d PK=%d name=%s}", item.ItemID, item.ID, item.Name)
}

func (item *QueueItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ItemID          int             `json:"id"`
		Path            string          `json:"path"`
		Name            string          `json:"name"`
		Status          QueueItemStatus `json:"status"`
		Stage           QueueItemStage  `json:"stage"`
		TitleInfo       *TitleInfo      `json:"title_info"`
		OmdbInfo        *OmdbInfo       `json:"omdb_info"`
		Trouble         Trouble         `json:"trouble"`
		ProfileTag      string          `json:"profile_tag"`
		FfmpegInstances []CommanderTask `json:"ffmpeg_instances"`
	}{
		item.ItemID,
		item.Path,
		item.Name,
		item.Status,
		item.Stage,
		item.TitleInfo,
		item.OmdbInfo,
		item.Trouble,
		item.ProfileTag,
		item.tpa.Ffmpeg().GetInstancesForItem(item.ItemID),
	})
}
