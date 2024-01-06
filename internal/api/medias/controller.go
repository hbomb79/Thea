package medias

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		GetMovie(movieID uuid.UUID) (*media.Movie, error)
		GetEpisode(episodeID uuid.UUID) (*media.Episode, error)
		GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error)
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.Transcode, error)
		GetAllTargets() []*ffmpeg.Target

		ListLatestMedia(allowedTypes []string, limit int) ([]*media.Container, error)
		ListMovie() ([]*media.Movie, error)
		ListSeriesStubs() ([]*media.SeriesStub, error)

		DeleteEpisode(episodeID uuid.UUID) error
		DeleteSeries(seriesID uuid.UUID) error
		DeleteSeason(seasonID uuid.UUID) error
		DeleteMovie(movieID uuid.UUID) error
	}

	TranscodeService interface {
		ActiveTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
	}

	Controller struct {
		store            Store
		transcodeService TranscodeService
	}
)

func New(validate *validator.Validate, transcodeService TranscodeService, store Store) *Controller {
	return &Controller{store: store, transcodeService: transcodeService}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/latest/", controller.listLatest)

	eg.GET("/movie/", controller.listMovies)
	eg.GET("/movie/:id/", controller.getMovie)
	eg.DELETE("/movie/:id/", controller.deleteMovie)

	eg.GET("/series/", controller.listSeries)
	eg.GET("/series/:id/", controller.getSeries)

	eg.GET("/episode/:id/", controller.getEpisode)

	eg.DELETE("/series/:id/", controller.deleteSeries)
	eg.DELETE("/season/:id/", controller.deleteSeason)
	eg.DELETE("/episode/:id/", controller.deleteEpisode)
}

// listLatest is an endpoint used to retrieve a list of movies and series which have been
// updated recently (this includes episodes being added to a series). The caller of this endpoint
// can specify filtering options such as the type (movie|series), a limit to the number
// of results, or the genres which apply to the content
//
// TODO: the genre stuff!
func (controller *Controller) listLatest(ec echo.Context) error {
	params := ec.QueryParams()
	allowedTypes, ok := params["allowedType"]
	if !ok {
		allowedTypes = []string{}
	}

	limit, err := strconv.Atoi(params.Get("limit"))
	if err != nil {
		limit = 0
	}

	results, err := controller.store.ListLatestMedia(allowedTypes, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	dtos, err := newListDtos(results)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return ec.JSON(http.StatusOK, dtos)
}

// listMovies returns a list of 'MovieStubDto's, which is an uninflated version
// of 'MovieDto' (which can be obtained via getMovie).
func (controller *Controller) listMovies(ec echo.Context) error {
	movies, err := controller.store.ListMovie()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error occurred while listing movies: %v", err))
	}

	dtos := make([]movieStubDto, len(movies))
	for k, v := range movies {
		dtos[k] = movieModelToDto(v)
	}

	return ec.JSON(http.StatusOK, dtos)
}

// listSeasons returns a list of 'SeasonStubDto's, which is essentially
// an uninflated 'SeasonDto'. A fully inflated season DTO can be obtained
// via 'getSeries', which returns all seasons (and episode stubs) embedded within
func (controller *Controller) listSeries(ec echo.Context) error {
	series, err := controller.store.ListSeriesStubs()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error occurred while listing series: %v", err))
	}

	dtos := make([]seriesStubDto, len(series))
	for k, v := range series {
		dtos[k] = inflatedSeriesModelToDto(v)
	}

	return ec.JSON(http.StatusOK, dtos)
}

func (controller *Controller) getMovie(ec echo.Context) error {
	// TODO: consider pushing all of this down in to a DB transaction
	wrap := wrapErrorGenerator("failed to fetch movie")
	movieId, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return wrap(err)
	}

	movie, err := controller.store.GetMovie(movieId)
	if err != nil {
		return wrap(err)
	}

	watchTargets, err := controller.getMediaWatchTargets(movieId)
	if err != nil {
		return wrap(err)
	}

	dto := movieDto{
		ID:           movie.ID,
		TmdbID:       movie.TmdbID,
		Title:        movie.Title,
		CreatedAt:    movie.CreatedAt,
		UpdatedAt:    movie.UpdatedAt,
		WatchTargets: watchTargets,
	}

	return ec.JSON(http.StatusOK, dto)
}

