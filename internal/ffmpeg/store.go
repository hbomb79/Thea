package ffmpeg

import (
	"github.com/google/uuid"
)

type Store struct{}

func (store *Store) GetTarget(uuid.UUID) *Target { return nil }
