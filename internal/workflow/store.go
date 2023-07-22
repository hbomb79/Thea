package workflow

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/workflow/match"
)

type Store struct{}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Workflow{}, match.Criteria{})
}

func (store *Store) Create(db database.Goqu, workflow *Workflow) error {
	// return db.Debug().Create(workflow).Error
	return nil
}

func (store *Store) Save(db database.Goqu, workflow *Workflow) error {
	// return db.Debug().Save(workflow).Error
	return nil
}

func (store *Store) Get(db database.Goqu, id uuid.UUID) *Workflow {
	// var workflow Workflow
	// if err := db.Debug().Where(&Workflow{ID: id}).Preload("Targets").Preload("Criteria").First(&workflow).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to find workflow in DB with ID = %v due to error: %s\n", id, err.Error())
	// 	return nil
	// }

	// return &workflow
	return nil
}

func (store *Store) GetAll(db database.Goqu) []*Workflow {
	// var workflows []*Workflow
	// if err := db.Debug().Preload("Criteria").Preload("Targets").Find(&workflows).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to query for all workflows in DB: %s\n", err.Error())
	// 	return nil
	// }

	// return workflows
	return nil
}

func (store *Store) Delete(db database.Goqu, id uuid.UUID) {
	// if err := db.Select(clause.Associations).Delete(&Workflow{ID: id}).Error; err != nil {
	// 	log.Emit(logger.ERROR, "Failed to delete workflow with ID = %v due to error: %s\n", id, err.Error())
	// }
}
