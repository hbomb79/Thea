package hls

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/stream/utils"
)

const (
	SegmentLength     = 5 // Every segment is 5 seconds long for now.
	SegmentFileFormat = "%d.ts"
)

func GetStreamManifest(media *media.Container) string {
	content := "#EXTM3U\n" +
		"#EXT-X-PLAYLIST-TYPE:VOD\n" +
		"#EXT-X-VERSION:3\n" +
		"#EXT-X-TARGETDURATION:" + strconv.Itoa(SegmentLength) + "\n" +
		"#EXT-X-MEDIA-SEQUENCE:0\n"

	remainingLength := media.DurationSecs()
	segmentIndex := 0

	for remainingLength > 0 {
		currentLength := min(SegmentLength, remainingLength)
		content += fmt.Sprintf("#EXTINF: %d,\n", currentLength)
		content += fmt.Sprintf(SegmentFileFormat+"\n", segmentIndex)
		segmentIndex += 1
		remainingLength -= SegmentLength
	}

	content += "#EXT-X-ENDLIST\n"
	return content
}

func GetStreamOutputPath(media *media.Container) (string, error) {
	return utils.EnsureOutputDirectoryExists(media.Id().String(), "HLS")
}

func GetFfmpegOptionsForSegmentGeneration(media *media.Container, segmentIndex int) (*ffmpeg.Opts, error) {
	outputDir, outputDirErr := utils.EnsureOutputDirectoryExists(media.Id().String(), "HLS")

	if outputDirErr != nil {
		return nil, outputDirErr
	}

	outputFormat := "hls"
	segmentFormat := filepath.Join(outputDir, SegmentFileFormat)
	segmentDuration := SegmentLength
	hlsPlaylistType := "vod"
	hlsListSize := 0
	preset := "veryfast"
	seekTime := strconv.Itoa(SegmentLength * segmentIndex)

	extraArgs := make(map[string]interface{})
	extraArgs["-start_number"] = segmentIndex
	extraArgs["-segment_start_number"] = segmentIndex

	return &ffmpeg.Opts{
		OutputFormat:       &outputFormat,
		HlsSegmentFilename: &segmentFormat,
		HlsSegmentDuration: &segmentDuration,
		HlsPlaylistType:    &hlsPlaylistType,
		HlsListSize:        &hlsListSize,
		Preset:             &preset,
		SeekTime:           &seekTime,
		ExtraArgs:          extraArgs,
	}, nil
}
