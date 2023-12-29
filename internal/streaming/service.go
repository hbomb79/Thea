package streaming

import (
	"fmt"
	"strconv"

	"github.com/hbomb79/Thea/internal/media"
)

type (
	streamingService struct{}
)

func New() (*streamingService, error) {
	return &streamingService{}, nil
}

func (service *streamingService) GenerateHSLPlaylist(container media.Container) string {
	segmentLength := 5 // Every segment is 5 seconds long for now.
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
