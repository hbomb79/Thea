package medias

import (
	"fmt"
	"net/http"

	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/labstack/echo/v4"
)

func newWatchTarget(target *ffmpeg.Target, t gen.MediaWatchTargetType, ready bool) gen.MediaWatchTarget {
	return gen.MediaWatchTarget{DisplayName: target.Label, Ready: ready, Type: t, TargetId: &target.ID, Enabled: true}
}

func episodeToStubDto(episode *media.Episode) gen.EpisodeStub {
	return gen.EpisodeStub{Adult: episode.Adult, Id: episode.ID, Title: episode.Title}
}

func episodesToStubDtos(episodes []*media.Episode) []gen.EpisodeStub {
	return util.ApplyConversion(episodes, episodeToStubDto)
}

func inflatedSeasonToDto(season *media.InflatedSeason) gen.Season {
	return gen.Season{Episodes: episodesToStubDtos(season.Episodes)}
}

func infaltedSeasonsToDtos(seasons []*media.InflatedSeason) []gen.Season {
	return util.ApplyConversion(seasons, inflatedSeasonToDto)
}

func inflatedSeriesToDto(series *media.InflatedSeries) gen.Series {
	return gen.Series{
		Id:      series.ID,
		Seasons: infaltedSeasonsToDtos(series.Seasons),
		Title:   series.Title,
		TmdbId:  series.TmdbID,
	}
}

func newListDtos(results []*media.MediaListResult) ([]gen.MediaListItem, error) {
	dtos := make([]gen.MediaListItem, len(results))
	for k, v := range results {
		dto, err := newListDto(v)
		if err != nil {
			return nil, err
		}
		dtos[k] = *dto
	}

	return dtos, nil
}

func newListDto(result *media.MediaListResult) (*gen.MediaListItem, error) {
	if result.IsMovie() {
		movie := result.Movie
		return &gen.MediaListItem{
			Type:        gen.MOVIE,
			Id:          movie.ID,
			Title:       movie.Title,
			TmdbId:      movie.TmdbID,
			UpdatedAt:   movie.UpdatedAt,
			SeasonCount: nil,
			Genres:      genreModelsToDtos(movie.Genres),
		}, nil
	} else if result.IsSeries() {
		series := result.Series
		return &gen.MediaListItem{
			Type:        gen.SERIES,
			Id:          series.ID,
			Title:       series.Title,
			TmdbId:      series.TmdbID,
			UpdatedAt:   series.UpdatedAt,
			SeasonCount: &series.SeasonCount,
			Genres:      genreModelsToDtos(series.Genres),
		}, nil
	}

	return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Media %v found during listing has an illegal type. Expected movie or series.", result))
}

func genreModelsToDtos(genres []*media.Genre) []gen.MediaGenre {
	dtos := make([]gen.MediaGenre, len(genres))
	for k, v := range genres {
		dtos[k] = gen.MediaGenre{Id: fmt.Sprint(v.ID), Label: v.Label}
	}

	return dtos
}
