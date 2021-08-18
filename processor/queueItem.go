package processor

import (
	"context"
	"errors"
	"fmt"
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
	StatusLine       string               `json:"statusLine" groups:"api"`
	TitleInfo        *TitleInfo           `json:"title_info"`
	OmdbInfo         *OmdbInfo            `json:"omdb_info"`
	Trouble          Trouble              `json:"trouble"`
	Processor        *Processor           `json:"-"`
	CmdContext       context.Context      `json:"-"`
	cmdContextCancel context.CancelFunc   `json:"-"`
}

func NewQueueItem(name string, path string, proc *Processor) *QueueItem {
	cmdCtx, cmdCancel := context.WithCancel(context.Background())
	return &QueueItem{
		Name:             name,
		Path:             path,
		Status:           Pending,
		Stage:            worker.Title,
		Processor:        proc,
		CmdContext:       cmdCtx,
		cmdContextCancel: cmdCancel,
	}
}

func (item *QueueItem) SetStatusLine(status string) {
	item.StatusLine = status
	item.NotifyUpdate()
}

func (item *QueueItem) SetStage(stage worker.PipelineStage) {
	item.Stage = stage
	item.NotifyUpdate()
}

func (item *QueueItem) SetStatus(status QueueItemStatus) {
	item.Status = status
	item.NotifyUpdate()
}

// SetTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *QueueItem) SetTrouble(trouble Trouble) error {
	fmt.Printf("[Trouble] Raising trouble (%T) for QueueItem (%v)!\n", trouble, item.Path)
	if item.Trouble == nil {
		item.Trouble = trouble
		item.SetStatus(NeedsResolving)

		return nil
	}

	return errors.New(fmt.Sprintf("Failed to raise trouble state for item(%v) as a trouble state already exists: %#v\n", item.Path, trouble))
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

// Cancel TODO
// In order for a queue item to cancel itself, the queue item needs to be
// able to interact directly with the task that is currently running. This means
// QueueItem needs a reference to a worker that is currently holding it, and the
// WorkerTaskMeta interface needs to be adjusted to enforce implementation of a cancel method
// as cancelling a task will vary based on the task being run.
func (item *QueueItem) Cancel() {
	if item.Status == Cancelled {
		return
	}

	// Cancel the context for this item
	item.Status = Cancelled
	item.cmdContextCancel()
}

func (item *QueueItem) NotifyUpdate() {
	item.Processor.UpdateChan <- item.Id
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
	Genre       StringList
	Title       string
	Description string `json:"plot"`
	ReleaseYear int
	Runtime     string
	ImdbId      string
	Type        string
	PosterUrl   string `json:"poster"`
	Response    OmdbResponseType
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
