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
	id      uuid.UUID
	path    string
	state   IngestItemState
	trouble *IngestTrouble
}

func (item *IngestItem) ingest() error {
	return errors.New("Not yet implemented")
}

func (item *IngestItem) modtimeDiff() (*time.Duration, error) {
	itemInfo, err := os.Stat(item.path)
	if err != nil {
		return nil, err
	}

	diff := time.Since(itemInfo.ModTime())
	return &diff, nil
}
