package internal

import (
	"github.com/floostack/transcoder/ffmpeg"
	f "github.com/hbomb79/TPA/internal/ffmpeg"
	"github.com/hbomb79/TPA/internal/profile"
	"github.com/hbomb79/TPA/pkg/logger"
)

type GetTroubleDetailsRequest struct{}
type ResolveTroubleRequest struct{}

type CoreService interface {
	GetTroubleDetails()
	ResolveTrouble()
	GetKnownFfmpegOptions() any
	GetFfmpegInstancesForItem(int) []f.CommanderTask
}

func (coreApi *coreService) GetTroubleDetails() {

}

func (coreApi *coreService) ResolveTrouble() {

}

func (service *coreService) GetKnownFfmpegOptions() any {
	return service.knownFfmpegOptions
}

func (service *coreService) GetFfmpegInstancesForItem(itemID int) []f.CommanderTask {
	return service.tpa.ffmpeg().GetInstancesForItem(itemID)
}

type coreService struct {
	tpa                TPA
	knownFfmpegOptions any
}

func NewCoreService(tpa TPA) CoreService {
	opts, err := profile.ToArgsMap(&ffmpeg.Options{})
	if err != nil {
		procLogger.Emit(logger.ERROR, "Failure to get known FFmpeg options as args map!")
	}

	return &coreService{
		tpa:                tpa,
		knownFfmpegOptions: opts,
	}
}
