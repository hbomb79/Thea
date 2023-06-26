package ingest

import (
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
)

type IngestItemState int

const (
	IDLE IngestItemState = iota
	IMPORT_HOLD
	INGESTING
	TROUBLED
)

type IngestItem struct {
	Id      uuid.UUID
	Path    string
	State   IngestItemState
	Trouble *IngestItemTrouble
}

func (item *IngestItem) ResolveTrouble() error { return errors.New("not yet implemented") }
func (item *IngestItem) ingest() error         { return errors.New("not yet implemented") }

func (item *IngestItem) modtimeDiff() (*time.Duration, error) {
	itemInfo, err := os.Stat(item.Path)
	if err != nil {
		return nil, err
	}

	diff := time.Since(itemInfo.ModTime())
	return &diff, nil
}
