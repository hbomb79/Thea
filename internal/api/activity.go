package api

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/ingests"
	"github.com/hbomb79/Thea/internal/http/websocket"
)

const (
	TITLE_INGEST_UPDATE = "INGEST_UPDATE"
)

type (
	IngestUpdate struct {
		IngestId uuid.UUID          `json:"ingest_id"`
		Ingest   *ingests.IngestDto `json:"ingest"`
	}

	TaskUpdate         struct{}
	TaskProgressUpdate struct{}
	WorkflowUpdate     struct{}
	MediaUpdate        struct{}

	broadcaster struct {
		socketHub     *websocket.SocketHub
		ingestService ingests.IngestService
		dataStore     DataStore
	}
)

func newBroadcaster(
	socketHub *websocket.SocketHub,
	ingestService ingests.IngestService,
	store DataStore,
) *broadcaster {
	return &broadcaster{socketHub, ingestService, store}
}

func (hub *broadcaster) BroadcastTaskUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastTaskProgressUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastWorkflowUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastMediaUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastIngestUpdate(id uuid.UUID) error {
	item := hub.ingestService.GetIngest(id)
	update := IngestUpdate{IngestId: id, Ingest: ingests.NewDto(item)}
	hub.broadcast(TITLE_INGEST_UPDATE, update)

	return nil
}

func (hub *broadcaster) broadcast(title string, update any) {
	hub.socketHub.Send(&websocket.SocketMessage{
		Title: title,
		Body:  map[string]interface{}{"arguments": update},
		Type:  websocket.Update,
	})
}
