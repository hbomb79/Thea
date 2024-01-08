package media

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type mediaGenreStore struct{}

// SaveMovieGenreAssociations handles only the upserting of the genre associations
// for a given movie model.
//
// NB: This query will FAIL if any of the given genres do not have a row in the genre table
func (store *mediaGenreStore) SaveMovieGenreAssociations(db database.Queryable, movieID uuid.UUID, genres []*Genre) error {
	if len(genres) > 0 {
		type genreAssoc struct {
			ID      uuid.UUID `db:"id"`
			MovieID uuid.UUID `db:"movie_id"`
			GenreID int       `db:"genre_id"`
		}
		genreAssocs := make([]genreAssoc, len(genres))
		for k, v := range genres {
			genreAssocs[k] = genreAssoc{uuid.New(), movieID, v.Id}
		}

		if err := database.InExec(db, `DELETE FROM movie_genres mg WHERE mg.movie_id=$1`, movieID); err != nil {
			return err
		}

		_, err := db.NamedExec(`
			INSERT INTO movie_genres(id, movie_id, genre_id)
			VALUES(:id, :movie_id, :genre_id)
			ON CONFLICT(movie_id, genre_id) DO NOTHING
		`, genreAssocs)

		return err
	}

	_, err := db.Exec(`
		DELETE FROM movie_genres WHERE media_id=$1`, movieID)
	return err
}

// SaveSeriesGenreAssociations handles only the upserting of the genre associations
// for a given series model.
//
// NB: This query will FAIL if any of the given genres do not have a row in the genre table
func (store *mediaGenreStore) SaveSeriesGenreAssociations(db database.Queryable, seriesID uuid.UUID, genres []*Genre) error {
	if len(genres) > 0 {
		type genreAssoc struct {
			ID       uuid.UUID `db:"id"`
			SeriesID uuid.UUID `db:"series_id"`
			GenreID  int       `db:"genre_id"`
		}
		genreAssocs := make([]genreAssoc, len(genres))
		for k, v := range genres {
			genreAssocs[k] = genreAssoc{uuid.New(), seriesID, v.Id}
		}

		if err := database.InExec(db, `DELETE FROM series_genres sg WHERE sg.series_id=$1`, seriesID); err != nil {
			return err
		}

		_, err := db.NamedExec(`
			INSERT INTO series_genres(id, series_id, genre_id)
			VALUES(:id, :series_id, :genre_id)
			ON CONFLICT(series_id, genre_id) DO NOTHING
		`, genreAssocs)

		return err
	}

	_, err := db.Exec(`
		DELETE FROM series_genres WHERE series_id=$1`, seriesID)
	return err
}

// SaveGenres saves the given genre labels to the database, ignoring any which
// already exist in the database (determined based on label conflicts).
// This function will return back all the genres referenced by the labels provided,
// regardless of whether the genres were already present in the database.
func (store *mediaGenreStore) SaveGenres(tx *sqlx.Tx, genres []*Genre) ([]*Genre, error) {
	if len(genres) == 0 {
		return []*Genre{}, nil
	}

	if _, err := tx.NamedExec(
		`INSERT INTO genre(label) VALUES (:label) ON CONFLICT(label) DO NOTHING`,
		genres,
	); err != nil {
		return nil, fmt.Errorf("failed to insert bulk genres: %w", err)
	}

	query, args, err := sqlx.Named(`SELECT * FROM genre WHERE label = any(:label)`, genres)
	if err != nil {
		return nil, fmt.Errorf("failed to construct named query: %w", err)
	}

	var results []*Genre
	if err := tx.Select(&results, tx.Rebind(query), pq.Array(args)); err != nil {
		return nil, fmt.Errorf("failed to select saved genres: %w [query %s and args %#v]", err, query, args)
	}

	return results, nil
}

func (store *mediaGenreStore) ListGenres(db database.Queryable) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, `SELECT * FROM genre`); err != nil {
		return nil, err
	}

	return results, nil
}

func (store *mediaGenreStore) GetGenresForMovie(db database.Queryable, movieID uuid.UUID) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, getGenresForSql("movie_genres", "movie_id"), movieID); err != nil {
		return nil, err
	}

	return results, nil
}

func (store *mediaGenreStore) GetGenresForSeries(db database.Queryable, seriesID uuid.UUID) ([]*Genre, error) {
	var results []*Genre
	if err := db.Select(&results, getGenresForSql("series_genres", "series_id"), seriesID); err != nil {
		return nil, err
	}

	return results, nil
}

func getGenresForSql(tableName string, tableColumn string) string {
	template := `
		SELECT genre.* FROM TABLENAME
		INNER JOIN genre
		ON genre.id = TABLENAME.genre_id
		WHERE TABLENAME.TABLECOLUMN = $1`

	return strings.ReplaceAll(strings.ReplaceAll(template, "TABLENAME", tableName), "TABLECOLUMN", tableColumn)
}
