package medias

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/labstack/echo/v4"
)

type (
	Dto struct{}

	seriesStubDto struct {
		Id          uuid.UUID `json:"id"`
		TmdbId      string    `json:"tmdb_id"`
		Title       string    `json:"title"`
		SeasonCount int       `json:"season_count"`
	}

	movieStubDto struct {
		Id     uuid.UUID `json:"id"`
		TmdbId string    `json:"tmdb_id"`
		Title  string    `json:"title"`
	}

	Store interface {
		ListMovie() ([]*media.Movie, error)
		ListSeriesStubs() ([]*media.SeriesStub, error)
		GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error)
	}

	Controller struct {
		Store Store
	}
)

func New(validate *validator.Validate, store Store) *Controller {
	return &Controller{Store: store}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/latest/", controller.listLatest)

	eg.GET("/movie/", controller.listMovies)
	eg.GET("/movie/:id/", controller.getMovie)
	eg.DELETE("/movie/:id/", controller.deleteMovie)

	eg.GET("/series/", controller.listSeries)
	eg.GET("/series/:id/", controller.getSeries)

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
	movies, err := controller.Store.ListMovie()
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
	series, err := controller.Store.ListSeriesStubs()
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
	return echo.NewHTTPError(http.StatusNotImplemented, "Not yet implemented")
}

func (controller *Controller) getSeries(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Target ID is not a valid UUID")
	}

	series, err := controller.Store.GetInflatedSeries(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to get series: %s", err))
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

func InflatedSeriesModelToDto(model *media.SeriesStub) seriesStubDto {
	return seriesStubDto{
		Id:          model.ID,
		TmdbId:      model.TmdbId,
		Title:       model.Title,
		SeasonCount: model.SeasonCount,
	}
}

func MovieModelToDto(model *media.Movie) movieStubDto {
	return movieStubDto{
		Id:     model.ID,
		TmdbId: model.TmdbId,
		Title:  model.Title,
	}
}
