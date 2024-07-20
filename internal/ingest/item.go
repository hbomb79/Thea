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
	IngestItemState int
	IngestItem      struct {
		ID              uuid.UUID
		Path            string
		State           IngestItemState
		Trouble         *Trouble
		ScrapedMetadata *media.FileMediaMetadata
		OverrideTmdbID  *string
	}
)

const (
	Idle IngestItemState = iota
	ImportHold
	Ingesting
	Troubled
	Complete
)

var (
	ErrNoTrouble                     = errors.New("ingestion has no trouble")
	ErrIngestNotFound                = errors.New("no ingest task could be found")
	ErrResolutionIncompatible        = errors.New("provided resolution method is not valid for ingestion trouble")
	ErrResolutionIncomplete          = errors.New("provided resolution context is missing information required to resolve the trouble")
	ErrResolutionContextIncompatible = errors.New("trouble resolution failed, consult logs for further information")
)

// ingest is the main task for an ingest task which:
// - Scrapes the metadata from the file
// - Searches TMDB for a match
// - Saves the episode/movie to the database
// Any of the above can encounter an error - if the error can be cast to the
// IngestItemTrouble type then it should be raised as a TROUBLE on the item.
func (item *IngestItem) ingest(eventBus event.EventCoordinator, scraper Scraper, searcher Searcher, data DataStore) error {
	log.Emit(logger.NEW, "Beginning ingestion of item %s\n", item)
	if item.ScrapedMetadata == nil {
		log.Emit(logger.DEBUG, "Performing file system scrape of %s\n", item.Path)
		if meta, err := scraper.ScrapeFileForMediaInfo(item.Path); err != nil {
			return Trouble{error: err, tType: MetadataFailure}
		} else if meta == nil {
			return Trouble{error: errors.New("metadata scrape returned no error, but nil payload received"), tType: MetadataFailure}
		} else {
			log.Emit(logger.WARNING, "Scraped metadata for item %s:\n%s\n", item, meta)
			item.ScrapedMetadata = meta
		}
	}

	meta := item.ScrapedMetadata
	if item.ScrapedMetadata.Episodic {
		return item.ingestEpisode(meta, data, searcher, eventBus)
	} else {
		return item.ingestMovie(meta, data, searcher, eventBus)
	}
}

func (item *IngestItem) ingestEpisode(meta *media.FileMediaMetadata, data DataStore, searcher Searcher, eventBus event.EventDispatcher) error {
	var series *tmdb.Series
	if item.OverrideTmdbID != nil {
		// This item WAS troubled, but a resolution has provided a new value for the TMDB ID which we should use now.
		tmdbID := *item.OverrideTmdbID
		item.OverrideTmdbID = nil

		log.Emit(logger.INFO, "Retrying ingestion item %s with provided TMDB ID override (from trouble resolution) of %s\n", item, tmdbID)
		if found, err := searcher.GetSeries(tmdbID); err != nil {
			return newTrouble(err)
		} else {
			series = found
		}
	} else {
		seriesID, err := searcher.SearchForSeries(meta)
		if err != nil {
			return newTrouble(err)
		}

		found, err := searcher.GetSeries(seriesID)
		if err != nil {
			return newTrouble(err)
		}
		series = found
	}

	season, err := searcher.GetSeason(series.ID.String(), meta.SeasonNumber)
	if err != nil {
		return newTrouble(err)
	}

	episode, err := searcher.GetEpisode(series.ID.String(), meta.SeasonNumber, meta.EpisodeNumber)
	if err != nil {
		return newTrouble(err)
	}

	log.Emit(logger.DEBUG, "Saving TMDB EPISODE: %v\nSEASON: %v\nSERIES: %v\n", episode, season, series)
	ep := tmdb.TmdbEpisodeToMedia(episode, series.Adult, item.ScrapedMetadata)
	if err := data.SaveEpisode(
		ep,
		tmdb.TmdbSeasonToMedia(season),
		tmdb.TmdbSeriesToMedia(series),
	); err != nil {
		return newTrouble(err)
	}

	log.Emit(logger.SUCCESS, "Saved newly ingested episode %v\n", ep)
	eventBus.Dispatch(event.NewMediaEvent, ep.ID)
	return nil
}

func (item *IngestItem) ingestMovie(meta *media.FileMediaMetadata, data DataStore, searcher Searcher, eventBus event.EventDispatcher) error {
	var movie *tmdb.Movie
	if item.OverrideTmdbID != nil {
		// This item WAS troubled, but a resolution has provided a new value for the TMDB ID which we should use now.
		tmdbID := *item.OverrideTmdbID
		item.OverrideTmdbID = nil

		log.Emit(logger.INFO, "Retrying ingestion item %s with provided TMDB ID override (from trouble resolution) of %s\n", item, tmdbID)
		if found, err := searcher.GetMovie(tmdbID); err != nil {
			return newTrouble(err)
		} else {
			movie = found
		}
	} else {
		movieID, err := searcher.SearchForMovie(item.ScrapedMetadata)
		if err != nil {
			return newTrouble(err)
		}

		found, err := searcher.GetMovie(movieID)
		if err != nil {
			return newTrouble(err)
		}
		movie = found
	}

	log.Emit(logger.DEBUG, "Saving newly ingested MOVIE: %v\n", movie)
	mov := tmdb.TmdbMovieToMedia(movie, meta)
	if err := data.SaveMovie(mov); err != nil {
		return newTrouble(err)
	}

	log.Emit(logger.SUCCESS, "Saved newly ingested movie %v\n", mov)
	eventBus.Dispatch(event.NewMediaEvent, mov.ID)

	return nil
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
	return fmt.Sprintf("IngestItem{ID=%s state=%s}", item.ID, item.State)
}

func (s IngestItemState) String() string {
	switch s {
	case Idle:
		return fmt.Sprintf("IDLE[%d]", s)
	case ImportHold:
		return fmt.Sprintf("IMPORT_HOLD[%d]", s)
	case Ingesting:
		return fmt.Sprintf("INGESTING[%d]", s)
	case Troubled:
		return fmt.Sprintf("TROUBLED[%d]", s)
	case Complete:
		return fmt.Sprintf("COMPLETE[%d]", s)
	default:
		return fmt.Sprintf("UNKNOWN[%d]", s)
	}
}
