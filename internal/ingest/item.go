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
	TroubleResolution struct {
		method  ResolutionType
		context map[string]any
	}

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
	IDLE IngestItemState = iota
	IMPORT_HOLD
	INGESTING
	TROUBLED
	COMPLETE
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
func (item *IngestItem) ingest(eventBus event.EventCoordinator, scraper scraper, searcher searcher, data DataStore) error {
	log.Emit(logger.NEW, "Beginning ingestion of item %s\n", item)
	if item.ScrapedMetadata == nil {
		log.Emit(logger.DEBUG, "Performing file system scrape of %s\n", item.Path)
		if meta, err := scraper.ScrapeFileForMediaInfo(item.Path); err != nil {
			return Trouble{error: err, tType: METADATA_FAILURE}
		} else if meta == nil {
			return Trouble{error: errors.New("metadata scrape returned no error, but nil payload received"), tType: METADATA_FAILURE}
		} else {
			log.Emit(logger.DEBUG, "Scraped metadata for item %s:\n%#v\n", item, meta)
			item.ScrapedMetadata = meta
		}
	}

	meta := item.ScrapedMetadata
	if item.ScrapedMetadata.Episodic {
		var series *tmdb.Series = nil
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
			if found, err := searcher.SearchForSeries(meta); err != nil {
				return newTrouble(err)
			} else {
				series = found
			}
		}

		season, err := searcher.GetSeason(series.Id.String(), meta.SeasonNumber)
		if err != nil {
			return newTrouble(err)
		}

		episode, err := searcher.GetEpisode(series.Id.String(), meta.SeasonNumber, meta.EpisodeNumber)
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
		eventBus.Dispatch(event.NEW_MEDIA, ep.ID)
	} else {
		var movie *tmdb.Movie = nil
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
			if found, err := searcher.SearchForMovie(item.ScrapedMetadata); err != nil {
				return newTrouble(err)
			} else {
				movie = found
			}
		}

		log.Emit(logger.DEBUG, "Saving newly ingested MOVIE: %v\n", movie)
		mov := tmdb.TmdbMovieToMedia(movie, meta)
		if err := data.SaveMovie(mov); err != nil {
			return newTrouble(err)
		}

		log.Emit(logger.SUCCESS, "Saved newly ingested movie %v\n", mov)
		eventBus.Dispatch(event.NEW_MEDIA, mov.ID)
	}

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
	case IDLE:
		return fmt.Sprintf("IDLE[%d]", s)
	case IMPORT_HOLD:
		return fmt.Sprintf("IMPORT_HOLD[%d]", s)
	case INGESTING:
		return fmt.Sprintf("INGESTING[%d]", s)
	case TROUBLED:
		return fmt.Sprintf("TROUBLED[%d]", s)
	default:
		return fmt.Sprintf("UNKNOWN[%d]", s)
	}
}
