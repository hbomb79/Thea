package transcodes

import (
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/transcode"
)

func progressToDto(progress *ffmpeg.Progress) *gen.TranscodeTaskProgress {
	return &gen.TranscodeTaskProgress{
		CurrentBitrate:  progress.CurrentBitrate,
		CurrentTime:     progress.CurrentTime,
		FramesProcessed: progress.FramesProcessed,
		Progress:        float32(progress.Progress),
		Speed:           progress.Speed,
	}
}

func statusToDto(status transcode.TranscodeTaskStatus) gen.TranscodeTaskStatus {
	switch status {
	case transcode.WAITING:
		return gen.TranscodeTaskStatusWAITING
	case transcode.WORKING:
		return gen.TranscodeTaskStatusWORKING
	case transcode.SUSPENDED:
		return gen.TranscodeTaskStatusSUSPENDED
	case transcode.CANCELLED:
		return gen.TranscodeTaskStatusCANCELLED
	case transcode.COMPLETE:
		return gen.TranscodeTaskStatusCOMPLETE
	default:
		return gen.TranscodeTaskStatusTROUBLED
	}
}

func NewDtoFromModel(model *transcode.Transcode) gen.TranscodeTask {
	return gen.TranscodeTask{Id: model.ID, MediaId: model.MediaID, TargetId: model.TargetID, OutputPath: model.MediaPath, Status: gen.TranscodeTaskStatusCOMPLETE, Progress: nil}
}

func NewDtoFromTask(model *transcode.TranscodeTask) gen.TranscodeTask {
	return gen.TranscodeTask{
		Id:         model.ID(),
		MediaId:    model.Media().ID(),
		TargetId:   model.Target().ID,
		OutputPath: model.OutputPath(),
		Status:     statusToDto(model.Status()),
		Progress:   progressToDto(model.LastProgress()),
	}
}
