package internal

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/pkg/docker"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Core")

type (
	RunnableService interface {
		Run(ctx context.Context) error
	}

	RestGateway interface {
		RunnableService
		BroadcastTranscodeUpdate(taskID uuid.UUID) error
		BroadcastTaskProgressUpdate(taskID uuid.UUID) error
		BroadcastWorkflowUpdate(workflowID uuid.UUID) error
		BroadcastMediaUpdate(mediaID uuid.UUID) error
		BroadcastIngestUpdate(ingestID uuid.UUID) error
	}

	TranscodeService interface {
		RunnableService
		NewTask(mediaID uuid.UUID, targetID uuid.UUID) error
		CancelTask(taskID uuid.UUID) error
		AllTasks() []*transcode.TranscodeTask
		Task(taskID uuid.UUID) *transcode.TranscodeTask
		PauseTask(taskID uuid.UUID) error
		ResumeTask(taskID uuid.UUID) error
		ActiveTaskForMediaAndTarget(mediaID uuid.UUID, targetID uuid.UUID) *transcode.TranscodeTask
		ActiveTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
		CancelTasksForMedia(mediaID uuid.UUID)
	}

	IngestService interface {
		RunnableService
		RemoveIngest(ingestID uuid.UUID) error
		GetIngest(ingestID uuid.UUID) *ingest.IngestItem
		GetAllIngests() []*ingest.IngestItem
		DiscoverNewFiles()
		ResolveTroubledIngest(itemID uuid.UUID, method ingest.ResolutionType, context map[string]string) error
	}
)

const (
	TheaUserDirSuffix = "/thea/"

	dockerShutdownTimeout = time.Second * 10
)

// Thea represents the top-level object for the server, and is responsible
// for initialising embedded support services, stores, event
// handling, et cetera...
type theaImpl struct {
	eventBus          event.EventCoordinator
	dockerManager     docker.DockerManager
	storeOrchestrator *storeOrchestrator
	activityService   *activityService
	config            TheaConfig

	restGateway      RestGateway
	ingestService    IngestService
	transcodeService TranscodeService
}

func New(config TheaConfig) *theaImpl {
	log.Emit(logger.DEBUG, "Bootstrapping Thea services using config: %#v\n", config)
	thea := &theaImpl{
		eventBus: event.New(),
		config:   config,
	}

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
	defer thea.dockerManager.Shutdown(dockerShutdownTimeout)

	ctx, cancel := context.WithCancel(parent)
	crashHandler := func(label string, err error) {
		log.Emit(logger.FATAL, "Service crash (%s)! %v\n", label, err)
		cancel()
	}

	log.Emit(logger.NEW, "Initialising Docker services...\n")
	if err := thea.initialiseDockerServices(thea.config, crashHandler); err != nil {
		return fmt.Errorf("failed to initialise docker services: %w", err)
	}

	log.Emit(logger.NEW, "Connecting to database...\n")
	db := database.New()
	if err := db.Connect(thea.config.Database); err != nil {
		return fmt.Errorf("failed to initialise connection to DB: %w", err)
	}

	store, err := newStoreOrchestrator(db, thea.eventBus)
	if err != nil {
		return fmt.Errorf("failed to construct data orchestrator: %w", err)
	}
	thea.storeOrchestrator = store
	if err := thea.syncDBPermissions(); err != nil {
		return fmt.Errorf("failed to sync db permissions: %w", err)
	}
	if err := thea.createInitialUserIfNonePresent(); err != nil {
		return fmt.Errorf("failed to create initial user: %w", err)
	}

	searcher := tmdb.NewSearcher(tmdb.Config{APIKey: thea.config.OmdbKey})
	scraper := media.NewScraper(media.ScraperConfig{FfprobeBinPath: thea.config.Format.FfprobeBinaryPath})
	if serv, err := ingest.New(thea.config.IngestService, searcher, scraper, thea.storeOrchestrator, thea.eventBus); err == nil {
		thea.ingestService = serv
	} else {
		return fmt.Errorf("failed to construct ingestion service due to error: %w", err)
	}

	if serv, err := transcode.New(thea.config.Format, thea.eventBus, thea.storeOrchestrator); err == nil {
		thea.transcodeService = serv
	} else {
		return fmt.Errorf("failed to construct transcode service due to error: %w", err)
	}

	thea.restGateway = api.NewRestGateway(&thea.config.RestConfig, thea.ingestService, thea.transcodeService, thea.storeOrchestrator)
	thea.activityService = newActivityService(thea.restGateway, thea.eventBus)

	wg := &sync.WaitGroup{}
	wg.Add(4)
	go thea.spawnService(ctx, wg, thea.ingestService, "ingest-service", crashHandler)
	go thea.spawnService(ctx, wg, thea.transcodeService, "transcode-service", crashHandler)
	go thea.spawnService(ctx, wg, thea.restGateway, "rest-gateway", crashHandler)
	go thea.spawnService(ctx, wg, thea.activityService, "activity-service", crashHandler)
	log.Emit(logger.SUCCESS, "Thea services spawned! [CTRL+C to stop]\n")

	wg.Wait()
	return nil
}

// spawnService will run the provided function/service as it's own
// go-routine, ensuring that the Thea service waitgroup is updated correctly.
func (thea *theaImpl) spawnService(context context.Context, wg *sync.WaitGroup, service RunnableService, serviceLabel string, crashHandler func(string, error)) {
	log.Emit(logger.NEW, "Spawning %s\n", serviceLabel)

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Service %s PANIC! Debug stack follows:\n---\n%s\n---\n", serviceLabel, string(debug.Stack()))
			crashHandler(serviceLabel, fmt.Errorf("panic %v", r))
		}
	}()

	defer wg.Done()
	if err := service.Run(context); err != nil {
		crashHandler(serviceLabel, err)
	}
}

// initialiseDockerServices will initialise all supporting services
// for Thea (Postgres, PgAdmin).
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

func (thea *theaImpl) syncDBPermissions() error {
	// Raise an error if a permission has been removed - a manual DB migration should be performed
	// to protect against accidental removal of a permission
	allPerms := permissions.All()
	outstanding, err := thea.storeOrchestrator.anyOutstandingPermissions(allPerms...)
	if err != nil {
		return err
	}
	if outstanding {
		return errors.New("permissions have been removed from code but still exist in db, manual migration required")
	}

	return thea.storeOrchestrator.createPermissions(permissions.All()...)
}

func (thea *theaImpl) createInitialUserIfNonePresent() error {
	users, err := thea.storeOrchestrator.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to check for existing users during bootstrapping: %w", err)
	} else if len(users) > 0 {
		log.Debugf("Existing users found (%d), not creating initial user\n", len(users))
		return nil
	}

	log.Emit(logger.NEW, "No existing users found, creating initial user [username='admin', password=REDACTED {refer to your configuration}]\n")
	_, err = thea.storeOrchestrator.CreateUser([]byte("admin"), []byte("admin"), permissions.All()...)
	return err
}
