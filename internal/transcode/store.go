package transcode

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDuplicate = errors.New("a transcode task already exists for the media/target specified")
)

type (
	Store struct{}

	Transcode struct {
		ID        uuid.UUID `db:"id"`
		MediaID   uuid.UUID `db:"media_id"`
		TargetID  uuid.UUID `db:"transcode_target_id"`
		MediaPath string    `db:"path"`
	}
)

// SaveTranscode inserts a row in to the database which represents the provided transcode task. If an existing
// row which conflicts with this insertion will cause the method to return an error.
func (store *Store) SaveTranscode(db database.Queryable, task *TranscodeTask) error {
	// TODO timestamp columns (created_at, updated_at)
	if _, err := db.Exec(`
		INSERT INTO media_transcodes(id, media_id, transcode_target_id, path)
		VALUES ($1, $2, $3, $4)`,
		task.id, task.media.ID(), task.target.ID, task.OutputPath(),
	); err != nil {
		return fmt.Errorf("failed to create transcode row: %w", err)
	}

	log.Emit(logger.SUCCESS, "Successfuly saved transcode %s to db\n", task)
	return nil
}

// GetAll ...
func (store *Store) GetAll(db database.Queryable) ([]*Transcode, error) {
	var dest []*Transcode
	if err := db.Select(&dest, `SELECT * FROM media_transcodes`); err != nil {
		return nil, fmt.Errorf("failed to select all transcodes: %w", err)
	}

	return dest, nil
}

// Get returns the singular completed transcode which matches the ID provided.
func (store *Store) Get(db database.Queryable, id uuid.UUID) *Transcode {
	dest := &Transcode{}
	if err := db.Get(dest, `SELECT * FROM media_transcodes WHERE id=$1`, id); err != nil {
		log.Warnf("Failed to find transcode with id=%s: %v\n", id, err)
		return nil
	}

	return dest
}

// GetForMedia returns all the saved/completed transcodes associated with the media ID
// provided. This function operates agnostically of the type of the media.
func (store *Store) GetForMedia(db database.Queryable, mediaID uuid.UUID) ([]*Transcode, error) {
	var dest []*Transcode
	if err := db.Select(&dest, `SELECT * FROM media_transcodes WHERE media_id=$1`, mediaID); err != nil {
		return nil, fmt.Errorf("failed query for all transcodes: %w", err)
	}

	return dest, nil
}

// Delete searches for and deletes the transcode with the ID provided. The path for this
// transcode is returned from the DELETE query, allowing file-system cleanup to be performed.
func (store *Store) Delete(db database.Queryable, id uuid.UUID) (string, error) {
	var result string
	if err := db.Get(&result, `DELETE FROM media_transcodes WHERE id=$1 RETURNING path`, id); err != nil {
		return "", err
	}

	return result, nil
}

func (store *Store) GetForMediaAndTarget(db database.Queryable, mediaID uuid.UUID, targetID uuid.UUID) (*Transcode, error) {
	dest := &Transcode{}
	if err := db.Get(dest, `
		SELECT * FROM media_transcodes
		WHERE media_id=$1
		  AND transcode_target_id=$2`,
		mediaID, targetID,
	); err != nil {
		return nil, fmt.Errorf("failed to find transcode for media %s and target %s: %v", mediaID, targetID, err)
	}

	return dest, nil
}

// DeleteForMedias deletes all media transcode row associated
// with any of the given media IDs. The paths of the deleted media
// transcodes are returned to allow for file-system cleanup
func (store *Store) DeleteForMedias(db database.Queryable, mediaIDs []uuid.UUID) ([]string, error) {
	query, args, err := sqlx.In(`
		DELETE FROM media_transcodes
		WHERE media_id IN ($1)
		RETURNING path`, mediaIDs)
	if err != nil {
		return nil, err
	}

	var result []string
	if err := db.Select(&result, db.Rebind(query), args); err != nil {
		return nil, err
	}

	return result, nil
}
