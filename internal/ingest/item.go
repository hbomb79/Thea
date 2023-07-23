package ingest

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/event"
	"github.com/hbomb79/Thea/internal/http/tmdb"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
)

type (
	TroubleType       int
	IngestItemTrouble struct {
		error
		Type TroubleType
	}
	IngestItemState int
	IngestItem      struct {
		Id      uuid.UUID
		Path    string
		State   IngestItemState
		Trouble *IngestItemTrouble

		ScrapedMetadata *media.FileMediaMetadata
	}
)

const (
	IDLE IngestItemState = iota
	IMPORT_HOLD
	INGESTING
	TROUBLED
	COMPLETE

	METADATA_FAILURE TroubleType = iota
	TMDB_FAILURE_UNKNOWN
	TMDB_FAILURE_MULTI
	TMDB_FAILURE_NONE
	GENERIC_FAILURE
)

func (item *IngestItem) ResolveTrouble(method string, context map[string]string) error {
	return nil
}

// ingest is the main task for an ingest task which:
// - Scrapes the metadata from the file
// - Searches TMDB for a match
// - Saves the episode/movie to the database
// Any of the above can encounter an error - if the error can be cast to the
// IngestItemTrouble type then it should be raised as a TROUBLE on the item.
func (item *IngestItem) ingest(eventBus event.EventCoordinator, scraper scraper, searcher searcher, data dataStore) error {
	log.Emit(logger.NEW, "Beginning ingestion of item %s\n", item)
	if item.ScrapedMetadata == nil {
		log.Emit(logger.DEBUG, "Performing file system metadata scrape\n")
		if meta, err := scraper.ScrapeFileForMediaInfo(item.Path); err != nil {
			return IngestItemTrouble{err, METADATA_FAILURE}
		} else if meta == nil {
			return IngestItemTrouble{errors.New("metadata scraping returned no error, but also returned nil"), METADATA_FAILURE}
		} else {
			log.Emit(logger.DEBUG, "Scrape for item %s complete:\n%#v\n", item, meta)
			item.ScrapedMetadata = meta
		}
	}

	log.Emit(logger.DEBUG, "Performing TMDB search\n")
	meta := item.ScrapedMetadata
	if item.ScrapedMetadata.Episodic {
		series, err := searcher.SearchForSeries(meta)
		if err != nil {
			return handleSearchError(err)
		}

		season, err := searcher.GetSeason(series.Id.String(), meta.SeasonNumber)
		if err != nil {
			return IngestItemTrouble{err, TMDB_FAILURE_UNKNOWN}
		}

		episode, err := searcher.GetEpisode(series.Id.String(), meta.SeasonNumber, meta.EpisodeNumber)
		if err != nil {
			return IngestItemTrouble{err, TMDB_FAILURE_UNKNOWN}
		}

		log.Emit(logger.DEBUG, "Saving TMDB EPISODE: %v\nSEASON: %v\nSERIES: %v\n", episode, season, series)
		ep := item.tmdbEpisodeToMedia(episode)
		if err := data.SaveEpisode(
			ep,
			item.tmdbSeasonToMedia(season),
			item.tmdbSeriesToMedia(series),
		); err != nil {
			return IngestItemTrouble{err, GENERIC_FAILURE}
		}

		log.Emit(logger.SUCCESS, "Saved newly ingested episode %v\n", ep)
		eventBus.Dispatch(event.NEW_MEDIA, ep.ID)
	} else {
		movie, err := searcher.SearchForMovie(item.ScrapedMetadata)
		if err != nil {
			return handleSearchError(err)
		}

		log.Emit(logger.DEBUG, "Saving newly ingested MOVIE: %v\n", movie)
		mov := item.tmdbMovieToMedia(movie)
		if err := data.SaveMovie(mov); err != nil {
			return IngestItemTrouble{err, GENERIC_FAILURE}
		}

		log.Emit(logger.SUCCESS, "Saved newly ingested movie %v\n", mov)
		eventBus.Dispatch(event.NEW_MEDIA, mov.ID)
	}

	return nil
}

