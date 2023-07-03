package ffmpeg

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db database.Manager) (*Store, error) {
	if instance := db.GetInstance(); instance != nil {
		db.RegisterModels(Target{})
		return &Store{db: instance}, nil
	}

	return nil, errors.New("database has no available instance")
}
func (store *Store) GetTarget(uuid.UUID) *Target { return nil }
