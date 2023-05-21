package internal

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/docker"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/pkg/worker"
)

var log = logger.Get("Thea")

// Thea exposes the core workflow for the processor. There are two main categories of methods exposed here:
//
// -- Service Layer APIs --
// These are the preferred way to interact with Thea. Methods enclosed within these APIs are aware of all state
// in the Thea runtime and will ensure that updates are applied correctly across all of it (e.g. cancelling an
// item may remove it from the queue, and cancel all ffmpeg actions, and send an update to the client). Each API
// here is a "service" as it encapsulates related behaviour - however a call to one service may incur calls to other
// services via their respective API as well (these "side-effects" after often required in order to ensure Theas state is
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
	ffmpeg() ffmpeg.FfmpegCommander
	profiles() profile.ProfileManager
	workerPool() *worker.WorkerPool
	config() TheaConfig
}

// Thea represents the top-level object for the server, and is responsible
// for initialising embedded support services, workers, threads, event
// handling, et cetera...
type theaImpl struct {
	UpdateManager
	CoreService
	ProfileService
	QueueService
	MovieService

	queueMgr   queue.QueueManager
	ffmpegMgr  ffmpeg.FfmpegCommander
	profileMgr profile.ProfileManager
	workers    *worker.WorkerPool

	cfg               TheaConfig
	theaCtx           context.Context
	theaCtxCancel     context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
}

const THEA_USER_DIR_SUFFIX = "/thea/"
const THEA_CACHE_FILE_PATH = "cache.json"
const THEA_PROFILE_FILE_PATH = "profiles.json"
const THEA_UPDATE_INTERVAL = time.Second * 2
const THEA_QUEUE_SYNC_INTERVAL = time.Second * 5

// ** PUBLIC API ** //

func NewThea(config TheaConfig, updateFn UpdateManagerSubmitFn) Thea {
	ctx, ctxCancel := context.WithCancel(context.Background())
	configDir := config.getConfigDir()
	cacheDir := config.getCacheDir()

	// Construct a Thea instance
	thea := &theaImpl{
		cfg:               config,
		theaCtx:           ctx,
		theaCtxCancel:     ctxCancel,
		shutdownWaitGroup: &sync.WaitGroup{},
	}

	// Inject services
	thea.UpdateManager = NewUpdateManager(updateFn, thea)
	thea.ProfileService = NewProfileService(thea)
	thea.CoreService = NewCoreService(thea)
	thea.QueueService = NewQueueService(thea)
	thea.MovieService = NewMovieService(thea)

	// Inject state managers
	thea.queueMgr = queue.NewProcessorQueue(filepath.Join(cacheDir, THEA_CACHE_FILE_PATH))
	thea.ffmpegMgr = ffmpeg.NewFfmpegCommander(thea, config.Format)
	thea.profileMgr = profile.NewProfileList(filepath.Join(configDir, THEA_PROFILE_FILE_PATH))
	thea.workers = worker.NewWorkerPool()

	return thea
}

// Start will start Thea by initialising all supporting services/objects and starting
// the event loops
func (thea *theaImpl) Start() error {
	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	log.Emit(logger.DEBUG, "Starting Thea initialisation with config: %#v\n", thea.config())

	defer thea.Stop()
	if err := thea.initialise(); err != nil {
		return fmt.Errorf("failed to initialise Thea: %s", err)
	}

	thea.spawnAsyncService(thea.workers.StartWorkers)
	thea.spawnAsyncService(thea.ffmpegMgr.Start)

	updateTicker := time.NewTicker(THEA_UPDATE_INTERVAL)
	queueSyncTicker := time.NewTicker(THEA_QUEUE_SYNC_INTERVAL)

	log.Emit(logger.SUCCESS, " --- Thea Startup Complete --- \n")

	for {
		select {
		case <-updateTicker.C:
			thea.SubmitUpdates()
		case <-queueSyncTicker.C:
			if err := thea.synchroniseQueue(); err != nil {
				log.Emit(logger.WARNING, "Failed to synchronise item queue: %s\n", err.Error())
			}
		case <-exitChannel:
			log.Emit(logger.STOP, "Interrupt detected!\n")
			return nil
		case <-thea.theaCtx.Done():
			log.Emit(logger.WARNING, "Context has been cancelled!\n")
			return nil
		}
	}
}

