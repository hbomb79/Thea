package processor

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/profile"
	"github.com/hbomb79/TPA/worker"
	"github.com/ilyakaznacheev/cleanenv"
)

// TPAConfig is the struct used to contain the
// various user config supplied by file, or
// manually inside the code.
type TPAConfig struct {
	Concurrent       ConcurrentConfig `yaml:"concurrency"`
	Format           FormatterConfig  `yaml:"formatter"`
	Database         DatabaseConfig   `yaml:"database"`
	OmdbKey          string           `yaml:"omdb_api_key"`
	CachePath        string           `yaml:"cache_path"`
	ProfileStorePath string           `yaml:"profile_store_path"`
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
		return fmt.Errorf("failed to load configuration for ProcessorConfig - %v", err.Error())
	}

	return nil
}

// The Processor struct contains all the context
// for the running instance of this program. It stores
// the queue of items, the pool of workers that are
// processing the queue, and the users configuration
type Processor struct {
	Config             *TPAConfig
	Queue              *processorQueue
	WorkerPool         *worker.WorkerPool
	Negotiator         Negotiator
	UpdateChan         chan int
	pendingUpdates     map[int]bool
	Profiles           profile.ProfileList
	FfmpegCommander    Commander
	KnownFfmpegOptions map[string]string
}

type Negotiator interface {
	OnProcessorUpdate(update *ProcessorUpdate)
}

type processorUpdateType = int

const (
	ITEM_UPDATE processorUpdateType = iota
	QUEUE_UPDATE
	PROFILE_UPDATE
)

type ProcessorUpdate struct {
	UpdateType   processorUpdateType
	QueueItem    *QueueItem
	ItemPosition int
	ItemId       int
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func NewProcessor() *Processor {
	opts, err := profile.ToArgsMap(&ffmpeg.Options{})
	if err != nil {
		fmt.Printf("[Processor] (!) Failed to initialise map of known FFMPEG options!")
	}

	return &Processor{
		WorkerPool:         worker.NewWorkerPool(),
		UpdateChan:         make(chan int),
		pendingUpdates:     make(map[int]bool),
		KnownFfmpegOptions: opts,
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
	p.Profiles = profile.NewList(p.Config.ProfileStorePath)

	tickInterval := time.Duration(p.Config.Format.ImportDirTickDelay * int(time.Second))
	if tickInterval <= 0 {
		log.Panic("Failed to start PollingWorker - TickInterval is non-positive (make sure 'import_polling_delay' is set in your config)")
	}

	go func(target <-chan time.Time) {
		for {
			p.SynchroniseQueue()

			<-target
		}
	}(time.NewTicker(tickInterval).C)

	p.FfmpegCommander = NewCommander(p)
	p.FfmpegCommander.SetWindowSize(2)
	p.FfmpegCommander.SetThreadPoolSize(8)

	go p.FfmpegCommander.Start()
	go p.handleUpdateStream()
	go p.handleItemModtimes()

	p.WorkerPool.PushWorker(worker.NewWorker("Title_Parser", &TitleTask{proc: p}, worker.Title, make(chan int)))
	p.WorkerPool.PushWorker(worker.NewWorker("OMDB_Handler", &OmdbTask{proc: p}, worker.Omdb, make(chan int)))
	p.WorkerPool.StartWorkers()
	p.WorkerPool.Wg.Wait()

	return nil
}

// SynchroniseQueue will first discover all items inside the import directory,
// synchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
func (p *Processor) SynchroniseQueue() error {
	// Reload the queues cache so that our exlusion list
	// is up to date (in case the cache was deleted or edited externally)
	p.Queue.cache.Load()

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
		if e := p.Queue.Push(NewQueueItem(info, path, p)); e != nil {
			fmt.Printf("[Processor] (!) Ignoring injestable item - %v\n", e.Error())
		}
	}

	return nil
}

func (p *Processor) PruneQueueCache() {
	// TODO
}

func (p *Processor) handleItemModtimes() {
	ticker := time.NewTicker(time.Second * 5).C
	checkModtime := func(q *processorQueue, idx int, item *QueueItem) bool {
		if item.Stage != worker.Import {
			return false
		}

		info, err := os.Stat(item.Path)
		if err != nil {
			fmt.Printf("[Processor] (!) Failed to get file info for %v during import stage: %v\n", item.Path, err.Error())
			return false
		}

		if time.Since(info.ModTime()) > time.Minute*2 {
			q.AdvanceStage(item)
			fmt.Printf("[Processor] (O) Item %v passed import checks - now in Title stage\n", item.Name)
		}

		return false
	}

main:
	for {
		_, ok := <-ticker
		if !ok {
			break main
		}

		p.Queue.ForEach(checkModtime)
	}
}

func (p *Processor) handleUpdateStream() {
	if p.Negotiator == nil {
		fmt.Printf("[Processor] (!) Processor began to listen for updates for transmission, however no Negotiator is attached. Aborting.\n")
		return
	}

	ticker := time.NewTicker(time.Millisecond * 500).C
main:
	for {
		select {
		case update, ok := <-p.UpdateChan:
			if !ok {
				// Channel closed
				break main
			}

			if update == -1 {
				// -1 update ID indicates a fundamental change to the queue, rather than
				// a particular item. Send out a processor update, which will tell all
				// connected clients to INVALIDATE their current queue index, and refetch from the server
				p.Negotiator.OnProcessorUpdate(&ProcessorUpdate{QUEUE_UPDATE, nil, -1, -1})
			} else if update == -2 {
				p.Profiles.Save()
				p.Negotiator.OnProcessorUpdate(&ProcessorUpdate{PROFILE_UPDATE, nil, -1, -1})
			} else {
				p.pendingUpdates[update] = true
			}

			p.wakeupWorkers()
		case _, ok := <-ticker:
			if !ok {
				break main
			}

			p.submitUpdates()
		}
	}
}

func (p *Processor) submitUpdates() {
	for k := range p.pendingUpdates {
		queueItem, idx := p.Queue.FindById(k)
		if queueItem == nil || idx < 0 {
			p.Negotiator.OnProcessorUpdate(&ProcessorUpdate{UpdateType: ITEM_UPDATE, QueueItem: nil, ItemPosition: -1, ItemId: k})
		} else {
			p.Negotiator.OnProcessorUpdate(&ProcessorUpdate{
				UpdateType:   ITEM_UPDATE,
				QueueItem:    queueItem,
				ItemPosition: idx,
				ItemId:       k,
			})
		}

		delete(p.pendingUpdates, k)
	}
}

func (p *Processor) wakeupWorkers() {
	// Processor state has changed - wake up all workers
	// and notify the commander
	p.WorkerPool.WakeupWorkers()

	// Non blocking wakeup
	select {
	case p.FfmpegCommander.WakeupChan() <- 1:
	default:
	}
}
