package processor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/pkg"
	"github.com/hbomb79/TPA/profile"
	"github.com/hbomb79/TPA/worker"
	"github.com/ilyakaznacheev/cleanenv"
)

// TPAConfig is the struct used to contain the
// various user config supplied by file, or
// manually inside the code.
type TPAConfig struct {
	Concurrent       ConcurrentConfig   `yaml:"concurrency" env-required:"true"`
	Format           FormatterConfig    `yaml:"formatter"`
	Database         pkg.DatabaseConfig `yaml:"database" env-required:"true"`
	OmdbKey          string             `yaml:"omdb_api_key" env:"OMDB_API_KEY" env-required:"true"`
	ExternalDatabase bool               `yaml:"external_database" env:"EXTERNAL_DATABASE"`
	CacheDirPath     string             `yaml:"cache_dir" env:"CACHE_DIR" env-default:".cache/tpa/"`
	ConfigDirPath    string             `yaml:"config_dir" env:"CONFIG_DIR" env-default:".config/tpa/"`
	ApiHostAddr      string             `yaml:"host" env:"HOST_ADDR" env-default:"0.0.0.0"`
	ApiHostPort      string             `yaml:"port" env:"HOST_PORT" env-default:"8080"`
}

// ConcurrentConfig is a subset of the configuration that focuses
// only on the concurrency related configs (number of threads to use
// for each stage of the pipeline)
type ConcurrentConfig struct {
	Title  int `yaml:"title_threads" env:"CONCURRENCY_TITLE_THREADS" env-default:"1"`
	OMBD   int `yaml:"omdb_threads" env:"CONCURRENCY_OMDB_THREADS" env-default:"1"`
	Format int `yaml:"ffmpeg_threads" env:"CONCURRENCY_FFMPEG_THREADS" env-default:"8"`
}

