package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/ilyakaznacheev/cleanenv"
)

// Each stage represents a certain stage in the pipeline
type PipelineStage int

// When a QueueItem is initially added, it should be of stage Import,
// each time a worker works on the task it should increment it's
// Stage (Title->Omdb->etc..) and set it's Status to 'Pending'
// to allow a worker to pick the item from the Queue
const (
	Import PipelineStage = iota
	Title
	Omdb
	Format
	Finish
)

// TPAConfig is the struct used to contain the
// various user config supplied by file, or
// manually inside the code.
type TPAConfig struct {
	Concurrent ConcurrentConfig `yaml:"concurrency"`
	Format     FormatterConfig  `yaml:"formatter"`
	Database   DatabaseConfig   `yaml:"database"`
}

// ConcurrentConfig is a subset of the configuration that focuses
// only on the concurrency related configs (number of threads to use
// for each stage of the pipeline)
type ConcurrentConfig struct {
	Import int `yaml:"import_threads"`
	Title  int `yaml:"title_threads"`
	OMBD   int `yaml:"omdb_threads"`
	Format int `yaml:"ffmpeg_threads"`
}

// FormatterConfig is the 'misc' container of the configuration, encompassing configuration
// not covered by either 'ConcurrentConfig' or 'DatabaseConfig'. Mainly configuration
// paramters for the FFmpeg executable.
type FormatterConfig struct {
	ImportPath         string `yaml:"import_path"`
	OutputPath         string `yaml:"output_path"`
	CacheFile          string `yaml:"cache_file"`
	TargetFormat       string `yaml:"target_format"`
	ImportDirTickDelay int    `yaml:"import_polling_delay"`
	FfmpegBinaryPath   string `yaml:"ffmpeg_binary"`
	FfprobeBinaryPath  string `yaml:"ffprobe_binary"`
}

// DatabaseConfig is a subset of the configuration focusing solely
// on database connection items
type DatabaseConfig struct {
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	Name       string `yaml:"name"`
	Host       string `yaml:"host"`
	Port       string `yaml:"port"`
	OmdbKey    string `yaml:"omdb_api_key"`
	OmdbApiUrl string `yaml:"omdb_api_url"`
}

// Loads a configuration file formatted in YAML in to a
// TPAConfig struct ready to be passed to Processor
func (config *TPAConfig) LoadFromFile(configPath string) error {
	err := cleanenv.ReadConfig(configPath, config)
	if err != nil {
		return errors.New(fmt.Sprintf("Cannot load configuration for ProcessorConfig -  %v\n", err.Error()))
	}

	return nil
}

// The Processor struct contains all the context
// for the running instance of this program. It stores
// the queue of items, the pool of workers that are
// processing the queue, and the users configuration
type Processor struct {
	Config     *TPAConfig
	Queue      *ProcessorQueue
	WorkerPool *WorkerPool
	Negotiator Negotiator
}

type Negotiator interface {
	OnProcessorUpdate(update *ProcessorUpdate)
}

type ProcessorUpdate struct {
	Title   string
	Context struct {
		QueueItem *QueueItem
		Trouble   *QueueTrouble
	}
}

type TitleFormatError struct {
	item    *QueueItem
	message string
}

func (e TitleFormatError) Error() string {
	return fmt.Sprintf("failed to format title(%v) - %v", e.item.Name, e.message)
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func New() *Processor {
	p := &Processor{
		Queue: &ProcessorQueue{
			Items: make([]*QueueItem, 0),
		},
	}

	p.WorkerPool = NewWorkerPool()
	return p
}

// Returns the processor provided after setting the Config
// to the value provided.
func (p *Processor) WithConfig(cfg *TPAConfig) *Processor {
	p.Config = cfg

	return p
}

// Returns the processor provided after setting the Negotiator
// to the value provided.
func (p *Processor) WithNegotiator(n Negotiator) *Processor {
	p.Negotiator = n

	return p
}

// Called when something has changed with the processor state,
// and we want our attached Negotiator to be alerted
func (p *Processor) PushUpdate(update *ProcessorUpdate) {
	if p.Negotiator == nil {
		return
	}

	p.Negotiator.OnProcessorUpdate(update)
}

// Start will start the workers inside the WorkerPool
// responsible for the various tasks inside the program
// This includes: HTTP RESTful API (NYI), user interaction (NYI),
// import directory polling, title formatting (NYI), OMDB querying (NYI),
// and the FFMPEG formatting (NYI)
// This method will wait on the WaitGroup attached to the WorkerPool
func (p *Processor) Start() error {
	tickInterval := time.Duration(p.Config.Format.ImportDirTickDelay * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}

	importWakeupChan := make(chan int)
	go func(target chan int) {
		source := time.NewTicker(tickInterval).C
		for {
			target <- 1
			<-source
		}
	}(importWakeupChan)

	// Start some workers in the pool to handle various tasks
	p.WorkerPool.NewWorkers(p.Config.Concurrent.Import, "Importer", p.pollingWorkerTask, importWakeupChan, Import)
	p.WorkerPool.NewWorkers(p.Config.Concurrent.Title, "TitleFormatter", p.titleWorkerTask, make(chan int), Title)
	p.WorkerPool.NewWorkers(p.Config.Concurrent.OMBD, "OMDBQuerant", p.networkWorkerTask, make(chan int), Omdb)
	p.WorkerPool.NewWorkers(p.Config.Concurrent.Format, "FFMPEG", p.formatterWorkerTask, make(chan int), Format)
	p.WorkerPool.StartWorkers()

	// Wait for all to finish
	p.WorkerPool.Wg.Wait()
	return nil
}

// PollInputSource will check the source input directory (from p.Config)
// pass along the files it finds to the p.Queue to be inserted if not present.
func (p *Processor) PollInputSource() (newItemsFound int, err error) {
	newItemsFound = 0
	walkFunc := func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.Panicf("PollInputSource failed - %v\n", err.Error())
		}

		if !dir.IsDir() {
			v, err := dir.Info()
			if err != nil {
				log.Panicf("Failed to get FileInfo for path %v - %v\n", path, err.Error())
			}

			if isNew := p.Queue.HandleFile(path, v); isNew {
				newItemsFound++
			}
		}

		return nil
	}

	err = filepath.WalkDir(p.Config.Format.ImportPath, walkFunc)
	return
}

