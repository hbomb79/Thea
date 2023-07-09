package workflow

import (
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type Store struct{}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Workflow{})
}

func (store *Store) Save(db *gorm.DB, workflow *Workflow) error { return nil }
func (store *Store) Get(db *gorm.DB, id uuid.UUID) *Workflow    { return nil }
func (store *Store) GetAll(db *gorm.DB) []*Workflow             { return make([]*Workflow, 0) }
func (store *Store) Delete(db *gorm.DB, id uuid.UUID)           {}
