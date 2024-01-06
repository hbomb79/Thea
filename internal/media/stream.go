package media

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("Stream")

const segmentLength = 10 // Every segment is 10 seconds long for now.

func GenerateHSLPlaylist(container *Container) string {
	content := "#EXTM3U\n" +
		"#EXT-X-PLAYLIST-TYPE:VOD\n" +
		"#EXT-X-VERSION:3\n" +
		"#EXT-X-TARGETDURATION:" + strconv.Itoa(segmentLength) + "\n" +
		"#EXT-X-MEDIA-SEQUENCE:0\n"

	remainingLength := container.DurationSecs()
	segmentIndex := 0

	for remainingLength > 0 {
		currentLength := segmentLength
		if remainingLength < segmentLength {
			currentLength = remainingLength
		}
		content += fmt.Sprintf("#EXTINF: %d,\n", currentLength)
		content += fmt.Sprintf("%d.ts\n", segmentIndex) // TODO: Generate a proper URL for the segment file
		segmentIndex += 1
		remainingLength -= segmentLength
	}

	content += "#EXT-X-ENDLIST\n"
	return content
}

func GenerateHSLSegments(ctx context.Context, media *Container, config ffmpeg.Config) {
	tempDir := "/tmp/"
	segmentOutputDir := tempDir + media.Id().String() + "/"

	if err := os.MkdirAll(segmentOutputDir, os.ModePerm); err != nil {
		log.Emit(logger.ERROR, "Unable to create segments output folder")
	}

	cmd := ffmpeg.NewCmd(media.Source(), segmentOutputDir+"stream.m3u8", config)
	updateHandler := func(prog *ffmpeg.Progress) {
		log.Emit(logger.DEBUG, cmd.RunningCommand().String())
		fmt.Printf("\rStream transcode progress: %d%%", int(prog.Progress))
	}

	outputFormat := "hls"
	segmentFormat := segmentOutputDir + "%d.ts"
	segmentDuration := segmentLength
	hlsPlaylistType := "vod"
	hlsListSize := 0
	preset := "veryfast"

	err := cmd.Run(ctx, ffmpeg.Opts{
		OutputFormat:       &outputFormat,
		HlsSegmentFilename: &segmentFormat,
		HlsSegmentDuration: &segmentDuration,
		HlsPlaylistType:    &hlsPlaylistType,
		HlsListSize:        &hlsListSize,
		Preset:             &preset,
	}, updateHandler)
	log.Emit(logger.DEBUG, cmd.String())

	log.Emit(logger.DEBUG, cmd.RunningCommand().String())

	if err != nil {
		log.Emit(logger.ERROR, err.Error())
	}
}
