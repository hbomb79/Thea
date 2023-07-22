package transcode

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
)

type (
	Store struct{}
)

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(TranscodeTask{})
}

func (store *Store) SaveTranscode(db database.Goqu, task *TranscodeTask) error {
	return errors.New("not yet implemented")
}
func (store *Store) GetAll(db database.Goqu) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
func (store *Store) Get(db database.Goqu, id uuid.UUID) *TranscodeTask {
	return nil
}
func (store *Store) GetForMedia(db database.Goqu, mediaId uuid.UUID) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
