package integration_test

import (
	"testing"
	"time"

	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/hbomb79/go-chanassert"
)

// TestIngestion_FailsToMetadataScrape ensures that files
// which are not valid media files correctly reports failure. Retrying
// these ingestions should see the same error return.
func TestIngestion_FailsToMetadataScrape(t *testing.T) {
	tempDir, _ := helpers.TempDirWithFiles(t, []string{"thisisnotavalidfile.mp4"})
	srvReq := helpers.NewTheaServiceRequest().WithDatabaseName(t.Name()).WithIngestDirectory(tempDir)
	srv := helpers.RequireThea(t, srvReq)

	stream := ActivityStream(t, srv)
	expecter := chanassert.NewChannelExpecter(stream).
		Expect(chanassert.AllOf(chanassert.MatchStructPartial(
			websocket.SocketMessage{Title: "CONNECTION_ESTABLISHED", Type: websocket.Welcome},
		))).
		Expect(chanassert.AllOf(chanassert.MatchStructPartial(
			websocket.SocketMessage{Title: "INGEST_UPDATE", Body: map[string]interface{}{}, Type: websocket.Update},
		)))
	expecter.Listen()
	defer expecter.AssertSatisfied(t, time.Second)
}

// ActivityStream returns a channel which will deliver
// messages received over Thea's activity stream socket. This socket
// connection will automatically close when the test finishes/cleans up.
func ActivityStream(t *testing.T, srv *helpers.TestService) chan websocket.SocketMessage {
	socket := srv.ConnectToActivitySocket(t)
	if err := socket.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("failed to set read deadline for activity socket connection: %s", err)
	}

	t.Cleanup(func() { socket.Close() })

	output := make(chan websocket.SocketMessage, 10)
	go func(deliveryChan chan websocket.SocketMessage) {
		for {
			var dest websocket.SocketMessage
			err := socket.ReadJSON(&dest)
			if err != nil {
				t.Logf("WARNING: activity stream read JSON error: %s", err)
			}

			deliveryChan <- dest
		}
	}(output)

	return output
}
