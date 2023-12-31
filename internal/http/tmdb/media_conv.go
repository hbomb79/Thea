package tmdb

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
)

func TmdbEpisodeToMedia(ep *Episode, isSeasonAdult bool, metadata *media.FileMediaMetadata) *media.Episode {
	return &media.Episode{
		Model: media.Model{ID: uuid.New(), TmdbID: ep.Id.String(), Title: ep.Name},
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{Width: *metadata.FrameW, Height: *metadata.FrameH},
			SourcePath:      metadata.Path,
			Duration:        runtimeToDurationSecs(metadata.Runtime),
			Adult:           isSeasonAdult,
		},
		EpisodeNumber: metadata.EpisodeNumber,
	}
}

func TmdbSeriesToMedia(series *Series) *media.Series {
	return &media.Series{
		Model: media.Model{ID: uuid.New(), TmdbID: series.Id.String(), Title: series.Name},
	}
}

func TmdbSeasonToMedia(season *Season) *media.Season {
	return &media.Season{
		Model: media.Model{ID: uuid.New(), TmdbID: season.Id.String(), Title: season.Name},
	}
}

func TmdbMovieToMedia(movie *Movie, metadata *media.FileMediaMetadata) *media.Movie {
	return &media.Movie{
		Model: media.Model{ID: uuid.New(), TmdbID: movie.Id.String(), Title: movie.Name},
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{Width: *metadata.FrameW, Height: *metadata.FrameH},
			SourcePath:      metadata.Path,
			Duration:        runtimeToDurationSecs(metadata.Runtime),
			Adult:           movie.Adult,
		},
	}
}

func runtimeToDurationSecs(runtime string) int {
	secsFloat, err := strconv.ParseFloat(runtime, 64)
	if err != nil {
		return 0 // TODO: Panic or handle if runtime is not a float number
	}

	secs := int(secsFloat)
	return secs
}
