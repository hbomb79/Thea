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

	"github.com/hbomb79/Thea/internal/db"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/docker"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/pkg/worker"
)

var procLogger = logger.Get("Thea")

// Thea exposes the core workflow for the processor. There are two main categories of methods exposed here:
//
// -- Service Layer APIs --
// These are the preferred way to interact with Thea. Methods enclosed within these APIs are aware of all state
// in the Thea runtime and will ensure that updates are applied correctly across all of it (e.g. cancelling an
// item may remove it from the queue, and cancel all ffmpeg actions, and send an update to the client). Each API
// here is a "service" as it encapsulates related behaviour - however a call to one service may incur calls to other
// services via their respective API as well (these "side-effects" after often required in order to ensure TPAs state is
// kept valid!).
//
// -- Internal APIs --
// Each of these internal APIs represent a single "unit" or component of the Thea state - updating one of these states without
// understanding the implications that may have on the other units is dangerous! (This is why the above service
// layer APIs are preferred). These internal APIs allow code that knows what it's doing to selectively change the
// state of Thea without unindented side-effects (however these side-effects are often a good thing!).
type Thea interface {
	UpdateManager
	CoreService
	ProfileService
	QueueService
	MovieService

	Start() error
	Stop()

	queue() queue.QueueManager
	ffmpeg() ffmpeg.FfmpegManager
	profiles() profile.ProfileManager
	workerPool() *worker.WorkerPool
	config() TheaConfig
}

// TPA represents the top-level object for the server, and is responsible
// for initialising embedded support services, workers, threads, event
// handling, et cetera...
type theaImpl struct {
	UpdateManager
	CoreService
	ProfileService
	QueueService
	MovieService

	queueMgr   queue.QueueManager
	ffmpegMgr  ffmpeg.FfmpegManager
	profileMgr profile.ProfileManager
	workers    *worker.WorkerPool

	cfg               TheaConfig
	theaCtx           context.Context
	theaCtxCancel     context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
}

const TPA_CONFIG_FILE_PATH = "/thea/config.json"
const TPA_CACHE_FILE_PATH = "/thea/cache.json"
const TPA_UPDATE_INTERVAL = time.Second * 2
const TPA_QUEUE_SYNC_INTERVAL = time.Second * 5

// ** PUBLIC API ** //

func NewThea(config TheaConfig, updateFn UpdateManagerSubmitFn) Thea {
	ctx, ctxCancel := context.WithCancel(context.Background())
	configPath := config.getConfigPath()
	cachePath := config.getCachePath()

	// Construct a Thea instance
	t := &theaImpl{
		cfg:               config,
		theaCtx:           ctx,
		theaCtxCancel:     ctxCancel,
		shutdownWaitGroup: &sync.WaitGroup{},
	}

	// Inject services
	t.UpdateManager = NewUpdateManager(updateFn, t)
	t.ProfileService = NewProfileService(t)
	t.CoreService = NewCoreService(t)
	t.QueueService = NewQueueService(t)
	t.MovieService = NewMovieService(t)

	// Inject state managers
	t.queueMgr = queue.NewProcessorQueue(cachePath)
	t.ffmpegMgr = ffmpeg.NewCommander(t, config.Format)
	t.profileMgr = profile.NewProfileList(configPath)
	t.workers = worker.NewWorkerPool()

	return t
}

// Start will start TPA by initialising all supporting services/objects and starting
// the event loops
func (thea *theaImpl) Start() error {
	if err := thea.initialise(); err != nil {
		return fmt.Errorf("failed to initialise TPA: %s", err)
	}

	// Initialise our async service managers
	thea.shutdownWaitGroup.Add(2)
	go thea.workers.StartWorkers(thea.shutdownWaitGroup)
	go thea.ffmpegMgr.Start(thea.shutdownWaitGroup)
	defer func() {
		procLogger.Emit(logger.DEBUG, "TPA event loop has terminated.. deferred shutdown!\n")
		thea.Stop()
	}()

	// Initialise some tickers
	updateTicker := time.NewTicker(TPA_UPDATE_INTERVAL)
	queueSyncTicker := time.NewTicker(TPA_QUEUE_SYNC_INTERVAL)

	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-updateTicker.C:
			thea.SubmitUpdates()
		case <-queueSyncTicker.C:
			thea.synchroniseQueue()
		case <-exitChannel:
			procLogger.Emit(logger.INFO, "Interrupt detected!\n")
			return nil
		case <-thea.theaCtx.Done():
			procLogger.Emit(logger.WARNING, "TPA context has been cancelled!\n")
			return nil
		}
	}
}

