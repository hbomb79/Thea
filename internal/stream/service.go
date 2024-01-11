package stream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/internal/stream/hls"
	"github.com/hbomb79/Thea/internal/stream/utils"
	"github.com/hbomb79/Thea/pkg/logger"
)

var log = logger.Get("StreamService")

type (
	// streamService is Thea's solution to streaming media from the direct source or transcoded on the fly.
	streamService struct {
		streams map[string]mediaStream
		config  StreamConfig

		streamRequested chan streamRequest
		taskProgressed  chan *streamTask
	}

	StreamConfig struct {
		FfmpegBinPath  string
		FfprobeBinPath string
	}

	// streamRequest represent a request for a specific segment for a stream
	streamRequest struct {
		stream       *mediaStream
		segmentIndex int
	}

	siblingIndices struct {
		before int
		after  int
	}
)

func New(config StreamConfig) (*streamService, error) {
	return &streamService{
		config:          config,
		streams:         make(map[string]mediaStream),
		streamRequested: make(chan streamRequest),
		taskProgressed:  make(chan *streamTask),
	}, nil
}

// Run is the main entry point for this service. This method will block
// until the provided context is cancelled.
// Note: when context is cancelled this method will not immediately return as it
// will wait for it's running transcode tasks to cancel.
func (service *streamService) Run(ctx context.Context) error {
	for {
		select {
		case request := <-service.streamRequested:
			if !request.stream.segmentGenerated(request.segmentIndex) {
				service.spawnNewStreamTask(ctx, request.stream, request.segmentIndex)
			}
		// case task := <-service.taskProgressed:
		// closestTask := task.stream.getClosestTaskToSegmentIndex(task.segmentIndex)
		// range := (task.segmentIndex, closestTask.segmentIndex)
		// if task.stream.rangeGenerated(range) {
		//   task.Cancel()
		// }
		case <-ctx.Done():
			// Handle cleaning up task output
			return nil
		}
	}
}

func (service *streamService) spawnNewStreamTask(ctx context.Context, mediaStream *mediaStream, segmentIndex int) error {
	newTask := streamTask{
		stream:       mediaStream,
		segmentIndex: segmentIndex,
		config: ffmpeg.Config{
			FfmpegBinPath:       service.config.FfmpegBinPath,
			FfprobeBinPath:      service.config.FfprobeBinPath,
			OutputBaseDirectory: filepath.Base(mediaStream.outputDirectory),
		},
		cmd: nil,
	}
	mediaStream.tasks[segmentIndex] = newTask
	newTask.Run(ctx, service.taskProgressed)

	log.Emit(logger.INFO, fmt.Sprintf("Spawned a new streak task for %s at index %d\n", mediaStream.media.Id().String(), segmentIndex))

	return nil
}

// GetStreamSegmentContent return content of a segment file on file system that matches the requested segmentIndex
func (service *streamService) GetStreamSegmentContent(media *media.Container, method StreamMethod, segmentIndex int) ([]byte, error) {
	log.Emit(logger.DEBUG, "here")
	// get or create new stream task for media
	stream, err := service.getOrCreateNewStream(media, method)

	if err != nil {
		return nil, err
	}

	// emit stream request
	service.streamRequested <- streamRequest{
		stream:       stream,
		segmentIndex: segmentIndex,
	}
	// // wait for segment file to exist
	segmentFilePath := stream.getSegmentFilePath(segmentIndex)
	utils.WaitForFile(segmentFilePath)

	// return segment file content
	return os.ReadFile(segmentFilePath)
}

// GetStreamManifestContent return the content of the stream manifest file
func (service *streamService) GetStreamManifestContent(media *media.Container, method StreamMethod) (string, error) {
	// get or create new stream for media
	stream, err := service.getOrCreateNewStream(media, method)

	if err != nil {
		return "", err
	}

	// emit stream request
	service.streamRequested <- streamRequest{
		stream:       stream,
		segmentIndex: 0,
	}
	// return stream manifest
	return stream.getManifestContent(), nil
}

func (service *streamService) getOrCreateNewStream(media *media.Container, method StreamMethod) (*mediaStream, error) {
	streamHash := media.Id().String() + method.String()

	outputDirectory, outputDirectoryErr := getStreamOutputPath(media, method)

	if outputDirectoryErr != nil {
		return nil, outputDirectoryErr
	}

	if _, ok := service.streams[streamHash]; !ok {
		service.streams[streamHash] = mediaStream{
			media:           media,
			outputDirectory: outputDirectory,
			method:          method,
			tasks:           make(map[int]streamTask),
		}
	}

	stream := service.streams[streamHash]
	return &stream, nil
}

func getStreamOutputPath(media *media.Container, method StreamMethod) (string, error) {
	if method == HLS {
		return hls.GetStreamOutputPath(media)
	}
	return "", nil
}
