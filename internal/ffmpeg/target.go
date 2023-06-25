package ffmpeg

import (
	"github.com/floostack/transcoder/ffmpeg"
	"github.com/google/uuid"
)

type Target struct{ id uuid.UUID }

func (target *Target) Ext() string                       { return "" }
func (target *Target) Label() string                     { return "" }
func (target *Target) Id() uuid.UUID                     { return target.id }
func (target *Target) TranscodeOptions() *ffmpeg.Options { return nil }
func (target *Target) RequiredThreads() int              { return 2 }
