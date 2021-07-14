package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/worker"
)

func notifyTrouble(proc *Processor, item *QueueItem) {
	proc.PushUpdate(&ProcessorUpdate{
		Title: "trouble",
		//TODO
		Context: processorUpdateContext{QueueItem: item},
	})
}

// TODO
type baseTask struct {
	troubleArgs map[string]string
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
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := task.proc.Queue.Pick(w.Stage())
			if queueItem == nil {
				// No item, break inner loop and sleep
				break workLoop
			}

			// Do our work..
			if err := queueItem.FormatTitle(); err != nil {
				if _, ok := err.(TitleFormatError); ok {
					// We caught an error, but it's a recoverable error - raise a trouble
					// sitation for this queue item to request user interaction to resolve it
					//TODO Raise trouble
					continue
				} else {
					// Unknown error
					return err
				}
			} else {
				// Release the QueueItem by advancing it to the next pipeline stage
				task.proc.Queue.AdvanceStage(queueItem)

				// Wakeup any pipeline workers that are sleeping
				task.proc.WorkerPool.WakeupWorkers(worker.Omdb)
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.Sleep(); !isAlive {
			return nil
		}
	}
}

func (task *TitleTask) RaiseTrouble(err error) error {
	return nil
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
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := task.proc.Queue.Pick(w.Stage())
			if queueItem == nil {
				break workLoop
			}

			// Ensure the previous pipeline actually provided information
			// in the TitleInfo struct.
			if queueItem.TitleInfo == nil {
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	"Unable to process queue item for OMDB processing as no title information is available. Previous stage of pipelined must have failed unexpectedly.",
				// 	Error,
				// 	nil,
				// })

				continue
			}

			// Form our API request
			baseApi := fmt.Sprintf(task.proc.Config.Database.OmdbApiUrl, task.proc.Config.Database.OmdbKey, queueItem.TitleInfo.Title)
			res, err := http.Get(baseApi)
			if err != nil {
				// HTTP request error
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	"Failed to fetch OMDB information for QueueItem - " + err.Error(),
				// 	Error,
				// 	nil,
				// })

				continue
			}
			defer res.Body.Close()

			// Read all the bytes from the response
			body, err := io.ReadAll(res.Body)
			if err != nil {
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	"Failed to read OMDB information for QueueItem - " + err.Error(),
				// 	Error,
				// 	nil,
				// })

				continue
			}

			// Unmarshal the JSON content in to our OmdbInfo struct
			var info OmdbInfo
			if err = json.Unmarshal(body, &info); err != nil {
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	"Failed to unmarshal JSON response from OMDB - " + err.Error(),
				// 	Error,
				// 	nil,
				// })

				continue
			}

			// Store OMDB result in QueueItem
			queueItem.OmdbInfo = &info
			if !queueItem.OmdbInfo.Response {
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	"OMDB response failed - " + queueItem.OmdbInfo.Error,
				// 	Error,
				// 	nil,
				// })

				continue
			}

			// Advance our item to the next stage
			task.proc.Queue.AdvanceStage(queueItem)

			// Wakeup any sleeping workers in next stage
			task.proc.WorkerPool.WakeupWorkers(worker.Format)
		}

		// If no work, wait for wakeup
		if isAlive := w.Sleep(); !isAlive {
			return nil
		}
	}
}

func (task *OmdbTask) RaiseTrouble(err error) error {
	return nil
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
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := task.proc.Queue.Pick(w.Stage())
			if queueItem == nil {
				break workLoop
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
				// queueItem.RaiseTrouble(&QueueTrouble{
				// 	err.Error(),
				// 	Error,
				// 	nil,
				// })

				continue
			}

			for v := range progress {
				log.Printf("[Progress] %#v\n", v)
			}

			// Advance our item to the next stage
			task.proc.Queue.AdvanceStage(queueItem)
		}

		// If no work, wait for wakeup
		if isAlive := w.Sleep(); !isAlive {
			return nil
		}
	}
}

func (task *FormatTask) RaiseTrouble(err error) error {
	return nil
}

func (task *FormatTask) ResolveTrouble(args map[string]interface{}) error {
	return nil
}
