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
	OmdbKey    string           `yaml:"omdb_api_key"`
	CachePath  string           `yaml:"cache_path"`
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
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
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
	Config         *TPAConfig
	Queue          *processorQueue
	WorkerPool     *worker.WorkerPool
	Negotiator     Negotiator
	UpdateChan     chan int
	pendingUpdates map[int]bool
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
func NewProcessor() *Processor {
	return &Processor{
		WorkerPool:     worker.NewWorkerPool(),
		UpdateChan:     make(chan int),
		pendingUpdates: make(map[int]bool),
	}
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

// Start will start the workers inside the WorkerPool
// responsible for the various tasks inside the program
// This method will wait on the WaitGroup attached to the WorkerPool
func (p *Processor) Start() error {
	p.Queue = NewProcessorQueue(p.Config.CachePath)

	tickInterval := time.Duration(p.Config.Format.ImportDirTickDelay * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}

	go func() {
		ticker := time.NewTicker(time.Second * 1).C
		for {
			select {
			case _ = <-ticker:
				p.submitUpdates()
			}
		}
	}()

	go func(target <-chan time.Time) {
		p.SynchroniseQueue()
		p.WorkerPool.WakeupWorkers(worker.Title)
	}(time.NewTicker(tickInterval).C)

	go p.listenForUpdates()

	// Start some workers in the pool to handle various tasks
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

// SynchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
// and removed from the queue.
func (p *Processor) SynchroniseQueue() error {
	presentItems, err := p.DiscoverItems()
	if err != nil {
		return err
	}

	p.InjestQueue(presentItems)

	p.Queue.Filter(func(queue *processorQueue, key int, item *QueueItem) bool {
		if _, ok := presentItems[item.Path]; !ok {
			item.Cancel()

			return false
		}

		return true
	})

	p.PruneQueueCache()

	return nil
}

// DiscoverItems will walk through the import directory and construct a map
// of all the items inside the import directory (or any nested directories).
// The key of the map is the path, and the value contains the FileInfo
func (p *Processor) DiscoverItems() (map[string]fs.FileInfo, error) {
	presentItems := make(map[string]fs.FileInfo, 0)
	err := filepath.WalkDir(p.Config.Format.ImportPath, func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dir.IsDir() {
			v, err := dir.Info()
			if err != nil {
				return err
			}

			presentItems[path] = v
		}

		return nil
	})

	if err != nil {
		return nil, errors.New("Failed to discover items for injestion: " + err.Error())
	}

	return presentItems, nil
}

// InjestQueue will check the input source directory for files, and
// add them to the Queue
func (p *Processor) InjestQueue(presentItems map[string]fs.FileInfo) error {
	for path, info := range presentItems {
		if e := p.Queue.Push(NewQueueItem(info.Name(), path, p)); e != nil {
			fmt.Printf("[Processor] (!) Ignoring injestable item - " + e.Error())
		}
	}

	return nil
}

func (p *Processor) PruneQueueCache() {
	// TODO
}

func (p *Processor) listenForUpdates() {
	if p.Negotiator == nil {
		fmt.Printf("[Processor] (!) Processor began to listen for updates for transmission, however no Negotiator is attached. Aborting.\n")
		return
	}

	for {
		update, ok := <-p.UpdateChan
		if !ok {
			break
		}

		p.pendingUpdates[update] = true
	}
}

func (p *Processor) submitUpdates() {
	for k := range p.pendingUpdates {
		queueItem := p.Queue.FindById(k)
		p.Negotiator.OnProcessorUpdate(&ProcessorUpdate{
			Title: queueItem.StatusLine,
			Context: processorUpdateContext{
				QueueItem: queueItem,
				Trouble:   queueItem.Trouble,
			},
		})

		delete(p.pendingUpdates, k)
	}
}
