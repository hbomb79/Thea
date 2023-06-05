package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/pkg/worker"
)

var taskLogger = logger.Get("Task")

const (
	// The URL used to query OMDB. First %s is the query type (s for seach, t for title, i for id),
	// second %s is the term to use for the above query. Third %s is the api key.
	OMDB_API string = "http://www.omdbapi.com/?%s=%s&apikey=%s"
)

// BaseTask is a struct that implements very little functionality, and is used
// to facilitate the other task types implemented in this file. This struct
// mainly just handled some repeated code definitions, such as the basic
// work/wait worker loop, and raising and notifying troubles
type BaseTask struct {
	ItemProducer
}

type taskFn func(worker.Worker, *Item) error
type onComplete func(*Item)
type ItemProducer interface {
	PickItem(ItemStage) *Item
}

// executeTask implements the core worker work/wait loop that
// searches for work to do - and if some work is available, the
// 'fn' taskFn is executed. If no work is available, the worker
// sleeps until woken up again.
func (task *BaseTask) executeTask(w worker.Worker, fn taskFn) error {
	for {
	inner:
		for {
			// Test the wakeup channel of this worker; reading on a closed channel will indicate that this worker is closing
			// and we should break our event loop (isAlive/w.Sleep in the outer loop will also ensure this).
			select {
			case _, ok := <-w.WakeupChan():
				if !ok {
					break inner
				}
			default:
			}

			item := task.PickItem(ItemStage(w.Stage()))
			if item == nil {
				break inner
			}

			err := fn(w, item)
			if item.Status == Cancelling {
				// Item wants to cancel and is waiting for us to finish... we've finished
				// with this task so mark it as fully cancelled.
				item.SetStatus(Cancelled)
				continue
			}

			if err != nil {
				e, ok := err.(Trouble)
				if ok {
					// Error implements the trouble interface so raise a trouble
					item.SetTrouble(e)

					continue
				}

				// Unhandled exception!
				return err
			} else {
				// Task finished successfully. Clear the troubles for this item
				item.ClearTrouble()
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
	OnComplete onComplete
	BaseTask
}

// Execute will utilise the baseTask.Execute method to run the task repeatedly
// in a worker work/wait loop
func (task *TitleTask) Execute(w worker.Worker) error {
	return task.executeTask(w, task.processTitle)
}

// processTroubleState will check if the queue item is troubled, and if so, will
// query the trouble for information about how the user wishes it to be resolved.
// This method will return an error if the processing fails. The second return (bool)
// indicates whether or not the item has been fully processed. Item trouble is cleared
// when this method returns
func (task *TitleTask) processTroubleState(queueItem *Item) (bool, error) {
	if queueItem.Trouble != nil {
		if trblCtx := queueItem.Trouble.ResolutionContext(); trblCtx != nil {
			// Check for the mandatory 'info' key.
			info, ok := trblCtx["info"]
			if !ok {
				return false, errors.New("resolution context is missing 'info' key as is therefore invalid - ignoring context")
			}

			// Assign the 'info' provided as the items TitleInfo and move on.
			titleInfo, ok := info.(*TitleInfo)
			if ok {
				queueItem.TitleInfo = titleInfo
				task.advance(queueItem)

				return true, nil
			}

			return false, errors.New("resolution contexts 'info' key contains an invalid value! Failed to cast to 'TitleInfo'")
		}
	}

	return false, nil
}

// Processes a given queueItem by filtering out irrelevant information from it's
// title, and finding relevant information such as the season, episode and resolution
func (task *TitleTask) processTitle(w worker.Worker, queueItem *Item) error {
	isComplete, err := task.processTroubleState(queueItem)
	if err != nil {
		taskLogger.Emit(logger.WARNING, "Unable to process items trouble state: %s\n", err.Error())
	}

	if !isComplete {
		if err := queueItem.FormatTitle(); err != nil {
			return &TitleTaskError{NewBaseTaskError(err.Error(), queueItem, TITLE_FAILURE)}
		}

		task.advance(queueItem)
	}

	return nil
}

// advances the item by advancing the stage of the item
func (task *TitleTask) advance(item *Item) {
	task.OnComplete(item)
}

// OmdbTask is the task responsible for querying to OMDB API for information
// about the queue item we've processed so far.
type OmdbTask struct {
	OmdbKey    string
	OnComplete onComplete
	BaseTask
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

// filterSearchItems accepts a slice of OmdbSearchItems and will use the 'strategy' provided
// to create a new slice containing only items that the strategy returned 'true' for.
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

// parse searches through the result from Omdb and tries to find just one search result
// to use based on filtering the results for their release date, type (movie/series) and
// title. If this fails, an error is returned (e.g. OmdbNoResultError, OmdbMultipleResultError)
func (result *OmdbSearchResult) parse(queueItem *Item, task *OmdbTask) (*OmdbSearchItem, error) {
	// Check the response by parsing the response, and total results.
	if !result.Response || result.Count == 0 {
		// Response from OMDB failed!
		return nil, &OmdbTaskError{NewBaseTaskError("Failed to parse OMDB result - response empty", queueItem, OMDB_NO_RESULT_FAILURE), nil}
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
			case 0, 1:
				// Invalid/no match
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
		return nil, &OmdbTaskError{NewBaseTaskError("parse failed: no valid choices remain after filtering", queueItem, OMDB_NO_RESULT_FAILURE), nil}
	} else if len(items) > 1 {
		return nil, &OmdbTaskError{NewBaseTaskError("parse failed: multiple choices remain after filtering", queueItem, OMDB_MULTIPLE_RESULT_FAILURE), items}
	}

	return items[0], nil
}

// Execute uses the provided baseTask.executeTask method to run this tasks
// work function in a work/wait worker loop
func (task *OmdbTask) Execute(w worker.Worker) error {
	return task.executeTask(w, func(w worker.Worker, queueItem *Item) error {
		isComplete, err := task.processTroubleState(queueItem)
		if err != nil {
			taskLogger.Emit(logger.WARNING, "Unable to process items trouble state: %s\n", err.Error())
		}

		if isComplete {
			return nil
		}

		return task.find(w, queueItem)
	})
}

// processTroubleState will check if the queue item is troubled, and if so, will
// query the trouble for information about how the user wishes it to be resolved.
// This method will return an error if the processing fails. The second return (bool)
// indicates whether or not the item has been fully processed. Trouble is cleared
// once this method returns.
func (task *OmdbTask) processTroubleState(queueItem *Item) (bool, error) {
	if queueItem.Trouble == nil {
		return false, nil
	}

	trblCtx := queueItem.Trouble.ResolutionContext()
	fetchId, omdbStruct, action := trblCtx["fetchId"], trblCtx["omdbStruct"], trblCtx["action"]
	if fetchId != nil {
		id, ok := fetchId.(string)
		if !ok {
			return false, errors.New("resolution context contains invalid 'fetchId' field (not string)")
		}

		result, err := task.fetch(id, queueItem)
		if err != nil {
			return false, err
		}

		queueItem.OmdbInfo = result
		task.advance(queueItem)

		return true, nil
	} else if omdbStruct != nil {
		info, ok := omdbStruct.(OmdbInfo)
		if !ok {
			return false, errors.New("resolution context contains invalid 'replacementStruct' field (not an OmdbInfo struct)")
		}

		queueItem.OmdbInfo = &info
		task.advance(queueItem)

		return true, nil
	} else if action != nil {
		actionVal, ok := action.(string)
		if !ok {
			return false, errors.New("resolution context contains invalid 'action' key (not a string)")
		} else if actionVal != "retry" {
			return false, fmt.Errorf("resolution context contains action with value '%s' which is invalid. Only 'retry' is permitted", actionVal)
		}

		return false, nil
	} else {
		return false, errors.New("resolution context contains none of acceptable fields (choiceId, imdbId, replacementStruct, action)")
	}
}

// search will perform a search query to OMDB and will return the result
// If multiple results are found a OmdbMultipleResultError is returned; if
// no results are found then an OmdbNoResultError is returned. If the request
// fails for another reason, an OmdbRequestError is returned.
func (task *OmdbTask) search(w worker.Worker, queueItem *Item) (*OmdbInfo, error) {
	// Peform the search
	res, err := http.Get(fmt.Sprintf(OMDB_API, "s", queueItem.TitleInfo.Title, task.OmdbKey))
	if err != nil {
		// Request exception
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("search failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("search failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
	}

	var searchResult OmdbSearchResult
	if err = json.Unmarshal(body, &searchResult); err != nil {
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("search failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
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
func (task *OmdbTask) fetch(imdbId string, queueItem *Item) (*OmdbInfo, error) {
	res, err := http.Get(fmt.Sprintf(OMDB_API, "i", imdbId, task.OmdbKey))
	if err != nil {
		// Request exception
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
	}

	var result OmdbInfo
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("fetch failed: %s", err.Error()), queueItem, OMDB_REQUEST_FAILURE), nil}
	}

	if !result.Response {
		return nil, &OmdbTaskError{NewBaseTaskError(fmt.Sprintf("fetch failed: OMDB response contained no data (%s)", result.Error), queueItem, OMDB_REQUEST_FAILURE), nil}
	}

	return &result, nil
}

// find is used to perform a search to Omdb using the title information stored inside
// of a QueueItem.
func (task *OmdbTask) find(w worker.Worker, queueItem *Item) error {
	if queueItem.TitleInfo == nil {
		return fmt.Errorf("cannot find OMDB info for queueItem (id: %v). TitleInfo missing", queueItem.ItemID)
	}

	res, err := task.search(w, queueItem)
	if err != nil {
		return err
	}

	queueItem.OmdbInfo = res
	task.advance(queueItem)
	return nil
}

// advance will push the stage of this queue item forward, and wakeup any workers
// for that stage
func (task *OmdbTask) advance(item *Item) {
	// Release the QueueItem by advancing it to the next pipeline stage
	task.OnComplete(item)
}

type commitHandler func(*Item) error
type DatabaseTask struct {
	OnComplete    onComplete
	CommitHandler commitHandler
	BaseTask
}

// Execute will utilise the baseTask.Execute method to run the task repeatedly
// in a worker work/wait loop
func (task *DatabaseTask) Execute(w worker.Worker) error {
	return task.executeTask(w, task.commitToDatabase)
}

// processTroubleState will check if the queue item is troubled, and if so, will
// query the trouble for information about how the user wishes it to be resolved.
// This method will return an error if the processing fails. The second return (bool)
// indicates whether or not the item has been fully processed. Item trouble is cleared
// when this method returns
func (task *DatabaseTask) processTroubleState(queueItem *Item) (bool, error) {
	if queueItem.Trouble != nil {
		if trblCtx := queueItem.Trouble.ResolutionContext(); trblCtx != nil {
			_, ok := trblCtx["retry"]
			if !ok {
				return false, nil
			}

			return false, errors.New("resolution contexts 'retry' key missing")
		}
	}

	return false, nil
}

// Processes a given queueItem by filtering out irrelevant information from it's
// title, and finding relevant information such as the season, episode and resolution
func (task *DatabaseTask) commitToDatabase(w worker.Worker, queueItem *Item) error {
	isComplete, err := task.processTroubleState(queueItem)
	if err != nil {
		taskLogger.Emit(logger.WARNING, "Unable to process items trouble state: %s\n", err.Error())
	}

	if isComplete {
		return nil
	}

	if err := task.CommitHandler(queueItem); err != nil {
		return &DatabaseTaskError{NewBaseTaskError(err.Error(), queueItem, DATABASE_FAILURE)}
	}

	task.advance(queueItem)
	return nil
}

// advances the item by advancing the stage of the item
func (task *DatabaseTask) advance(item *Item) {
	task.OnComplete(item)
}