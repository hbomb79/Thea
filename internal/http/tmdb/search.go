package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/hbomb79/Thea/internal/media"
)

const (
	tmdbBaseUrl = "https://api.themoviedb.org/3"

	tmdbSearchMovieTemplate  = "%s/search/movie?query=%s&api_key=%s"
	tmdbSearchSeriesTemplate = "%s/search/tv?query=%s&api_key=%s"

	tmdbGetMovieTemplate   = "%s/movie/%s?api_key=%s"
	tmdbGetSeriesTemplate  = "%s/tv/%s?api_key=%s"
	tmdbGetSeasonTemplate  = "%s/tv/%s/season/%d?api_key=%s"
	tmdbGetEpisodeTemplate = "%s/tv/%s/season/%d/episode/%d?api_key=%s"
)

type (
	Date   struct{ time.Time }
	Config struct {
		ApiKey string
	}

	SearchResult struct {
		Results      []SearchResultItem
		TotalPages   int `json:"total_pages"`
		TotalResults int `json:"total_results"`
	}

	SearchResultItem struct {
		Id           json.Number `json:"id"`
		Adult        bool        `json:"adult"`
		Title        string      `json:"name"`
		Plot         string      `json:"overview"`
		PosterPath   string      `json:"poster_path"`
		FirstAirDate *Date       `json:"first_air_date"`
		ReleaseDate  *Date       `json:"release_date"`
	}

	Movie struct {
		Id          json.Number `json:"id"`
		Adult       bool        `json:"adult"`
		ReleaseDate string      `json:"release_date"`
		Name        string      `json:"title"`
		Tagline     string      `json:"tagline"`
		Overview    string      `json:"overview"`
	}

	Episode struct {
		Id       json.Number `json:"id"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
	}

	Season struct {
		Id       json.Number `json:"id"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
	}

	Series struct {
		Id       json.Number `json:"id"`
		Adult    bool        `json:"adult"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
	}

	// tmdbSearcher is the primary search method for the Ingest and
	// Download service to find content on the TMDB API.
	// See https://developer.themoviedb.org/reference/intro/getting-started for
	// information on the TMDB API.
	tmdbSearcher struct {
		config Config
	}
)

func NewSearcher(config Config) *tmdbSearcher {
	return &tmdbSearcher{config}
}

// SearchForEpisode will search the TMDB API for a match using the
// provided file media metadata. An error will be raised if:
//   - A query to TMDB fails
//   - A search returns zero results
//   - A search returns multiple results
func (searcher *tmdbSearcher) SearchForSeries(metadata *media.FileMediaMetadata) (*Series, error) {
	season := metadata.SeasonNumber
	episode := metadata.EpisodeNumber
	if !metadata.Episodic {
		return nil, &IllegalRequestError{"metadata provided claims media is not-episodic, but request is searching for an episode"}
	} else if season == -1 || episode == -1 {
		return nil, &IllegalRequestError{"metadata provided fails to supply valid season/episode information for an episodic media file"}
	}

	// Search for the series
	path := fmt.Sprintf(tmdbSearchSeriesTemplate, tmdbBaseUrl, url.QueryEscape(metadata.Title), searcher.config.ApiKey)
	var searchResult SearchResult
	if err := httpGetJsonResponse(path, &searchResult); err != nil {
		return nil, err
	}

	if result, err := searcher.handleSearchResults(searchResult.Results, metadata); err == nil {
		return &Series{Id: result.Id}, nil
	} else {
		return nil, err
	}
}

// SearchForMovie will search the TMDB API for a match using the
// provided file media metadata. An error will be raised if:
//   - A query to TMDB fails
//   - A search returns zero results
//   - A search returns multiple results and the searcher cannot decide which is correct
func (searcher *tmdbSearcher) SearchForMovie(metadata *media.FileMediaMetadata) (*Movie, error) {
	if metadata.Episodic {
		return nil, &IllegalRequestError{"metadata provided claims media is episodic, but request is searching for a movie"}
	}

	// Search for the movie stub
	path := fmt.Sprintf(tmdbSearchMovieTemplate, tmdbBaseUrl, url.QueryEscape(metadata.Title), searcher.config.ApiKey)
	var searchResult SearchResult
	if err := httpGetJsonResponse(path, &searchResult); err != nil {
		return nil, err
	}

	if result, err := searcher.handleSearchResults(searchResult.Results, metadata); err == nil {
		return &Movie{Id: result.Id}, nil
	} else {
		return nil, err
	}
}

// GetMovie will query the TMDB API for the movie with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetMovie(movieId string) (*Movie, error) {
	path := fmt.Sprintf(tmdbGetMovieTemplate, tmdbBaseUrl, movieId, searcher.config.ApiKey)
	var movie Movie
	if err := httpGetJsonResponse(path, &movie); err != nil {
		return nil, err
	}

	return &movie, nil
}

// GetSeries will query TMDB API for the series with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeries(seriesId string) (*Series, error) {
	path := fmt.Sprintf(tmdbGetSeriesTemplate, tmdbBaseUrl, seriesId, searcher.config.ApiKey)
	var series Series
	if err := httpGetJsonResponse(path, &series); err != nil {
		return nil, err
	}

	return &series, nil
}

// GetEpisode queries TMDB using the seriesID combined with the season and episode number. It is expected
// that the seriesID provided is a valid TMDB ID, else the request will fail.
func (searcher *tmdbSearcher) GetEpisode(seriesId string, seasonNumber int, episodeNumber int) (*Episode, error) {
	path := fmt.Sprintf(tmdbGetEpisodeTemplate, tmdbBaseUrl, seriesId, seasonNumber, episodeNumber, searcher.config.ApiKey)
	var episode Episode
	if err := httpGetJsonResponse(path, &episode); err != nil {
		return nil, err
	}

	return &episode, nil
}

// GetSeason will query TMDB API for the season with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeason(seriesId string, seasonNumber int) (*Season, error) {
	path := fmt.Sprintf(tmdbGetSeasonTemplate, tmdbBaseUrl, seriesId, seasonNumber, searcher.config.ApiKey)
	var season Season
	if err := httpGetJsonResponse(path, &season); err != nil {
		return nil, err
	}

	return &season, nil
}

// PruneSearchResults accepts a list of search stubs from TMDB and attempts
// to whittle them down to a singular result. To do so, the year and popularity
// of the results is taken in to consideration
func (searcher *tmdbSearcher) handleSearchResults(results []SearchResultItem, metadata *media.FileMediaMetadata) (*SearchResultItem, error) {
	if metadata.Year != nil {
		if metadata.Episodic {
			filterResultsInPlace(&results, metadata, func(resultDate time.Time, metadataDate time.Time) bool {
				return resultDate.Compare(metadataDate) >= 0
			})
		} else {
			filterResultsInPlace(&results, metadata, func(resultDate time.Time, metadataDate time.Time) bool {
				return resultDate.Compare(metadataDate) == 0
			})
		}
	}

	if len(results) == 1 {
		return &results[0], nil
	} else if len(results) == 0 {
		return nil, &NoResultError{}
	}

	metric := &metrics.Hamming{CaseSensitive: false}
	stringSimilarity := make([]float64, len(results))
	for i, res := range results {
		stringSimilarity[i] = strutil.Similarity(res.Title, metadata.Title, metric)
	}

	sort.SliceStable(results, func(i, j int) bool { return stringSimilarity[i] < stringSimilarity[j] })
	if stringSimilarity[0] > stringSimilarity[1]+0.25 {
		return &results[0], nil
	}

	return nil, &MultipleResultError{results}
}

func (entry *SearchResultItem) effectiveDate() *Date {
	if entry.FirstAirDate != nil {
		return entry.FirstAirDate
	}

	return entry.ReleaseDate
}

func (date *Date) UnmarshalJSON(dateBytes []byte) error {
	trimmedDateString := string(dateBytes[1 : len(dateBytes)-1])
	parsed, err := time.Parse(time.DateOnly, trimmedDateString)
	if err != nil {
		return fmt.Errorf("cannot unmarshal Date due to error: %s", err.Error())
	}

	*date = Date{parsed}
	return nil
}

// filterResultsInPlace will filter the given array of results IN PLACE by modifying
// the provided slice and returning
func filterResultsInPlace(results *[]SearchResultItem, metadata *media.FileMediaMetadata, filterFn func(dateFromResult time.Time, dateFromMetadata time.Time) bool) {
	timeFromYear := func(year int) time.Time {
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	yearFromMetadata := timeFromYear(*metadata.Year)
	insertionIndex := 0
	for _, v := range *results {
		yearFromResult := timeFromYear(v.effectiveDate().Year())
		if filterFn(yearFromResult, yearFromMetadata) {
			(*results)[insertionIndex] = v
			insertionIndex++
		}
	}

	*results = (*results)[:insertionIndex]
}

func httpGetJsonResponse(urlPath string, targetInterface interface{}) error {
	resp, err := http.Get(urlPath)
	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to perform GET(%s) to TMDB: %s", urlPath, err.Error())}
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var tmdbError tmdbError
		if err := json.Unmarshal(respBody, &tmdbError); err != nil {
			return &FailedRequestError{httpCode: resp.StatusCode, message: "non-OK response could not be unmarshalled", tmdbCode: -1}
		}

		return &FailedRequestError{httpCode: resp.StatusCode, message: tmdbError.StatusMessage, tmdbCode: tmdbError.StatusCode}
	}

	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to read response body: %s", err.Error())}
	}

	if err := json.Unmarshal(respBody, targetInterface); err != nil {
		return &UnknownRequestError{fmt.Sprintf("response JSON could not be unmarshalled: %s", err.Error())}
	}

	return nil
}

type (
	tmdbError struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
	}
	FailedRequestError struct {
		httpCode int
		tmdbCode int
		message  string
	}
	NoResultError       struct{}
	MultipleResultError struct{ results []SearchResultItem }
	UnknownRequestError struct{ reason string }
	IllegalRequestError struct{ reason string }
)

func (err *UnknownRequestError) Error() string {
	return fmt.Sprintf("unknown error occurred while communicating with TMDB: %s", err.reason)
}
func (err *IllegalRequestError) Error() string {
	return fmt.Sprintf("illegal search request because %s", err.reason)
}
func (err *FailedRequestError) Error() string {
	return fmt.Sprintf("Request failure (HTTP %d): %s", err.httpCode, err.message)
}
func (err *NoResultError) Error() string       { return "no results returned from TMDB" }
func (err *MultipleResultError) Error() string { return "too many results returned from TMDB" }
