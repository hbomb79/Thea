package transcode

import (
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
)

type TranscodeWorkflow struct {
	targets    []*ffmpeg.FfmpegTarget
	conditions []any
}

func (workflow *TranscodeWorkflow) IsMediaEligible(*media.MediaContainer) bool {
	return false
}

func (workflow *TranscodeWorkflow) Targets() *[]*ffmpeg.FfmpegTarget {
	return &workflow.targets
}
