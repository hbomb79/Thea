package stream

import (
	"context"

	"github.com/hbomb79/Thea/internal/ffmpeg"
)

type (
	// streamTask represent a transcoding process for a specific media begining at a specific time in media
	streamTask struct {
		stream       *mediaStream
		cmd          *ffmpeg.TranscodeCmd
		config       ffmpeg.Config
		segmentIndex int
	}
)

func (task *streamTask) Run(ctx context.Context, outputChan chan *streamTask) error {
	opts, err := task.stream.getFfmpegOptionsForSegmentGeneration(task.segmentIndex)
	if err != nil {
		return err
	}

	updateHandler := func(prog *ffmpeg.Progress) {
		outputChan <- task
	}
	cmd := ffmpeg.NewCmd(task.stream.media.Source(), task.stream.outputDirectory, task.config)
	task.cmd = cmd
	go cmd.Run(ctx, opts, updateHandler)

	return nil
}
