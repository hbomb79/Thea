package transcode

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type (
	Store struct{}
)

func (store *Store) SaveTranscode(db *sqlx.DB, task *TranscodeTask) error {
	return errors.New("not yet implemented")
}
func (store *Store) GetAll(db *sqlx.DB) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
func (store *Store) Get(db *sqlx.DB, id uuid.UUID) *TranscodeTask {
	return nil
}
func (store *Store) GetForMedia(db *sqlx.DB, mediaId uuid.UUID) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
