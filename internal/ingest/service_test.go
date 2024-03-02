// ingest_test is responsible for ensuring that
// files from the host filesystem are correctly detected,
// ingested, and saved to Thea. No transcoding or other
// processing of this ingested content is performed, and the TMDB
// and DB integration is mocked.
package ingest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/ingest"
	mocks "github.com/hbomb79/Thea/mocks/ingest"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// A default event bus which should be used as a NOOP event bus. DO NOT subscribe to this
// inside of a test as the subscriber are not removed between tests.
var defaultEventBus = event.New()

func init() {
	logger.SetMinLoggingLevel(logger.VERBOSE.Level())
}

type Service interface {
	DiscoverNewFiles()
	GetAllIngests() []*ingest.IngestItem
}

// startService starts an ingest service instance using the
// config and mocks provided. A teardown function is returned, which
// should be called when the test is complete.
func startService(t *testing.T, config ingest.Config, searcherMock *mocks.MockSearcher, scraperMock *mocks.MockScraper, storeMock *mocks.MockDataStore) Service {
	srv, err := ingest.New(config, searcherMock, scraperMock, storeMock, defaultEventBus)
	assert.Nil(t, err)

	// Start ingest service
	wg := sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		assert.Nil(t, srv.Run(ctx))
	}()

	t.Cleanup(func() {
		fmt.Println("Waiting for service to close...")
		cancel()
		wg.Wait()
	})

	return srv
}

func tempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "thea_ingest_test")
	assert.Nil(t, err, "failed to create temporary dir")
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	return tempDir
}

func tempDirWithFiles(t *testing.T, files []string) (string, []string) {
	dirPath := tempDir(t)
	filePaths := make([]string, 0, len(files))
	for _, filename := range files {
		fileName, err := os.CreateTemp(dirPath, filename)
		filePaths = append(filePaths, fileName.Name())
		assert.Nil(t, err, "failed to create temporary file in temporary dir")
	}

	return dirPath, filePaths
}

func Test_EpisodeImports_CorrectlySaved(t *testing.T) {
	// Start service
	// Provide new file
	// Ensure detected
	// Mock scraper to provide episodic metadata
	// Mock searcher to provide information for series, movie and episode
	// Mock data store and ensure the three are saved as expected
}

func Test_MovieImports_CorrectlySaved(t *testing.T) {
	// Start service
	// Provide new file
	// Ensure detected
	// Mock scraper to provide movie metadata
	// Mock searcher to provide information for a movie
	// Mock data store to ensure the single move is saved as expected
}

func Test_NewFile_CorrectlyHeld(t *testing.T) {
	expectedErr := errors.New("test: expected error")

	// Construct a new ingest service with the import delay set to a low value
	// and noop mocks for the dependencies.
	tempDir, files := tempDirWithFiles(t, []string{"anynameworks"})
	assert.Len(t, files, 1, "expected only one temp file")

	cfg := ingest.Config{ForceSyncSeconds: 100, IngestPath: tempDir, RequiredModTimeAgeSeconds: 2, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	scraperMock.EXPECT().ScrapeFileForMediaInfo(files[0]).Return(nil, expectedErr)
	storeMock.EXPECT().GetAllMediaSourcePaths().Return([]string{}, nil)

	srv := startService(t, cfg, searcherMock, scraperMock, storeMock)

	// Assert that dummy item is in import hold shortly after service startup
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		all := srv.GetAllIngests()
		assert.Len(c, all, 1)
		assert.Equal(c, ingest.ImportHold, all[0].State)
	}, 1*time.Second, 500*time.Millisecond)

	// Assert dummy still import held after forced resync
	srv.DiscoverNewFiles()
	all := srv.GetAllIngests()
	assert.Len(t, all, 1)
	assert.Equal(t, ingest.ImportHold, all[0].State)

	// Assert dummy item is now unheld and has failed with expected error
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		all := srv.GetAllIngests()
		assert.Len(c, all, 1)

		item := all[0]
		assert.Equal(c, ingest.Troubled, item.State)
		assert.NotNil(c, ingest.MetadataFailure, item.Trouble)
		if item.Trouble != nil {
			assert.Equal(c, ingest.MetadataFailure, item.Trouble.Type())
			assert.Equal(c, expectedErr.Error(), item.Trouble.Error())
		}
	}, 3*time.Second, 500*time.Millisecond)
}
