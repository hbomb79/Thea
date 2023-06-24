package transcode

import (
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
)

type TranscodeWorkflow struct {
	targets    []*ffmpeg.Target
	conditions []any
}

func (workflow *TranscodeWorkflow) IsMediaEligible(*media.Container) bool {
	return false
}

func (workflow *TranscodeWorkflow) Targets() *[]*ffmpeg.Target {
	return &workflow.targets
}
