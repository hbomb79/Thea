package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/worker"
)

type taskFn func(*worker.Worker, *QueueItem) error
type troubleResolver func(*Trouble, map[string]interface{}) error
type taskErrorHandler func(*QueueItem, error) error
type troubleTag int

const (
	TitleFailure troubleTag = iota
	OmdbResponseFailure
	OmdbMultipleOptions
	FormatError
)

// When a processor task encounters an error that requires
// user intervention to continue - a 'trouble' is raised.
// This trouble is raised, and resolved, via the 'Trouble'
// struct. This struct mainly acts as a way for the
// task to continue working on other items whilst
// keeping track of the trouble(s) that are pending
type Trouble struct {
	item     *QueueItem `json:"-"`
	args     map[string]string
	resolver troubleResolver
	tag      troubleTag
}

// validate accepts a map of arguments and checks to ensure
// that all the arguments required by this trouble instance
// are present. Returns an error if not.
func (trouble *Trouble) validate(args map[string]interface{}) error {
	return nil
}

// Resolve is a method that is used to initiate the resolution of
// a trouble instance. The args provided are first validated before
// being passed to the Trouble's 'resolver' for processing.
func (trouble *Trouble) Resolve(args map[string]interface{}) error {
	if err := trouble.validate(args); err != nil {
		return err
	}

	return nil
}

// Args returns the arguments required by this trouble
// in order to resolve this trouble instance.
func (trouble *Trouble) Args() map[string]string {
	return trouble.args
}

// Tag returns the 'tag' for this trouble, which is used by
// resolving functions to tell which type of trouble they've received.
func (trouble *Trouble) Tag() troubleTag {
	return trouble.tag
}

// Item returns the QueueItem that this trouble is attached to
func (trouble *Trouble) Item() *QueueItem {
	return trouble.item
}

// baseTask is a struct that implements very little functionality, and is used
// to facilitate the other task types implemented in this file. This struct
// mainly just handled some repeated code definitions, such as the basic
// work/wait worker loop, and raising and notifying troubles
type baseTask struct {
	troubles     []*Trouble
	assignedItem *QueueItem
}

// raiseTrouble is a helper method used to push a new trouble
// in to the slice for this task
func (task *baseTask) raiseTrouble(proc *Processor, trouble *Trouble) {
	trouble.Item().RaiseTrouble(trouble)
	task.troubles = append(task.troubles, trouble)

	task.notifyTrouble(proc, trouble)
}

// notifyTrouble sends a ProcessorUpdate to the processor which
// is likely then pushed along to any connected clients on
// the web socket
func (task *baseTask) notifyTrouble(proc *Processor, trouble *Trouble) {
	proc.PushUpdate(&ProcessorUpdate{
		Title:   "TROUBLE",
		Context: processorUpdateContext{Trouble: trouble, QueueItem: trouble.Item()},
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

// Processes a given queueItem, filtering out irrelevant information
func (task *TitleTask) processTitle(w *worker.Worker, queueItem *QueueItem) error {
	if err := queueItem.FormatTitle(); err != nil {
		return task.handleError(queueItem, err)
	}

	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(queueItem)

	// Wakeup any pipeline workers that are sleeping
	task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
	return nil
}

// TODO
func (task *TitleTask) handleError(item *QueueItem, err error) error {

	// Not an error we want to raise trouble for.
	return err
}

// TODO
func (task *TitleTask) resolveTrouble(args map[string]interface{}) error {
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
func (task *OmdbTask) resolveTrouble(args map[string]interface{}) error {
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
func (task *FormatTask) resolveTrouble(args map[string]interface{}) error {
	return nil
}

type TitleFormatError struct {
	item    *QueueItem
	message string
}

func (e TitleFormatError) Error() string {
	return fmt.Sprintf("failed to format title(%v) - %v", e.item.Name, e.message)
}
