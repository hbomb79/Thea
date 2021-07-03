package processor

import (
	"io/fs"
	"log"
	"regexp"
	"strconv"
	"sync"
)

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

type TitleInfo struct {
	Title    string
	Episodic bool
	Season   int
	Episode  int
	Year     int
}

type OmdbInfo struct {
	Genre       string
	Title       string
	Plot        string
	ReleaseYear int
	Runtime     string
	ImdbId      string
	Type        string
	Poster      string
}

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

// RaiseTrouble is a method that can be called from
// tasks that indicates a trouble-state has occured which
// requires some form of intervention from the user
func (queue *ProcessorQueue) RaiseTrouble(item *QueueItem, trouble *Trouble) {
	queue.Lock()
	defer queue.Unlock()

	log.Printf("[Trouble] Raising trouble (%v) for QueueItem (%v)!\n", trouble.Message, item.Path)
	if item.Trouble == nil {
		item.Status = Troubled
		item.Trouble = trouble
	} else {
		log.Fatalf("Failed to raise trouble state for item(%v) as a trouble state already exists: %#v\n", item.Path, trouble)
	}
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

// FormatTitle accepts a string (title) and reformats it
// based on text-filtering configuration provided by
// the user
func (item *QueueItem) FormatTitle() error {
	normaliserMatcher := regexp.MustCompile(`(?i)[\.\s]`)
	seasonMatcher := regexp.MustCompile(`(?i)^(.*?)\_?s(\d+)\_?e(\d+)\_*((?:20|19)\d{2})?`)
	movieMatcher := regexp.MustCompile(`(?i)^(.+?)\_*((?:20|19)\d{2})`)

	title := normaliserMatcher.ReplaceAllString(item.Name, "_")

	// Search for season info and optional year information
	if seasonGroups := seasonMatcher.FindStringSubmatch(title); len(seasonGroups) >= 1 {
		info := &TitleInfo{
			Episodic: true,
			Title:    seasonGroups[1],
			Season:   convertToInt(seasonGroups[2]),
			Episode:  convertToInt(seasonGroups[3]),
			Year:     convertToInt(seasonGroups[4]),
		}

		item.TitleInfo = info
		return nil
	}

	// Try find if it's a movie instead
	if movieGroups := movieMatcher.FindStringSubmatch(item.Name); len(movieGroups) >= 1 {
		info := &TitleInfo{
			Episodic: false,
			Title:    movieGroups[1],
			Year:     convertToInt(movieGroups[2]),
		}

		item.TitleInfo = info
		return nil
	}

	// Didn't match either case; return error so that trouble
	// can be raised by the worker.
	return TitleFormatError{item, "Failed to match RegExp!"}
}
