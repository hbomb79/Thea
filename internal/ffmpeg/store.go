package ffmpeg

import (
	"errors"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
)

type Store struct {
}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Target{})
}

func (store *Store) Save(db database.Goqu, target *Target) error {
	// return db.Save(target).Error
	return errors.New("not yet implemented")
}

func (store *Store) Get(db database.Goqu, id uuid.UUID) *Target {
	return nil
	// var result Target
	// if err := db.Where(&Target{ID: id}).First(&result).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to find target (id=%s): %s\n", id, err.Error())
	// 	return nil
	// }

	// return &result
}

func (store *Store) GetAll(db database.Goqu) []*Target {
	// results := make([]*Target, 0)
	// if err := db.Find(&results).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to fetch all targets: %s\n", err.Error())
	// 	return make([]*Target, 0)
	// }

	// return results
	return nil
}

func (store *Store) GetMany(db database.Goqu, ids ...uuid.UUID) []*Target {
	// if len(ids) == 0 {
	// 	return make([]*Target, 0)
	// }

	// var results []*Target
	// if err := db.Debug().Find(&results, ids).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to get targets with ID = %v due to error: %s\n", ids, err.Error())
	// 	return make([]*Target, 0)
	// }

	// return results
	return nil
}

func (store *Store) Delete(db database.Goqu, id uuid.UUID) {
	// err := db.Delete(&Target{ID: id}).Error
	// if err != nil {
	// 	log.Emit(logger.ERROR, "Failed to delete target (id=%s): %s\n", id, err.Error())
	// }
}
