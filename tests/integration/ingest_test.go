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

const (
	activityDebounceTime time.Duration = time.Second * 5
	activityMaxTime      time.Duration = time.Second * 20
)

// TestIngestion_MetadataFailure ensures that files
// which are not valid media files correctly reports failure. Retrying
// these ingestions should see the same error return.
func TestIngestion_MetadataFailure(t *testing.T) {
	tempDir, paths := helpers.TempDirWithEmptyFiles(t, []string{"thisisnotavalidfile.mp4"})

	// Caution: ensure this parallel call does not occur after the Thea service request. This is
	// because it will start to poll the ingest directory immediately, and if the test is 'paused'
	// before the expecter is setup, then it will miss the messages
	t.Parallel()

	req := helpers.NewTheaServiceRequest().WithIngestDirectory(tempDir)
	srv := helpers.RequireThea(t, req)
	user, client := srv.NewClientWithDefaultAdminUser(t)

	exp := srv.ActivityExpecter(t, user).Expect(
		chanassert.ExactlyNOf(2, helpers.MatchIngestUpdate(paths[0], ingest.Troubled)),
	)
	exp.Listen()

	ingest := assertIngestEventually(t, client, func(c *assert.CollectT, ingest gen.Ingest) {
		assert.Equal(c, gen.IngestStateTROUBLED, ingest.State, "Ingest state never became troubled")
		if assert.NotNil(c, ingest.Trouble, "expected non-nil trouble") {
			assert.Equal(c, gen.METADATAFAILURE, ingest.Trouble.Type, "Ingest trouble type never became correct")
			assert.Contains(c, ingest.Trouble.AllowedResolutionTypes, gen.RETRY)
			assert.Empty(c, ingest.Trouble.Context, "Expected Ingest trouble context to be empty")
		}
	})
	if ingest == nil {
		return
	}

	// Retry this item and observe that it will persistently fail. Sleep for the 'debounce' time
	// of the WS messages to ensure we receive both
	time.Sleep(activityDebounceTime)

	_, err := client.ResolveIngestWithResponse(ctx, ingest.Id, gen.ResolveIngestJSONRequestBody{Method: gen.RETRY, Context: map[string]string{}})
	assert.NoError(t, err)
	exp.AssertSatisfied(t, activityMaxTime)
}

// TestIngestion_TMDB_NoMatches tests that a file which
// contains valid media metadata but has no in TMDB.
//
//nolint:dupl
func TestIngestion_TMDB_NoMatches(t *testing.T) {
	ingestDir, files := helpers.TempDirWithFiles(t, map[string]string{
		"./testdata/validmedia/short-sample.mkv": "notarealmoviesurely.S01E01.1920x1080.mkv",
	})

	// Caution: ensure this parallel call does not occur after the Thea service request. This is
	// because it will start to poll the ingest directory immediately, and if the test is 'paused'
	// before the expecter is setup, then it will miss the messages
	t.Parallel()

	req := helpers.NewTheaServiceRequest().WithIngestDirectory(ingestDir).RequiresTMDB()
	srv := helpers.RequireThea(t, req)
	user, client := srv.NewClientWithDefaultAdminUser(t)

	exp := srv.ActivityExpecter(t, user).Expect(
		chanassert.OneOf(helpers.MatchIngestUpdate(files[0], ingest.Troubled)),
	)
	exp.Listen()

	assertIngestEventually(t, client, func(c *assert.CollectT, ingest gen.Ingest) {
		assert.Equal(c, gen.IngestStateTROUBLED, ingest.State, "Ingest state never became troubled")
		if assert.NotNil(c, ingest.Trouble, "expected non-nil trouble") {
			assert.Equal(c, gen.TMDBFAILURENORESULT, ingest.Trouble.Type, "Ingest trouble type never became correct")
			assert.Empty(c, ingest.Trouble.Context, "Expected Ingest trouble context to be empty")
		}
	})

	exp.AssertSatisfied(t, activityMaxTime)
}

// TestIngestion_TMDB_MultipleMatches tests that a file which
// contains valid media metadata but has multiple matches
// in TMDB.
//
//nolint:dupl
func TestIngestion_TMDB_MultipleMatches(t *testing.T) {
	ingestDir, files := helpers.TempDirWithFiles(t, map[string]string{
		"./testdata/validmedia/short-sample.mkv": "Sample.S01E01.1280x760.mkv",
	})

	// Caution: ensure this parallel call does not occur after the Thea service request. This is
	// because it will start to poll the ingest directory immediately, and if the test is 'paused'
	// before the expecter is setup, then it will miss the messages
	t.Parallel()

	req := helpers.NewTheaServiceRequest().WithIngestDirectory(ingestDir).RequiresTMDB()
	srv := helpers.RequireThea(t, req)
	user, client := srv.NewClientWithDefaultAdminUser(t)

	exp := srv.ActivityExpecter(t, user).Expect(
		chanassert.OneOf(helpers.MatchIngestUpdate(files[0], ingest.Troubled)),
	)
	exp.Listen()

	assertIngestEventually(t, client, func(c *assert.CollectT, ingest gen.Ingest) {
		assert.Equal(c, gen.IngestStateTROUBLED, ingest.State, "Ingest state never became troubled")
		if assert.NotNil(c, ingest.Trouble, "expected non-nil trouble") {
			assert.Equal(c, gen.TMDBFAILUREMULTIRESULT, ingest.Trouble.Type, "Ingest trouble type never became correct")
			assert.NotEmpty(c, ingest.Trouble.Context, "Expected Ingest trouble context to be non-empty")
		}
	})

	exp.AssertSatisfied(t, activityMaxTime)
}

func assertIngestEventually(t *testing.T, client gen.ClientWithResponsesInterface, cond func(c *assert.CollectT, i gen.Ingest)) *gen.Ingest {
	var ingestItem gen.Ingest
	if ok := assert.EventuallyWithT(t, func(c *assert.CollectT) {
		ingestsResponse, err := client.ListIngestsWithResponse(ctx)
		assert.NoError(c, err)
		assert.NotNil(c, ingestsResponse.JSON200, "expected JSON response to be non-nil")
		if !assert.Len(c, *ingestsResponse.JSON200, 1, "expected ingests to have length 1") {
			return // early return to prevent OOB error below
		}

		ingestItem = (*ingestsResponse.JSON200)[0]
		cond(c, ingestItem)
	}, 10*time.Second, 1*time.Second, "Ingestion state never became correct"); ok {
		return &ingestItem
	}

	return nil
}