func (item *IngestItem) tmdbEpisodeToMedia(ep *tmdb.Episode) *media.Episode {
	scrapedMetadata := item.ScrapedMetadata
	return &media.Episode{
		Model: media.Model{ID: uuid.New(), TmdbId: ep.Id.String(), Title: ep.Name},
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{
				Width:  *scrapedMetadata.FrameW,
				Height: *scrapedMetadata.FrameH,
			},
			SourcePath: item.Path,
		},
		EpisodeNumber: scrapedMetadata.EpisodeNumber,
	}
}

func (item *IngestItem) tmdbSeriesToMedia(series *tmdb.Series) *media.Series {
	return &media.Series{
		Model: media.Model{ID: uuid.New(), TmdbId: series.Id.String(), Title: series.Name},
		Adult: series.Adult,
	}
}

func (item *IngestItem) tmdbSeasonToMedia(season *tmdb.Season) *media.Season {
	return &media.Season{
		Model: media.Model{ID: uuid.New(), TmdbId: season.Id.String(), Title: season.Name},
	}

}

func (item *IngestItem) tmdbMovieToMedia(movie *tmdb.Movie) *media.Movie {
	scrapedMetadata := item.ScrapedMetadata
	return &media.Movie{
		Model: media.Model{ID: uuid.New(), TmdbId: movie.Id.String(), Title: movie.Name},
		Watchable: media.Watchable{
			MediaResolution: media.MediaResolution{Width: *scrapedMetadata.FrameW, Height: *scrapedMetadata.FrameH},
			SourcePath:      item.Path,
		},
		Adult: movie.Adult,
	}
}

func handleSearchError(err error) error {
	switch e := err.(type) {
	case *tmdb.NoResultError:
		return IngestItemTrouble{e, TMDB_FAILURE_NONE}
	case *tmdb.MultipleResultError:
		return IngestItemTrouble{e, TMDB_FAILURE_MULTI}
	case *tmdb.IllegalRequestError:
		return IngestItemTrouble{e, TMDB_FAILURE_UNKNOWN}
	}

	return IngestItemTrouble{err, TMDB_FAILURE_UNKNOWN}
}

func (item *IngestItem) modtimeDiff() (*time.Duration, error) {
	itemInfo, err := os.Stat(item.Path)
	if err != nil {
		return nil, err
	}

	diff := time.Since(itemInfo.ModTime())
	return &diff, nil
}

func (item *IngestItem) String() string {
	return fmt.Sprintf("IngestItem{ID=%s state=%s}", item.Id, item.State)
}

func (t TroubleType) String() string {
	switch t {
	case METADATA_FAILURE:
		return fmt.Sprintf("METADATA_FAILURE[%d]", t)
	case TMDB_FAILURE_UNKNOWN:
		return fmt.Sprintf("TMDB_FAILURE_UNKNOWN[%d]", t)
	case TMDB_FAILURE_MULTI:
		return fmt.Sprintf("TMDB_FAILURE_MULTI[%d]", t)
	case TMDB_FAILURE_NONE:
		return fmt.Sprintf("TMDB_FAILURE_NONE[%d]", t)
	}

	return fmt.Sprintf("UNKNOWN[%d]", t)
}

func (s IngestItemState) String() string {
	switch s {
	case IDLE:
		return fmt.Sprintf("IDLE[%d]", s)
	case IMPORT_HOLD:
		return fmt.Sprintf("IMPORT_HOLD[%d]", s)
	case INGESTING:
		return fmt.Sprintf("INGESTING[%d]", s)
	case TROUBLED:
		return fmt.Sprintf("TROUBLED[%d]", s)
	}

	return fmt.Sprintf("UNKNOWN[%d]", s)
}