// Stop will terminate TPA
func (thea *theaImpl) Stop() {
	procLogger.Emit(logger.STOP, "--- TPA is shutting down ---\n")

	procLogger.Emit(logger.STOP, "Cancelling context...\n")
	thea.theaCtxCancel()

	procLogger.Emit(logger.STOP, "Closing all managers...\n")
	thea.workers.CloseWorkers()
	thea.ffmpegMgr.Stop()
	thea.shutdownWaitGroup.Wait()

	procLogger.Emit(logger.STOP, "Closing all containers...\n")
	docker.DockerMgr.Shutdown(time.Second * 15)
}

// ** INTERNAL API ** //
func (thea *theaImpl) queue() queue.QueueManager        { return thea.queueMgr }
func (thea *theaImpl) ffmpeg() ffmpeg.FfmpegManager     { return thea.ffmpegMgr }
func (thea *theaImpl) profiles() profile.ProfileManager { return thea.profileMgr }
func (thea *theaImpl) workerPool() *worker.WorkerPool   { return thea.workers }
func (thea *theaImpl) config() TheaConfig               { return thea.cfg }

// ** PRIVATE IMPL ** //
// synchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
func (thea *theaImpl) synchroniseQueue() error {
	// Find new items
	thea.queueMgr.Reload()
	presentItems, err := thea.discoverItems()
	if err != nil {
		return err
	}

	for path, info := range presentItems {
		thea.queueMgr.Push(queue.NewQueueItem(info, path, thea))
	}

	thea.queueMgr.Filter(func(queue queue.QueueManager, key int, item *queue.QueueItem) bool {
		if _, ok := presentItems[item.Path]; !ok {
			item.Cancel()
			return false
		}

		return true
	})

	thea.queueMgr.ForEach(func(q queue.QueueManager, idx int, item *queue.QueueItem) bool {
		if item.Stage != queue.Import {
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

// discoverItems will walk through the import directory and construct a map
// of all the items inside the import directory (or any nested directories).
// The key of the map is the path, and the value contains the FileInfo
func (thea *theaImpl) discoverItems() (map[string]fs.FileInfo, error) {
	presentItems := make(map[string]fs.FileInfo, 0)
	config := thea.cfg
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
func (thea *theaImpl) initialiseSupportServices() error {
	// Instantiate watcher for async errors for the below containers
	config := thea.cfg
	serviceConfig := config.Services
	asyncErrorReport := make(chan error, 2)
	go func() {
		err := <-asyncErrorReport
		procLogger.Emit(logger.ERROR, "One or more support services has crashed with error: %v ... Shutting down", err)

		// Shutdown now because a support service has crashed...
		thea.theaCtxCancel()
	}()

	// Initialise all services which are enabled. If a service is disabled, then the
	// user doesn't want us to create it for them. For the DB, this means the user *must*
	// provide the DB themselves
	if serviceConfig.EnablePostgres {
		procLogger.Emit(logger.INFO, "Initialising embedded database...\n")
		_, err := db.InitialiseDockerDatabase(config.Database, asyncErrorReport)
		if err != nil {
			return err
		}
	}

	if serviceConfig.EnablePgAdmin {
		procLogger.Emit(logger.INFO, "Initialising embedded pgAdmin server...\n")
		_, err := db.InitialiseDockerPgAdmin(asyncErrorReport)
		if err != nil {
			return err
		}
	}

	if serviceConfig.EnableFrontend {
		// TODO
	}

	return nil

}

// initialise will intialise all support services and workers, and connect to the backing DB
func (thea *theaImpl) initialise() error {
	if err := thea.initialiseSupportServices(); err != nil {
		return err
	}

	procLogger.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := db.DB.Connect(thea.cfg.Database); err != nil {
		return err
	}

	advanceFunc := thea.queue().AdvanceStage
	baseTask := queue.BaseTask{ItemProducer: thea}
	thea.workers.PushWorker(worker.NewWorker("Title_Parser", &queue.TitleTask{OnComplete: advanceFunc, BaseTask: baseTask}, int(queue.Title), make(chan int)))
	thea.workers.PushWorker(worker.NewWorker("OMDB_Handler", &queue.OmdbTask{OnComplete: advanceFunc, BaseTask: baseTask}, int(queue.Omdb), make(chan int)))
	thea.workers.PushWorker(worker.NewWorker("Database_Committer", &queue.DatabaseTask{OnComplete: advanceFunc, BaseTask: baseTask}, int(queue.Database), make(chan int)))

	return nil
}
