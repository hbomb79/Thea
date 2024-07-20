// ingest_test is responsible for ensuring that
// files from the host filesystem are correctly detected,
// ingested, and saved to Thea. No transcoding or other
// processing of this ingested content is performed, and the TMDB
// and DB integration is mocked.
package ingest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/ingest"
	mocks "github.com/hbomb79/Thea/internal/ingest/mocks"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// A default event bus which should be used as a NOOP event bus. DO NOT subscribe to this
// inside of a test as the subscriber are not removed between tests.
var (
	defaultEventBus = event.New()
	errExpected     = errors.New("test: expected error")
)

func init() {
	logger.SetMinLoggingLevel(logger.VERBOSE.Level())
}

type Service interface {
	DiscoverNewFiles()
	GetAllIngests() []*ingest.IngestItem
}

func startServiceWithBus(
	t *testing.T,
	config ingest.Config,
	searcherMock *mocks.MockSearcher,
	scraperMock *mocks.MockScraper,
	storeMock *mocks.MockDataStore,
	eventBus event.EventCoordinator,
) Service {
	srv, err := ingest.New(config, searcherMock, scraperMock, storeMock, eventBus)
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

// startService starts an ingest service instance using the
// config and mocks provided. A teardown function is returned, which
// should be called when the test is complete.
func startService(t *testing.T, config ingest.Config, searcherMock *mocks.MockSearcher, scraperMock *mocks.MockScraper, storeMock *mocks.MockDataStore) Service {
	return startServiceWithBus(t, config, searcherMock, scraperMock, storeMock, defaultEventBus)
}

func Test_EpisodeImports_CorrectlySaved(t *testing.T) {
	t.Parallel()
	tempDir, files := helpers.TempDirWithEmptyFiles(t, []string{"episode"})

	cfg := ingest.Config{ForceSyncSeconds: 100, IngestPath: tempDir, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	year := 2023
	frameSize := 10
	seriesID := "123"
	seasonID := "456"
	episodeID := "789"
	expectedMetdata := media.FileMediaMetadata{
		Title:         "Test Episode",
		Episodic:      true,
		SeasonNumber:  1,
		EpisodeNumber: 1,
		Runtime:       "69420",
		Year:          year,
		FrameW:        frameSize,
		FrameH:        frameSize,
		Path:          files[0],
	}

	expectedSeries := &tmdb.Series{
		ID:       json.Number(seriesID),
		Adult:    false,
		Name:     "Test Series",
		Overview: "...",
		Genres:   []tmdb.Genre{{ID: json.Number("1"), Name: "Action"}, {ID: json.Number("2"), Name: "Adventure"}},
	}
	expectedSeason := &tmdb.Season{ID: json.Number(seasonID), Name: "Test Season", Overview: "..."}
	expectedEpisode := &tmdb.Episode{ID: json.Number(episodeID), Name: "Test Episode", Overview: "..."}

	storeMock.EXPECT().GetAllMediaSourcePaths().Return([]string{}, nil)

	// Allow ingestion to get metadata for this episode
	scraperMock.EXPECT().ScrapeFileForMediaInfo(files[0]).Return(&expectedMetdata, nil).Once()

	// Allow ingestion to find TMDB metadata for this metadata
	searcherMock.EXPECT().SearchForSeries(&expectedMetdata).Return(seriesID, nil).Once()
	searcherMock.EXPECT().GetSeries(seriesID).Return(expectedSeries, nil).Once()
	searcherMock.EXPECT().GetSeason(seriesID, expectedMetdata.SeasonNumber).Return(expectedSeason, nil).Once()
	searcherMock.EXPECT().GetEpisode(seriesID, expectedMetdata.SeasonNumber, expectedMetdata.EpisodeNumber).Return(expectedEpisode, nil).Once()

	// match a save call, but with custom matchers to ignore generated UUIDs
	var savedUUID *uuid.UUID = nil
	storeMock.EXPECT().SaveEpisode(
		mock.MatchedBy(func(given *media.Episode) bool {
			expected := tmdb.TmdbEpisodeToMedia(expectedEpisode, false, &expectedMetdata)
			expected.ID = given.ID
			savedUUID = &given.ID
			return reflect.DeepEqual(expected, given)
		}),
		mock.MatchedBy(func(given *media.Season) bool {
			expected := tmdb.TmdbSeasonToMedia(expectedSeason)
			expected.ID = given.ID
			return reflect.DeepEqual(expected, given)
		}),
		mock.MatchedBy(func(given *media.Series) bool {
			expected := tmdb.TmdbSeriesToMedia(expectedSeries)
			expected.ID = given.ID
			return reflect.DeepEqual(expected, given)
		}),
	).Return(nil).Once()

	bus := event.New()
	receivedIngestComplete := false
	receivedMediaMessage := false
	bus.RegisterHandlerFunction(event.NewMediaEvent, func(ev event.Event, payload event.Payload) {
		receivedMediaMessage = true
		assert.Equal(t, ev, event.NewMediaEvent)
		assert.Equal(t, payload, *savedUUID, "expected UUID emitted on event bus to match save call")
	})
	bus.RegisterHandlerFunction(event.IngestCompleteEvent, func(_ event.Event, _ event.Payload) {
		receivedIngestComplete = true
	})

	srv := startServiceWithBus(t, cfg, searcherMock, scraperMock, storeMock, bus)

	// Wait for item to leave the queue
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotNil(c, savedUUID)
		assert.True(c, receivedIngestComplete, "never received ingestion completion message on event bus")
		assert.True(c, receivedMediaMessage, "never received new media message on event bus")

		allIngests := srv.GetAllIngests()
		if len(allIngests) > 0 {
			assert.Len(c, allIngests, 1)
			item := allIngests[0]
			assert.NotNil(c, item)
			assert.NotEqual(c, item.State, ingest.ImportHold)
			assert.NotEqual(c, item.State, ingest.Idle)
		}
	}, time.Second*2, time.Millisecond*100)
}

