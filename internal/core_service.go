package internal

import (
	"github.com/floostack/transcoder/ffmpeg"
	f "github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/pkg/logger"
)

type GetTroubleDetailsRequest struct{}
type ResolveTroubleRequest struct{}

type CoreService interface {
	GetKnownFfmpegOptions() any
	GetFfmpegInstancesForItem(int) []f.FfmpegInstance
}

func (service *coreService) GetKnownFfmpegOptions() any {
	return service.knownFfmpegOptions
}

func (service *coreService) GetFfmpegInstancesForItem(itemID int) []f.FfmpegInstance {
	return service.thea.ffmpeg().GetInstancesForItem(itemID)
}

type coreService struct {
	thea               Thea
	knownFfmpegOptions any
}

func NewCoreService(thea Thea) CoreService {
	opts, err := profile.ToArgsMap(&ffmpeg.Options{})
	if err != nil {
		log.Emit(logger.ERROR, "Failure to get known FFmpeg options as args map!")
	}

	return &coreService{
		thea:               thea,
		knownFfmpegOptions: opts,
	}
}
