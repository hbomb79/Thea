package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/activity"
	"github.com/hbomb79/Thea/internal/api"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/workflow"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Core")

type (
	EventParticipator interface {
		RegisterEventCoordinator(*activity.EventCoordinator)
	}

	RunnableService interface {
		Run(context.Context)
	}

	RestGateway interface {
		RunnableService
	}

	ActivityService interface {
		RunnableService
	}

	TranscodeService interface {
		RunnableService
		NewTask(uuid.UUID, uuid.UUID) error
		CancelTask(uuid.UUID)
		AllTasks() []*transcode.TranscodeTask
		Task(uuid.UUID) *transcode.TranscodeTask
		TaskForMediaAndTarget(uuid.UUID, uuid.UUID) *transcode.TranscodeTask
	}

	DownloadService interface {
		RunnableService
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
	eventBus          activity.EventCoordinator
	shutdownWaitGroup *sync.WaitGroup
	config            TheaConfig

	mediaStore     *media.Store
	workflowStore  *workflow.Store
	targetStore    *ffmpeg.Store
	transcodeStore *transcode.Store

	activityService  ActivityService
	ingestService    IngestService
	transcodeService TranscodeService
	restGateway      RestGateway
}

const THEA_USER_DIR_SUFFIX = "/thea/"

func New(config TheaConfig) *theaImpl {
	/**  Bootstrapping  **/
	thea := &theaImpl{}
	thea.config = config
	thea.eventBus = activity.NewEventHandler()
	thea.shutdownWaitGroup = &sync.WaitGroup{}

	/**     Stores      **/
	thea.workflowStore = &workflow.Store{}
	thea.targetStore = &ffmpeg.Store{}
	thea.mediaStore = &media.Store{}
	thea.transcodeStore = &transcode.Store{}

	/**    Services     **/
	if serv, err := ingest.New(config.IngestService, thea.mediaStore); err == nil {
		thea.ingestService = serv
	} else {
		panic(fmt.Sprintf("failed to construct ingestion service due to error: %s", err.Error()))
	}

	if serv, err := activity.New(); err == nil {
		thea.activityService = serv
	} else {
		panic(fmt.Sprintf("failed to construct activity service due to error: %s", err.Error()))
	}

	if serv, err := transcode.New(config.Format, thea.mediaStore, thea.workflowStore, thea.targetStore, thea.transcodeStore); err == nil {
		thea.transcodeService = serv
	} else {
		panic(fmt.Sprintf("failed to construct transcode service due to error: %s", err.Error()))
	}

	thea.restGateway = api.NewRestGateway(&config.RestConfig, thea.ingestService, nil)

	return thea
}

// Run will start Thea by initialising all supporting services/objects and starting
// the event loops
func (thea *theaImpl) Run(ctx context.Context) error {
	config := thea.config
	log.Emit(logger.DEBUG, "Starting Thea initialisation with config: %#v\n", config)

	if err := thea.initialise(config); err != nil {
		return fmt.Errorf("failed to initialise Thea: %s", err)
	}

	thea.spawnAsyncService(ctx, thea.activityService, "activity-service")
	thea.spawnAsyncService(ctx, thea.ingestService, "ingest-service")
	thea.spawnAsyncService(ctx, thea.transcodeService, "transcode-service")
	thea.spawnAsyncService(ctx, thea.restGateway, "rest-gateway")
	thea.shutdownWaitGroup.Wait()

	return nil
}

func (thea *theaImpl) EventHandler() activity.EventHandler { return thea.eventBus }

func (thea *theaImpl) ActivityService() ActivityService   { return thea.activityService }
func (thea *theaImpl) IngestService() IngestService       { return thea.ingestService }
func (thea *theaImpl) TranscodeService() TranscodeService { return thea.transcodeService }

// spawnAsyncService will run the provided function/service as it's own
// go-routine, ensuring that the Thea service waitgroup is updated correctly
func (thea *theaImpl) spawnAsyncService(context context.Context, service RunnableService, label string) {
	if r, ok := service.(EventParticipator); ok {
		log.Emit(logger.NEW, "Injecting event coordinator into %s\n", label)
		r.RegisterEventCoordinator(&thea.eventBus)
	}

	thea.shutdownWaitGroup.Add(1)
	go func() {
		defer thea.shutdownWaitGroup.Done()
		service.Run(context)
	}()
}

// initialiseDockerServices will initialise all supporting services
// for Thea (Docker based Postgres, PgAdmin and Web front-end)
func (thea *theaImpl) initialiseDockerServices(config TheaConfig) error {
	// Instantiate watcher for async errors for the below containers
	asyncErrorReport := make(chan error, 2)
	go func() {
		err := <-asyncErrorReport
		log.Emit(logger.ERROR, "One or more support services has crashed with error: %v ... Shutting down", err)

		// Shutdown now because a support service has crashed...
		//TODO
		// thea.theaCtxCancel()
	}()

	// Initialise all services which are enabled. If a service is disabled, then the
	// user doesn't want us to create it for them. For the DB, this means the user *must*
	// provide the DB themselves
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

	return nil

}

// initialise will intialise all support services and workers, and connect to the backing DB
func (thea *theaImpl) initialise(config TheaConfig) error {
	log.Emit(logger.INFO, "Initialising Docker services...\n")
	if err := thea.initialiseDockerServices(config); err != nil {
		return err
	}

	log.Emit(logger.INFO, "Connecting to database with GORM...\n")
	if err := database.DB.Connect(config.Database); err != nil {
		return err
	}

	return nil
}
