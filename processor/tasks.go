package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/worker"
	"github.com/mitchellh/mapstructure"
)

type taskFn func(*worker.Worker, *QueueItem) error

const (
	OMDB_API string = "http://www.omdbapi.com/?%s=%s&apikey=%s"
)

// toArgsMap takes a given struct and will go through all
// fields
func toArgsMap(in interface{}) (map[string]string, error) {
	out := make(map[string]string)

	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("toArgsMap only accepts structs - got %T", v)
	}

	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		var typeName string

		fi := typ.Field(i)
		if v, ok := fi.Tag.Lookup("decode"); ok {
			if v == "-" {
				// Field wants to be ignored
				continue
			}

			// Field has a tag to specify the decode type. Use that instead
			typeName = v
		} else {
			// Use actual type name
			typeName = fi.Type.Name()
		}

		out[fi.Name] = typeName
	}

	return out, nil
}

// baseTask is a struct that implements very little functionality, and is used
// to facilitate the other task types implemented in this file. This struct
// mainly just handled some repeated code definitions, such as the basic
// work/wait worker loop, and raising and notifying troubles
type baseTask struct {
	assignedItem *QueueItem
}

// raiseTrouble is a helper method used to push a new trouble
// in to the slice for this task
func (task *baseTask) raiseTrouble(proc *Processor, trouble *Trouble) {
	trouble.Item.RaiseTrouble(trouble)

	task.notifyTrouble(proc, trouble)
}

// notifyTrouble sends a ProcessorUpdate to the processor which
// is likely then pushed along to any connected clients on
// the web socket
func (task *baseTask) notifyTrouble(proc *Processor, trouble *Trouble) {
	proc.PushUpdate(&ProcessorUpdate{
		Title:   "TROUBLE",
		Context: processorUpdateContext{Trouble: trouble, QueueItem: trouble.Item},
	})
}

