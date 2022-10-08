package internal

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

	"github.com/hbomb79/TPA/internal/dockerService"
	"github.com/hbomb79/TPA/pkg/docker"
	"github.com/hbomb79/TPA/pkg/logger"
	"github.com/hbomb79/TPA/pkg/worker"
)

var procLogger = logger.Get("Proc")

// TPA exposes the core workflow for the processor. There are two main categories of methods exposed here:
//
// -- Service Layer APIs --
// These are the preferred way to interact with TPA. Methods enclosed within these APIs are aware of all state
// in the TPA runtime and will ensure that updates are applied correctly across all of it (e.g. cancelling an
// item may remove it from the queue, and cancel all ffmpeg actions, and send an update to the client). Each API
// here is a "service" as it encapsulates related behaviour - however a call to one service may incur calls to other
// services via their respective API as well (these "side-effects" after often required in order to ensure TPAs state is
// kept valid!).
//
// -- Internal APIs --
// Each of these internal APIs represent a single "unit" or component of the TPA state - updating one of these states without
// understanding the implications that may have on the other units is dangerous! (This is why the above service
// layer APIs are preferred). These internal APIs allow code that knows what it's doing to selectively change the
// state of TPA without unindented side-effects (however these side-effects are often a good thing!).
type TPA interface {
	// ** Public API ** //
	Start() error
	Stop()

	// ** Service Layer APIs ** //
	CoreService() CoreService
	QueueService() QueueService
	MovieService() MovieService

	// ** Internal APIs ** //
	Queue() QueueManager
	Ffmpeg() Commander
	Profiles() ProfileList
	Updates() UpdateManager
	WorkerPool() *worker.WorkerPool
	Config() TPAConfig
}

// TPA represents the top-level object for the server, and is responsible
// for initialising embedded support services, workers, threads, event
// handling, et cetera...
type tpa struct {
	coreService       CoreService
	queueService      QueueService
	movieService      MovieService
	queue             QueueManager
	ffmpeg            Commander
	profile           ProfileList
	updates           UpdateManager
	workerPool        *worker.WorkerPool
	config            TPAConfig
	tpaContext        context.Context
	tpaContextCancel  context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
}

const TPA_CONFIG_FILE_PATH = "/tpa/config.json"
const TPA_CACHE_FILE_PATH = "/tpa/cache.json"
const TPA_UPDATE_INTERVAL = time.Second * 2
const TPA_QUEUE_SYNC_INTERVAL = time.Second * 5

// ** PUBLIC API ** //

func NewTPA(config TPAConfig, updateFn UpdateManagerSubmitFn) TPA {
	// Construct a tpa instance with all supporting services injected
	t := &tpa{config: config}

	configPath := config.getConfigPath()
	cachePath := config.getCachePath()

	ctx, ctxCancel := context.WithCancel(context.Background())
	t.tpaContext = ctx
	t.tpaContextCancel = ctxCancel

	t.coreService = NewCoreApi(t)
	t.queueService = NewQueueApi(t)
	t.queue = NewProcessorQueue(cachePath)
	t.ffmpeg = NewCommander(t)
	t.profile = NewProfileList(configPath)
	t.updates = NewUpdateManager(updateFn, t)
	t.workerPool = worker.NewWorkerPool()
	t.shutdownWaitGroup = &sync.WaitGroup{}

	return t
}

// Start will start TPA by initialising all supporting services/objects and starting
// the event loops
func (tpa *tpa) Start() error {
	if err := tpa.initialise(); err != nil {
		return fmt.Errorf("failed to initialise TPA: %s", err)
	}

	// Initialise our async service managers
	go tpa.workerPool.StartWorkers(tpa.shutdownWaitGroup)
	go tpa.ffmpeg.Start(tpa.shutdownWaitGroup)

	// Initialise some tickers
	updateTicker := time.NewTicker(TPA_UPDATE_INTERVAL)
	queueSyncTicker := time.NewTicker(TPA_QUEUE_SYNC_INTERVAL)

	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	defer tpa.Stop()
	for {
		select {
		case <-updateTicker.C:
			tpa.updates.SubmitUpdates()
		case <-queueSyncTicker.C:
			tpa.synchroniseQueue()
		case <-exitChannel:
			procLogger.Emit(logger.INFO, "Interrupt detected!\n")
			return nil
		case <-tpa.tpaContext.Done():
			procLogger.Emit(logger.WARNING, "TPA context has been cancelled!\n")
		}
	}
}

