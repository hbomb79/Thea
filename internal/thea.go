package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/pkg/docker"
	"github.com/hbomb79/Thea/pkg/logger"
	"gorm.io/gorm"
)

var log = logger.Get("Core")

type (
	EventParticipator interface {
		RegisterEventCoordinator(event.EventCoordinator)
	}

	RunnableService interface {
		Run(context.Context) error
	}

	DatabaseServer interface {
		Connect(database.DatabaseConfig) error
		GetInstance() *gorm.DB
		RegisterModels(...any)
	}

	RestGateway interface {
		RunnableService
		BroadcastTaskUpdate(uuid.UUID) error
		BroadcastTaskProgressUpdate(uuid.UUID) error
		BroadcastWorkflowUpdate(uuid.UUID) error
		BroadcastDownloadUpdate(uuid.UUID) error
		BroadcastDownloadProgressUpdate(uuid.UUID) error
		BroadcastMediaUpdate(uuid.UUID) error
		BroadcastIngestUpdate(uuid.UUID) error
	}

	TranscodeService interface {
		RunnableService
		EventParticipator
		NewTask(uuid.UUID, uuid.UUID) error
		CancelTask(uuid.UUID)
		AllTasks() []*transcode.TranscodeTask
		Task(uuid.UUID) *transcode.TranscodeTask
		TaskForMediaAndTarget(uuid.UUID, uuid.UUID) *transcode.TranscodeTask
	}

	IngestService interface {
		RunnableService
		RemoveItem(uuid.UUID) error
		Item(uuid.UUID) *ingest.IngestItem
		AllItems() []*ingest.IngestItem
	}
)

// Thea represents the top-level object for the server, and is responsible
// for initialising embedded support services, services, stores, event
// handling, et cetera...
type theaImpl struct {
	eventBus        event.EventCoordinator
	activityManager *activityManager
	config          TheaConfig
	dockerManager   docker.DockerManager

	mediaStore     *media.Store
	workflowStore  *workflow.Store
	targetStore    *ffmpeg.Store
	transcodeStore *transcode.Store

	restGateway      RestGateway
	ingestService    IngestService
	transcodeService TranscodeService
}

const THEA_USER_DIR_SUFFIX = "/thea/"

func New(config TheaConfig) *theaImpl {
	log.Emit(logger.DEBUG, "Bootstrapping Thea services using config: %#v\n", config)
	thea := &theaImpl{
		eventBus: event.New(),
		config:   config,
	}

	if serv, err := ingest.New(config.IngestService, thea.mediaStore); err == nil {
		thea.ingestService = serv
	} else {
		panic(fmt.Sprintf("failed to construct ingestion service due to error: %s", err.Error()))
	}

	if serv, err := transcode.New(config.Format, thea.mediaStore, thea.workflowStore, thea.targetStore, thea.transcodeStore); err == nil {
		thea.transcodeService = serv
	} else {
		panic(fmt.Sprintf("failed to construct transcode service due to error: %s", err.Error()))
	}

	thea.restGateway = api.NewRestGateway(&config.RestConfig, thea.ingestService, nil)
	thea.activityManager = newActivityManager(thea.restGateway, thea.eventBus)

	return thea
}

// Run will start all of Thea by bringing up all required services and connections, such as:
// - Docker services
// - Stores
// - Database connection
// - Service instances
//
// This function will not return until Thea is stopped.
// To stop Thea, the provided context must be cancelled. Errors from which Thea cannot recover
// will also cause Thea to stop.
func (thea *theaImpl) Run(parent context.Context) error {
	thea.dockerManager = docker.NewDockerManager()
	defer thea.dockerManager.Shutdown(time.Second * 10)

	ctx, cancel := context.WithCancel(parent)
	crashHandler := func(label string, err error) {
		log.Emit(logger.FATAL, "Service crash (%s)! %s\n", label, err.Error())
		cancel()
	}

	log.Emit(logger.NEW, "Initialising Docker services...\n")
	if err := thea.initialiseDockerServices(thea.config, crashHandler); err != nil {
		return err
	}

	log.Emit(logger.NEW, "Connecting to database with GORM...\n")
	db := database.New()
	thea.initialiseStores(db)
	if err := db.Connect(thea.config.Database); err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	thea.spawnAsyncService(ctx, wg, thea.ingestService, "ingest-service", crashHandler)
	thea.spawnAsyncService(ctx, wg, thea.transcodeService, "transcode-service", crashHandler)
	thea.spawnAsyncService(ctx, wg, thea.restGateway, "rest-gateway", crashHandler)
	log.Emit(logger.SUCCESS, "Thea services spawned!\n")

	wg.Wait()
	return nil
}

// spawnAsyncService will run the provided function/service as it's own
// go-routine, ensuring that the Thea service waitgroup is updated correctly
func (thea *theaImpl) spawnAsyncService(context context.Context, wg *sync.WaitGroup, service RunnableService, serviceLabel string, crashHandler func(string, error)) {
	log.Emit(logger.NEW, "Spawning %s\n", serviceLabel)
	wg.Add(1)

	go func(wg *sync.WaitGroup, label string, crash func(string, error)) {
		defer func() {
			if r := recover(); r != nil {
				crash(label, fmt.Errorf("panic %v", r))
			}
		}()

		defer wg.Done()
		if err := service.Run(context); err != nil {
			crash(label, err)
		}
	}(wg, serviceLabel, crashHandler)
}

// initialiseDockerServices will initialise all supporting services
// for Thea (Postgres, PgAdmin)
func (thea *theaImpl) initialiseDockerServices(config TheaConfig, crashHandler func(string, error)) error {
	if config.Services.EnablePostgres {
		log.Emit(logger.INFO, "Initialising embedded database...\n")
		if _, err := database.InitialiseDockerDatabase(
			thea.dockerManager,
			config.Database,
			func(err error) { crashHandler("docker-postgres", err) },
		); err != nil {
			return err
		}
	}

	if config.Services.EnablePgAdmin {
		log.Emit(logger.INFO, "Initialising embedded pgAdmin server...\n")
		if _, err := database.InitialiseDockerPgAdmin(
			thea.dockerManager,
			func(err error) { crashHandler("docker-pgadmin", err) },
		); err != nil {
			return err
		}
	}

	return nil
}

func (thea *theaImpl) initialiseStores(db DatabaseServer) error {
	if store, err := media.NewStore(db); err == nil {
		thea.mediaStore = store
	} else {
		return err
	}
	if store, err := workflow.NewStore(db); err == nil {
		thea.workflowStore = store
	} else {
		return err
	}
	if store, err := ffmpeg.NewStore(db); err == nil {
		thea.targetStore = store
	} else {
		return err
	}
	if store, err := transcode.NewStore(db); err == nil {
		thea.transcodeStore = store
	} else {
		return err
	}

	return nil
}
