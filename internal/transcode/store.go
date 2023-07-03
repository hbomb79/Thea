package transcode

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type (
	Store struct{ db *gorm.DB }
)

func NewStore(db database.Manager) (*Store, error) {
	if instance := db.GetInstance(); instance != nil {
		db.RegisterModels(TranscodeTask{})
		return &Store{db: instance}, nil
	}

	return nil, errors.New("database has no available instance")
}

func (store *Store) SaveTranscode(*TranscodeTask) error { return errors.New("not yet implemented") }
func (store *Store) GetAll() ([]*TranscodeTask, error)  { return nil, errors.New("not yet implemented") }
func (store *Store) GetForMedia(mediaId uuid.UUID) ([]*TranscodeTask, error) {
	return nil, errors.New("not yet implemented")
}
