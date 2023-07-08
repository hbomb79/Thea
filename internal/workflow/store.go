package workflow

import (
	"errors"

	"github.com/hbomb79/Thea/internal/database"
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

	db.RegisterModels(Workflow{})
	return &Store{db: db}, nil
}

func (store *Store) GetWorkflows() []*Workflow { return make([]*Workflow, 0) }
