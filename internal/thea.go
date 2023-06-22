package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/service/activity"
	"github.com/hbomb79/Thea/internal/service/ingest"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Core")

// // ProfileManager is an interface which allows Thea to store
// // and manipulate Transcode targets and workflows.
// type ProfileManager interface {
// 	Profiles() []profile.Profile
// 	InsertProfile(profile.Profile) error
// 	RemoveProfile(string) error
// 	FindProfile(func(profile.Profile) bool) (int, profile.Profile)
// 	FindProfileByTag(string) (int, profile.Profile)
// 	MoveProfile(string, int) error
// 	Save()
// }

type ActivityService interface {
	AsyncService
}

type TranscodeService interface {
	AsyncService
}

type DownloadService interface {
	AsyncService
}

type IngestService interface {
	AsyncService
}

type AsyncService interface {
	Run(context.Context)
}

// Thea represents the top-level object for the server, and is responsible
// for initialising embedded support services, workers, threads, event
// handling, et cetera...
type theaImpl struct {
	eventBus         activity.EventHandler
	serviceWaitGroup *sync.WaitGroup
	config           TheaConfig

	// profileManager ProfileManager

	activityService  ActivityService
	downloadService  DownloadService
	ingestService    IngestService
	transcodeService TranscodeService
}

const THEA_USER_DIR_SUFFIX = "/thea/"

func New(config TheaConfig) *theaImpl {
	eventBus := activity.NewEventHandler()
	thea := &theaImpl{
		eventBus:         eventBus,
		serviceWaitGroup: &sync.WaitGroup{},
		config:           config,
	}

	if serv, err := ingest.New(ingest.Config{}); err == nil {
		thea.ingestService = serv
	} else {
		panic(fmt.Sprintf("failed to construct ingestion service due to error: %s", err.Error()))
	}

	// thea.activityService = activity.New()
	// thea.downloadService = download.New()
	// thea.transcodeService = transcode.NewFfmpegCommander(thea, config.Format)

	return thea
}

// Start will start Thea by initialising all supporting services/objects and starting
// the event loops
func (thea *theaImpl) Start(ctx context.Context) error {
	config := thea.config
	log.Emit(logger.DEBUG, "Starting Thea initialisation with config: %#v\n", config)

	if err := thea.initialise(config); err != nil {
		return fmt.Errorf("failed to initialise Thea: %s", err)
	}

	thea.spawnAsyncService(ctx, thea.activityService)
	thea.spawnAsyncService(ctx, thea.downloadService)
	thea.spawnAsyncService(ctx, thea.ingestService)
	thea.spawnAsyncService(ctx, thea.transcodeService)
	thea.serviceWaitGroup.Wait()

	return nil
}

func (thea *theaImpl) EventHandler() activity.EventHandler { return thea.eventBus }

// func (thea *theaImpl) Profiles() ProfileManager            { return thea.profileManager }
func (thea *theaImpl) ActivityService() ActivityService   { return thea.activityService }
func (thea *theaImpl) DownloadService() DownloadService   { return thea.downloadService }
func (thea *theaImpl) IngestService() IngestService       { return thea.ingestService }
func (thea *theaImpl) TranscodeService() TranscodeService { return thea.transcodeService }

// ** PRIVATE IMPL ** //

// spawnAsyncService will run the provided function/service as it's own
// go-routine, ensuring that the Thea service waitgroup is updated correctly
func (thea *theaImpl) spawnAsyncService(context context.Context, service AsyncService) {
	thea.serviceWaitGroup.Add(1)

	go func() {
		defer thea.serviceWaitGroup.Done()
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
