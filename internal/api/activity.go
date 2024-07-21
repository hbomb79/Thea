package api

import (
	"errors"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/controllers/ingests"
	"github.com/hbomb79/Thea/internal/api/controllers/transcodes"
	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/internal/user/permissions"
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

	clientScopes map[authScope][]uuid.UUID
	clientMutex  *sync.Mutex
}

func newBroadcaster(
	socketHub *websocket.SocketHub,
	ingestService ingests.IngestService,
	transcodeService TranscodeService,
	store Store,
) *broadcaster {
	return &broadcaster{socketHub, ingestService, transcodeService, store, make(map[authScope][]uuid.UUID, 0), &sync.Mutex{}}
}

type authScope int

const (
	mediaScope authScope = iota
	transcodeScope
	ingestScope
)

var scopePerms = map[authScope][]string{
	mediaScope:     {permissions.AccessMediaPermission},
	transcodeScope: {permissions.AccessTranscodePermission},
	ingestScope:    {permissions.AccessIngestsPermission},
}

// sliceContainsAll returns true if the slice 'a' contains
// ALL the elements inside of 'b'.
func sliceContainsAll[T comparable](a, b []T) bool {
	for _, v := range b {
		if !slices.Contains(a, v) {
			return false
		}
	}

	return true
}

func (hub *broadcaster) RegisterClient(clientID uuid.UUID, permissions []string) {
	hub.clientMutex.Lock()
	defer hub.clientMutex.Unlock()

	for scope, requiredPerms := range scopePerms {
		if sliceContainsAll(permissions, requiredPerms) {
			hub.clientScopes[scope] = append(hub.clientScopes[scope], clientID)
		}
	}
}

func (hub *broadcaster) DeregisterClient(clientID uuid.UUID) {
	hub.clientMutex.Lock()
	defer hub.clientMutex.Unlock()

	for k, clients := range hub.clientScopes {
		hub.clientScopes[k] = slices.DeleteFunc(clients, func(id uuid.UUID) bool { return id == clientID })
	}
}

func (hub *broadcaster) protectedSend(scope authScope, title string, body map[string]interface{}) {
	clients := hub.clientScopes[scope]
	for _, client := range clients {
		// TODO: this could cause quite the number of messages to be sent. Probably fine for
		// now, but maybe a queue + worker pool might make sense?
		hub.socketHub.Send(&websocket.SocketMessage{
			Target: &client,
			Title:  title,
			Body:   body,
			Type:   websocket.Update,
		})
	}
}

func (hub *broadcaster) BroadcastTranscodeUpdate(id uuid.UUID) error {
	item := hub.transcodeService.Task(id)
	hub.protectedSend(transcodeScope, TitleTranscodeUpdate, map[string]interface{}{
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

	hub.protectedSend(transcodeScope, TitleTranscodeProgressUpdate, map[string]interface{}{
		"transcode_id": id,
		"progress":     item.LastProgress(),
	})
	return nil
}

func (hub *broadcaster) BroadcastIngestUpdate(id uuid.UUID) error {
	item := hub.ingestService.GetIngest(id)
	hub.protectedSend(ingestScope, TitleIngestUpdate, map[string]interface{}{
		"ingest_id": id,
		"ingest":    nullsafeNewDto(item, ingests.NewDto),
	})
	return nil
}

func (hub *broadcaster) BroadcastWorkflowUpdate(id uuid.UUID) error {
	return errors.New("not yet implemented")
}

func (hub *broadcaster) BroadcastMediaUpdate(id uuid.UUID) error {
	media := hub.store.GetMedia(id)
	hub.protectedSend(mediaScope, TitleMediaUpdate, map[string]interface{}{
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
