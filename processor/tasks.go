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
type taskErrorHandler func(*QueueItem, error) error

func notifyTrouble(proc *Processor, item *QueueItem) {
	proc.PushUpdate(&ProcessorUpdate{
		Title: "trouble",
		//TODO
		Context: processorUpdateContext{QueueItem: item},
	})
}

func executeTask(w *worker.Worker, proc *Processor, fn taskFn, errHandler taskErrorHandler) error {
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

// TODO
type baseTask struct {
	troubleArgs  map[string]string
	assignedItem *QueueItem
}

func (task *baseTask) TroubleArgs() map[string]string {
	return task.troubleArgs
}

// TODO
type TitleTask struct {
	proc *Processor
	baseTask
}

func (task *TitleTask) Execute(w *worker.Worker) error {
	return executeTask(w, task.proc, task.ProcessTitle, task.RaiseTrouble)
}

func (task *TitleTask) ProcessTitle(w *worker.Worker, queueItem *QueueItem) error {
	if err := queueItem.FormatTitle(); err != nil {
		return task.RaiseTrouble(queueItem, err)
	}

	// Release the QueueItem by advancing it to the next pipeline stage
	task.proc.Queue.AdvanceStage(queueItem)

	// Wakeup any pipeline workers that are sleeping
	task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
	return nil
}

func (task *TitleTask) RaiseTrouble(item *QueueItem, err error) error {
	// TODO Handle

	// Not an error we want to raise trouble for.
	return err
}

func (task *TitleTask) ResolveTrouble(args map[string]interface{}) error {
	return nil
}

// TODO
type OmdbTask struct {
	proc *Processor
	baseTask
}

func (task *OmdbTask) Execute(w *worker.Worker) error {
	return executeTask(w, task.proc, task.Query, task.RaiseTrouble)
}

func (task *OmdbTask) Query(w *worker.Worker, queueItem *QueueItem) error {
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

func (task *OmdbTask) RaiseTrouble(item *QueueItem, err error) error {
	// TODO Handle

	// Not an error we want to raise trouble for.
	return err
}

func (task *OmdbTask) ResolveTrouble(args map[string]interface{}) error {
	return nil
}

// TODO
type FormatTask struct {
	proc *Processor
	baseTask
}

func (task *FormatTask) Execute(w *worker.Worker) error {
	return executeTask(w, task.proc, task.Format, task.RaiseTrouble)
}

func (task *FormatTask) Format(w *worker.Worker, queueItem *QueueItem) error {
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

func (task *FormatTask) RaiseTrouble(item *QueueItem, err error) error {
	// TODO Handle

	// Not an error we want to raise trouble for.
	return err
}

func (task *FormatTask) ResolveTrouble(args map[string]interface{}) error {
	return nil
}

type TitleFormatError struct {
	item    *QueueItem
	message string
}

func (e TitleFormatError) Error() string {
	return fmt.Sprintf("failed to format title(%v) - %v", e.item.Name, e.message)
}
