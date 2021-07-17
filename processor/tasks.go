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
// fields of the provided input and create an output map where
// each key is the name of the field, and each value is a string
// representation of the type of the field (e.g. string, int, bool)
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
func (task *baseTask) raiseTrouble(proc *Processor, trouble Trouble) {
	trouble.Item().RaiseTrouble(trouble)

	task.notifyTrouble(proc, trouble)
}

// notifyTrouble sends a ProcessorUpdate to the processor which
// is likely then pushed along to any connected clients on
// the web socket
func (task *baseTask) notifyTrouble(proc *Processor, trouble Trouble) {
	proc.PushUpdate(&ProcessorUpdate{
		Title:   "TROUBLE",
		Context: processorUpdateContext{Trouble: trouble, QueueItem: trouble.Item()},
	})
}

// executeTask implements the core worker work/wait loop that
// searches for work to do - and if some work is available, the
// 'fn' taskFn is executed. If no work is available, the worker
// sleeps until woken up again.
func (task *baseTask) executeTask(w *worker.Worker, proc *Processor, fn taskFn) error {
	for {
	inner:
		for {
			item := proc.Queue.Pick(w.Stage())
			if item == nil {
				break inner
			}

			if err := fn(w, item); err != nil {
				e, ok := err.(Trouble)
				if ok {
					// Error implements the trouble interface so raise a trouble
					task.raiseTrouble(proc, e)
					continue
				}

				// Unhandled exception!
				return err
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.Sleep(); !isAlive {
			return nil
		}
	}
}

// TitleTaskError is an error raised during the processing of the
// title task
type TitleTaskError struct {
	Message   string
	queueItem *QueueItem
	task      *TitleTask
}

// Error provided so the struct implements 'error'
func (ex TitleTaskError) Error() string {
	return ex.Message
}

// Args returns the arguments required to resolve this
// trouble
func (ex TitleTaskError) Args() map[string]string {
	v, err := toArgsMap(TitleInfo{})
	if err != nil {
		panic(err)
	}

	return v
}

// Resolve will attempt to resolve the error by taking the arguments provided
// to the method, and casting it to a TitleInfo struct if possible.
func (ex TitleTaskError) Resolve(args map[string]interface{}) error {
	// The trouble must be resolved by passing arguments that can be used to
	// build a TitleInfo struct. We use mapstructure to attempt to build
	// the struct here - if it succeeds, we can resolve the trouble
	var result TitleInfo
	err := mapstructure.WeakDecode(args, &result)
	if err != nil {
		return err
	}

	log.Printf("Successfully decoded incoming arguments to struct %T: %#v\n", result, result)

	ex.queueItem.TitleInfo = &result
	ex.queueItem.ResetTrouble()
	ex.task.advance(ex.queueItem)

	return nil
}

// Item returns the QueueItem that is attached to this trouble
func (ex TitleTaskError) Item() *QueueItem {
	return ex.queueItem
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
	return task.executeTask(w, task.proc, task.processTitle)
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

// advances the item by advancing the stage of the item, and waking up
// any sleeping workers in the next stage
func (task *TitleTask) advance(item *QueueItem) {
	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(item)

	// Wakeup any pipeline workers that are sleeping
	task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
}

// An OmdbNoResultError is returned from the OmdbTask when we couldn't find
// any result that matches the queue item we're trying to find information for
// Resolving this error (.Resolve) will expect an IMDB ID to fetch the information
// from in order to populate the queueItem
type OmdbNoResultError struct {
	Message   string `json:"message"`
	queueItem *QueueItem
	task      *OmdbTask
}

// Error returns the task message formatted for use as an error message. Implemented so this
// struct satisfies the error interface
func (ex OmdbNoResultError) Error() string {
	return fmt.Sprintf("Omdb task error: %v", ex.Message)
}

// Args returns the arguments required by this error to successfully resolve
func (ex OmdbNoResultError) Args() map[string]string {
	return map[string]string{
		"imdbId": "string",
	}
}

// Resolve will attempt to resolve this error by fetching an OMDB entry using
// a specific imdbId provided via the arguments to this method
func (ex OmdbNoResultError) Resolve(args map[string]interface{}) error {
	// Attempt to fetch the Imdb item with this ID
	id, ok := args["imdbId"]
	if !ok {
		return fmt.Errorf("Failed to resolve OmdbNoResultError - Mising imdbId (string) key in args")
	}

	info, err := ex.task.fetch(id.(string), ex.queueItem)
	if err != nil {
		return fmt.Errorf("Failed to resolve OmdbNoResultError - %v", err.Error())
	}

	ex.queueItem.OmdbInfo = info
	ex.queueItem.ResetTrouble()
	ex.task.advance(ex.queueItem)
	return nil
}

// Item returns the QueueItem this trouble is attached to
func (ex OmdbNoResultError) Item() *QueueItem {
	return ex.queueItem
}

// OmdbMultipleResultError is an error/trouble raised when a search query
// to OMDB has many possible results. This trouble allows the user to chose which
// option they'd like to use
type OmdbMultipleResultError struct {
	Message   string `json:"message"`
	queueItem *QueueItem
	task      *OmdbTask
	Choices   []*OmdbSearchItem `json:"choices"`
}

// Error returns the trouble message formatted for use as an error message. Implemented so
// this struct satisfies the error interface
func (ex OmdbMultipleResultError) Error() string {
	return fmt.Sprintf("Omdb task error: %v", ex.Message)
}

// Args returns the arguments required to resolve this trouble
func (ex OmdbMultipleResultError) Args() map[string]string {
	return map[string]string{
		"choice": "int",
	}
}

// Resolve attempts to resolve this error by fetching details from OMDB for the choice the user
// selected. This selection is provided by use of an 'id' representing an index inside the choice
// array for this trouble.
func (ex OmdbMultipleResultError) Resolve(args map[string]interface{}) error {
	v, ok := args["choice"]
	if !ok {
		return fmt.Errorf("Failed to resolve OmdbMultipleResultError - Missing choice (int) key in args")
	}

	choice, ok := v.(int)
	if !ok || len(ex.Choices)-1 < choice {
		return fmt.Errorf("Faield to resolve OmdbMultipleResultError - Bad value for choice (int) key in args")
	}

	// Okay, we have a valid choice from the user. Fetch that choice from OMDB and store
	info, err := ex.task.fetch(ex.Choices[choice].ImdbId, ex.queueItem)
	if err != nil {
		return fmt.Errorf("Failed to resolve OmdbMultipleResultError - %v", err.Error())
	}

	ex.queueItem.OmdbInfo = info
	ex.queueItem.ResetTrouble()
	ex.task.advance(ex.queueItem)
	return nil
}

// Item returns the QueueItem attached to this trouble
func (ex OmdbMultipleResultError) Item() *QueueItem {
	return ex.queueItem
}

type OmdbRequestError struct {
	Message   string
	queueItem *QueueItem
	task      *OmdbTask
}

func (err OmdbRequestError) Error() string {
	return fmt.Sprintf("Omdb task error: %v", err.Message)
}

func (err OmdbRequestError) Args() map[string]string {
	return map[string]string{}
}

func (err OmdbRequestError) Resolve(args map[string]interface{}) error {
	err.queueItem.ResetTrouble()
	err.task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
	return nil
}

func (ex OmdbRequestError) Item() *QueueItem {
	return ex.queueItem
}

// OmdbTask is the task responsible for querying to OMDB API for information
// about the queue item we've processed so far.
type OmdbTask struct {
	proc   *Processor
	apiKey string
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
		} else {
		}
	}

	return out
}

func (result *OmdbSearchResult) parse(queueItem *QueueItem, task *OmdbTask) (*OmdbSearchItem, error) {
	// Check the response by parsing the response, and total results.
	if !result.Response || result.Count == 0 {
		// Response from OMDB failed!
		return nil, &OmdbNoResultError{"Failed to parse OMDB result - response empty", queueItem, task}
	}

	items := result.Results
	desiredType := "movie"
	if queueItem.TitleInfo.Episodic {
		desiredType = "series"
	}

	// 1. Discard items that are of a different type (i.e movie/series)
	items = filterSearchItems(items, func(item *OmdbSearchItem) bool {
		return strings.Compare(item.EntryType, desiredType) == 0
	})

	// 2. Discard items that have release years that don't match ours
	if queueItem.TitleInfo.Year > -1 {
		yearMatcher := regexp.MustCompile(`(\d+)(–?)(\d+)?`)
		items = filterSearchItems(items, func(item *OmdbSearchItem) bool {
			yearMatches := yearMatcher.FindStringSubmatch(item.Year)
			if yearMatches == nil {
				return false
			}

			var length int = 0
			for i := 0; i < len(yearMatches); i++ {
				if yearMatches[i] != "" {
					length++
				}
			}

			switch length {
			case 0:
				// No match
				return false
			case 1:
				// Hm, we have only one match group. This *should*
				// be impossible as FindStringSubmatch always returns
				// the entire match as index 0, and then the capture groups
				// as index 1 through to N.
				return false
			case 2:
				// We found a basic year and nothing more. Compare if the dates match
				if y, err := strconv.Atoi(yearMatches[1]); err == nil {
					return y == queueItem.TitleInfo.Year
				}
			case 3:
				// We found a basic year, *and* an additional capture group
				// This should mean we have a date range with no closing date
				if y, err := strconv.Atoi(yearMatches[1]); err == nil {
					return queueItem.TitleInfo.Year >= y
				}
			case 4:
				// We found a year range with opening and closing year
				startYear, sErr := strconv.Atoi(yearMatches[1])
				endYear, eErr := strconv.Atoi(yearMatches[3])
				if !(sErr == nil && eErr == nil) {
					return false
				}

				return queueItem.TitleInfo.Year >= startYear && queueItem.TitleInfo.Year <= endYear
			}

			return false
		})
	}

	// If we still have multiple responses then we need help from the user to decide.
	if len(items) == 0 {
		return nil, &OmdbNoResultError{"parse failed: no valid choices remain after filtering", queueItem, task}
	} else if len(items) > 1 {
		return nil, &OmdbMultipleResultError{"parse failed: multiple choices remain after filtering", queueItem, task, items}
	}

	return items[0], nil
}

// Execute uses the provided baseTask.executeTask method to run this tasks
// work function in a work/wait worker loop
func (task *OmdbTask) Execute(w *worker.Worker) error {
	return task.executeTask(w, task.proc, task.find)
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
		return nil, &OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error()), queueItem, task}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error()), queueItem, task}
	}

	var searchResult OmdbSearchResult
	if err = json.Unmarshal(body, &searchResult); err != nil {
		return nil, &OmdbRequestError{fmt.Sprintf("search failed: %s", err.Error()), queueItem, task}
	}

	resultItem, err := searchResult.parse(queueItem, task)
	if err != nil {
		return nil, err
	}

	// We got a result however OMDB search results don't contain all the info we need
	// so we need to perform a direct query for this item
	omdbResult, err := task.fetch(resultItem.ImdbId, queueItem)
	if err != nil {
		return nil, err
	}

	return omdbResult, nil
}

