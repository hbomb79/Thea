package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hbomb79/Thea/internal/media"
)

const (
	tmdbBaseUrl = "https://api.themoviedb.org/3"

	tmdbSearchMovieTemplate  = "%s/search/movie?query=%s&apiKey=%s"
	tmdbSearchSeriesTemplate = "%s/search/series?query=%s&apiKey=%s"

	tmdbGetMovieTemplate = "%s/movie/%s?apiKey=%s"

	tmdbGetSeriesTemplate  = "%s/tv/%s?apiKey=%s"
	tmdbGetSeasonTemplate  = "%s/tv/%s/season/%d?apiKey=%s"
	tmdbGetEpisodeTemplate = "%s/tv/%s/season/%d/episode/%d?apiKey=%s"
)

type Config struct {
	apiKey string
}

type TmdbSearchResult struct {
	Results      []*TmdbSearchResultEntry
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

type TmdbSearchResultEntry struct {
	Id         string `json:"id"`
	Adult      bool   `json:"adult"`
	Title      string `json:"name"`
	Plot       string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

func (entry *TmdbSearchResultEntry) toMediaStub() *media.SearchStub {
	return &media.SearchStub{
		Type:       media.EPISODE,
		PosterPath: entry.PosterPath,
		Title:      entry.Title,
		SourceID:   entry.Id,
	}
}

type TmdbMovie struct{}

func (movie *TmdbMovie) toMediaMovie() *media.Movie { return nil }

type TmdbEpisode struct{}

func (ep *TmdbEpisode) toMediaEpisode() *media.Episode { return nil }

type TmdbSeason struct{}

func (season *TmdbSeason) toMediaSeason() *media.Season { return nil }

type TmdbSeries struct{}

func (series *TmdbSeries) toMediaSeries() *media.Series { return nil }

// tmdbSearcher is the primary search method for the Ingest and
// Download service to find content on the TMDB API.
// See https://developer.themoviedb.org/reference/intro/getting-started for
// information on the TMDB API.
type tmdbSearcher struct {
	config Config
}

func NewSearcher(config Config) *tmdbSearcher {
	return &tmdbSearcher{config}
}

// SearchForEpisode will search the TMDB API for a match using the
// provided file media metadata. An error will be raised if:
// A query to TMDB fails
// A search returns zero results
// A search returns multiple results and the searcher cannot decide which is correct
//
// TMDB episode information can only be gathered by first finding the 'show'/series, and then
// querying specifically for the episode using the season/episode number.
func (searcher *tmdbSearcher) SearchForEpisode(metadata *media.FileMediaMetadata) (*media.Episode, error) {
	season := metadata.SeasonNumber
	episode := metadata.EpisodeNumber
	if !metadata.Episodic {
		return nil, &IllegalRequestError{"metadata provided claims media is not-episodic, but request is searching for an episode"}
	} else if season == -1 || episode == -1 {
		return nil, &IllegalRequestError{"metadata provided fails to supply valid season/episode information for an episodic media file"}
	}

	// Search for the series
	path := fmt.Sprintf(tmdbSearchSeriesTemplate, tmdbBaseUrl, metadata.Title, searcher.config.apiKey)
	var searchResult TmdbSearchResult
	if err := httpGetJsonResponse(path, &searchResult); err != nil {
		return nil, err
	}

	if searchResult.TotalResults == 0 {
		return nil, &NoResultError{}
	} else if searchResult.TotalResults > 1 {
		stubs := make([]*media.SearchStub, len(searchResult.Results))
		for i, r := range searchResult.Results {
			stubs[i] = r.toMediaStub()
		}
		return nil, &MultipleResultError{&stubs}
	}

	// Get the episode
	series := searchResult.Results[0]
	return searcher.GetEpisode(series.Id, metadata.SeasonNumber, metadata.EpisodeNumber)
}

// SearchForMovie will search the TMDB API for a match using the
// provided file media metadata. An error will be raised if:
// A query to TMDB fails
// A search returns zero results
// A search returns multiple results and the searcher cannot decide which is correct
func (searcher *tmdbSearcher) SearchForMovie(metadata *media.FileMediaMetadata) (*media.Movie, error) {
	if metadata.Episodic {
		return nil, &IllegalRequestError{"metadata provided claims media is episodic, but request is searching for a movie"}
	}

	// Search for the movie stub
	path := fmt.Sprintf(tmdbSearchMovieTemplate, tmdbBaseUrl, metadata.Title, searcher.config.apiKey)
	var searchResult TmdbSearchResult
	if err := httpGetJsonResponse(path, &searchResult); err != nil {
		return nil, err
	}

	if searchResult.TotalResults == 0 {
		return nil, &NoResultError{}
	} else if searchResult.TotalResults > 1 {
		stubs := make([]*media.SearchStub, len(searchResult.Results))
		for i, r := range searchResult.Results {
			stubs[i] = r.toMediaStub()
		}
		return nil, &MultipleResultError{&stubs}
	}

	// Get the movie detaila
	movie := searchResult.Results[0]
	return searcher.GetMovie(movie.Id)

}

func (searcher *tmdbSearcher) GetMovie(movieId string) (*media.Movie, error) {
	path := fmt.Sprintf(tmdbGetMovieTemplate, tmdbBaseUrl, movieId, searcher.config.apiKey)
	var movie TmdbMovie
	if err := httpGetJsonResponse(path, &movie); err != nil {
		return nil, err
	}

	return movie.toMediaMovie(), nil
}

// GetSeries will query TMDB API for the series with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeries(seriesId string) (*media.Series, error) {
	path := fmt.Sprintf(tmdbGetSeriesTemplate, tmdbBaseUrl, seriesId, searcher.config.apiKey)
	var series TmdbSeries
	if err := httpGetJsonResponse(path, &series); err != nil {
		return nil, err
	}

	return series.toMediaSeries(), nil
}

// GetEpisode queries TMDB using the seriesID combined with the season and episode number. It is expected
// that the seriesID provided is a valid TMDB ID, else the request will fail.
func (searcher *tmdbSearcher) GetEpisode(seriesId string, seasonNumber int, episodeNumber int) (*media.Episode, error) {
	path := fmt.Sprintf(tmdbGetEpisodeTemplate, tmdbBaseUrl, seriesId, seasonNumber, episodeNumber, searcher.config.apiKey)
	var episode TmdbEpisode
	if err := httpGetJsonResponse(path, &episode); err != nil {
		return nil, err
	}

	return episode.toMediaEpisode(), nil
}

// GetSeason will query TMDB API for the season with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeason(seriesId string, seasonNumber int) (*media.Season, error) {
	path := fmt.Sprintf(tmdbGetSeasonTemplate, tmdbBaseUrl, seriesId, seasonNumber, searcher.config.apiKey)
	var season TmdbSeason
	if err := httpGetJsonResponse(path, &season); err != nil {
		return nil, err
	}

	return season.toMediaSeason(), nil
}

