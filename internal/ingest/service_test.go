// ingest_test is responsible for ensuring that
// files from the host filesystem are correctly detected,
// ingested, and saved to Thea. No transcoding or other
// processing of this ingested content is performed, and the TMDB
// and DB integration is mocked.
package ingest_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// A default event bus which should be used as a NOOP event bus. DO NOT subscribe to this
// inside of a test as the subscriber are not removed between tests.
var defaultEventBus = event.New()

func init() {
	logger.SetMinLoggingLevel(logger.VERBOSE.Level())
}

type mockSearcher struct {
	mock.Mock
}

func (mock *mockSearcher) SearchForSeries(metadata *media.FileMediaMetadata) (string, error) {
	args := mock.Called(metadata)
	return args.String(0), args.Error(1)
}

func (mock *mockSearcher) SearchForMovie(metadata *media.FileMediaMetadata) (string, error) {
	args := mock.Called(metadata)
	return args.String(0), args.Error(1)
}

func (mock *mockSearcher) GetSeason(seriesID string, seasonNumber int) (*tmdb.Season, error) {
	args := mock.Called(seriesID, seasonNumber)
	//nolint:forcetypeassert
	return args.Get(0).(*tmdb.Season), args.Error(1)
}

func (mock *mockSearcher) GetSeries(seriesID string) (*tmdb.Series, error) {
	args := mock.Called(seriesID)
	//nolint:forcetypeassert
	return args.Get(0).(*tmdb.Series), args.Error(1)
}

func (mock *mockSearcher) GetEpisode(seriesID string, seasonNumber int, episodeNumber int) (*tmdb.Episode, error) {
	args := mock.Called(seriesID, seasonNumber, episodeNumber)
	//nolint:forcetypeassert
	return args.Get(0).(*tmdb.Episode), args.Error(1)
}

func (mock *mockSearcher) GetMovie(movieID string) (*tmdb.Movie, error) {
	args := mock.Called(movieID)
	//nolint:forcetypeassert
	return args.Get(0).(*tmdb.Movie), args.Error(1)
}

type mockScraper struct {
	mock.Mock
}

func (mock *mockScraper) ScrapeFileForMediaInfo(path string) (*media.FileMediaMetadata, error) {
	args := mock.Called(path)
	if v, ok := args.Get(0).(*media.FileMediaMetadata); ok {
		return v, args.Error(1)
	} else {
		return nil, args.Error(1)
	}
}

type mockStore struct {
	mock.Mock
}

func (mock *mockStore) GetAllMediaSourcePaths() ([]string, error) {
	args := mock.Called()
	//nolint:forcetypeassert
	return args.Get(0).([]string), args.Error(1)
}

func (mock *mockStore) GetSeasonWithTmdbID(seasonID string) (*media.Season, error) {
	args := mock.Called(seasonID)
	//nolint:forcetypeassert
	return args.Get(0).(*media.Season), args.Error(1)
}

func (mock *mockStore) GetSeriesWithTmdbID(seriesID string) (*media.Series, error) {
	args := mock.Called(seriesID)
	//nolint:forcetypeassert
	return args.Get(0).(*media.Series), args.Error(1)
}

func (mock *mockStore) GetEpisodeWithTmdbID(episodeID string) (*media.Episode, error) {
	args := mock.Called(episodeID)
	//nolint:forcetypeassert
	return args.Get(0).(*media.Episode), args.Error(1)
}

func (mock *mockStore) SaveEpisode(episode *media.Episode, season *media.Season, series *media.Series) error {
	args := mock.Called(episode, season, series)
	return args.Error(0)
}

func (mock *mockStore) SaveMovie(movie *media.Movie) error {
	args := mock.Called(movie)
	return args.Error(0)
}

type Service interface {
	DiscoverNewFiles()
	GetAllIngests() []*ingest.IngestItem
}

// startService starts an ingest service instance using the
// config and mocks provided. A teardown function is returned, which
// should be called when the test is complete.
func startService(t *testing.T, config ingest.Config, searcherMock *mockSearcher, scraperMock *mockScraper, storeMock *mockStore) (Service, func()) {
	srv, err := ingest.New(config, searcherMock, scraperMock, storeMock, defaultEventBus)
	assert.Nil(t, err)

	// Start ingest service
	wg := sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		assert.Nil(t, srv.Run(ctx))
		wg.Done()
	}()

	return srv, func() {
		cancel()
		wg.Wait()
	}
}

func tempDir(t *testing.T) (string, *os.File, func()) {
	tempDir, err := os.MkdirTemp("", "thea_ingest_test")
	assert.Nil(t, err)

	tempFile, err := os.CreateTemp(tempDir, "dummy_file")
	assert.Nil(t, err)

	return tempDir, tempFile, func() { os.RemoveAll(tempDir) }
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
	// Construct a new ingest service with the import delay set to a low value
	// and noop mocks for the dependencies.
	path, file, cleanup := tempDir(t)
	defer cleanup()

	cfg := ingest.Config{
		ForceSyncSeconds:          100,
		IngestPath:                path,
		RequiredModTimeAgeSeconds: 2,
		IngestionParallelism:      1,
	}

	searcherMock := new(mockSearcher)
	scraperMock := new(mockScraper)
	storeMock := new(mockStore)

	scraperMock.On("ScrapeFileForMediaInfo", file.Name()).Return(nil, errors.New("TESTING NOOP"))
	storeMock.On("GetAllMediaSourcePaths").Return([]string{}, nil)

	srv, teardown := startService(t, cfg, searcherMock, scraperMock, storeMock)
	defer teardown()

	// Assert that dummy item is in import hold shortly after service startup
	time.Sleep(1 * time.Second)
	{
		all := srv.GetAllIngests()
		assert.Len(t, all, 1)
		assert.Equal(t, ingest.ImportHold, all[0].State)
	}

	// Force a re-sync
	srv.DiscoverNewFiles()

	// Assert dummy still import held
	{
		all := srv.GetAllIngests()
		assert.Len(t, all, 1)
		assert.Equal(t, ingest.ImportHold, all[0].State)
	}

	// Wait 3 seconds
	time.Sleep(3 * time.Second)

	// Assert dummy item is now unheld and has failed due to NOOP scraper mock
	{
		all := srv.GetAllIngests()
		assert.Len(t, all, 1)
		i := all[0]
		assert.Equal(t, ingest.Troubled, i.State)
		assert.Equal(t, ingest.MetadataFailure, i.Trouble.Type())
		assert.Equal(t, "TESTING NOOP", i.Trouble.Error())
	}
}
