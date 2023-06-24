package ffmpeg

import (
	"fmt"

	"github.com/floostack/transcoder"
	"github.com/floostack/transcoder/ffmpeg"
)

func ProbeFile(path string) (transcoder.Metadata, error) {
	cfg := ffmpeg.Config{}
	transcoder := ffmpeg.New(&cfg).Input(path)
	metadata, err := transcoder.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to extract file metadata information using ffprobe: %s", err.Error())
	}

	return metadata, nil
}
