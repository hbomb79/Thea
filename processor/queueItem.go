package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hbomb79/TPA/pkg"
	"github.com/hbomb79/TPA/profile"
	"github.com/hbomb79/TPA/worker"
	"gorm.io/gorm"
)

var itemLogger = pkg.Log.GetLogger("QueueItem", pkg.CORE)

func init() {
	pkg.DB.RegisterModel(&QueueItem{}, &TitleInfo{}, &OmdbInfo{}, &ExportDetail{})
}

// Responses from OMDB come packaged in quotes; trimQuotesFromByteSlice is
// used to remove the surrounding quotes from the provided byte slice
// and any remaining whitespace is trimmed off. The altered string is then
// returned to the caller
func trimQuotesFromByteSlice(data []byte) string {
	strData := string(data)
	if len(strData) >= 2 && strData[0] == '"' && strData[len(strData)-1] == '"' {
		strData = strData[1 : len(strData)-1]
	}

	return strings.TrimSpace(strData)
}

// convertToInt is a helper method that accepts
// a string input and will attempt to convert that string
// to an integer - if it fails, -1 is returned
func convertToInt(input string) int {
	v, err := strconv.Atoi(input)
	if err != nil {
		return -1
	}

	return v
}

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
	ItemID        int                  `json:"id" groups:"api" gorm:"-"`
	Path          string               `json:"path"`
	Name          string               `json:"name" groups:"api"`
	Status        QueueItemStatus      `json:"status" groups:"api" gorm:"-"`
	Stage         worker.PipelineStage `json:"stage" groups:"api" gorm:"-"`
	TitleInfo     *TitleInfo           `json:"title_info"`
	OmdbInfo      *OmdbInfo            `json:"omdb_info"`
	Trouble       Trouble              `json:"trouble" gorm:"-"`
	ProfileTag    string               `json:"profile_tag"`
	processor     *Processor           `json:"-" gorm:"-"`
	ExportDetails []*ExportDetail      `json:"export_details"`
}

type ExportDetail struct {
	gorm.Model   `json:"-"`
	QueueItemID  uint   `json:"-"`
	ProfileLabel string `json:"profile_label"`
	Path         string `json:"path"`
}

func NewQueueItem(info fs.FileInfo, path string, proc *Processor) *QueueItem {
	return &QueueItem{
		Name:      info.Name(),
		Path:      path,
		Status:    Pending,
		Stage:     worker.Import,
		processor: proc,
	}
}

func (item *QueueItem) SetStage(stage worker.PipelineStage) {
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
		queue := item.processor.Queue
		queue.Remove(item)
		queue.cache.PushItem(item.Path, "cancelled")
	}

	item.NotifyUpdate()
}

func (item *QueueItem) SetProfileTag(tag string) error {
	if idx, _ := item.processor.Profiles.FindProfileByTag(tag); idx == -1 {
		return errors.New("profile tag invalid - profile does not exist in Processor.ProfileList")
	}

	item.ProfileTag = tag
	item.NotifyUpdate()

	return nil
}

// SetTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *QueueItem) SetTrouble(trouble Trouble) {
	defer item.NotifyUpdate()

	itemLogger.Emit(pkg.WARNING, "Raising trouble %T for QueueItem %s\n", trouble, item)
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
func (item *QueueItem) ValidateProfileSuitable(pr profile.Profile) bool {
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
			itemLogger.Emit(pkg.ERROR, "FAILED to validate if profile (%s) match condition (%v) is suitable for item %s because: %v\n", pr, condition, item, err.Error())
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
	db := pkg.DB.GetInstance()
	if err := db.Debug().Save(item).Error; err != nil {
		return fmt.Errorf("failed to commit item %s to database: %s", item, err.Error())
	}

	return nil
}

func (item *QueueItem) Pause() error {
	return errors.New("NYI")
}

func (item *QueueItem) NotifyUpdate() {
	item.processor.UpdateChan <- item.ItemID
}

func (item *QueueItem) String() string {
	return fmt.Sprintf("{%d PK=%d name=%s}", item.ItemID, item.ID, item.Name)
}

func (item *QueueItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ItemID          int                  `json:"id"`
		Path            string               `json:"path"`
		Name            string               `json:"name"`
		Status          QueueItemStatus      `json:"status"`
		Stage           worker.PipelineStage `json:"stage"`
		TitleInfo       *TitleInfo           `json:"title_info"`
		OmdbInfo        *OmdbInfo            `json:"omdb_info"`
		Trouble         Trouble              `json:"trouble"`
		ProfileTag      string               `json:"profile_tag"`
		ExportDetails   []*ExportDetail      `json:"export_details"`
		FfmpegInstances []CommanderTask      `json:"ffmpeg_instances"`
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
		item.ExportDetails,
		item.processor.FfmpegCommander.GetInstancesForItem(item.ItemID),
	})
}

// TitleInfo contains the information about the import QueueItem
// that is gleamed from the pathname given; such as the title and
// if the show is an episode or a movie.
type TitleInfo struct {
	gorm.Model
	QueueItemID uint
	Title       string
	Episodic    bool
	Season      int
	Episode     int
	Year        int
	Resolution  string
}

// OutputPath is a method to calculate the path to which this
// item should be output to - based on the TitleInformation
func (tInfo *TitleInfo) OutputPath() string {
	if tInfo.Episodic {
		fName := fmt.Sprintf("%v_%v_%v_%v_%v", tInfo.Episode, tInfo.Season, tInfo.Title, tInfo.Resolution, tInfo.Year)
		return filepath.Join(tInfo.Title, fmt.Sprint(tInfo.Season), fName)
	}

	return fmt.Sprintf("%v_%v_%v", tInfo.Title, tInfo.Resolution, tInfo.Year)
}

// OmdbInfo is used as an unmarshaller target for JSON. It's embedded
// inside the QueueItem to allow us to use the information to generate
// a file structure, and also to store the information inside
// of a cache file or a database.
type OmdbInfo struct {
	gorm.Model
	QueueItemID uint
	Genre       StringList `decode:"string" mapstructure:"-" gorm:"-"`
	Title       string
	Description string `json:"plot"`
	ReleaseYear int
	Runtime     string
	ImdbId      string
	Type        string
	PosterUrl   string           `json:"poster"`
	Response    OmdbResponseType `decode:"bool" gorm:"-"`
	Error       string           `gorm:"-"`
}

type StringList []string
type OmdbResponseType bool

// UnmarshalJSON on StringList will unmarshal the data provided by
// removing the surrounding quotes and splitting the provided
// information in to a slice (comma-separated)
func (sl *StringList) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)

	list := strings.Split(t, ", ")
	*sl = append(*sl, list...)

	return nil
}

// UnmarshalJSON on OmdbResponseType converts the given string
// from OMDB in to a golang boolean - this method is required
// because the response from OMDB is not a JSON-bool as it's
// capitalised
func (rt *OmdbResponseType) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)
	switch t {
	case "True":
		*rt = true
	case "False":
	default:
		*rt = false
	}

	return nil
}

type TitleFormatError struct {
	item    *QueueItem
	message string
}

func (e TitleFormatError) Error() string {
	return fmt.Sprintf("failed to format title(%v) - %v", e.item.Name, e.message)
}
