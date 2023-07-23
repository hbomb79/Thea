package ffmpeg

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/jmoiron/sqlx"
)

const (
	TargetTable = "transcode_target"
)

type Store struct {
}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Target{})
}

func (store *Store) Save(db *sqlx.DB, target *Target) error {
	_, err := db.NamedExec(`
		INSERT INTO transcode_target(id, label, ffmpeg_options, extension)
		VALUES (:id, :label, :ffmpeg_options, :extension)
		ON CONFLICT(id) DO UPDATE
		SET (label, ffmpeg_options, extension) = (EXCLUDED.label, EXCLUDED.ffmpeg_options, EXCLUDED.extension)
	`, target)

	return err
}

func (store *Store) Get(db *sqlx.DB, id uuid.UUID) *Target {
	var result Target
	err := db.Get(&result, `SELECT * FROM transcode_target WHERE id=$1;`, id)
	if err != nil {
		log.Warnf("Failed to find target (id=%s): %s\n", id, err.Error())
		return nil
	}

	return &result
}

func (store *Store) GetAll(db *sqlx.DB) []*Target {
	var results []*Target
	err := db.Select(&results, `SELECT * FROM transcode_target;`)
	if err != nil {
		log.Fatalf("Failed to fetch all targets: %s\n", err.Error())
		return make([]*Target, 0)
	}

	return results
}

func (store *Store) GetMany(db *sqlx.DB, ids ...uuid.UUID) []*Target {
	query, args, err := sqlx.In(`SELECT * FROM transcode_target WHERE id IN (?);`)
	if err != nil {
		log.Fatalf("Unable to create SELECT .. IN (a,b,c,...) query: %s", err.Error())
		return nil
	}

	db.Rebind(query)

	var results []*Target
	err = db.Select(results, query, args)
	if err != nil {
		log.Fatalf("Failed to batch get targets with IDs=%#v: %s\n", ids, err.Error())
		return nil
	}

	return results
}

func (store *Store) Delete(db *sqlx.DB, id uuid.UUID) {
	_, err := db.NamedExec(`--sql
		DELETE FROM transcode_target WHERE id=$1;`,
		id)
	if err != nil {
		log.Fatalf("Failed to delete target (ID=%s): %s\n", id, err.Error())
	}
}
