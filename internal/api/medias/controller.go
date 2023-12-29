package medias

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	watchTargetType int

	// watchTargetDto represents a way in which a particular type
	// of media could be watched:
	// - If a particular target has a pre-transcode COMPLETE for the given media,
	//   then a target to that effect will be present (with ready = true),
	// - If a particular target has a pre-transcode IN PROGRESS for the given media,
	//   then a target with ready=false will be present,
	// - If no pre-transcode for the given target and media is found, AND the target is
	//   eligible for on-the-fly transcode, then a target of type 'on-the-fly' will
	//   be present (with ready = true).
	// - ADDITIONALLY, a watch target with NO target_id will be available of type
	//   on-the-fly and ready=true will be present, which represents being able to stream
	//   from the source media directly.
	watchTargetDto struct {
		Name     string          `json:"display_name"`
		TargetID *uuid.UUID      `json:"target_id,omitempty"`
		Enabled  bool            `json:"enabled"`
		Type     watchTargetType `json:"type"`
		Ready    bool            `json:"ready"`
		// TODO: may want to include some additional information about the
		// target here, such as bitrate and resolution.
	}

	seriesStubDto struct {
		Id          uuid.UUID `json:"id"`
		Title       string    `json:"title"`
		SeasonCount int       `json:"season_count"`
	}

	movieStubDto struct {
		Id    uuid.UUID `json:"id"`
		Title string    `json:"title"`
		// TODO: poster path, runtime
	}

	episodeDto struct {
		Id           uuid.UUID         `json:"id"`
		TmdbId       string            `json:"tmdb_id"`
		Title        string            `json:"title"`
		CreatedAt    time.Time         `json:"created_at"`
		UpdatedAt    time.Time         `json:"updated_at"`
		WatchTargets []*watchTargetDto `json:"watch_targets"`
	}

	episodeStubDto struct {
		Id    uuid.UUID `json:"id"`
		Title string    `json:"title"`
		Adult bool      `json:"adult"`
	}

	seasonDto struct {
		Episodes []*episodeStubDto `json:"episodes"`
	}

	seriesDto struct {
		Id      uuid.UUID    `json:"id"`
		TmdbId  string       `json:"tmdb_id"`
		Title   string       `json:"title"`
		Seasons []*seasonDto `json:"seasons"`
	}

	// movieDto is a fully inflated version of the more common movieStubDto, which encodes more
	// information such as the watch targets which are eligible for the media
	movieDto struct {
		Id           uuid.UUID         `json:"id"`
		TmdbId       string            `json:"tmdb_id"`
		Title        string            `json:"title"`
		CreatedAt    time.Time         `json:"created_at"`
		UpdatedAt    time.Time         `json:"updated_at"`
		WatchTargets []*watchTargetDto `json:"watch_targets"`
	}

	Store interface {
		ListMovie() ([]*media.Movie, error)
		GetMovie(movieID uuid.UUID) (*media.Movie, error)
		GetEpisode(episodeID uuid.UUID) (*media.Episode, error)
		ListSeriesStubs() ([]*media.SeriesStub, error)
		GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error)
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.Transcode, error)
		GetAllTargets() []*ffmpeg.Target
	}

	TranscodeService interface {
		ActiveTasksForMedia(uuid.UUID) []*transcode.TranscodeTask
	}

	Controller struct {
		store            Store
		transcodeService TranscodeService
	}
)

const (
	PreTranscoded watchTargetType = iota
	LiveTranscode
)

func New(validate *validator.Validate, service TranscodeService, store Store) *Controller {
	return &Controller{store: store, transcodeService: service}
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

// listLatest ...
func (controller *Controller) listLatest(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
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
		dtos[k] = MovieModelToDto(v)
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
		dtos[k] = InflatedSeriesModelToDto(v)
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

	watchTargets, err := controller.constructWatchTargetsForMedia(movieId)
	if err != nil {
		return wrap(err)
	}

	dto := movieDto{
		Id:           movie.ID,
		TmdbId:       movie.TmdbId,
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

	watchTargets, err := controller.constructWatchTargetsForMedia(episodeID)
	if err != nil {
		return wrap(err)
	}

	dto := episodeDto{
		Id:           episode.ID,
		TmdbId:       episode.TmdbId,
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
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteMovie(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteSeries(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteSeason(ec echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) constructWatchTargetsForMedia(mediaID uuid.UUID) ([]*watchTargetDto, error) {
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

func InflatedSeriesModelToDto(model *media.SeriesStub) seriesStubDto {
	return seriesStubDto{
		Id:          model.ID,
		Title:       model.Title,
		SeasonCount: model.SeasonCount,
	}
}

func MovieModelToDto(model *media.Movie) movieStubDto {
	return movieStubDto{
		Id:    model.ID,
		Title: model.Title,
	}
}

func newWatchTarget(target *ffmpeg.Target, t watchTargetType, ready bool) *watchTargetDto {
	return &watchTargetDto{
		Name:     target.Label,
		Ready:    ready,
		Type:     t,
		TargetID: &target.ID, // TODO: this needs to actually come from the target
		Enabled:  true,
	}
}

func wrapErrorGenerator(message string) func(err error) error {
	return func(err error) error {
		if errors.Is(err, media.ErrNoRowFound) {
			return echo.ErrNotFound
		}
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to fetch episode: %v", err))
	}
}