// NoResultError is used when a TMDB search has returned no results.
type NoResultError struct{}

func (err *NoResultError) Error() string {
	return "no results returned from TMDB"
}

// MutlipleResultError is returned when a search command has returned multiple
// results. The results are contained within the error so the user
// can use the IDs embedded in the search stubs to retrieve their desired result.
type MultipleResultError struct{ results *[]*media.SearchStub }

func (err *MultipleResultError) Error() string {
	return "too many results returned from TMDB"
}

// UnknownRequestError is to represent an unexpected error that has occurred
// when communicating with TMDB
type UnknownRequestError struct{ reason string }

func (err *UnknownRequestError) Error() string {
	return fmt.Sprintf("unknown error occurred while communicating with TMDB: %s", err.reason)
}

// IllegalRequestError is used when a request is provided with file metadata that
// is conflicting with the request (e.g., a 'SearchForEpisode' called with metadata
// belonging to a movie).
type IllegalRequestError struct{ reason string }

func (err *IllegalRequestError) Error() string {
	return fmt.Sprintf("illegal search request because %s", err.reason)
}

func httpGetJsonResponse(urlPath string, targetInterface interface{}) error {
	resp, err := http.Get(urlPath)
	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to perform GET(%s) to TMDB: %s", urlPath, err.Error())}
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to read response body: %s", err.Error())}
	}

	if err := json.Unmarshal(respBody, targetInterface); err != nil {
		return &UnknownRequestError{fmt.Sprintf("response JSON could not be unmarshalled: %s", err.Error())}
	}

	return nil
}
