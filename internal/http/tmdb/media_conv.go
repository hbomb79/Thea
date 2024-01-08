package tmdb

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
)

func TmdbEpisodeToMedia(ep *Episode, isSeasonAdult bool, metadata *media.FileMediaMetadata) *media.Episode {
	return &media.Episode{
		Model: media.Model{ID: uuid.New(), TmdbID: ep.Id.String(), Title: ep.Name},
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{Width: *metadata.FrameW, Height: *metadata.FrameH},
			SourcePath:      metadata.Path,
			Adult:           isSeasonAdult,
		},
		EpisodeNumber: metadata.EpisodeNumber,
	}
}

func TmdbGenresToMedia(genres []Genre) []*media.Genre {
	gs := make([]*media.Genre, len(genres))
	for k, v := range genres {
		gs[k] = &media.Genre{Id: -1, Label: v.Name}
	}

	return gs
}

func TmdbSeriesToMedia(series *Series) *media.Series {
	return &media.Series{
		Model:  media.Model{ID: uuid.New(), TmdbID: series.Id.String(), Title: series.Name},
		Genres: TmdbGenresToMedia(series.Genres),
	}
}

func TmdbSeasonToMedia(season *Season) *media.Season {
	return &media.Season{
		Model: media.Model{ID: uuid.New(), TmdbID: season.Id.String(), Title: season.Name},
	}
}

func TmdbMovieToMedia(movie *Movie, metadata *media.FileMediaMetadata) *media.Movie {
	return &media.Movie{
		Model:  media.Model{ID: uuid.New(), TmdbID: movie.Id.String(), Title: movie.Name},
		Genres: TmdbGenresToMedia(movie.Genres),
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{Width: *metadata.FrameW, Height: *metadata.FrameH},
			SourcePath:      metadata.Path,
			Adult:           movie.Adult,
		},
	}
}
