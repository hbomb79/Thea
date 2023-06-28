package transcode

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	Store struct{ db *gorm.DB }
)

func (store *Store) SaveTranscode(*TranscodeTask) error { return errors.New("Not yet implemented") }
func (store *Store) GetAll() ([]*TranscodeTask, error)  { return nil, errors.New("Not yet implemented") }
func (store *Store) GetForMedia(mediaId uuid.UUID) ([]*TranscodeTask, error) {
	return nil, errors.New("Not yet implemented")
}
