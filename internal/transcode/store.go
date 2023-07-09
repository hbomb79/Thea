package transcode

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type (
	Store struct{}
)

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(TranscodeTask{})
}

func (store *Store) SaveTranscode(db *gorm.DB, task *TranscodeTask) error {
	return errors.New("not yet implemented")
}
func (store *Store) GetAll(db *gorm.DB) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
func (store *Store) Get(db *gorm.DB, id uuid.UUID) *TranscodeTask {
	return nil
}
func (store *Store) GetForMedia(db *gorm.DB, mediaId uuid.UUID) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