// Stop will terminate Thea
func (thea *theaImpl) Stop() {
	log.Emit(logger.STOP, "--- Thea is shutting down ---\n")

	log.Emit(logger.STOP, "Closing all managers...\n")
	thea.workers.CloseWorkers()
	thea.ffmpegMgr.Stop()
	thea.shutdownWaitGroup.Wait()

	log.Emit(logger.STOP, "Closing Docker containers...\n")
	docker.DockerMgr.Shutdown(time.Second * 15)

	log.Emit(logger.STOP, "Cancelling context...\n")
	thea.theaCtxCancel()
	log.Emit(logger.DEBUG, "Thea core shutdown complete\n")
}

// ** INTERNAL API ** //
func (thea *theaImpl) queue() queue.QueueManager        { return thea.queueMgr }
func (thea *theaImpl) ffmpeg() ffmpeg.FfmpegCommander   { return thea.ffmpegMgr }
func (thea *theaImpl) profiles() profile.ProfileManager { return thea.profileMgr }
func (thea *theaImpl) workerPool() *worker.WorkerPool   { return thea.workers }
func (thea *theaImpl) config() TheaConfig               { return thea.cfg }

// ** PRIVATE IMPL ** //

// spawnAsyncService will run the provided function/service as it's own
// go-routine, ensuring that the Thea service waitgroup is updated correctly
func (thea *theaImpl) spawnAsyncService(service func()) {
	thea.shutdownWaitGroup.Add(1)

	go func() {
		defer thea.shutdownWaitGroup.Done()
		service()
	}()
}

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

	thea.queueMgr.Filter(func(queue queue.QueueManager, key int, item *queue.Item) bool {
		if _, ok := presentItems[item.Path]; !ok {
			thea.CancelItem(item.ItemID)
			return false
		}

		return true
	})

	thea.queueMgr.ForEach(func(q queue.QueueManager, idx int, item *queue.Item) bool {
		if item.Stage != queue.Import {
			return false
		}

		info, err := os.Stat(item.Path)
		if err != nil {
			log.Emit(logger.WARNING, "Failed to get file info for %v during import stage: %v\n", item.Path, err.Error())
			return false
		}

		if time.Since(info.ModTime()) > time.Minute*2 {
			log.Emit(logger.INFO, "Advancing item %s from Import hold as it's exceeded modtime threshold\n", item)
			thea.AdvanceItem(item)
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
		return nil, fmt.Errorf("failed to discover items for injestion: %s", err.Error())
	}

	return presentItems, nil
}

// initialiseDockerServices will initialise all supporting services
// for Thea (Docker based Postgres, PgAdmin and Web front-end)
func (thea *theaImpl) initialiseDockerServices() error {
	// Instantiate watcher for async errors for the below containers
	asyncErrorReport := make(chan error, 2)
	go func() {
		err := <-asyncErrorReport
		log.Emit(logger.ERROR, "One or more support services has crashed with error: %v ... Shutting down", err)

		// Shutdown now because a support service has crashed...
		thea.theaCtxCancel()
	}()

	// Initialise all services which are enabled. If a service is disabled, then the
	// user doesn't want us to create it for them. For the DB, this means the user *must*
	// provide the DB themselves
	config := thea.cfg
	if config.Services.EnablePostgres {
		log.Emit(logger.INFO, "Initialising embedded database...\n")
		_, err := database.InitialiseDockerDatabase(config.Database, asyncErrorReport)
		if err != nil {
			return err
		}
	}
	if config.Services.EnablePgAdmin {
		log.Emit(logger.INFO, "Initialising embedded pgAdmin server...\n")
		_, err := database.InitialiseDockerPgAdmin(asyncErrorReport)
		if err != nil {
			return err
		}
	}
	// TODO
	// if serviceConfig.EnableFrontend {
	// }

	return nil

}

// initialise will intialise all support services and workers, and connect to the backing DB
func (thea *theaImpl) initialise() error {
	log.Emit(logger.INFO, "Initialising Docker services...\n")
	if err := thea.initialiseDockerServices(); err != nil {
		return err
	}

	log.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := database.DB.Connect(thea.cfg.Database); err != nil {
		return err
	}

	advanceFunc := thea.AdvanceItem
	baseTask := queue.BaseTask{ItemProducer: thea}
	thea.workers.PushWorker(worker.NewWorker("Title_Parser", &queue.TitleTask{OnComplete: advanceFunc, BaseTask: baseTask}, int(queue.Title)))
	thea.workers.PushWorker(worker.NewWorker("OMDB_Handler", &queue.OmdbTask{OnComplete: advanceFunc, BaseTask: baseTask, OmdbKey: thea.cfg.OmdbKey}, int(queue.Omdb)))
	thea.workers.PushWorker(worker.NewWorker("Database_Committer", &queue.DatabaseTask{OnComplete: advanceFunc, CommitHandler: thea.ExportItem, BaseTask: baseTask}, int(queue.Database)))

	return nil
}
