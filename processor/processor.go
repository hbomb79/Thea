package processor

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/hbomb79/TPA/worker"
	"github.com/ilyakaznacheev/cleanenv"
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
	WorkerPool *worker.WorkerPool
	Negotiator Negotiator
}

type Negotiator interface {
	OnProcessorUpdate(update *ProcessorUpdate)
}

type processorUpdateContext struct {
	QueueItem *QueueItem
	Trouble   Trouble
}
type ProcessorUpdate struct {
	Title   string
	Context processorUpdateContext
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func New() *Processor {
	p := &Processor{
		Queue: &ProcessorQueue{
			Items: make([]*QueueItem, 0),
		},
	}

	p.WorkerPool = worker.NewWorkerPool()
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

	go func(target <-chan time.Time) {
		p.PollInputSource()
		p.WorkerPool.WakeupWorkers(worker.Title)
	}(time.NewTicker(tickInterval).C)

	// Start some workers in the pool to handle various tasks

	// TODO: This is *incredibly* ugly code.. it makes me
	// want to cry. I couldn't figure out any other way
	// to solve this without the Go Generics soon to come.
	// The problem is that I can't simply pass something
	// like []*ImportTask to a method that is expecting
	// a slice like []worker.WorkerTaskMeta as the two
	// types are different.
	threads, workers := p.Config.Concurrent, make([]*worker.Worker, 0)
	for i := 0; i < threads.Title; i++ {
		workers = append(workers, worker.NewWorker(fmt.Sprintf("Title:%v", i), &TitleTask{proc: p}, worker.Title, make(chan int)))
	}
	for i := 0; i < threads.OMBD; i++ {
		workers = append(workers, worker.NewWorker(fmt.Sprintf("Omdb:%v", i), &OmdbTask{proc: p}, worker.Omdb, make(chan int)))
	}
	for i := 0; i < threads.Format; i++ {
		workers = append(workers, worker.NewWorker(fmt.Sprintf("Format:%v", i), &FormatTask{proc: p}, worker.Format, make(chan int)))
	}

	p.WorkerPool.PushWorker(workers...)
	p.WorkerPool.StartWorkers()
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
