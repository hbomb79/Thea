package ffmpeg

import (
	"fmt"

	"github.com/floostack/transcoder"
	"github.com/floostack/transcoder/ffmpeg"
)

func ProbeFile(path string, probePath string) (transcoder.Metadata, error) {
	transcoder := ffmpeg.New(&ffmpeg.Config{FfprobeBinPath: probePath}).Input(path)
	metadata, err := transcoder.GetMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to extract file metadata information using ffprobe: %v", err)
	}

	return metadata, nil
}
