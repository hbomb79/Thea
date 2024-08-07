package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
)

const (
	tmdbBaseURL = "https://api.themoviedb.org/3"

	tmdbSearchMovieTemplate  = "%s/search/movie?query=%s&api_key=%s"
	tmdbSearchSeriesTemplate = "%s/search/tv?query=%s&api_key=%s"

	tmdbGetMovieTemplate   = "%s/movie/%s?api_key=%s"
	tmdbGetSeriesTemplate  = "%s/tv/%s?api_key=%s"
	tmdbGetSeasonTemplate  = "%s/tv/%s/season/%d?api_key=%s"
	tmdbGetEpisodeTemplate = "%s/tv/%s/season/%d/episode/%d?api_key=%s"
)

var log = logger.Get("TMDB")

type (
	Date   struct{ time.Time }
	Config struct {
		APIKey string
	}

	Genre struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	}

	SearchResult struct {
		Results      []SearchResultItem
		TotalPages   int `json:"total_pages"`
		TotalResults int `json:"total_results"`
	}

	SearchResultItem struct {
		ID           json.Number `json:"id"`
		Adult        bool        `json:"adult"`
		Title        string      `json:"name"`
		Plot         string      `json:"overview"`
		PosterPath   string      `json:"poster_path"`
		FirstAirDate *Date       `json:"first_air_date"`
		ReleaseDate  *Date       `json:"release_date"`
	}

	Movie struct {
		ID          json.Number `json:"id"`
		Adult       bool        `json:"adult"`
		ReleaseDate string      `json:"release_date"`
		Name        string      `json:"title"`
		Tagline     string      `json:"tagline"`
		Overview    string      `json:"overview"`
		Genres      []Genre     `json:"genres"`
	}

	Episode struct {
		ID       json.Number `json:"id"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
	}

	Season struct {
		ID       json.Number `json:"id"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
	}

	Series struct {
		ID       json.Number `json:"id"`
		Adult    bool        `json:"adult"`
		Name     string      `json:"name"`
		Overview string      `json:"overview"`
		Genres   []Genre     `json:"genres"`
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
// provided file media metadata, returning it's ID on success.
// An error will be raised if:
//   - A query to TMDB fails
//   - A search returns zero results
//   - A search returns multiple results
func (searcher *tmdbSearcher) SearchForSeries(metadata *media.FileMediaMetadata) (string, error) {
	season := metadata.SeasonNumber
	episode := metadata.EpisodeNumber
	if !metadata.Episodic {
		return "", &IllegalRequestError{"metadata provided claims media is not-episodic, but request is searching for an episode"}
	} else if season == -1 || episode == -1 {
		return "", &IllegalRequestError{"metadata provided fails to supply valid season/episode information for an episodic media file"}
	}

	// Search for the series
	path := fmt.Sprintf(tmdbSearchSeriesTemplate, tmdbBaseURL, url.QueryEscape(metadata.Title), searcher.config.APIKey)
	var searchResult SearchResult
	if err := httpGetJSONResponse(path, &searchResult); err != nil {
		return "", err
	}

	if result, err := searcher.handleSearchResults(searchResult.Results, metadata); err == nil {
		return result.ID.String(), nil
	} else {
		return "", err
	}
}

// SearchForMovie will search the TMDB API for a match using the
// provided file media metadata, returning it's ID on success.
// An error will be raised if:
//   - A query to TMDB fails
//   - A search returns zero results
//   - A search returns multiple results and the searcher cannot decide which is correct
func (searcher *tmdbSearcher) SearchForMovie(metadata *media.FileMediaMetadata) (string, error) {
	if metadata.Episodic {
		return "", &IllegalRequestError{"metadata provided claims media is episodic, but request is searching for a movie"}
	}

	// Search for the movie stub
	path := fmt.Sprintf(tmdbSearchMovieTemplate, tmdbBaseURL, url.QueryEscape(metadata.Title), searcher.config.APIKey)
	var searchResult SearchResult
	if err := httpGetJSONResponse(path, &searchResult); err != nil {
		return "", err
	}

	if result, err := searcher.handleSearchResults(searchResult.Results, metadata); err == nil {
		return result.ID.String(), nil
	} else {
		return "", err
	}
}

// GetMovie will query the TMDB API for the movie with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetMovie(movieID string) (*Movie, error) {
	path := fmt.Sprintf(tmdbGetMovieTemplate, tmdbBaseURL, movieID, searcher.config.APIKey)
	var movie Movie
	if err := httpGetJSONResponse(path, &movie); err != nil {
		return nil, err
	}

	return &movie, nil
}