func (p *Processor) RaiseTrouble(item *QueueItem, trouble *QueueTrouble) error {
	err := item.RaiseTrouble(trouble)
	if err != nil {
		return err
	}

	// Broadcast notification of new trouble
	p.PushUpdate(&ProcessorUpdate{
		Title: "trouble",
		Context: struct {
			QueueItem *QueueItem
			Trouble   *QueueTrouble
		}{item, trouble},
	})
	return nil
}

// pollingWorkerTask is a WorkerTask that is responsible
// for polling the import directory for new items to
// add to the Queue
func (p *Processor) pollingWorkerTask(w *Worker) error {
	for {
		// Wait for wakeup tick
		if isAlive := w.sleep(); !isAlive {
			return nil
		}

		// Do work
		if notify, err := p.PollInputSource(); err != nil {
			return err
		} else if notify > 0 {
			p.WorkerPool.WakeupWorkers(Title)
		}
	}
}

// titleWorkerTask is a WorkerTask that will
// pick a new item from the queue that needs it's
// title formatted to remove superfluous information.
func (p *Processor) titleWorkerTask(w *Worker) error {
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := p.Queue.Pick(w.pipelineStage)
			if queueItem == nil {
				// No item, break inner loop and sleep
				break workLoop
			}

			// Do our work..
			if err := queueItem.FormatTitle(); err != nil {
				if _, ok := err.(TitleFormatError); ok {
					// We caught an error, but it's a recoverable error - raise a trouble
					// sitation for this queue item to request user interaction to resolve it
					// queueItem.RaiseTrouble(&QueueTrouble{err.Error(), Error, nil})
					continue
				} else {
					// Unknown error
					return err
				}
			} else {
				// Release the QueueItem by advancing it to the next pipeline stage
				p.Queue.AdvanceStage(queueItem)

				// Wakeup any pipeline workers that are sleeping
				p.WorkerPool.WakeupWorkers(Omdb)
			}
		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}

// networkWorkerTask will pick an item from the queue that
// needs some stats found from OMDB. Stats include the genre,
// rating, runtime, etc. This worker will attempt to find the
// item at OMDB, and if it fails it will try to refine the
// title until it can't anymore - in which case the Queue item
// will have a trouble state raised.
func (p *Processor) networkWorkerTask(w *Worker) error {
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := p.Queue.Pick(w.pipelineStage)
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
			baseApi := fmt.Sprintf(p.Config.Database.OmdbApiUrl, p.Config.Database.OmdbKey, queueItem.TitleInfo.Title)
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
			p.Queue.AdvanceStage(queueItem)

			// Wakeup any sleeping workers in next stage
			p.WorkerPool.WakeupWorkers(Format)
		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}

// formatterWorkerTask is a WorkerTask that can be
// provided to a Worker that will pick a pending item
// from the queue and will execute FFmpeg to format
// the file while automatically storing progress
// on the QueueItem being processed
func (p *Processor) formatterWorkerTask(w *Worker) error {
	for {
	workLoop:
		for {
			// Check if work can be done...
			queueItem := p.Queue.Pick(w.pipelineStage)
			if queueItem == nil {
				break workLoop
			}

			outputFormat := p.Config.Format.TargetFormat
			ffmpegOverwrite := true
			ffmpegOpts, ffmpegCfg := &ffmpeg.Options{
				OutputFormat: &outputFormat,
				Overwrite:    &ffmpegOverwrite,
			}, &ffmpeg.Config{
				ProgressEnabled: true,
				FfmpegBinPath:   p.Config.Format.FfmpegBinaryPath,
				FfprobeBinPath:  p.Config.Format.FfprobeBinaryPath,
			}

			itemOutputPath := fmt.Sprintf("%s.%s", queueItem.TitleInfo.OutputPath(), outputFormat)
			itemOutputPath = filepath.Join(p.Config.Format.OutputPath, itemOutputPath)
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
			p.Queue.AdvanceStage(queueItem)
		}

		// If no work, wait for wakeup
		if isAlive := w.sleep(); !isAlive {
			return nil
		}
	}
}
