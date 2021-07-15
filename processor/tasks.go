package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"reflect"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/worker"
	"github.com/mitchellh/mapstructure"
)

type taskFn func(*worker.Worker, *QueueItem) error

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
	if true {
		return TitleFormatError{queueItem, "Help me!"}
	}

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

// OmdbTask is the task responsible for querying to OMDB API for information
// about the queue item we've processed so far.
type OmdbTask struct {
	proc *Processor
	baseTask
}

// Execute uses the provided baseTask.executeTask method to run this tasks
// work function in a work/wait worker loop
func (task *OmdbTask) Execute(w *worker.Worker) error {
	return task.executeTask(w, task.proc, task.query, task.handleError)
}

// query sends an API request to the OMDB api, searching for information
// about the queue item provided
func (task *OmdbTask) query(w *worker.Worker, queueItem *QueueItem) error {
	// Ensure the previous pipeline actually provided information
	// in the TitleInfo struct.
	if queueItem.TitleInfo == nil {
		return errors.New("QueueItem contains no title information")
	}

	// Form our API request
	baseApi := fmt.Sprintf(task.proc.Config.Database.OmdbApiUrl, task.proc.Config.Database.OmdbKey, queueItem.TitleInfo.Title)
	res, err := http.Get(baseApi)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Read all the bytes from the response
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// Unmarshal the JSON content in to our OmdbInfo struct
	var info OmdbInfo
	if err = json.Unmarshal(body, &info); err != nil {
		return err
	}

	// Store OMDB result in QueueItem
	queueItem.OmdbInfo = &info
	if !queueItem.OmdbInfo.Response {
		return errors.New("Invalid response from OMDB - 'Response' was false")
	}

	// Advance our item to the next stage
	task.proc.Queue.AdvanceStage(queueItem)

	// Wakeup any sleeping workers in next stage
	task.proc.WorkerPool.WakeupWorkers(worker.Format)
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
