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

// DTOs
type (
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
	watchTargetType int
	watchTargetDto  struct {
		Name     string     `json:"display_name"`
		TargetID *uuid.UUID `json:"target_id,omitempty"`
		Enabled  bool       `json:"enabled"`
		Type     string     `json:"type"`
		Ready    bool       `json:"ready"`
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
)

const (
	PreTranscoded watchTargetType = iota
	LiveTranscode
)

type (
	Store interface {
		ListMovie() ([]*media.Movie, error)
		GetMovie(movieID uuid.UUID) (*media.Movie, error)
		GetEpisode(episodeID uuid.UUID) (*media.Episode, error)
		DeleteEpisode(episodeID uuid.UUID) error
		DeleteMovie(movieID uuid.UUID) error
		ListSeriesStubs() ([]*media.SeriesStub, error)
		GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error)
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.Transcode, error)
		GetAllTargets() []*ffmpeg.Target
	}

	Data interface {
		GetActiveTranscodeTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
		CancelTranscodeTasksForMedia(mediaID uuid.UUID)
		GetMediaWatchTargets(mediaID uuid.UUID) ([]*media.WatchTarget, error)
	}

	Controller struct {
		store Store
		data  Data
	}
)

func New(validate *validator.Validate, dataManager Data, store Store) *Controller {
	return &Controller{store: store, data: dataManager}
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

	watchTargets, err := controller.data.GetMediaWatchTargets(movieId)
	if err != nil {
		return wrap(err)
	}

	dto := movieDto{
		Id:           movie.ID,
		TmdbId:       movie.TmdbId,
		Title:        movie.Title,
		CreatedAt:    movie.CreatedAt,
		UpdatedAt:    movie.UpdatedAt,
		WatchTargets: newWatchTargetDtos(watchTargets),
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

	watchTargets, err := controller.data.GetMediaWatchTargets(episodeID)
	if err != nil {
		return wrap(err)
	}

	dto := episodeDto{
		Id:           episode.ID,
		TmdbId:       episode.TmdbId,
		Title:        episode.Title,
		CreatedAt:    episode.CreatedAt,
		UpdatedAt:    episode.UpdatedAt,
		WatchTargets: newWatchTargetDtos(watchTargets),
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

	// Delete the episode first, and *then* cancel the tasks so that if episode deletion fails we
	// won't have cancelled the tasks (as this is non-reversable).
	// As the episode will be deleted from the DB, the inability to satisfy the foreign key
	// constraint will prevent any *perfectly* time transcodes from inserting new transcode rows
	// between the episode deletion and the cancellation of the tasks.
	if err := controller.store.DeleteEpisode(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	controller.data.CancelTranscodeTasksForMedia(id)

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) deleteMovie(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Movie ID is not a valid UUID")
	}

	// Delete the movie first, and *then* cancel the tasks so that if movie deletion fails we
	// won't have cancelled the tasks (as this is non-reversable).
	// As the movie will be deleted from the DB, the inability to satisfy the foreign key
	// constraint will prevent any *perfectly* time transcodes from inserting new transcode rows
	// between the movie deletion and the cancellation of the tasks.
	if err := controller.store.DeleteMovie(id); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	controller.data.CancelTranscodeTasksForMedia(id)

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) deleteSeries(ec echo.Context) error {
	// TODO:
	// - Find all episodes nested inside this series' seasons
	// - Delete the series (which will delete all seasons and episodes due to FK cascading)
	// - Cancel all the tasks associated with the episodes above if the deletion was successful
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) deleteSeason(ec echo.Context) error {
	// TODO:
	// - Find all episodes nested inside this series' seasons
	// - Delete the series (which will delete all seasons and episodes due to FK cascading)
	// - Cancel all the tasks associated with the episodes above if the deletion was successful
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func inflatedSeriesModelToDto(model *media.SeriesStub) seriesStubDto {
	return seriesStubDto{
		Id:          model.ID,
		Title:       model.Title,
		SeasonCount: model.SeasonCount,
	}
}

func movieModelToDto(model *media.Movie) movieStubDto {
	return movieStubDto{
		Id:    model.ID,
		Title: model.Title,
	}
}

func newWatchTargetDtos(watchTargets []*media.WatchTarget) []*watchTargetDto {
	dtos := make([]*watchTargetDto, len(watchTargets))
	for k, v := range watchTargets {
		dtos[k] = newWatchTargetDto(v)
	}

	return dtos
}

func newWatchTargetDto(watchTarget *media.WatchTarget) *watchTargetDto {
	var t string = "pre_transcode"
	if watchTarget.Type == media.LiveTranscode {
		t = "live_transcode"
	}

	return &watchTargetDto{Name: watchTarget.Name, Ready: watchTarget.Ready, Type: t, TargetID: watchTarget.TargetID, Enabled: watchTarget.Enabled}
}

func wrapErrorGenerator(message string) func(err error) error {
	return func(err error) error {
		if errors.Is(err, media.ErrNoRowFound) {
			return echo.ErrNotFound
		}
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to fetch episode: %v", err))
	}
}