// executeTask implements the core worker work/wait loop that
// searches for work to do - and if some work is available, the
// 'fn' taskFn is executed. If no work is available, the worker
// sleeps until woken up again.
func (task *baseTask) executeTask(w *worker.Worker, proc *Processor, fn taskFn, errHandler taskErrorHandler) error {
	for {
	inner:
		for {
			item := proc.Queue.Pick(w.Stage())
			if item == nil {
				break inner
			}

			if err := fn(w, item); err != nil {
				if e := errHandler(item, err); e != nil {
					return err
				}

				// Trouble has been raised, continue to next item
				continue inner
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.Sleep(); !isAlive {
			return nil
		}
	}
}

// TitleTask is the task responsible for searching through the
// queue items raw path name and filtering out relevant information
// such as the title, season/episode information, release year, and resolution.
type TitleTask struct {
	proc *Processor
	baseTask
}

// Execute will utilise the baseTask.Execute method to run the task repeatedly
// in a worker work/wait loop
func (task *TitleTask) Execute(w *worker.Worker) error {
	return task.executeTask(w, task.proc, task.processTitle, task.handleError)
}

// Processes a given queueItem by filtering out irrelevant information from it's
// title, and finding relevant information such as the season, episode and resolution
// TODO: Maybe we should be checking file metadata to get accurate resolution
// and runtime information - this info could also be found in the FormatTask via
// ffprobe
func (task *TitleTask) processTitle(w *worker.Worker, queueItem *QueueItem) error {
	if err := queueItem.FormatTitle(); err != nil {
		return err
	}

	task.advance(queueItem)
	return nil
}

func (task *TitleTask) advance(item *QueueItem) {
	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(item)

	// Wakeup any pipeline workers that are sleeping
	task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
}

// handleError will check the err provided and if it's a TitleFormatError, it will
// raise a trouble on the queue item provided.
func (task *TitleTask) handleError(item *QueueItem, err error) error {
	if v, ok := err.(TitleFormatError); ok {
		// Raise trouble
		tArgs, tErr := toArgsMap(TitleInfo{})
		if tErr != nil {
			return tErr
		}

		task.raiseTrouble(task.proc, &Trouble{"Title Processor Trouble", v, item, tArgs, TitleFailure, task.resolveTrouble})
		return nil
	}

	return err
}

// resolveTrouble accepts a trouble and a map of arguments, and will attempt
// to build a TitleInfo struct out of the arguments provided. The trouble provided
// MUST be a TitleFailure trouble (tag). If success, the queueItem has it's TitleInfo
// set to the resulting struct, and it's advanced to the next stage
func (task *TitleTask) resolveTrouble(trouble *Trouble, args map[string]interface{}) error {
	if trouble.Tag != TitleFailure {
		return fmt.Errorf("failed to resolve trouble; unexpected 'tag' %v", trouble.Tag)
	}

	// The trouble must be resolved by passing arguments that can be used to
	// build a TitleInfo struct. We use mapstructure to attempt to build
	// the struct here - if it succeeds, we can resolve the trouble
	var result TitleInfo
	err := mapstructure.WeakDecode(args, &result)
	if err != nil {
		return err
	}

	log.Printf("Successfully decoded incoming arguments to struct %T: %#v\n", result, result)

	item := trouble.Item
	item.TitleInfo = &result
	task.advance(item)

	return nil
}

type OmdbNoResultError struct{ message string }

func (err OmdbNoResultError) Error() string {
	return err.message
}

type OmdbMultipleResultError struct{ message string }

func (err OmdbMultipleResultError) Error() string {
	return err.message
}

type OmdbRequestError struct{ message string }

func (err OmdbRequestError) Error() string {
	return err.message
}

// OmdbTask is the task responsible for querying to OMDB API for information
// about the queue item we've processed so far.
type OmdbTask struct {
	proc *Processor
	baseTask
}

// OmdbSearchItem is the struct that encapsulates some of the information from
// the OMDB api that is nested inside of a search result (OmdbSearchResult)
type OmdbSearchItem struct {
	Title     string
	Year      string
	ImdbId    string `json:"imdbId"`
	EntryType string `json:"Type"`
}

// OmdbSearchResult is the struct used to unmarshal JSON from the OMDB api
// after a search has been performed. Note that this is not the same as OmdbInfo, which
// is what the QueueItem stores - this struct is filled via a JSON unmarshal in 'fetch'
type OmdbSearchResult struct {
	Results  []*OmdbSearchItem `json:"Search"`
	Response OmdbResponseType
	Count    int `json:"totalResults,string"`
}

func filterSearchItems(items []*OmdbSearchItem, strategy func(*OmdbSearchItem) bool) []*OmdbSearchItem {
	out := make([]*OmdbSearchItem, 0)
	for i := 0; i < len(items); i++ {
		item := items[i]
		if strategy(item) {
			out = append(out, item)
		}
	}

	return out
}

func (result *OmdbSearchResult) parse(queueItem *QueueItem) (*OmdbSearchItem, error) {
	// Check the response by parsing the response, and total results.
	if !result.Response || result.Count == 0 {
		// Response from OMDB failed!
		return nil, OmdbNoResultError{}
	}

	if result.Count > 1 {
		// We have multiple results from OMDB.
		items := result.Results
		desiredType := "movie"
		if queueItem.TitleInfo.Episodic {
			desiredType = "series"
		}

		// 1. Discard items that are of a different type (i.e movie/series)
		items = filterSearchItems(items, func(item *OmdbSearchItem) bool {
			return strings.Compare(item.EntryType, desiredType) == 0
		})

		// 2. Discard items that are of a different year
		yearMatcher := regexp.MustCompile(`\d+`)
		items = filterSearchItems(items, func(item *OmdbSearchItem) bool {
			yearString := yearMatcher.FindString(item.Year)
			year, err := strconv.Atoi(yearString)
			if err != nil {
				return false
			}

			return year == queueItem.TitleInfo.Year
		})

		// If we still have multiple responses then we need help from the user to decide.
		if len(items) == 0 {
			return nil, OmdbNoResultError{}
		} else if len(items) > 1 {
			return nil, OmdbMultipleResultError{}
		}

		return items[0], nil
	} else if result.Count < 1 {
		return nil, OmdbNoResultError{}
	}

	return result.Results[0], nil
}

// Execute uses the provided baseTask.executeTask method to run this tasks
// work function in a work/wait worker loop
func (task *OmdbTask) Execute(w *worker.Worker) error {
	return task.executeTask(w, task.proc, task.find, task.handleError)
}

// search will perform a search query to OMDB and will return the result
// If multiple results are found a OmdbMultipleResultError is returned; if
// no results are found then an OmdbNoResultError is returned. If the request
// fails for another reason, an OmdbRequestError is returned.
func (task *OmdbTask) search(w *worker.Worker, queueItem *QueueItem) (*OmdbInfo, error) {
	// Peform the search
	cfg := task.proc.Config.Database
	res, err := http.Get(fmt.Sprintf(OMDB_API, "s", queueItem.TitleInfo.Title, cfg.OmdbKey))
	if err != nil {
		// Request exception
		return nil, OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error())}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error())}
	}

	var searchResult OmdbSearchResult
	if err = json.Unmarshal(body, &searchResult); err != nil {
		return nil, OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error())}
	}

	resultItem, err := searchResult.parse(queueItem)
	if err != nil {
		return nil, err
	}

	// We got a result however OMDB search results don't contain all the info we need
	// so we need to perform a direct query for this item
	omdbResult, err := task.fetch(w, resultItem.ImdbId)
	if err != nil {
		return nil, err
	}

	return omdbResult, nil
}

