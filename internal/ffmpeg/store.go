package ffmpeg

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
	"gorm.io/gorm"
)

type Store struct {
}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Target{})
}

func (store *Store) Save(db *gorm.DB, target *Target) error {
	return db.Save(target).Error
}

func (store *Store) Get(db *gorm.DB, id uuid.UUID) *Target {
	var result Target
	if err := db.Where(&Target{ID: id}).First(&result).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to find target (id=%s): %s\n", id, err.Error())
		return nil
	}

	return &result
}

func (store *Store) GetAll(db *gorm.DB) []*Target {
	results := make([]*Target, 0)
	if err := db.Find(&results).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to fetch all targets: %s\n", err.Error())
		return make([]*Target, 0)
	}

	return results
}

func (store *Store) GetMany(db *gorm.DB, ids ...uuid.UUID) []*Target {
	return make([]*Target, 0)
}

func (store *Store) Delete(db *gorm.DB, id uuid.UUID) {
	err := db.Delete(&Target{ID: id}).Error
	if err != nil {
		log.Emit(logger.ERROR, "Failed to delete target (id=%s): %s\n", id, err.Error())
	}
}
