package integration_test

import (
	"testing"

	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/stretchr/testify/assert"
)

// TestIngestion_FailsToMetadataScrape ensures that files
// which are not valid media files correctly reports failure. Retrying
// these ingestions should see the same error return.
func TestIngestion_FailsToMetadataScrape(t *testing.T) {
	tempDir, _ := helpers.TempDirWithFiles(t, []string{"thisisnotavalidfile.mp4"})
	srvReq := helpers.NewTheaServiceRequest().WithDatabaseName(t.Name()).WithIngestDirectory(tempDir)
	srv := helpers.RequireThea(t, srvReq)

	// Connect to the websocket
	ws := srv.ConnectToActivitySocket(t)
	assert.NotNil(t, ws, "ahh")
}
