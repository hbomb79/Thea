package processor

import (
	"fmt"

	"github.com/hbomb79/TPA/worker"
	"github.com/mitchellh/mapstructure"
)

type TroubleType = int

const (
	TITLE_FAILURE TroubleType = iota
	OMDB_NO_RESULT_FAILURE
	OMDB_MULTIPLE_RESULT_FAILURE
	OMDB_REQUEST_FAILURE
	FFMPEG_FAILURE
)

// TitleTaskError is an error raised during the processing of the
// title task
type TitleTaskError struct {
	Message   string `json:"message"`
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

	ex.queueItem.TitleInfo = &result
	ex.queueItem.ResetTrouble()
	ex.task.advance(ex.queueItem)

	return nil
}

// Item returns the QueueItem that is attached to this trouble
func (ex TitleTaskError) Item() *QueueItem {
	return ex.queueItem
}

// Type returns the type of trouble case this is - predominantly for
// code using the websocket API to tell what trouble it is, and how to deal
// with it
func (ex TitleTaskError) Type() TroubleType {
	return TITLE_FAILURE
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

// Type returns the type of trouble case this is - predominantly for
// code using the websocket API to tell what trouble it is, and how to deal
// with it
func (ex OmdbNoResultError) Type() TroubleType {
	return OMDB_NO_RESULT_FAILURE
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

	choiceFloat, ok := v.(float64)
	if !ok {
		return fmt.Errorf("Faield to resolve OmdbMultipleResultError - Bad value for choice (int) key in args (not a number)")
	}

	choice := int(choiceFloat)
	if !ok || len(ex.Choices)-1 < choice {
		return fmt.Errorf("Faield to resolve OmdbMultipleResultError - Bad value for choice (int) key in args (not in range)")
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

// Type returns the type of trouble case this is - predominantly for
// code using the websocket API to tell what trouble it is, and how to deal
// with it
func (ex OmdbMultipleResultError) Type() TroubleType {
	return OMDB_MULTIPLE_RESULT_FAILURE
}

// OmdbRequestError is an error/trouble type that is raised when a request to
// the OMDB api fails for an unknown reason - likely not related to the thing
// we're searching for(e.g. a bad response from OMDB that is not JSON)
type OmdbRequestError struct {
	Message   string `json:"message"`
	queueItem *QueueItem
	task      *OmdbTask
}

// Error is implemented to satisfy the 'error' interface, and simply returns
// a string describing the reason for this trouble
func (err OmdbRequestError) Error() string {
	return fmt.Sprintf("Omdb task error: %v", err.Message)
}

// Args returns a map of the arguments required, and their types (string, int, bool, etc)
func (err OmdbRequestError) Args() map[string]string {
	return map[string]string{}
}

// Resolve will attempt to resolve this issue by resetting the queue items trouble
// state and awakening any workers that are sleeping in the next stage - this essentially
// just means the queue item will be retried by a worker when one is available.
func (err OmdbRequestError) Resolve(args map[string]interface{}) error {
	err.queueItem.ResetTrouble()
	err.task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
	return nil
}

// Item returns the queue item attached to this trouble
func (ex OmdbRequestError) Item() *QueueItem {
	return ex.queueItem
}

// Type returns the type of trouble case this is - predominantly for
// code using the websocket API to tell what trouble it is, and how to deal
// with it
func (ex OmdbRequestError) Type() TroubleType {
	return OMDB_REQUEST_FAILURE
}

// FormatTaskError is an error/trouble type that is raised when ffmpeg/ffprobe encounters
// an error. The only real solution to this is to retry because an error of this type
// indicates that a glitch occurred, or that the input file is malformed.
type FormatTaskError struct {
	Message   string `json:"message"`
	queueItem *QueueItem
	task      *FormatTask
}

// Error is implemented to satisfy the 'error' type. It returns a string that
// describes the reason for this trouble
func (ex FormatTaskError) Error() string {
	return fmt.Sprintf("Format task trouble: %v", ex.Message)
}

// Args returns a map of the arguments required to resolve this trouble. Each key is the argument
// name, and each value is a string representation of the type (bool, string, int, etc)
func (ex FormatTaskError) Args() map[string]string {
	return map[string]string{}
}

// Resolve will attempt to resolve this trouble by resetting the queue items status
// and waking up any sleeping workers in the format worker pool. This essentially means
// that a worker will try this queue item again. Repeated failures likely means the input
// file is bad.
func (ex FormatTaskError) Resolve(map[string]interface{}) error {
	ex.queueItem.ResetTrouble()
	ex.task.proc.WorkerPool.WakeupWorkers(worker.Format)
	return nil
}

// Item returns the queue item attached to this trouble
func (ex FormatTaskError) Item() *QueueItem {
	return ex.queueItem
}

// Type returns the type of trouble case this is - predominantly for
// code using the websocket API to tell what trouble it is, and how to deal
// with it
func (ex FormatTaskError) Type() TroubleType {
	return FFMPEG_FAILURE
}