func Test_MovieImports_CorrectlySaved(t *testing.T) {
	t.Parallel()
	tempDir, files := helpers.TempDirWithEmptyFiles(t, []string{"movie"})

	cfg := ingest.Config{ForceSyncSeconds: 100, IngestPath: tempDir, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	year := 2023
	frameSize := 10
	movieID := "123"
	expectedMetdata := media.FileMediaMetadata{
		Title:    "Test Movie",
		Episodic: false,
		Runtime:  "69420",
		Year:     year,
		FrameW:   frameSize,
		FrameH:   frameSize,
		Path:     files[0],
	}

	expectedMovie := &tmdb.Movie{
		ID:       json.Number(movieID),
		Adult:    false,
		Name:     "Test Series",
		Overview: "...",
		Genres: []tmdb.Genre{
			{ID: json.Number("1"), Name: "Action"},
			{ID: json.Number("2"), Name: "Adventure"},
		},
	}

	storeMock.EXPECT().GetAllMediaSourcePaths().Return([]string{}, nil)

	// Allow ingestion to get metadata for this episode
	scraperMock.EXPECT().ScrapeFileForMediaInfo(files[0]).Return(&expectedMetdata, nil).Once()

	// Allow ingestion to find TMDB metadata for this metadata
	searcherMock.EXPECT().SearchForMovie(&expectedMetdata).Return(movieID, nil).Once()
	searcherMock.EXPECT().GetMovie(movieID).Return(expectedMovie, nil).Once()

	// match a save call, but with custom matchers to ignore generated UUIDs
	var savedUUID *uuid.UUID = nil
	storeMock.EXPECT().SaveMovie(
		mock.MatchedBy(func(given *media.Movie) bool {
			expected := tmdb.TmdbMovieToMedia(expectedMovie, &expectedMetdata)
			expected.ID = given.ID
			savedUUID = &given.ID
			return reflect.DeepEqual(expected, given)
		}),
	).Return(nil).Once()

	bus := event.New()
	receivedIngestComplete := false
	receivedMediaMessage := false
	bus.RegisterHandlerFunction(event.NewMediaEvent, func(ev event.Event, payload event.Payload) {
		receivedMediaMessage = true
		assert.Equal(t, ev, event.NewMediaEvent)
		assert.Equal(t, payload, *savedUUID, "expected UUID emitted on event bus to match save call")
	})
	bus.RegisterHandlerFunction(event.IngestCompleteEvent, func(_ event.Event, _ event.Payload) {
		receivedIngestComplete = true
	})

	srv := startServiceWithBus(t, cfg, searcherMock, scraperMock, storeMock, bus)

	// Wait for item to leave the queue
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotNil(c, savedUUID)
		assert.True(c, receivedIngestComplete, "never received ingestion completion message on event bus")
		assert.True(c, receivedMediaMessage, "never received new media message on event bus")

		allIngests := srv.GetAllIngests()
		if len(allIngests) > 0 {
			assert.Len(c, allIngests, 1)
			item := allIngests[0]
			assert.NotNil(c, item)
			assert.NotEqual(c, item.State, ingest.ImportHold)
			assert.NotEqual(c, item.State, ingest.Idle)
		}
	}, time.Second*2, time.Millisecond*100)
}

func Test_NewFile_IgnoredIfAlreadyImported(t *testing.T) {
	t.Parallel()
	tempDir, files := helpers.TempDirWithEmptyFiles(t, []string{"anynameworks"})

	cfg := ingest.Config{ForceSyncSeconds: 100, IngestPath: tempDir, RequiredModTimeAgeSeconds: 2, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	storeMock.EXPECT().GetAllMediaSourcePaths().Return([]string{files[0]}, nil)

	srv := startService(t, cfg, searcherMock, scraperMock, storeMock)
	srv.DiscoverNewFiles()

	// Ensure file is not in queue as it matches an existing import.
	assert.Never(t, func() bool { return len(srv.GetAllIngests()) > 0 }, 2*time.Second, 500*time.Millisecond)
}

func Test_NewFile_CorrectlyHeld(t *testing.T) {
	t.Parallel()
	// Construct a new ingest service with the import delay set to a low value
	// and noop mocks for the dependencies.
	tempDir, files := helpers.TempDirWithEmptyFiles(t, []string{"anynameworks"})

	cfg := ingest.Config{ForceSyncSeconds: 100, IngestPath: tempDir, RequiredModTimeAgeSeconds: 2, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	scraperMock.EXPECT().ScrapeFileForMediaInfo(files[0]).Return(nil, errExpected)
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
			assert.Equal(c, errExpected.Error(), item.Trouble.Error())
		}
	}, 3*time.Second, 500*time.Millisecond)
}

func Test_PollsFilesystemPeriodically(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	cfg := ingest.Config{ForceSyncSeconds: 1, IngestPath: tempDir, RequiredModTimeAgeSeconds: 2, IngestionParallelism: 1}
	searcherMock := mocks.NewMockSearcher(t)
	scraperMock := mocks.NewMockScraper(t)
	storeMock := mocks.NewMockDataStore(t)

	calls := 0
	storeMock.EXPECT().GetAllMediaSourcePaths().RunAndReturn(func() ([]string, error) {
		calls++
		return []string{}, nil
	})

	_ = startService(t, cfg, searcherMock, scraperMock, storeMock)
	time.Sleep(4 * time.Second)
	assert.GreaterOrEqual(t, calls, 3, "Expected at least calls to 'GetAllMediaSourcePaths'")
}
