package transcode

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDuplicate = errors.New("a transcode task already exists for the media/target specified")
)

type (
	Store struct{}

	Transcode struct {
		Id        uuid.UUID `db:"id"`
		MediaID   uuid.UUID `db:"media_id"`
		TargetID  uuid.UUID `db:"transcode_target_id"`
		MediaPath string    `db:"path"`
	}
)

// SaveTranscode inserts a row in to the database which represents the provided transcode task. If an existing
// row which conflicts with this insertion will cause the method to return an error.
func (store *Store) SaveTranscode(db *sqlx.DB, task *TranscodeTask) error {
	// TODO timestamp columns (created_at, updated_at)
	if _, err := db.Exec(`
		INSERT INTO media_transcodes(id, media_id, transcode_target_id, path)
		VALUES ($1, $2, $3, $4)`,
		task.id, task.media.Id(), task.target.ID, task.OutputPath(),
	); err != nil {
		return fmt.Errorf("failed to create transcode row: %w", err)
	}

	log.Emit(logger.SUCCESS, "Successfuly saved transcode %s to db\n", task)
	return nil
}

// GetAll ...
func (store *Store) GetAll(db *sqlx.DB) ([]*Transcode, error) {
	var dest []*Transcode
	if err := db.Select(&dest, `SELECT * FROM media_transcodes`); err != nil {
		log.Warnf("Failed to get all workflows: %v\n", err)
		return nil, fmt.Errorf("failed to select all transcodes: %w", err)
	}

	return dest, nil
}

// Get ...
func (store *Store) Get(db *sqlx.DB, id uuid.UUID) *Transcode {
	dest := &Transcode{}
	if err := db.Get(dest, `SELECT * FROM media_transcodes WHERE id=$1`, id); err != nil {
		log.Warnf("Failed to find transcode with id=%s: %v\n", id, err)
		return nil
	}

	return dest
}

// GetForMedia ...
func (store *Store) GetForMedia(db *sqlx.DB, mediaId uuid.UUID) ([]*Transcode, error) {
	return nil, errors.New("not yet implemented")
}

func (store *Store) GetForMediaAndTarget(db *sqlx.DB, mediaId uuid.UUID, targetId uuid.UUID) (*Transcode, error) {
	dest := &Transcode{}
	if err := db.Get(dest, `
		SELECT * FROM media_transcodes
		WHERE media_id=$1
		  AND transcode_target_id=$2`,
		mediaId, targetId,
	); err != nil {
		return nil, fmt.Errorf("Failed to find transcode for media %s and target %s: %v\n", mediaId, targetId, err)
	}

	return dest, nil
}
