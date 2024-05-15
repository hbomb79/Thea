package helpers

import (
	"github.com/hbomb79/Thea/internal/api/controllers/ingests"
	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/go-chanassert"
)

// MatchSocketMessage returns a matcher which will match messages which have
// the title and message type provided.
func MatchSocketMessage(title string, typ websocket.SocketMessageType) chanassert.Matcher[websocket.SocketMessage] {
	return chanassert.MatchStructPartial(websocket.SocketMessage{Title: title, Type: typ})
}

// MatchIngestUpdate returns a chanassert matcher which will
// match any websocket messages regarding ingestion updates
// which contain the given ingest.
func MatchIngestUpdate(path string, state ingest.IngestItemState) chanassert.Matcher[websocket.SocketMessage] {
	return chanassert.MatchPredicate(func(message websocket.SocketMessage) bool {
		if message.Title != "INGEST_UPDATE" {
			return false
		}

		updatedIngest, ok := message.Body["ingest"].(map[string]any)
		if !ok {
			return false
		}

		return updatedIngest["path"] == path && updatedIngest["state"] == string(ingests.IngestStateModelToDto(state))
	})
}

// type wrapMatcher[T any] struct {
// 	matcher chanassert.Matcher[T]
// 	latch   bool
// 	cb      func(T)
// }

// func (wrapper *wrapMatcher[T]) DoesMatch(t T) bool {
// 	res := wrapper.matcher.DoesMatch(t)
// 	if res && !wrapper.latch {
// 		wrapper.latch = true
// 		wrapper.cb(t)
// 	}

// 	return res
// }

// // matchNotifyWrapper accepts a matcher and will wrap it
// // such that the given callback function is called when
// // the provided matcher successfully matches a message.
// func matchNotifyWrapper[T any](matcher chanassert.Matcher[T], cb func(t T)) chanassert.Matcher[T] {
// 	return &wrapMatcher[T]{matcher: matcher, cb: cb}
// }
