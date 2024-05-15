package integration_test

import (
	"testing"
	"time"

	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/hbomb79/go-chanassert"
	"github.com/stretchr/testify/assert"
)

// TestIngestion_MetadataFailure ensures that files
// which are not valid media files correctly reports failure. Retrying
// these ingestions should see the same error return.
func TestIngestion_MetadataFailure(t *testing.T) {
	tempDir, paths := helpers.TempDirWithFiles(t, []string{"thisisnotavalidfile.mp4"})
	req := helpers.NewTheaServiceRequest().
		WithDatabaseName(t.Name()).
		WithIngestDirectory(tempDir).
		WithEnvironmentVariable("INGEST_MODTIME_THRESHOLD_SECONDS", "0")
	srv := helpers.RequireThea(t, req)

	exp := srv.ActivityExpecter(t).Expect(
		chanassert.ExactlyNOf(2, helpers.MatchIngestUpdate(paths[0], ingest.Troubled)),
	)
	exp.Listen()

	client := srv.NewClientWithDefaultAdminUser(t)

	ingestsResponse, err := client.ListIngestsWithResponse(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, ingestsResponse.JSON200, "expected JSON response to be non-nil")
	assert.Len(t, *ingestsResponse.JSON200, 1, "expected ingests to have length 1")

	ingestItem := (*ingestsResponse.JSON200)[0]
	assert.Equal(t, gen.IngestStateTROUBLED, ingestItem.State)
	assert.Contains(t, ingestItem.Trouble.AllowedResolutionTypes, gen.RETRY)

	// Retry this item and observe that it will persistently fail. Sleep for the 'debounce' time
	// of the WS messages to ensure we receive both
	time.Sleep(time.Second * 2)

	_, err = client.ResolveIngestWithResponse(ctx, ingestItem.Id, gen.ResolveIngestJSONRequestBody{Method: gen.RETRY, Context: map[string]string{}})
	assert.NoError(t, err)
	exp.AssertSatisfied(t, time.Second*6)
}
