package internal

import (
	"context"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
)

type (
	RunnableService interface {
		Run(context.Context) error
	}

	RestGateway interface {
		RunnableService
		BroadcastTaskUpdate(taskID uuid.UUID) error
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

	// The serviceOrchestrator is responsible for exposing speciailised
	// data management tasks which require co-operation between
	// multiple services. To reduce complexity in Thea's codebase,
	// all services are expected to route their methods through
	// the serviceOrchestrator, even if there is no need for
	// coordination at the time: This allows for changes in the way
	// data is internally related *without* having to make breaking API changes.
	//
	// In general, it is discouraged to interface directly with
	// services due to the unknown nature of how data will intertwine
	// in the future. Rather, code should always endevour to
	// use the serviceOrchestrator to lodge requests (especially modification)
	// against services. In addition to minimizing the surface of API
	// breakage, it also hides the 'service abstraction', which results
	// in an API which is not only easier to maintain, but also to use.
	serviceOrchestrator struct {
		ingestService    IngestService
		transcodeService TranscodeService

		store *storeOrchestrator
	}
)

func newServiceOrchestrator(store *storeOrchestrator, ingestService IngestService, transcodeService TranscodeService) *serviceOrchestrator {
	return &serviceOrchestrator{store: store, ingestService: ingestService, transcodeService: transcodeService}
}

// * Media * //

func (orchestrator *serviceOrchestrator) GetMediaWatchTargets(mediaID uuid.UUID) ([]*media.WatchTarget, error) {
	targets := orchestrator.store.GetAllTargets()
	findTarget := func(tid uuid.UUID) *ffmpeg.Target {
		for _, v := range targets {
			if v.ID == tid {
				return v
			}
		}

		panic("Media references a target which does not exist. This should simply be unreachable unless the DB has lost referential integrity")
	}

	activeTranscodes := orchestrator.GetActiveTranscodeTasksForMedia(mediaID)
	completedTranscodes, err := orchestrator.store.GetTranscodesForMedia(mediaID)
	if err != nil {
		return nil, err
	}

	// 1. Add completed transcodes as valid pre-transcoded targets
	targetsNotEligibleForLiveTranscode := make(map[uuid.UUID]struct{}, len(activeTranscodes))
	watchTargets := make([]*media.WatchTarget, len(completedTranscodes))
	for k, v := range completedTranscodes {
		targetsNotEligibleForLiveTranscode[v.TargetID] = struct{}{}
		watchTargets[k] = newWatchTarget(findTarget(v.TargetID), media.PreTranscoded, true)
	}

	// 2. Add in-progress transcodes (as not ready to watch)
	for _, v := range activeTranscodes {
		targetsNotEligibleForLiveTranscode[v.Target().ID] = struct{}{}
		watchTargets = append(watchTargets, newWatchTarget(v.Target(), media.PreTranscoded, false))
	}

	// 3. Any targets which do NOT have a complete or in-progress pre-transcode are eligible for live transcoding/streaming
	for _, v := range targets {
		// TODO: check if the specified target allows for live transcoding
		if _, ok := targetsNotEligibleForLiveTranscode[v.ID]; ok {
			continue
		}

		watchTargets = append(watchTargets, newWatchTarget(v, media.LiveTranscode, true))
	}

	// 4. We can directly stream the source media itself, so add that too
	// TODO: at some point we may want this to be configurable
	watchTargets = append(watchTargets, &media.WatchTarget{Name: "Source", Ready: true, Type: media.LiveTranscode, TargetID: nil, Enabled: true})

	return watchTargets, nil
}

func (orchestrator *serviceOrchestrator) DeleteMovie(movieID uuid.UUID) error {
	return nil
}

func (orchestrator *serviceOrchestrator) DeleteSeries(movieID uuid.UUID) error {
	return nil
}

func (orchestrator *serviceOrchestrator) DeleteSeason(movieID uuid.UUID) error {
	return nil
}

func (orchestrator *serviceOrchestrator) DeleteEpisode(movieID uuid.UUID) error {
	return nil
}

func newWatchTarget(target *ffmpeg.Target, t media.WatchTargetType, ready bool) *media.WatchTarget {
	return &media.WatchTarget{Name: target.Label, Ready: ready, Type: t, TargetID: &target.ID, Enabled: true}
}

// * Ingests * //

func (orchestrator *serviceOrchestrator) RemoveIngest(ingestID uuid.UUID) error {
	return orchestrator.ingestService.RemoveIngest(ingestID)
}

func (orchestrator *serviceOrchestrator) GetIngest(ingestID uuid.UUID) *ingest.IngestItem {
	return orchestrator.ingestService.GetIngest(ingestID)
}

func (orchestrator *serviceOrchestrator) GetAllIngests() []*ingest.IngestItem {
	return orchestrator.ingestService.GetAllIngests()
}

func (orchestrator *serviceOrchestrator) DiscoverNewIngestableFiles() {
	orchestrator.ingestService.DiscoverNewFiles()
}

func (orchestrator *serviceOrchestrator) ResolveTroubledIngest(itemID uuid.UUID, method ingest.ResolutionType, context map[string]string) error {
	return orchestrator.ingestService.ResolveTroubledIngest(itemID, method, context)
}

// * Transcodes * //

func (orchestrator *serviceOrchestrator) NewTranscodeTask(mediaID uuid.UUID, targetID uuid.UUID) error {
	return orchestrator.transcodeService.NewTask(mediaID, targetID)
}

func (orchestrator *serviceOrchestrator) CancelTranscodeTask(taskID uuid.UUID) error {
	return orchestrator.transcodeService.CancelTask(taskID)
}

func (orchestrator *serviceOrchestrator) GetAllTranscodeTasks() []*transcode.TranscodeTask {
	return orchestrator.transcodeService.AllTasks()
}

func (orchestrator *serviceOrchestrator) GetTranscodeTask(taskID uuid.UUID) *transcode.TranscodeTask {
	return orchestrator.transcodeService.Task(taskID)
}

func (orchestrator *serviceOrchestrator) GetActiveTranscodeTasksForMediaAndTarget(mediaID uuid.UUID, targetID uuid.UUID) *transcode.TranscodeTask {
	return orchestrator.transcodeService.ActiveTaskForMediaAndTarget(mediaID, targetID)
}

func (orchestrator *serviceOrchestrator) GetActiveTranscodeTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask {
	return orchestrator.transcodeService.ActiveTasksForMedia(mediaID)
}

func (orchestrator *serviceOrchestrator) CancelTranscodeTasksForMedia(mediaID uuid.UUID) {
	orchestrator.transcodeService.CancelTasksForMedia(mediaID)
}
