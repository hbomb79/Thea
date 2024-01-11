package stream

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/floostack/transcoder"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/stream/hls"
	"github.com/hbomb79/Thea/internal/stream/utils"
)

type (
	mediaStream struct {
		media           *media.Container
		tasks           map[int]streamTask
		method          StreamMethod
		outputDirectory string
	}

	StreamMethod int
)

const (
	HLS StreamMethod = iota
	DASH
)

func (stream *mediaStream) getFfmpegOptionsForSegmentGeneration(segmentIndex int) (transcoder.Options, error) {
	if stream.method == HLS {
		return hls.GetFfmpegOptionsForSegmentGeneration(stream.media, segmentIndex)
	}
	return nil, nil // TODO Support DASH protocol
}

func (stream *mediaStream) getManifestContent() string {
	if stream.method == HLS {
		return hls.GetStreamManifest(stream.media)
	}
	return "" // TODO Support DASH protocol
}

func (stream *mediaStream) getSegmentFilePath(segmentIndex int) string {
	if stream.method == HLS {
		return filepath.Join(stream.outputDirectory, fmt.Sprintf(hls.SegmentFileFormat, segmentIndex))
	} else {
		return "" // TODO Support DASH protocol
	}
}

func (stream *mediaStream) getSegmentLength() int {
	if stream.method == HLS {
		return hls.SegmentLength
	} else {
		return 0 // TODO Support DASH protocol
	}
}

func (stream *mediaStream) segmentGenerated(segmentIndex int) bool {
	if _, ok := stream.tasks[segmentIndex]; ok {
		return true
	}
	segmentFilePath := stream.getSegmentFilePath(segmentIndex)
	return utils.FileExists(segmentFilePath)
}

// return two segment indices that come before and after the specified segment index
func (stream *mediaStream) getSiblingIndicesOfSegmentIndex(segmentIndex int) siblingIndices {
	before := 0
	after := math.MaxInt

	if len(stream.tasks) == 0 {
		return siblingIndices{
			before: 0,
			after:  segmentIndex,
		}
	}

	for index, _ := range stream.tasks {
		if index < segmentIndex && index > before {
			before = index
		} else if index > segmentIndex && index < after {
			after = index
		}
	}

	return siblingIndices{
		before: before,
		after:  after,
	}
}

func (method StreamMethod) String() string {
	if method == HLS {
		return "HLS"
	}
	return "DASH"
}
