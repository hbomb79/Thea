package workflow

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
	"gorm.io/gorm"
)

type Store struct{}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Workflow{})
}

func (store *Store) Create(db *gorm.DB, workflow *Workflow) error {
	return db.Debug().Create(workflow).Error
}

func (store *Store) Save(db *gorm.DB, workflow *Workflow) error {
	return db.Debug().Save(workflow).Error
}

func (store *Store) Get(db *gorm.DB, id uuid.UUID) *Workflow {
	var workflow Workflow
	if err := db.Where(&Workflow{ID: id}).Preload("Targets").First(&workflow).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to find workflow in DB with ID = %v due to error: %s\n", id, err.Error())
		return nil
	}

	return &workflow
}

func (store *Store) GetAll(db *gorm.DB) []*Workflow {
	workflows := make([]*Workflow, 0)
	if err := db.Preload("Targets").Find(&workflows).Error; err != nil {
		log.Emit(logger.ERROR, "Failed to query for all workflows in DB: %s\n", err.Error())
		return nil
	}

	return workflows
}

func (store *Store) Delete(db *gorm.DB, id uuid.UUID) {
	db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&Workflow{ID: id}).Association("Targets").Clear()
		if err := tx.Delete(&Workflow{ID: id}).Error; err != nil {
			log.Emit(logger.ERROR, "Failed to delete workflow with ID = %v due to error: %s\n", id, err.Error())
		}

		return nil
	})
}
