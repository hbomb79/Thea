package tmdb

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
)

func TmdbEpisodeToMedia(ep *Episode, metadata *media.FileMediaMetadata) *media.Episode {
	return &media.Episode{
		Model:         media.Model{ID: uuid.New(), TmdbId: ep.Id.String(), Title: ep.Name},
		Watchable:     mediaMetadataToWatchable(metadata),
		EpisodeNumber: metadata.EpisodeNumber,
	}
}

func TmdbSeriesToMedia(series *Series) *media.Series {
	return &media.Series{
		Model: media.Model{ID: uuid.New(), TmdbId: series.Id.String(), Title: series.Name},
		Adult: series.Adult,
	}
}

func TmdbSeasonToMedia(season *Season) *media.Season {
	return &media.Season{
		Model: media.Model{ID: uuid.New(), TmdbId: season.Id.String(), Title: season.Name},
	}
}

func TmdbMovieToMedia(movie *Movie, metadata *media.FileMediaMetadata) *media.Movie {
	return &media.Movie{
		Model:     media.Model{ID: uuid.New(), TmdbId: movie.Id.String(), Title: movie.Name},
		Watchable: mediaMetadataToWatchable(metadata),
		Adult:     movie.Adult,
	}
}

func mediaMetadataToWatchable(metadata *media.FileMediaMetadata) media.Watchable {
	return media.Watchable{
		MediaResolution: media.MediaResolution{Width: *metadata.FrameW, Height: *metadata.FrameH},
		SourcePath:      metadata.Path,
	}
}