// GetSeries will query TMDB API for the series with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeries(seriesID string) (*Series, error) {
	path := fmt.Sprintf(tmdbGetSeriesTemplate, tmdbBaseURL, seriesID, searcher.config.APIKey)
	var series Series
	if err := httpGetJSONResponse(path, &series); err != nil {
		return nil, err
	}

	return &series, nil
}

// GetEpisode queries TMDB using the seriesID combined with the season and episode number. It is expected
// that the seriesID provided is a valid TMDB ID, else the request will fail.
func (searcher *tmdbSearcher) GetEpisode(seriesID string, seasonNumber int, episodeNumber int) (*Episode, error) {
	path := fmt.Sprintf(tmdbGetEpisodeTemplate, tmdbBaseURL, seriesID, seasonNumber, episodeNumber, searcher.config.APIKey)
	var episode Episode
	if err := httpGetJSONResponse(path, &episode); err != nil {
		return nil, err
	}

	return &episode, nil
}

// GetSeason will query TMDB API for the season with the provided string ID. This ID
// must be a valid TMDB ID, or else an error will be returned.
func (searcher *tmdbSearcher) GetSeason(seriesID string, seasonNumber int) (*Season, error) {
	path := fmt.Sprintf(tmdbGetSeasonTemplate, tmdbBaseURL, seriesID, seasonNumber, searcher.config.APIKey)
	var season Season
	if err := httpGetJSONResponse(path, &season); err != nil {
		return nil, err
	}

	return &season, nil
}

// PruneSearchResults accepts a list of search stubs from TMDB and attempts
// to whittle them down to a singular result. To do so, the year and popularity
// of the results is taken in to consideration.
func (searcher *tmdbSearcher) handleSearchResults(results []SearchResultItem, metadata *media.FileMediaMetadata) (*SearchResultItem, error) {
	if metadata.Year != 0 {
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
	similarityThreshold := 0.25
	if stringSimilarity[0] > stringSimilarity[1]+similarityThreshold {
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
		return fmt.Errorf("cannot unmarshal Date: %w", err)
	}

	*date = Date{parsed}
	return nil
}

// filterResultsInPlace will filter the given array of results IN PLACE by modifying
// the provided slice and returning.
func filterResultsInPlace(results *[]SearchResultItem, metadata *media.FileMediaMetadata, filterFn func(dateFromResult time.Time, dateFromMetadata time.Time) bool) {
	timeFromYear := func(year int) time.Time {
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	yearFromMetadata := timeFromYear(metadata.Year)
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

func httpGetJSONResponse(urlPath string, targetInterface interface{}) error {
	log.Verbosef("GET -> %s\n", urlPath)
	resp, err := http.Get(urlPath) //nolint
	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to perform GET(%s) to TMDB: %v", urlPath, err)}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var tmdbError tmdbError
		if err := json.NewDecoder(resp.Body).Decode(&tmdbError); err != nil {
			return &FailedRequestError{httpCode: resp.StatusCode, message: "non-OK response could not be unmarshalled", tmdbCode: -1}
		}

		return &FailedRequestError{httpCode: resp.StatusCode, message: tmdbError.StatusMessage, tmdbCode: tmdbError.StatusCode}
	}

	if err != nil {
		return &UnknownRequestError{fmt.Sprintf("failed to read response body: %v", err)}
	}

	if err := json.NewDecoder(resp.Body).Decode(targetInterface); err != nil {
		return &UnknownRequestError{fmt.Sprintf("response JSON could not be unmarshalled: %v", err)}
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

func (err UnknownRequestError) Error() string {
	return fmt.Sprintf("unknown error occurred while communicating with TMDB: %s", err.reason)
}

func (err IllegalRequestError) Error() string {
	return fmt.Sprintf("illegal search request because %s", err.reason)
}

func (err FailedRequestError) Error() string {
	return fmt.Sprintf("Request failure (HTTP %d): %s", err.httpCode, err.message)
}
func (err NoResultError) Error() string                      { return "no results returned from TMDB" }
func (err MultipleResultError) Error() string                { return "too many results returned from TMDB" }
func (err MultipleResultError) Choices() *[]SearchResultItem { return &err.results }
