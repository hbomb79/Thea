package ffmpeg

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/jmoiron/sqlx"
)

const (
	TargetTable = "transcode_target"
)

type Store struct{}

func (store *Store) Save(db database.Queryable, target *Target) error {
	_, err := db.NamedExec(`
		INSERT INTO transcode_target(id, label, ffmpeg_options, extension)
		VALUES (:id, :label, :ffmpeg_options, :extension)
		ON CONFLICT(id) DO UPDATE
		SET (label, ffmpeg_options, extension) = (EXCLUDED.label, EXCLUDED.ffmpeg_options, EXCLUDED.extension)
	`, target)

	return err
}

func (store *Store) Get(db database.Queryable, id uuid.UUID) *Target {
	var result Target
	err := db.Get(&result, `SELECT * FROM transcode_target WHERE id=$1;`, id)
	if err != nil {
		log.Warnf("Failed to find target (id=%s): %v\n", id, err)
		return nil
	}

	return &result
}

func (store *Store) GetAll(db database.Queryable) []*Target {
	var results []*Target
	err := db.Select(&results, `SELECT * FROM transcode_target;`)
	if err != nil {
		log.Fatalf("Failed to fetch all targets: %v\n", err)
		return make([]*Target, 0)
	}

	return results
}

func (store *Store) GetMany(db database.Queryable, ids ...uuid.UUID) []*Target {
	query, args, err := sqlx.In(`SELECT * FROM transcode_target WHERE id IN (?);`)
	if err != nil {
		log.Fatalf("Unable to create SELECT .. IN (a,b,c,...) query: %v", err)
		return nil
	}

	var results []*Target
	err = db.Select(results, db.Rebind(query), args)
	if err != nil {
		log.Fatalf("Failed to batch get targets with IDs=%#v: %v\n", ids, err)
		return nil
	}

	return results
}

func (store *Store) Delete(db database.Queryable, id uuid.UUID) {
	if _, err := db.Exec(`DELETE FROM transcode_target WHERE id=$1`, id); err != nil {
		log.Fatalf("Failed to delete target (ID=%s): %v\n", id, err)
	}
}
