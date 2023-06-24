package ffmpeg

import (
	"github.com/floostack/transcoder/ffmpeg"
	"github.com/google/uuid"
)

type FfmpegTarget struct{ id uuid.UUID }

func (target *FfmpegTarget) Id() *uuid.UUID { return &target.id }

func (target *FfmpegTarget) TranscodeOptions() *ffmpeg.Options { return nil }

func (target *FfmpegTarget) RequiredThreads() int { return 2 }
