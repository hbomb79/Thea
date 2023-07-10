package ingest

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
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

	METADATA_FAILURE TroubleType = iota
	TMDB_FAILURE_UNKNOWN
	TMDB_FAILURE_MULTI
	TMDB_FAILURE_NONE
)

func (item *IngestItem) ResolveTrouble() error { return errors.New("not yet implemented") }

func (item *IngestItem) ingest(scraper scraper, searcher searcher, data dataStore) error {
	log.Emit(logger.NEW, "Beginning ingestion of item %s\n", item)
	if item.ScrapedMetadata == nil {
		log.Emit(logger.DEBUG, "Performing file system metadata scrape\n")
		if meta, err := scraper.ScrapeFileForMediaInfo(item.Path); err != nil {
			log.Emit(logger.DEBUG, "Err %s\n", err.Error())
			return IngestItemTrouble{err, METADATA_FAILURE}
		} else if meta == nil {
			log.Emit(logger.DEBUG, "Nil\n")
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

		season, err := searcher.GetSeason(series.Id, meta.SeasonNumber)
		if err != nil {
			return IngestItemTrouble{err, TMDB_FAILURE_UNKNOWN}
		}

		ep, err := searcher.GetEpisode(series.Id, meta.SeasonNumber, meta.EpisodeNumber)
		if err != nil {
			return IngestItemTrouble{err, TMDB_FAILURE_UNKNOWN}
		}

		log.Emit(logger.DEBUG, "Saving newly ingested EPISODE: %s\nSEASON: %s\nSERIES: %s\n", ep, season, series)
		return data.SaveEpisode(ep.ToMediaEpisode(), season.ToMediaSeason(), series.ToMediaSeries())
	} else {
		movie, err := searcher.SearchForMovie(item.ScrapedMetadata)
		if err != nil {
			return handleSearchError(err)
		}

		log.Emit(logger.DEBUG, "Saving newly ingested MOVIE: %s", movie)
		return data.SaveMovie(movie.ToMediaMovie())
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
	return fmt.Sprintf("IngestItem{ID=%s state=%d}", item.Id, item.State)
}
