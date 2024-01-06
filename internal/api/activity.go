package api

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/api/transcodes"
	"github.com/hbomb79/Thea/internal/http/websocket"
)

const (
	TITLE_INGEST_UPDATE             = "INGEST_UPDATE"
	TITLE_TRANSCODE_UPDATE          = "TRANSCODE_TASK_UPDATE"
	TITLE_TRANSCODE_PROGRESS_UPDATE = "TRANSCODE_TASK_PROGRESS_UPDATE"
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
	var dto *transcodes.TranscodeDto = nil
	if item != nil {
		d := transcodes.NewDtoFromTask(item)
		dto = &d
	}

	hub.broadcast(TITLE_TRANSCODE_UPDATE, map[string]interface{}{
		"id":        id,
		"transcode": dto,
	})

	return nil
}

func (hub *broadcaster) BroadcastTaskProgressUpdate(id uuid.UUID) error {
	item := hub.transcodeService.Task(id)
	if item == nil {
		return nil
	}

	hub.broadcast(TITLE_TRANSCODE_PROGRESS_UPDATE, map[string]interface{}{
		"transcode_id": id,
		"progress":     item.LastProgress(),
	})

	return nil
}

func (hub *broadcaster) BroadcastWorkflowUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastMediaUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastIngestUpdate(id uuid.UUID) error {
	item := hub.ingestService.GetIngest(id)
	hub.broadcast(TITLE_INGEST_UPDATE, map[string]interface{}{
		"ingest_id": id,
		"ingest":    ingests.NewDto(item),
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