// fetch will perform an Omdb request using the given ID as the API argument.
// If no match is found a OmdbNoResultError is returned - if the request fails
// for another reason, an OmdbRequestError is returned.
func (task *OmdbTask) fetch(imdbId string, queueItem *QueueItem) (*OmdbInfo, error) {
	cfg := task.proc.Config.Database
	res, err := http.Get(fmt.Sprintf(OMDB_API, "i", imdbId, cfg.OmdbKey))
	if err != nil {
		// Request exception
		return nil, &OmdbRequestError{fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, task}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &OmdbRequestError{fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, task}
	}

	var result OmdbInfo
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, &OmdbRequestError{fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, task}
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
	task.advance(queueItem)
	return nil
}

func (task *OmdbTask) advance(item *QueueItem) {
	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(item)

	// Wakeup any pipeline workers that are sleeping
	task.proc.WorkerPool.WakeupWorkers(worker.Format)
}

type FormatTaskError struct {
	Message   string
	queueItem *QueueItem
	task      *FormatTask
}

func (ex FormatTaskError) Error() string {
	return fmt.Sprintf("Format task trouble: %v", ex.Message)
}

func (ex FormatTaskError) Args() map[string]string {
	return map[string]string{}
}

func (ex FormatTaskError) Resolve(map[string]interface{}) error {
	ex.queueItem.ResetTrouble()
	ex.task.proc.WorkerPool.WakeupWorkers(worker.Format)
	return nil
}

func (ex FormatTaskError) Item() *QueueItem {
	return ex.queueItem
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
	return task.executeTask(w, task.proc, task.format)
}

// format will take the provided queueItem and format the file in to
// a new format.
func (task *FormatTask) format(w *worker.Worker, queueItem *QueueItem) error {
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
		return &FormatTaskError{err.Error(), queueItem, task}
	}

	for v := range progress {
		log.Printf("[Progress] %#v\n", v)
	}

	// Advance our item to the next stage
	task.advance(queueItem)
	return nil
}

func (task *FormatTask) advance(item *QueueItem) {
	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(item)
}