// fetch will perform an Omdb request using the given ID as the API argument.
// If no match is found a OmdbNoResultError is returned - if the request fails
// for another reason, an OmdbRequestError is returned.
func (task *OmdbTask) fetch(w *worker.Worker, imdbId string) (*OmdbInfo, error) {
	cfg := task.proc.Config.Database
	res, err := http.Get(fmt.Sprintf(OMDB_API, "i", imdbId, cfg.OmdbKey))
	if err != nil {
		// Request exception
		return nil, OmdbRequestError{err.Error()}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, OmdbRequestError{err.Error()}
	}

	var result OmdbInfo
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, OmdbRequestError{err.Error()}
	}

	return &result, nil
}

// find is used to perform a search to Omdb using the title information stored inside
// of a QueueItem.
func (task *OmdbTask) find(w *worker.Worker, queueItem *QueueItem) error {
	if queueItem.TitleInfo == nil {
		return fmt.Errorf("cannot find OMDB info for queueItem (id: %v). TitleInfo missing!", queueItem.Id)
	}

	res, err := task.search(w, queueItem)
	if err != nil {
		return err
	}

	queueItem.OmdbInfo = res
	return nil
}

// TODO
func (task *OmdbTask) handleError(item *QueueItem, err error) error {
	// Not an error we want to raise trouble for.
	return err
}

// TODO
func (task *OmdbTask) resolveTrouble(trouble *Trouble, args map[string]interface{}) error {
	return nil
}

// FormatTask is a task that is responsible for performing the transcoding of
// the queue items to MP4 format, to allow for viewing/streaming directly
// inside of any modern web browsers
type FormatTask struct {
	proc *Processor
	baseTask
}

// Execute uses the baseTask.executeTask to run this workers
// task in a worker loop
func (task *FormatTask) Execute(w *worker.Worker) error {
	return task.executeTask(w, task.proc, task.format, task.handleError)
}

// format will take the provided queueItem and format the file in to
// a new format.
func (task *FormatTask) format(w *worker.Worker, queueItem *QueueItem) error {
	if true {
		return fmt.Errorf("HIT babey. omdbInfo: %#v", queueItem.OmdbInfo)
	}

	outputFormat := task.proc.Config.Format.TargetFormat
	ffmpegOverwrite := true
	ffmpegOpts, ffmpegCfg := &ffmpeg.Options{
		OutputFormat: &outputFormat,
		Overwrite:    &ffmpegOverwrite,
	}, &ffmpeg.Config{
		ProgressEnabled: true,
		FfmpegBinPath:   task.proc.Config.Format.FfmpegBinaryPath,
		FfprobeBinPath:  task.proc.Config.Format.FfprobeBinaryPath,
	}

	itemOutputPath := fmt.Sprintf("%s.%s", queueItem.TitleInfo.OutputPath(), outputFormat)
	itemOutputPath = filepath.Join(task.proc.Config.Format.OutputPath, itemOutputPath)
	progress, err := ffmpeg.
		New(ffmpegCfg).
		Input(queueItem.Path).
		Output(itemOutputPath).
		WithOptions(ffmpegOpts).
		Start(ffmpegOpts)

	if err != nil {
		return err
	}

	for v := range progress {
		log.Printf("[Progress] %#v\n", v)
	}

	// Advance our item to the next stage
	task.proc.Queue.AdvanceStage(queueItem)
	return nil
}

// TODO
func (task *FormatTask) handleError(item *QueueItem, err error) error {
	// Not an error we want to raise trouble for.
	return err
}

// TODO
func (task *FormatTask) resolveTrouble(trouble *Trouble, args map[string]interface{}) error {
	return nil
}
