package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"regexp"

	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/pkg/logger"
	"gorm.io/gorm"
)

var itemLogger = logger.Get("QueueItem")

// Each stage represents a certain stage in the pipeline
type ItemStage int

// When a QueueItem is initially added, it should be of stage Import,
// each time a worker works on the task it should increment it's
// Stage (Title->Omdb->etc..) and set it's Status to 'Pending'
// to allow a worker to pick the item from the Queue
// TODO Really.. worker should have no concept of pipeline stages
// as it's only relevant in this package. We could further de-couple
// this codebase by waking up workers based on their label, rather
// than the worker.PipelineStage enum
const (
	Import ItemStage = iota
	Title
	Omdb
	Format
	Database
	Finish
)

// The current status of this QueueItem
type ItemStatus int

const (
	// Inidicates that this item is waiting to be worked on. It's currently idle
	Pending ItemStatus = iota

	// Thea is making progress on this item
	Processing

	// This item has been completed
	Completed

	// Inidcates that work on this item has HALTED due to an error. The user
	// should inspect the 'Trouble' parameter to resolve this issue
	NeedsResolving

	// The user has marked this item for cancellation, but Thea is performing
	// a non-interuptable task. Once Thea is complete, it will notice this
	// Status and move the item to 'Cancelled'
	Cancelling

	// This item has been cancelled and should be removed from the processor queue
	Cancelled

	// When Paused, Thea (both workers and the commander) will ignore this item entirely
	Paused

	// Indicates to the user that this item is experiencing a problem *but* it's not
	// interfering with all progress. The user should still work to solve this problem
	// as a 'NeedsAttention' status may be increased to 'NeedsResolving' if Thea detects
	// that work can no longer be made while the problem remains
	NeedsAttention
)

// Item contains all the information needed to fully
// encapsulate the state of each item in the formatting queue.
// This includes information found from the file-system, OMDB,
// and the current processing status/stage
type Item struct {
	gorm.Model
	ItemID           int              `json:"id" groups:"api" gorm:"-"`
	Path             string           `json:"path"`
	Name             string           `json:"name" groups:"api"`
	Status           ItemStatus       `json:"status" groups:"api" gorm:"-"`
	StatusText       string           `json:"statusText" groups:"api" gorm:"-"`
	Stage            ItemStage        `json:"stage" groups:"api" gorm:"-"`
	TitleInfo        *TitleInfo       `json:"title_info"`
	OmdbInfo         *OmdbInfo        `json:"omdb_info"`
	Trouble          Trouble          `json:"trouble" gorm:"-"`
	ProfileTag       string           `json:"profile_tag" gorm:"-"`
	changeSubscriber ChangeSubscriber `json:"-" gorm:"-"`
}

type ChangeSubscriber interface {
	NotifyItemUpdate(int)
}

func NewQueueItem(info fs.FileInfo, path string, changeSubscriber ChangeSubscriber) *Item {
	return &Item{
		Name:             info.Name(),
		Path:             path,
		Status:           Pending,
		Stage:            Import,
		changeSubscriber: changeSubscriber,
	}
}

func (item *Item) SetStage(stage ItemStage) {
	item.Stage = stage

	item.NotifyUpdate()
}

func (item *Item) SetStatus(status ItemStatus) {
	if item.Status == status {
		return
	}

	item.Status = status
	item.StatusText = ""
	item.NotifyUpdate()
}

func (item *Item) SetStatusWithMessage(status ItemStatus, message string) {
	if item.Status == status && item.StatusText == message {
		return
	}

	item.Status = status
	item.StatusText = message
	item.NotifyUpdate()
}

// SetTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *Item) SetTrouble(trouble Trouble) {
	if trouble == nil {
		itemLogger.Emit(logger.WARNING, "Ignoring QueueItem#SetTrouble as the trouble provided is 'nil'!\n")
		return
	} else if reflect.TypeOf(item.Trouble) == reflect.TypeOf(trouble) {
		// Raising an error of the same type is not allowed!
		itemLogger.Emit(logger.DEBUG, "Ignoring QueueItem#SetTrouble as the trouble provided has the same type as thee existing trouble set on this item (%s)\n", reflect.TypeOf(trouble))
		return
	}

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
func (item *Item) ClearTrouble() {
	if item.Trouble == nil {
		return
	}

	item.Trouble = nil
	item.NotifyUpdate()
}

// FormatTitle accepts a string (title) and reformats it
// based on text-filtering configuration provided by
// the user
func (item *Item) FormatTitle() error {
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
	return fmt.Errorf("failed to parse title, failed to match on either regular expressions")
}

// ValidateProfileSuitable accepts a profile and will check it's match conditions, and potentially
// other criteria, to asertain if the profile should be used when transcoding this items content
// via the FFmpeg commander.
func (item *Item) ValidateProfileSuitable(pr profile.Profile) bool {
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
		case profile.TITLE:
			v = item.TitleInfo.Title
		case profile.RESOLUTION:
			v = item.TitleInfo.Resolution
		case profile.EPISODE_NUMBER:
			if item.TitleInfo.Episodic && item.TitleInfo.Episode != -1 {
				v = item.TitleInfo.Episode
			} else {
				v = nil
			}
		case profile.SEASON_NUMBER:
			if item.TitleInfo.Episodic && item.TitleInfo.Season != -1 {
				v = item.TitleInfo.Season
			} else {
				v = nil
			}
		case profile.SOURCE_EXTENSION:
			v = item.Path
		case profile.SOURCE_NAME:
			v = item.Name
		case profile.SOURCE_PATH:
			v = item.Path
		}

		isMatch, err := condition.IsMatch(v)
		if err != nil {
			itemLogger.Emit(logger.ERROR, "FAILED to validate if profile (%s) match condition (%v) is suitable for item %s because: %v\n", pr, condition, item, err.Error())
		}

		if currentEval {
			currentEval = isMatch
		}

		if condition.Modifier == profile.OR {
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
func (item *Item) Cancel() error {
	itemLogger.Emit(logger.WARNING, "queue.Item#Cancel is DEPRECATED - prefer QueueService#cancelItem")
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

func (item *Item) SetPaused(paused bool) error {
	if (paused && item.Status == Paused) ||
		(!paused && item.Status != Paused) {
		return nil
	}

	if paused {
		item.Status = Paused
	} else {
		item.Status = Pending
	}

	return nil
}

func (item *Item) NotifyUpdate() {
	item.changeSubscriber.NotifyItemUpdate(item.ItemID)
}

func (item *Item) String() string {
	return fmt.Sprintf("{%d PK=%d name=%s}", item.ItemID, item.ID, item.Name)
}

func (item *Item) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ItemID    int        `json:"id"`
		Path      string     `json:"path"`
		Name      string     `json:"name"`
		Status    ItemStatus `json:"status"`
		Stage     ItemStage  `json:"stage"`
		TitleInfo *TitleInfo `json:"title_info"`
		OmdbInfo  *OmdbInfo  `json:"omdb_info"`
		Trouble   Trouble    `json:"trouble"`
	}{
		item.ItemID,
		item.Path,
		item.Name,
		item.Status,
		item.Stage,
		item.TitleInfo,
		item.OmdbInfo,
		item.Trouble,
	})
}
