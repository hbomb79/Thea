package medias

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/labstack/echo/v4"
)

const (
	PreTranscoded watchTargetType = "pre-transcode"
	LiveTranscode watchTargetType = "live-transcode"
)

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
	watchTargetType string
	watchTargetDto  struct {
		Name     string          `json:"display_name"`
		TargetID *uuid.UUID      `json:"target_id,omitempty"`
		Enabled  bool            `json:"enabled"`
		Type     watchTargetType `json:"type"`
		Ready    bool            `json:"ready"`
		// TODO: may want to include some additional information about the
		// target here, such as bitrate and resolution.
	}

	seriesStubDto struct {
		ID          uuid.UUID `json:"id"`
		Title       string    `json:"title"`
		SeasonCount int       `json:"season_count"`
	}

	movieStubDto struct {
		ID    uuid.UUID `json:"id"`
		Title string    `json:"title"`
		// TODO: poster path, runtime
	}

	listDto struct {
		Type        string    `json:"type"`
		ID          uuid.UUID `json:"id"`
		Title       string    `json:"title"`
		TmdbID      string    `json:"tmdb_id"`
		UpdatedAt   time.Time `json:"updated_at"`
		SeasonCount *int      `json:"season_count,omitempty"`
		// TODO poster path, optional movie/series specific information such as runtime and season count
	}

	episodeDto struct {
		ID           uuid.UUID         `json:"id"`
		TmdbID       string            `json:"tmdb_id"`
		Title        string            `json:"title"`
		CreatedAt    time.Time         `json:"created_at"`
		UpdatedAt    time.Time         `json:"updated_at"`
		WatchTargets []*watchTargetDto `json:"watch_targets"`
	}

	episodeStubDto struct {
		ID    uuid.UUID `json:"id"`
		Title string    `json:"title"`
		Adult bool      `json:"adult"`
	}

	seasonDto struct {
		Episodes []*episodeStubDto `json:"episodes"`
	}

	seriesDto struct {
		ID      uuid.UUID    `json:"id"`
		TmdbID  string       `json:"tmdb_id"`
		Title   string       `json:"title"`
		Seasons []*seasonDto `json:"seasons"`
	}

	// movieDto is a fully inflated version of the more common movieStubDto, which encodes more
	// information such as the watch targets which are eligible for the media
	movieDto struct {
		ID           uuid.UUID         `json:"id"`
		TmdbID       string            `json:"tmdb_id"`
		Title        string            `json:"title"`
		CreatedAt    time.Time         `json:"created_at"`
		UpdatedAt    time.Time         `json:"updated_at"`
		WatchTargets []*watchTargetDto `json:"watch_targets"`
	}
)

func newWatchTarget(target *ffmpeg.Target, t watchTargetType, ready bool) *watchTargetDto {
	return &watchTargetDto{Name: target.Label, Ready: ready, Type: t, TargetID: &target.ID, Enabled: true}
}

func newListDtos(results []*media.MediaListResult) ([]*listDto, error) {
	dtos := make([]*listDto, len(results))
	for k, v := range results {
		dto, err := newListDto(v)
		if err != nil {
			return nil, err
		}
		dtos[k] = dto
	}

	return dtos, nil
}

func newListDto(result *media.MediaListResult) (*listDto, error) {
	if result.IsMovie() {
		movie := result.Movie
		return &listDto{Type: "movie", ID: movie.ID, Title: movie.Title, TmdbID: movie.TmdbID, UpdatedAt: movie.UpdatedAt, SeasonCount: nil}, nil
	} else if result.IsSeries() {
		series := result.Series
		return &listDto{Type: "series", ID: series.ID, Title: series.Title, TmdbID: series.TmdbID, UpdatedAt: series.UpdatedAt, SeasonCount: &series.SeasonCount}, nil
	}

	return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Media %v found during listing has an illegal type. Expected movie or series.", result))
}

func inflatedSeriesModelToDto(model *media.SeriesStub) seriesStubDto {
	return seriesStubDto{
		ID:          model.ID,
		Title:       model.Title,
		SeasonCount: model.SeasonCount,
	}
}

func movieModelToDto(model *media.Movie) movieStubDto {
	return movieStubDto{
		ID:    model.ID,
		Title: model.Title,
	}
}
