package medias

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
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
		GetTranscodesForMedia(uuid.UUID) ([]*transcode.Transcode, error)
		GetAllTargets() []*ffmpeg.Target

		ListMedia(includeTypes []media.MediaListType, orderBy []media.MediaListOrderBy, offset int, limit int) ([]*media.MediaListResult, error)

		DeleteEpisode(episodeID uuid.UUID) error
		DeleteSeries(seriesID uuid.UUID) error
		DeleteSeason(seasonID uuid.UUID) error
		DeleteMovie(movieID uuid.UUID) error
	}

	TranscodeService interface {
		ActiveTasksForMedia(mediaID uuid.UUID) []*transcode.TranscodeTask
	}

	StreamingService interface {
		GenerateHSLPlaylist(*media.Container) string
	}

	Controller struct {
		store            Store
		transcodeService TranscodeService
		config           *transcode.Config
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

func New(validate *validator.Validate, transcodeService TranscodeService, store Store, config *transcode.Config) *Controller {
	return &Controller{store: store, transcodeService: transcodeService, config: config}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.GET("/", controller.list)

	eg.GET("/movie/:id/", controller.getMovie)
	eg.DELETE("/movie/:id/", controller.deleteMovie)

	eg.GET("/series/:id/", controller.getSeries)

	eg.GET("/episode/:id/", controller.getEpisode)

	eg.DELETE("/series/:id/", controller.deleteSeries)
	eg.DELETE("/season/:id/", controller.deleteSeason)
	eg.DELETE("/episode/:id/", controller.deleteEpisode)

	eg.GET("/:id/stream/direct/stream.m3u8/", controller.getDirectStreamPlaylist)
	eg.GET("/:id/stream/direct/:file/", controller.getDirectStreamSegment)
}

// list is an endpoint used to retrieve a list of movies and series which have been
// updated recently (this includes episodes being added to a series). The caller of this endpoint
// can specify filtering options such as the type (movie|series), a limit to the number
// of results, or the genres which apply to the content
//
// TODO: the genre stuff!
func (controller *Controller) list(ec echo.Context) error {
	params := ec.QueryParams()
	allowedTypesRaw, ok := params["allowedType"]
	if !ok {
		allowedTypesRaw = []string{}
	}

	allowedTypes := make([]media.MediaListType, len(allowedTypesRaw))
	for k, v := range allowedTypesRaw {
		if vv, ok := mediaListTypeMapping[v]; ok {
			allowedTypes[k] = vv
			continue
		}

		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("allowedType '%v' is not recognized", v))
	}

	orderByRaw, ok := params["orderBy"]
	if !ok {
		orderByRaw = []string{}
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

		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("orderBy column '%v' is not recognized", v))
	}

	limit, err := strconv.Atoi(params.Get("limit"))
	if err != nil || limit < 0 {
		limit = 0
	}
	offset, err := strconv.Atoi(params.Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	results, err := controller.store.ListMedia(allowedTypes, orderBy, offset, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	dtos, err := newListDtos(results)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
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

func (controller *Controller) getDirectStreamPlaylist(ec echo.Context) error {
	mediaID, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Media ID is not a valid UUID")
	}

	mediaContainer := controller.store.GetMedia(mediaID)
	if mediaContainer == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No Media found")
	}

	playlistContent := media.GenerateHSLPlaylist(mediaContainer)
	response := ec.Response().Writer

	response.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
	response.Write([]byte(playlistContent))

	return nil
}

func (controller *Controller) getDirectStreamSegment(ec echo.Context) error {
	mediaID, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Media ID is not a valid UUID")
	}

	tempDir := "/tmp/"
	segmentOutputDir := tempDir + mediaID.String() + "/"
	file := ec.Param("file")
	segmentIndex, ext, _ := strings.Cut(file, ".")

	if _, err := os.Stat(segmentOutputDir + file); err == nil {
		return ec.File(segmentOutputDir + file)
	} else if os.IsNotExist(err) && ext != "ts" {
		return echo.ErrNotFound
	}

	// From this point on, the file request is a .ts file
	if _, err := strconv.Atoi(segmentIndex); err != nil {
		return echo.ErrNotFound
	}

	mediaContainer := controller.store.GetMedia(mediaID)
	if mediaContainer == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No Media found")
	}

	go media.GenerateHSLSegments(ec.Request().Context(), mediaContainer, ffmpeg.Config{
		FfmpegBinPath:       controller.config.FfmpegBinaryPath,
		FfprobeBinPath:      controller.config.FfprobeBinaryPath,
		OutputBaseDirectory: segmentOutputDir,
	})

	waitErr := waitForFile(segmentOutputDir + file)
	if waitErr != nil {
		return ec.String(http.StatusInternalServerError, fmt.Sprintf("Error: %s", waitErr.Error()))
	}

	// Serve the file after it becomes available
	return ec.File(segmentOutputDir + file)
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

func waitForFile(filePath string) error {
	maxWaitDuration := 30 * time.Second // Maximum duration to wait for the file
	pollingInterval := 1 * time.Second  // Interval for checking the file existence

	startTime := time.Now()

	for {
		_, err := os.Stat(filePath)
		if err == nil {
			// File exists, return it
			return nil
		}

		if time.Since(startTime) > maxWaitDuration {
			// Timeout reached, file not found
			return fmt.Errorf("file %s not found after waiting", filePath)
		}

		time.Sleep(pollingInterval)
	}
}
