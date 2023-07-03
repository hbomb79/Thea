package workflow

import (
	"errors"

	"github.com/hbomb79/Thea/internal/database"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db database.Manager) (*Store, error) {
	if instance := db.GetInstance(); instance != nil {
		db.RegisterModels(Workflow{})
		return &Store{db: instance}, nil
	}

	return nil, errors.New("database has no available instance")
}

func (store *Store) GetWorkflows() []*Workflow { return make([]*Workflow, 0) }