// FormatterConfig is the 'misc' container of the configuration, encompassing configuration
// not covered by either 'ConcurrentConfig' or 'DatabaseConfig'. Mainly configuration
// paramters for the FFmpeg executable.
type FormatterConfig struct {
	ImportPath         string `yaml:"import_path" env:"FORMAT_IMPORT_PATH" env-required:"true"`
	OutputPath         string `yaml:"default_output_dir" env:"FORMAT_DEFAULT_OUTPUT_DIR" env-required:"true"`
	TargetFormat       string `yaml:"target_format" env:"FORMAT_TARGET_FORMAT" env-default:"mp4"`
	ImportDirTickDelay int    `yaml:"import_polling_delay" env:"FORMAT_IMPORT_POLLING_DELAY" env-default:"3600"`
	FfmpegBinaryPath   string `yaml:"ffmpeg_binary" env:"FORMAT_FFMPEG_BINARY_PATH" env-default:"/usr/bin/ffmpeg"`
	FfprobeBinaryPath  string `yaml:"ffprobe_binary" env:"FORMAT_FFPROBE_BINARY_PATH" env-default:"/usr/bin/ffprobe"`
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
	DatabaseServer     pkg.DatabaseServer
	ctxCancel          context.CancelFunc
	ctx                context.Context
	serviceWg          *sync.WaitGroup
	managerWg          *sync.WaitGroup
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

	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		WorkerPool:         worker.NewWorkerPool(),
		UpdateChan:         make(chan int),
		pendingUpdates:     make(map[int]bool),
		KnownFfmpegOptions: opts,
		DatabaseServer:     pkg.NewDatabaseServer(),
		managerWg:          &sync.WaitGroup{},
		serviceWg:          &sync.WaitGroup{},
		ctx:                ctx,
		ctxCancel:          cancel,
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
func (p *Processor) Start(readyChan chan bool) error {
	defer p.shutdown()

	dbErr := make(chan error)
	if !p.Config.ExternalDatabase {
		fmt.Printf("[Processor] Initialising embedded database...\n")
		_, err := pkg.InitialiseDockerDatabase(p.Config.Database, dbErr)
		if err != nil {
			return err
		}
	}

	fmt.Printf("[Processor] Initialising embedded pgAdmin server...\n")
	_, err := pkg.InitialiseDockerPgAdmin(dbErr)
	if err != nil {
		return err
	}

	fmt.Printf("[Processor] Connecting to database with GORM...\n")
	if err := p.DatabaseServer.Connect(p.Config.Database); err != nil {
		return err
	}
	fmt.Printf("[Processor] Auto-migrating database...\n")
	p.DatabaseServer.GetInstance().AutoMigrate(&QueueItem{}, &TitleInfo{}, &OmdbInfo{})

	var cacheDir string = p.Config.CacheDirPath
	var configDir string = p.Config.ConfigDirPath
	if dir, err := os.UserCacheDir(); err == nil {
		cacheDir = dir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		configDir = dir
	}

	p.Queue = NewProcessorQueue(filepath.Join(cacheDir, "/tpa/cache.json"))
	p.Profiles = profile.NewList(filepath.Join(configDir, "/tpa/profiles.json"))

	p.FfmpegCommander = NewCommander(p)
	p.FfmpegCommander.SetWindowSize(2)
	p.FfmpegCommander.SetThreadPoolSize(8)

	p.WorkerPool.PushWorker(worker.NewWorker("Title_Parser", &TitleTask{proc: p}, worker.Title, make(chan int)))
	p.WorkerPool.PushWorker(worker.NewWorker("OMDB_Handler", &OmdbTask{proc: p}, worker.Omdb, make(chan int)))
	p.WorkerPool.PushWorker(worker.NewWorker("Database_Committer", &DatabaseTask{proc: p}, worker.Database, make(chan int)))

	// When constructing our WaitGroups, we use two groups so that we can shutdown the senders of data before the receivers.
	// In many places in TPA we intentionally send in a blocking manner, closing the receivers before the senders would result
	// in deadlocking sender threads - this will mean TPA can never shutdown and will hang indefinitely.
	p.serviceWg.Add(3)
	go p.handleUpdateStream(p.ctx, p.serviceWg)
	go p.handleItemModtimes(p.ctx, p.serviceWg)
	go p.handleQueueSync(p.ctx, p.serviceWg, time.Duration(p.Config.Format.ImportDirTickDelay*int(time.Second)))

	p.managerWg.Add(2)
	go p.FfmpegCommander.Start(p.managerWg)
	go p.WorkerPool.StartWorkers(p.managerWg)

	exit := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	readyChan <- true
	select {
	case <-exit:
		fmt.Printf("[Processor] (!!) SIGTERM/Interrupt caught! Shutting down...\n")
	case msg := <-dbErr:
		fmt.Printf("[Processor] (!!) Database failure: %v\nShutting down...\n", msg)
	}

	return nil
}

func (p *Processor) shutdown() {
	fmt.Print("[Processor] (X) Closing all managers...\n")
	p.WorkerPool.CloseWorkers()
	if p.FfmpegCommander != nil {
		p.FfmpegCommander.Stop()
	}
	p.managerWg.Wait()

	fmt.Print("[Processor] (X) Closing all containers...\n")
	pkg.Docker.Shutdown(time.Second * 15)

	fmt.Print("[Processor] (X) Closing all data streams...\n")
	p.ctxCancel()
	p.serviceWg.Wait()
}

// SynchroniseQueue will first discover all items inside the import directory,
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
		p.Queue.Push(NewQueueItem(info, path, p))
	}

	return nil
}

func (p *Processor) PruneQueueCache() {
	// TODO
}

func (p *Processor) handleQueueSync(ctx context.Context, wg *sync.WaitGroup, tickInterval time.Duration) {
	defer wg.Done()
	go func(target <-chan time.Time) {
		for {
			p.SynchroniseQueue()

			<-target
		}
	}(time.NewTicker(tickInterval).C)
}

func (p *Processor) handleItemModtimes(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
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

	for {
		select {
		case <-ticker:
			p.Queue.ForEach(checkModtime)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) handleUpdateStream(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
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
		case <-ticker:
			p.submitUpdates()
		case <-ctx.Done():
			return
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