func (controller *Controller) getEpisode(ec echo.Context) error {
	// TODO: consider pushing all of this down in to a DB transaction
	wrap := wrapErrorGenerator("failed to fetch episode")
	episodeID, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return wrap(err)
	}

	episode, err := controller.store.GetEpisode(episodeID)
	if err != nil {
		return wrap(err)
	}

	watchTargets, err := controller.getMediaWatchTargets(episodeID)
	if err != nil {
		return wrap(err)
	}

	dto := episodeDto{
		ID:           episode.ID,
		TmdbID:       episode.TmdbID,
		Title:        episode.Title,
		CreatedAt:    episode.CreatedAt,
		UpdatedAt:    episode.UpdatedAt,
		WatchTargets: watchTargets,
	}

	return ec.JSON(http.StatusOK, dto)
}

func (controller *Controller) getSeries(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	series, err := controller.store.GetInflatedSeries(id)
	if err != nil {
		return wrapErrorGenerator("Failed to get series")(err)
	}

	return ec.JSON(http.StatusOK, series)
}

func (controller *Controller) deleteEpisode(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Episode ID is not a valid UUID")
	}

	if err := controller.store.DeleteEpisode(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) deleteMovie(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Movie ID is not a valid UUID")
	}

	if err := controller.store.DeleteMovie(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) deleteSeries(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Movie ID is not a valid UUID")
	}

	if err := controller.store.DeleteSeries(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) deleteSeason(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Season ID is not a valid UUID")
	}

	if err := controller.store.DeleteSeason(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) getMediaWatchTargets(mediaID uuid.UUID) ([]*watchTargetDto, error) {
	targets := controller.store.GetAllTargets()
	findTarget := func(tid uuid.UUID) *ffmpeg.Target {
		for _, v := range targets {
			if v.ID == tid {
				return v
			}
		}

		panic("Media references a target which does not exist. This should simply be unreachable unless the DB has lost referential integrity")
	}

	activeTranscodes := controller.transcodeService.ActiveTasksForMedia(mediaID)
	completedTranscodes, err := controller.store.GetTranscodesForMedia(mediaID)
	if err != nil {
		return nil, err
	}

	// 1. Add completed transcodes as valid pre-transcoded targets
	targetsNotEligibleForLiveTranscode := make(map[uuid.UUID]struct{}, len(activeTranscodes))
	watchTargets := make([]*watchTargetDto, len(completedTranscodes))
	for k, v := range completedTranscodes {
		targetsNotEligibleForLiveTranscode[v.TargetID] = struct{}{}
		watchTargets[k] = newWatchTarget(findTarget(v.TargetID), PreTranscoded, true)
	}

	// 2. Add in-progress transcodes (as not ready to watch)
	for _, v := range activeTranscodes {
		targetsNotEligibleForLiveTranscode[v.Target().ID] = struct{}{}
		watchTargets = append(watchTargets, newWatchTarget(v.Target(), PreTranscoded, false))
	}

	// 3. Any targets which do NOT have a complete or in-progress pre-transcode are eligible for live transcoding/streaming
	for _, v := range targets {
		// TODO: check if the specified target allows for live transcoding
		if _, ok := targetsNotEligibleForLiveTranscode[v.ID]; ok {
			continue
		}

		watchTargets = append(watchTargets, newWatchTarget(v, LiveTranscode, true))
	}

	// 4. We can directly stream the source media itself, so add that too
	// TODO: at some point we may want this to be configurable
	watchTargets = append(watchTargets, &watchTargetDto{Name: "Source", Ready: true, Type: LiveTranscode, TargetID: nil, Enabled: true})

	return watchTargets, nil
}

func wrapErrorGenerator(message string) func(err error) error {
	return func(err error) error {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.ErrNotFound
		}
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%s: %v", message, err))
	}
}
