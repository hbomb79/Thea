package workflow

type Store struct{}

func (store *Store) GetWorkflows() []*Workflow { return make([]*Workflow, 0) }
