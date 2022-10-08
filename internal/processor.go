package internal

/*
import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/internal/dockerService"
	"github.com/hbomb79/TPA/pkg/docker"
	"github.com/hbomb79/TPA/pkg/logger"
	"github.com/hbomb79/TPA/pkg/worker"
)

// The Processor struct contains all the context
// for the running instance of this program. It stores
// the queue of items, the pool of workers that are
// processing the queue, and the users configuration
type Processor struct {
	Config             *TPAConfig
	Queue              QueueManager
	WorkerPool         *worker.WorkerPool
	Negotiator         Negotiator
	UpdateChan         chan int
	pendingUpdates     map[int]bool
	Profiles           ProfileList
	FfmpegCommander    Commander
	KnownFfmpegOptions map[string]string
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
func NewProcessor() (*Processor, error) {
	opts, err := ToArgsMap(&ffmpeg.Options{})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		WorkerPool:         worker.NewWorkerPool(),
		UpdateChan:         make(chan int),
		pendingUpdates:     make(map[int]bool),
		KnownFfmpegOptions: opts,
		managerWg:          &sync.WaitGroup{},
		serviceWg:          &sync.WaitGroup{},
		ctx:                ctx,
		ctxCancel:          cancel,
	}, nil
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

func (p *Processor) initialiseSupportingServices(asyncErrorReport chan error) error {
	procLogger.Emit(logger.INFO, "Initialising embedded database...\n")
	_, err := dockerService.InitialiseDockerDatabase(p.Config.Database, asyncErrorReport)
	if err != nil {
		return err
	}

	procLogger.Emit(logger.INFO, "Initialising embedded pgAdmin server...\n")
	_, err = dockerService.InitialiseDockerPgAdmin(asyncErrorReport)
	if err != nil {
		return err
	}

	procLogger.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := dockerService.DB.Connect(p.Config.Database); err != nil {
		return err
	}

	return nil
}

// Start will start the workers inside the WorkerPool
// responsible for the various tasks inside the program
// This method will wait on the WaitGroup attached to the WorkerPool
func (p *Processor) Start(readyChan chan bool) error {
	defer p.shutdown()
	defer func() {
		if r := recover(); r != nil {
			procLogger.Emit(logger.FATAL, "\n\nPANIC! %v\n\nShutting Down!\n\n", r)
		}
	}()

	dbErr := make(chan error)
	p.initialiseSupportingServices(dbErr)

	var cacheDir string = p.Config.CacheDirPath
	var configDir string = p.Config.ConfigDirPath
	if dir, err := os.UserCacheDir(); err == nil {
		cacheDir = dir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		configDir = dir
	}

	p.Queue = NewProcessorQueue(filepath.Join(cacheDir, "/tpa/cache.json"))
	p.Profiles = NewProfileList(filepath.Join(configDir, "/tpa/profiles.json"))

	p.FfmpegCommander = NewCommander(p)
	p.FfmpegCommander.SetWindowSize(2)
	p.FfmpegCommander.SetThreadPoolSize(8)

	p.WorkerPool.PushWorker(worker.NewWorker("Title_Parser", &TitleTask{tpa: p}, int(Title), make(chan int)))
	p.WorkerPool.PushWorker(worker.NewWorker("OMDB_Handler", &OmdbTask{proc: p}, int(Omdb), make(chan int)))
	p.WorkerPool.PushWorker(worker.NewWorker("Database_Committer", &DatabaseTask{proc: p}, int(Database), make(chan int)))

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
		procLogger.Emit(logger.INFO, "SIGTERM/Interrupt caught! Shutting down...\n")
	case msg := <-dbErr:
		procLogger.Emit(logger.FATAL, "Database failure: %v\nShutting down...\n", msg)
	}

	return nil
}

func (p *Processor) shutdown() {
	procLogger.Emit(logger.STOP, "Closing all managers...\n")
	p.WorkerPool.CloseWorkers()
	if p.FfmpegCommander != nil {
		p.FfmpegCommander.Stop()
	}
	p.managerWg.Wait()

	procLogger.Emit(logger.STOP, "Closing all containers...\n")
	docker.DockerMgr.Shutdown(time.Second * 15)

	procLogger.Emit(logger.STOP, "Closing all data streams...\n")
	p.ctxCancel()
	p.serviceWg.Wait()
}

// SynchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
func (p *Processor) SynchroniseQueue() error {
	p.Queue.Reload()
	presentItems, err := p.DiscoverItems()
	if err != nil {
		return err
	}

	p.InjestQueue(presentItems)

	p.Queue.Filter(func(queue QueueManager, key int, item *QueueItem) bool {
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
	checkModtime := func(q QueueManager, idx int, item *QueueItem) bool {
		if item.Stage != Import {
			return false
		}

		info, err := os.Stat(item.Path)
		if err != nil {
			procLogger.Emit(logger.WARNING, "Failed to get file info for %v during import stage: %v\n", item.Path, err.Error())
			return false
		}

		if time.Since(info.ModTime()) > time.Minute*2 {
			q.AdvanceStage(item)
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
		procLogger.Emit(logger.WARNING, "Processor began to listen for updates for transmission, however no Negotiator is attached. Aborting.\n")
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
		// case <-ticker:
		// p.submitUpdates()
		case <-ctx.Done():
			return
		}
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

*/
