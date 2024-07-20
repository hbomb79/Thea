package api

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/controllers/ingests"
	"github.com/hbomb79/Thea/internal/api/controllers/transcodes"
	"github.com/hbomb79/Thea/internal/http/websocket"
)

const (
	TitleIngestUpdate            = "INGEST_UPDATE"
	TitleMediaUpdate             = "MEDIA_UPDATE"
	TitleTranscodeUpdate         = "TRANSCODE_TASK_UPDATE"
	TitleTranscodeProgressUpdate = "TRANSCODE_TASK_PROGRESS_UPDATE"
)

type broadcaster struct {
	socketHub        *websocket.SocketHub
	ingestService    ingests.IngestService
	transcodeService TranscodeService
	store            Store
}

func newBroadcaster(
	socketHub *websocket.SocketHub,
	ingestService ingests.IngestService,
	transcodeService TranscodeService,
	store Store,
) *broadcaster {
	return &broadcaster{socketHub, ingestService, transcodeService, store}
}

func (hub *broadcaster) BroadcastTranscodeUpdate(id uuid.UUID) error {
	item := hub.transcodeService.Task(id)
	hub.broadcast(TitleTranscodeUpdate, map[string]interface{}{
		"id":        id,
		"transcode": nullsafeNewDto(item, transcodes.NewDtoFromTask),
	})
	return nil
}

func (hub *broadcaster) BroadcastTaskProgressUpdate(id uuid.UUID) error {
	item := hub.transcodeService.Task(id)
	if item == nil {
		return nil
	}

	hub.broadcast(TitleTranscodeProgressUpdate, map[string]interface{}{
		"transcode_id": id,
		"progress":     item.LastProgress(),
	})
	return nil
}

func (hub *broadcaster) BroadcastIngestUpdate(id uuid.UUID) error {
	item := hub.ingestService.GetIngest(id)
	hub.broadcast(TitleIngestUpdate, map[string]interface{}{
		"ingest_id": id,
		"ingest":    nullsafeNewDto(item, ingests.NewDto),
	})
	return nil
}

func (hub *broadcaster) broadcast(title string, update map[string]interface{}) {
	hub.socketHub.Send(&websocket.SocketMessage{
		Title: title,
		Body:  update,
		Type:  websocket.Update,
	})
}

func (hub *broadcaster) BroadcastWorkflowUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastMediaUpdate(id uuid.UUID) error {
	media := hub.store.GetMedia(id)
	hub.broadcast(TitleMediaUpdate, map[string]interface{}{
		"media_id": id,
		"media":    media,
	})

	return nil
}

// nullsafeNewDto returns nil if the given model is nil, else it will call the
// provided generator with the model as it's only parameter. This is basically
// shorthand for "only try and create a DTO if the 'model' isn't nil".
func nullsafeNewDto[M any, D any](model *M, generator func(*M) D) *D {
	if model == nil {
		return nil
	}

	out := generator(model)
	return &out
}
