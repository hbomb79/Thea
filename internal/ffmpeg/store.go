package ffmpeg

import (
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
)

const (
	TargetTable = "transcode_target"
)

type Store struct {
}

func (store *Store) RegisterModels(db database.Manager) {
	db.RegisterModels(Target{})
}

func (store *Store) Save(db database.Goqu, target *Target) error {
	_, err := db.Insert(TargetTable).Rows(target).Executor().Exec()
	return err
}

func (store *Store) Get(db database.Goqu, id uuid.UUID) *Target {
	var result *Target = nil
	found, err := db.From(TargetTable).Where(goqu.C("id").Is(id)).ScanStruct(&result)
	if err != nil {
		log.Fatalf("Failed to find target (id=%v): %s\n", id, err.Error())
		return nil
	}

	if found {
		return result
	}

	return nil
}

func (store *Store) GetAll(db database.Goqu) []*Target {
	var results []*Target
	err := db.From(TargetTable).ScanStructs(&results)
	if err != nil {
		log.Fatalf("Failed to fetch all targets: %s\n", err.Error())
		return make([]*Target, 0)
	}

	return results
}

func (store *Store) GetMany(db database.Goqu, ids ...uuid.UUID) []*Target {
	var results []*Target
	err := db.From(TargetTable).Where(goqu.C("id").In(ids)).ScanStructs(&results)
	if err != nil {
		log.Fatalf("Failed to get targets with IDs=%#v: %s\n", ids, err.Error())
		return nil
	}

	return results
}

func (store *Store) Delete(db database.Goqu, id uuid.UUID) {
	_, err := db.From(TargetTable).Where(goqu.C("id").Is(id)).Delete().Executor().Exec()
	if err != nil {
		log.Fatalf("Failed to delete target (ID=%s): %s\n", id, err.Error())
	}
}
