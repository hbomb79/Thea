package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hbomb79/Thea/internal/media"
)

const tmdbBaseUrl = "https://api.themoviedb.org/3"
const tmdbSearchMovieTemplate = "%s/search/movie?query=%s&apiKey=%s"
const tmdbSearchSeriesTemplate = "%s/search/series?query=%s&apiKey=%s"
const tmdbGetSeasonTemplate = "%s/tv/%s/season/%d"
const tmdbGetEpisodeTemplate = "%s/tv/%s/season/%d/episode/%d"

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

type TmdbEpisode struct{}

func (ep *TmdbEpisode) toMediaEpisode() *media.Episode {
	return &media.Episode{}
}

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
	resp, err := http.Get(path)
	if err != nil {
		return nil, &UnknownRequestError{err.Error()}
	} else if resp == nil {
		return nil, &UnknownRequestError{"response is nil"}
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &UnknownRequestError{err.Error()}
	}

	var result TmdbSearchResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &UnknownRequestError{"response JSON could not be unmarhsaled"}
	}

	if result.TotalResults == 0 {
		return nil, &NoResultError{}
	} else if result.TotalResults > 1 {
		stubs := make([]*media.SearchStub, len(result.Results))
		for i, r := range result.Results {
			stubs[i] = r.toMediaStub()
		}
		return nil, &MultipleResultError{&stubs}
	}

	// Get the episode
	series := result.Results[0]
	return searcher.GetEpisode(series.Id, metadata.SeasonNumber, metadata.EpisodeNumber)
}

// SearchForMovie will search the TMDB API for a match using the
// provided file media metadata. An error will be raised if:
// A query to TMDB fails
// A search returns zero results
// A search returns multiple results and the searcher cannot decide which is correct
func (searcher *tmdbSearcher) SearchForMovie(metadata *media.FileMediaMetadata) (*media.Movie, error) {
	return &media.Movie{}, nil
}

func (searcher *tmdbSearcher) GetMovie(movieId string) (*media.Movie, error) {
	return &media.Movie{}, nil
}

// GetSeries will query TMDB API for the series with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeries(seriesId string) (*media.Series, error) {
	return &media.Series{}, nil
}

// GetEpisode queries TMDB using the seriesID combined with the season and episode number. It is expected
// that the seriesID provided is a valid TMDB ID, else the request will fail.
func (searcher *tmdbSearcher) GetEpisode(seriesId string, seasonNumber int, episodeNumber int) (*media.Episode, error) {
	// Search for the series
	resp, err := http.Get(fmt.Sprintf(tmdbGetEpisodeTemplate, tmdbBaseUrl, seriesId, seasonNumber, episodeNumber))
	if err != nil {
		return nil, &UnknownRequestError{err.Error()}
	} else if resp == nil {
		return nil, &UnknownRequestError{"response is nil"}
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &UnknownRequestError{err.Error()}
	}

	var result TmdbEpisode
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &UnknownRequestError{"response JSON could not be unmarhsaled"}
	}

	// Get the episode
	return result.toMediaEpisode(), nil
}

// GetSeason will query TMDB API for the season with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeason(string, int) (*media.Season, error) {
	return &media.Season{}, nil
}

type NoResultError struct{}

func (err *NoResultError) Error() string {
	return "no results returned from TMDB"
}

type MultipleResultError struct{ results *[]*media.SearchStub }

func (err *MultipleResultError) Error() string {
	return "too many results returned from TMDB"
}

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
