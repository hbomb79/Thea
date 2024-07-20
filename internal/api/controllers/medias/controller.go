package medias

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		GetMedia(mediaID uuid.UUID) *media.Container
		GetMovie(movieID uuid.UUID) (*media.Movie, error)
		GetEpisode(episodeID uuid.UUID) (*media.Episode, error)
		GetInflatedSeries(seriesID uuid.UUID) (*media.InflatedSeries, error)
		GetTranscodesForMedia(mediaID uuid.UUID) ([]*transcode.Transcode, error)
		GetAllTargets() []*ffmpeg.Target

		ListMedia(includeTypes []media.MediaListType, titleFilter string, includeGenres []int, orderBy []media.MediaListOrderBy, offset int, limit int) ([]*media.MediaListResult, error)
		ListGenres() ([]*media.Genre, error)

		DeleteEpisode(episodeID uuid.UUID) error
		DeleteSeries(seriesID uuid.UUID) error
		DeleteSeason(seasonID uuid.UUID) error
		DeleteMovie(movieID uuid.UUID) error
	}

	TranscodeService interface {
		ActiveTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
	}

	MediaController struct {
		store            Store
		transcodeService TranscodeService
	}
)

var (
	mediaListTypeMapping = map[string]media.MediaListType{
		"movie":  media.MovieType,
		"series": media.SeriesType,
	}

	mediaListOrderColumnMapping = map[string]media.MediaListOrderColumn{
		"id":        media.IDColumn,
		"updatedAt": media.UpdatedAtColumn,
		"createdAt": media.CreatedAtColumn,
		"title":     media.TitleColumn,
	}
)

func New(transcodeService TranscodeService, store Store) *MediaController {
	return &MediaController{store: store, transcodeService: transcodeService}
}

// ListMedia is an endpoint used to retrieve a list of movies and series which have been
// updated recently (this includes episodes being added to a series). The caller of this endpoint
// can specify filtering options such as the type (movie|series), a limit to the number
// of results, or the genres which apply to the content.
func (controller *MediaController) ListMedia(ec echo.Context, request gen.ListMediaRequestObject) (gen.ListMediaResponseObject, error) {
	allowedTypesRaw := []string{}
	if request.Params.AllowedType != nil {
		allowedTypesRaw = *request.Params.AllowedType
	}

	allowedTypes := make([]media.MediaListType, len(allowedTypesRaw))
	for k, v := range allowedTypesRaw {
		if vv, ok := mediaListTypeMapping[v]; ok {
			allowedTypes[k] = vv
			continue
		}

		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("allowedType '%v' is not recognized", v))
	}

	allowedGenresRaw := []string{}
	if request.Params.Genre != nil {
		allowedGenresRaw = *request.Params.Genre
	}

	allowedGenres := make([]int, len(allowedGenresRaw))
	for k, v := range allowedGenresRaw {
		vv, err := strconv.Atoi(v)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("genre '%v' is not recognized", v))
		}
		allowedGenres[k] = vv
	}

	orderByRaw := []string{}
	if request.Params.OrderBy != nil {
		orderByRaw = *request.Params.OrderBy
	}

	orderBy := make([]media.MediaListOrderBy, len(orderByRaw))
	for k, v := range orderByRaw {
		// If value begins with a '+/-', then this dictates the ordering
		// and should be stripped from the mapping lookup. Default ordering
		// is ascending (+).
		isDecending := false
		switch v[:1] {
		case "+":
			v = v[1:]
		case "-":
			v = v[1:]
			isDecending = true
		}

		if vv, ok := mediaListOrderColumnMapping[v]; ok {
			orderBy[k] = media.MediaListOrderBy{Column: vv, Descending: isDecending}
			continue
		}

		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("orderBy column '%v' is not recognized", v))
	}

	limit := 0
	offset := 0
	if request.Params.Limit != nil && *request.Params.Limit > 0 {
		limit = *request.Params.Limit
	}
	if request.Params.Offset != nil && *request.Params.Offset > 0 {
		limit = *request.Params.Offset
	}

	titleFilter := ""
	if request.Params.TitleFilter != nil {
		titleFilter = *request.Params.TitleFilter
	}

	results, err := controller.store.ListMedia(allowedTypes, titleFilter, allowedGenres, orderBy, offset, limit)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	dtos, err := newListDtos(results)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.ListMedia200JSONResponse(dtos), nil
}

func (controller *MediaController) ListGenres(ec echo.Context, _ gen.ListGenresRequestObject) (gen.ListGenresResponseObject, error) {
	genres, err := controller.store.ListGenres()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.ListGenres200JSONResponse(genreModelsToDtos(genres)), nil
}

