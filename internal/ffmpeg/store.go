package ffmpeg

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
)

type Store struct {
	db database.Manager
}

// NewStore uses the provided DB manager to register
// the models that this store defines, before storing
// a reference to the manager for use later when
// performing queries.
//
// Note: The manager provided is expected to NOT be
// connected, and it is expected to have become
// connected before any other store methods are used.
func NewStore(db database.Manager) (*Store, error) {
	if db.GetInstance() != nil {
		return nil, errors.New("database is already connected")
	}

	db.RegisterModels(Target{})
	return &Store{db: db}, nil
}

func (store *Store) Save(target *Target) error {
	return store.db.GetInstance().Save(target).Error
}

func (store *Store) Get(id uuid.UUID) *Target {
	var result Target
	if err := store.db.GetInstance().Where(&Target{ID: id}).First(&result).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to find target (id=%s): %s\n", id, err.Error())
		return nil
	}

	return &result
}

func (store *Store) GetAll() []*Target {
	results := make([]*Target, 0)
	if err := store.db.GetInstance().Find(&results).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to fetch all targets: %s\n", err.Error())
		return make([]*Target, 0)
	}

	return results
}

func (store *Store) Delete(id uuid.UUID) {
	err := store.db.GetInstance().Delete(&Target{ID: id}).Error
	if err != nil {
		log.Emit(logger.ERROR, "Failed to delete target (id=%s): %s\n", id, err.Error())
	}
}
