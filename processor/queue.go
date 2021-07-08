package processor

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/api"
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
	Troubled
)

// QueueItem contains all the information needed to fully
// encapsulate the state of each item in the formatting queue.
// This includes information found from the file-system, OMDB,
// and the current processing status/stage
type QueueItem struct {
	Path       string
	Name       string
	Status     QueueItemStatus
	Stage      PipelineStage
	StatusLine string
	Trouble    *Trouble
	TitleInfo  *TitleInfo
	OmdbInfo   *OmdbInfo
}

// RaiseTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (item *QueueItem) RaiseTrouble(trouble *Trouble) {
	log.Printf("[Trouble] Raising trouble (%v) for QueueItem (%v)!\n", trouble.Message, item.Path)
	if item.Trouble == nil {
		item.Status = Troubled
		item.Trouble = trouble
	} else {
		panic(fmt.Sprintf("Failed to raise trouble state for item(%v) as a trouble state already exists: %#v\n", item.Path, trouble))
	}
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
	Type        OmdbType
	PosterUrl   string `json:"poster"`
	Response    OmdbResponseType
	Error       string
}

type StringList []string

// UnmarshalJSON on StringList will unmarshal the data provided by
// removing the surrounding quotes and splitting the provided
// information in to a slice (comma-separated)
func (sl *StringList) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)

	list := strings.Split(t, ", ")
	*sl = append(*sl, list...)

	return nil
}

const (
	movie OmdbType = iota
	series
)

type OmdbType int

// UnmarshalJSON on OmdbType will look at the data provided
// and will set the type to an integer corresponding to the
// OmdbType const.
func (omdbType *OmdbType) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)
	switch t {
	case "series":
		*omdbType = series
	case "movie":
		*omdbType = movie
	default:
		return errors.New("unable to unmarshal JSON for 'Type' - unknown value " + t)
	}

	return nil
}

type OmdbResponseType bool

// UnmarshalJSON on OmdbResponseType converts the given string
// from OMDB in to a golang boolean - this method is required
// because the response from OMDB is not a JSON-bool as it's
// capitalised
func (rt *OmdbResponseType) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)
	switch t {
	case "False":
		*rt = false
	case "True":
		*rt = true
	}

	return nil
}

// ProcessorQueue is the Queue of items to be processed by this
// processor
type ProcessorQueue struct {
	Items []*QueueItem
	sync.Mutex
}

// HandleFile will take the provided file and if it's not
// currently inside the queue, it will be inserted in to the queue.
// If it is in the queue, the entry is skipped - this is because
// this method is usually called as a result of polling the
// input directory many times a day for new files.
func (queue *ProcessorQueue) HandleFile(path string, fileInfo fs.FileInfo) bool {
	queue.Lock()
	defer queue.Unlock()

	if !queue.isInQueue(path) {
		queue.Items = append(queue.Items, &QueueItem{
			Name:   fileInfo.Name(),
			Path:   path,
			Status: Pending,
			Stage:  Title,
		})

		return true
	}

	return false
}

// Pick will search through the queue items looking for the first
// QueueItem that has the stage and status we're looking for.
// This is how workers should query the work pool for new tasks
// Note: this method will lock the Mutex for protected access
// to the shared queue.
func (queue *ProcessorQueue) Pick(stage PipelineStage) *QueueItem {
	queue.Lock()
	defer queue.Unlock()

	for _, item := range queue.Items {
		if item.Stage == stage && item.Status == Pending {
			item.Status = Processing
			return item
		}
	}

	return nil
}

// AdvanceStage will take the QueueItem this method is attached to,
// and set it's stage to the next stage and reset it's status to Pending
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *ProcessorQueue) AdvanceStage(item *QueueItem) {
	queue.Lock()
	defer queue.Unlock()

	if item.Stage == Finish {
		item.Status = Completed
	} else if item.Stage == Format {
		item.Stage = Finish
		item.Status = Completed
	} else {
		item.Stage++
		item.Status = Pending
	}
}

// PromoteItem accepts a QueueItem and will restructure the processor
// queue items to mean that the item provided is the first QueueItem in
// the slice. Returns an error if the queue item provided is not found
// inside the queue slice.
// Note: this method will lock the mutex for protected access to the
// shared queue.
func (queue *ProcessorQueue) PromoteItem(item *QueueItem) error {
	queue.Lock()
	defer queue.Unlock()

	// Restructures the slice by taking the items before and
	// after the index given, and appending them together
	// before appending the result to a new slice containing
	// only the item referenced by the index given.
	promote := func(source []*QueueItem, index int) {
		out := append([]*QueueItem{source[index]}, source[:index]...)

		source = append(out, source[index+1:]...)
	}

	// Search for the item and promote it if/when found
	for position := 0; position <= len(queue.Items); position++ {
		if queue.Items[position] == item {
			promote(queue.Items, position)

			return nil
		}
	}

	// Not found, return error
	return errors.New("cannot promote: item does not exist inside this queue")
}

// isInQueue will return true if the queue contains a QueueItem
// with a path field matching the path provided to this method
// Note: callers responsiblity to ensure the queues Mutex is
// already locked before use - otherwise the queue contents
// may mutate while iterating through it
func (queue *ProcessorQueue) isInQueue(path string) bool {
	for _, v := range queue.Items {
		if v.Path == path {
			return true
		}
	}

	return false
}

// apiQueueIndex returns the current processor queue
func (queue *ProcessorQueue) ApiQueueIndex(w http.ResponseWriter, r *http.Request) {
	api.JsonMarshal(w, queue.Items)
}

// apiQueueGet returns full details for a queue item at the index {item_id} inside the queue
func (queue *ProcessorQueue) ApiQueueGet(w http.ResponseWriter, r *http.Request) {
	stringId := mux.Vars(r)["id"]
	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonError(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	if len(queue.Items) <= id {
		api.JsonError(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
		return
	}

	api.JsonMarshal(w, queue.Items[id])
}

// apiQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func (queue *ProcessorQueue) ApiQueueUpdate(w http.ResponseWriter, r *http.Request) {

}
