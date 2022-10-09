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

	"github.com/hbomb79/TPA/internal/db"
	"github.com/hbomb79/TPA/internal/ffmpeg"
	"github.com/hbomb79/TPA/internal/profile"
	"github.com/hbomb79/TPA/internal/queue"
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
	config() TPAConfig
}

// TPA represents the top-level object for the server, and is responsible
// for initialising embedded support services, workers, threads, event
// handling, et cetera...
type tpa struct {
	UpdateManager
	CoreService
	ProfileService
	QueueService
	MovieService

	queueMgr   queue.QueueManager
	ffmpegMgr  ffmpeg.FfmpegManager
	profileMgr profile.ProfileManager
	workers    *worker.WorkerPool

	cfg               TPAConfig
	tpaContext        context.Context
	tpaContextCancel  context.CancelFunc
	shutdownWaitGroup *sync.WaitGroup
}

const TPA_CONFIG_FILE_PATH = "/tpa/config.json"
const TPA_CACHE_FILE_PATH = "/tpa/cache.json"
const TPA_UPDATE_INTERVAL = time.Second * 2
const TPA_QUEUE_SYNC_INTERVAL = time.Second * 5

// ** PUBLIC API ** //

func NewTpa(config TPAConfig, updateFn UpdateManagerSubmitFn) TPA {
	ctx, ctxCancel := context.WithCancel(context.Background())
	configPath := config.getConfigPath()
	cachePath := config.getCachePath()

	// Construct a tpa instance
	t := &tpa{
		cfg:               config,
		tpaContext:        ctx,
		tpaContextCancel:  ctxCancel,
		shutdownWaitGroup: &sync.WaitGroup{},
	}

	// Inject services
	t.UpdateManager = NewUpdateManager(updateFn, t)
	t.ProfileService = NewProfileService(t)
	t.CoreService = NewCoreApi(t)
	t.QueueService = NewQueueApi(t)
	t.MovieService = nil

	// Inject state managers
	t.queueMgr = queue.NewProcessorQueue(cachePath)
	t.ffmpegMgr = ffmpeg.NewCommander(t, config.Format)
	t.profileMgr = profile.NewProfileList(configPath)
	t.workers = worker.NewWorkerPool()

	return t
}

// Start will start TPA by initialising all supporting services/objects and starting
// the event loops
func (tpa *tpa) Start() error {
	if err := tpa.initialise(); err != nil {
		return fmt.Errorf("failed to initialise TPA: %s", err)
	}

	// Initialise our async service managers
	go tpa.workers.StartWorkers(tpa.shutdownWaitGroup)
	go tpa.ffmpegMgr.Start(tpa.shutdownWaitGroup)
	defer tpa.Stop()

	// Initialise some tickers
	updateTicker := time.NewTicker(TPA_UPDATE_INTERVAL)
	queueSyncTicker := time.NewTicker(TPA_QUEUE_SYNC_INTERVAL)

	exitChannel := make(chan os.Signal, 1)
	signal.Notify(exitChannel, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-updateTicker.C:
			tpa.SubmitUpdates()
		case <-queueSyncTicker.C:
			tpa.synchroniseQueue()
		case <-exitChannel:
			procLogger.Emit(logger.INFO, "Interrupt detected!\n")
			return nil
		case <-tpa.tpaContext.Done():
			procLogger.Emit(logger.WARNING, "TPA context has been cancelled!\n")
			return nil
		}
	}
}

// Stop will terminate TPA
func (tpa *tpa) Stop() {
	procLogger.Emit(logger.STOP, "--- TPA is shutting down ---\n")
	procLogger.Emit(logger.STOP, "Closing all managers...\n")
	tpa.workers.CloseWorkers()
	tpa.ffmpegMgr.Stop()
	tpa.shutdownWaitGroup.Wait()

	procLogger.Emit(logger.STOP, "Closing all containers...\n")
	docker.DockerMgr.Shutdown(time.Second * 15)

	procLogger.Emit(logger.STOP, "Closing all data streams...\n")
	tpa.tpaContextCancel()
}

// ** INTERNAL API ** //
func (tpa *tpa) queue() queue.QueueManager        { return tpa.queueMgr }
func (tpa *tpa) ffmpeg() ffmpeg.FfmpegManager     { return tpa.ffmpegMgr }
func (tpa *tpa) profiles() profile.ProfileManager { return tpa.profileMgr }
func (tpa *tpa) workerPool() *worker.WorkerPool   { return tpa.workers }
func (tpa *tpa) config() TPAConfig                { return tpa.cfg }

// ** PRIVATE IMPL ** //
// synchroniseQueue will first discover all items inside the import directory,
// and then will injest any that do not already exist in the queue. Any items
// in the queue that no longer exist in the discovered items will also be cancelled
func (tpa *tpa) synchroniseQueue() error {
	// Find new items
	tpa.queueMgr.Reload()
	presentItems, err := tpa.discoverItems()
	if err != nil {
		return err
	}

	for path, info := range presentItems {
		tpa.queueMgr.Push(queue.NewQueueItem(info, path, tpa))
	}

	tpa.queueMgr.Filter(func(queue queue.QueueManager, key int, item *queue.QueueItem) bool {
		if _, ok := presentItems[item.Path]; !ok {
			item.Cancel()
			return false
		}

		return true
	})

	tpa.queueMgr.ForEach(func(q queue.QueueManager, idx int, item *queue.QueueItem) bool {
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
func (tpa *tpa) discoverItems() (map[string]fs.FileInfo, error) {
	presentItems := make(map[string]fs.FileInfo, 0)
	config := tpa.cfg
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
	config := tpa.cfg
	serviceConfig := config.Services
	asyncErrorReport := make(chan error, 2)
	go func() {
		err := <-asyncErrorReport
		procLogger.Emit(logger.ERROR, "One or more support services has crashed with error: %v ... Shutting down", err)

		// Shutdown now because a support service has crashed...
		tpa.Stop()
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
func (tpa *tpa) initialise() error {
	if err := tpa.initialiseSupportServices(); err != nil {
		return err
	}

	procLogger.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := db.DB.Connect(tpa.cfg.Database); err != nil {
		return err
	}

	advanceFunc := tpa.queue().AdvanceStage
	tpa.workers.PushWorker(worker.NewWorker("Title_Parser", &queue.TitleTask{OnComplete: advanceFunc}, int(queue.Title), make(chan int)))
	tpa.workers.PushWorker(worker.NewWorker("OMDB_Handler", &queue.OmdbTask{OnComplete: advanceFunc}, int(queue.Omdb), make(chan int)))
	tpa.workers.PushWorker(worker.NewWorker("Database_Committer", &queue.DatabaseTask{OnComplete: advanceFunc}, int(queue.Database), make(chan int)))

	return nil
}