func (controller *MediaController) GetMovie(ec echo.Context, request gen.GetMovieRequestObject) (gen.GetMovieResponseObject, error) {
	wrap := wrapErrorGenerator("failed to fetch movie")
	movie, err := controller.store.GetMovie(request.Id)
	if err != nil {
		return nil, wrap(err)
	}

	watchTargets, err := controller.getMediaWatchTargets(request.Id)
	if err != nil {
		return nil, wrap(err)
	}

	dto := gen.Movie{
		Id:           movie.ID,
		TmdbId:       movie.TmdbID,
		Title:        movie.Title,
		CreatedAt:    movie.CreatedAt,
		UpdatedAt:    movie.UpdatedAt,
		WatchTargets: watchTargets,
	}

	return gen.GetMovie200JSONResponse(dto), nil
}

func (controller *MediaController) GetEpisode(ec echo.Context, request gen.GetEpisodeRequestObject) (gen.GetEpisodeResponseObject, error) {
	wrap := wrapErrorGenerator("failed to fetch episode")
	episode, err := controller.store.GetEpisode(request.Id)
	if err != nil {
		return nil, wrap(err)
	}

	watchTargets, err := controller.getMediaWatchTargets(request.Id)
	if err != nil {
		return nil, wrap(err)
	}

	dto := gen.Episode{
		Id:           episode.ID,
		TmdbId:       episode.TmdbID,
		Title:        episode.Title,
		CreatedAt:    episode.CreatedAt,
		UpdatedAt:    episode.UpdatedAt,
		WatchTargets: watchTargets,
	}

	return gen.GetEpisode200JSONResponse(dto), nil
}

func (controller *MediaController) GetSeries(ec echo.Context, request gen.GetSeriesRequestObject) (gen.GetSeriesResponseObject, error) {
	series, err := controller.store.GetInflatedSeries(request.Id)
	if err != nil {
		return nil, wrapErrorGenerator("Failed to get series")(err)
	}

	return gen.GetSeries200JSONResponse(inflatedSeriesToDto(series)), nil
}

func (controller *MediaController) DeleteMovie(ec echo.Context, request gen.DeleteMovieRequestObject) (gen.DeleteMovieResponseObject, error) {
	if err := controller.store.DeleteMovie(request.Id); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.DeleteMovie201Response{}, nil
}

func (controller *MediaController) DeleteSeries(ec echo.Context, request gen.DeleteSeriesRequestObject) (gen.DeleteSeriesResponseObject, error) {
	if err := controller.store.DeleteSeries(request.Id); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.DeleteSeries201Response{}, nil
}

func (controller *MediaController) DeleteSeason(ec echo.Context, request gen.DeleteSeasonRequestObject) (gen.DeleteSeasonResponseObject, error) {
	if err := controller.store.DeleteSeason(request.Id); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.DeleteSeason201Response{}, nil
}

func (controller *MediaController) DeleteEpisode(ec echo.Context, request gen.DeleteEpisodeRequestObject) (gen.DeleteEpisodeResponseObject, error) {
	if err := controller.store.DeleteEpisode(request.Id); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	return gen.DeleteEpisode201Response{}, nil
}

func (controller *MediaController) getMediaWatchTargets(mediaID uuid.UUID) ([]gen.MediaWatchTarget, error) {
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
	watchTargets := make([]gen.MediaWatchTarget, 0, len(completedTranscodes))
	for _, v := range completedTranscodes {
		targetsNotEligibleForLiveTranscode[v.TargetID] = struct{}{}
		watchTargets = append(watchTargets, newWatchTarget(findTarget(v.TargetID), gen.PRETRANSCODE, true))
	}

	// 2. Add in-progress transcodes (as not ready to watch)
	for _, v := range activeTranscodes {
		targetsNotEligibleForLiveTranscode[v.Target().ID] = struct{}{}
		watchTargets = append(watchTargets, newWatchTarget(v.Target(), gen.PRETRANSCODE, false))
	}

	// 3. Any targets which do NOT have a complete or in-progress pre-transcode are eligible for live transcoding/streaming
	for _, v := range targets {
		// TODO: check if the specified target allows for live transcoding
		if _, ok := targetsNotEligibleForLiveTranscode[v.ID]; ok {
			continue
		}

		watchTargets = append(watchTargets, newWatchTarget(v, gen.LIVETRANSCODE, true))
	}

	// 4. We can directly stream the source media itself, so add that too
	// TODO: at some point we may want this to be configurable
	watchTargets = append(watchTargets, gen.MediaWatchTarget{DisplayName: "Direct", Ready: true, Type: gen.LIVETRANSCODE, TargetId: nil, Enabled: true})

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