// Stop will terminate TPA
func (tpa *tpa) Stop() {
	procLogger.Emit(logger.STOP, "--- TPA is shutting down ---\n")
	procLogger.Emit(logger.STOP, "Closing all managers...\n")
	tpa.workerPool.CloseWorkers()
	tpa.ffmpeg.Stop()
	tpa.shutdownWaitGroup.Wait()

	procLogger.Emit(logger.STOP, "Closing all containers...\n")
	docker.DockerMgr.Shutdown(time.Second * 15)

	procLogger.Emit(logger.STOP, "Closing all data streams...\n")
	tpa.tpaContextCancel()
}

// synchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
func (tpa *tpa) synchroniseQueue() error {
	// Find new items
	tpa.queue.Reload()
	presentItems, err := tpa.discoverItems()
	if err != nil {
		return err
	}

	for path, info := range presentItems {
		tpa.queue.Push(NewQueueItem(info, path, tpa))
	}

	tpa.queue.Filter(func(queue QueueManager, key int, item *QueueItem) bool {
		if _, ok := presentItems[item.Path]; !ok {
			item.Cancel()
			return false
		}

		return true
	})

	tpa.queue.ForEach(func(q QueueManager, idx int, item *QueueItem) bool {
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
	})

	return nil
}

func (tpa *tpa) CoreService() CoreService   { return tpa.coreService }
func (tpa *tpa) QueueService() QueueService { return tpa.queueService }
func (tpa *tpa) MovieService() MovieService { return tpa.movieService }

// ** INTERNAL API ** //
func (tpa *tpa) Queue() QueueManager            { return tpa.queue }
func (tpa *tpa) Ffmpeg() Commander              { return tpa.ffmpeg }
func (tpa *tpa) Profiles() ProfileList          { return tpa.profile }
func (tpa *tpa) Updates() UpdateManager         { return tpa.updates }
func (tpa *tpa) WorkerPool() *worker.WorkerPool { return tpa.workerPool }
func (tpa *tpa) Config() TPAConfig              { return tpa.config }

// ** PRIVATE IMPL ** //

// discoverItems will walk through the import directory and construct a map
// of all the items inside the import directory (or any nested directories).
// The key of the map is the path, and the value contains the FileInfo
func (tpa *tpa) discoverItems() (map[string]fs.FileInfo, error) {
	presentItems := make(map[string]fs.FileInfo, 0)
	config := tpa.config
	err := filepath.WalkDir(config.Format.ImportPath, func(path string, dir fs.DirEntry, err error) error {
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

// initialiseSupportServices will initialise all supporting services
// for TPA (Docker based Postgres, PgAdmin and Web front-end)
func (tpa *tpa) initialiseSupportServices() error {
	// Instantiate watcher for async errors for the below containers
	config := tpa.config
	serviceConfig := config.Services
	asyncErrorReport := make(chan error, 2)
	go func() {
		err, _ := <-asyncErrorReport
		procLogger.Emit(logger.ERROR, "One or more support services has crashed with error: %v ... Shutting down", err)

		// Shutdown now because a support service has crashed...
		tpa.Stop()
	}()

	// Initialise all services which are enabled. If a service is disabled, then the
	// user doesn't want us to create it for them. For the DB, this means the user *must*
	// provide the DB themselves
	if serviceConfig.EnablePostgres {
		procLogger.Emit(logger.INFO, "Initialising embedded database...\n")
		_, err := dockerService.InitialiseDockerDatabase(config.Database, asyncErrorReport)
		if err != nil {
			return err
		}
	}

	if serviceConfig.EnablePgAdmin {
		procLogger.Emit(logger.INFO, "Initialising embedded pgAdmin server...\n")
		_, err := dockerService.InitialiseDockerPgAdmin(asyncErrorReport)
		if err != nil {
			return err
		}
	}

	if serviceConfig.EnableFrontend {
		// TODO
	}

	return nil

}

func (tpa *tpa) initialiseDatabaseConnection() error {
	procLogger.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := dockerService.DB.Connect(tpa.config.Database); err != nil {
		return err
	}

	return nil
}

func (tpa *tpa) initialiseWorkers() {
	tpa.workerPool.PushWorker(worker.NewWorker("Title_Parser", &TitleTask{tpa: tpa}, int(Title), make(chan int)))
	tpa.workerPool.PushWorker(worker.NewWorker("OMDB_Handler", &OmdbTask{tpa: tpa}, int(Omdb), make(chan int)))
	tpa.workerPool.PushWorker(worker.NewWorker("Database_Committer", &DatabaseTask{tpa: tpa}, int(Database), make(chan int)))
}

// initialise will intialise all support services and workers, and connect to the backing DB
func (tpa *tpa) initialise() error {
	if err := tpa.initialiseSupportServices(); err != nil {
		return err
	}

	if err := tpa.initialiseDatabaseConnection(); err != nil {
		return err
	}

	tpa.initialiseWorkers()
	return nil
}
