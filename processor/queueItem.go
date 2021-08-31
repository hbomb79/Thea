package processor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hbomb79/TPA/worker"
)

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

// QueueItemStatus represents whether or not the
// QueueItem is currently being worked on, or if
// it's waiting for a worker to pick it up
// and begin working on the task
type QueueItemStatus int

// If a task is Pending, it's waiting for a worker
// ... if processing, it's currently being worked on.
// When a stage in the pipeline is finished with the task,
// it should set the Stage to the next stage, and set the
// Status to pending - except for Format stage, which should
// mark it as completed
const (
	Pending QueueItemStatus = iota
	Processing
	Completed
	NeedsResolving
	Cancelling
	Cancelled
)

// QueueItem contains all the information needed to fully
// encapsulate the state of each item in the formatting queue.
// This includes information found from the file-system, OMDB,
// and the current processing status/stage
type QueueItem struct {
	Id               int                  `json:"id" groups:"api"`
	Path             string               `json:"path"`
	Name             string               `json:"name" groups:"api"`
	Status           QueueItemStatus      `json:"status" groups:"api"`
	Stage            worker.PipelineStage `json:"stage" groups:"api"`
	TaskFeedback     string               `json:"taskFeedback" groups:"api"`
	TitleInfo        *TitleInfo           `json:"title_info"`
	OmdbInfo         *OmdbInfo            `json:"omdb_info"`
	Trouble          Trouble              `json:"trouble"`
	processor        *Processor           `json:"-"`
	CmdContext       context.Context      `json:"-"`
	cmdContextCancel context.CancelFunc   `json:"-"`
}

func NewQueueItem(info fs.FileInfo, path string, proc *Processor) *QueueItem {
	cmdCtx, cmdCancel := context.WithCancel(context.Background())
	return &QueueItem{
		Name:             info.Name(),
		Path:             path,
		Status:           Pending,
		Stage:            worker.Import,
		processor:        proc,
		CmdContext:       cmdCtx,
		cmdContextCancel: cmdCancel,
	}
}

func (item *QueueItem) SetTaskFeedback(status string) {
	item.TaskFeedback = status
	item.NotifyUpdate()
}

func (item *QueueItem) SetStage(stage worker.PipelineStage) {
	item.Stage = stage
	item.processor.WorkerPool.WakeupWorkers(stage)

	item.SetTaskFeedback("")
	item.NotifyUpdate()
}

func (item *QueueItem) SetStatus(status QueueItemStatus) {
	item.Status = status

	if item.Status == Cancelled {
		// Item has been cancelled and has wrapped up what it was doing
		// Remove this item from the queue, mark it in the queue cache
		// so we don't re-ingest it later
		queue := item.processor.Queue
		queue.Remove(item)
		queue.cache.PushItem(item.Path, "cancelled")
	}

	item.SetTaskFeedback("")
	item.NotifyUpdate()
}

// SetTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *QueueItem) SetTrouble(trouble Trouble) {
	fmt.Printf("[Trouble] Raising trouble (%T) for QueueItem (%v)!\n", trouble, item.Path)
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

	item.cmdContextCancel()
	return nil
}

func (item *QueueItem) Pause() error {
	return errors.New("NYI")
}

func (item *QueueItem) NotifyUpdate() {
	item.processor.UpdateChan <- item.Id
}

// TitleInfo contains the information about the import QueueItem
// that is gleamed from the pathname given; such as the title and
// if the show is an episode or a movie.
type TitleInfo struct {
	Title      string
	Episodic   bool
	Season     int
	Episode    int
	Year       int
	Resolution string
}

// OutputPath is a method to calculate the path to which this
// item should be output to - based on the TitleInformatio
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
	Genre       StringList `decode:"string"`
	Title       string
	Description string `json:"plot"`
	ReleaseYear int
	Runtime     string
	ImdbId      string
	Type        string
	PosterUrl   string           `json:"poster"`
	Response    OmdbResponseType `decode:"bool"`
	Error       string
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
