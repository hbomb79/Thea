package transcode

import (
	"context"
	"errors"
	"fmt"

	"github.com/floostack/transcoder"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
)

type Command interface {
	Run(context.Context, transcoder.Options, func(*ffmpeg.Progress)) error
}

type TranscodeTaskStatus int

const (
	WAITING TranscodeTaskStatus = iota
	WORKING
	SUSPENDED
	TROUBLED
	CANCELLED
	COMPLETE
)

// TranscodeTask represents an active transcode task being processed
// by the TranscodeService. The ID held inside of the item is what
// should be used to retrieve the task item from the service for
// management & monitoring.
type TranscodeTask struct {
	id         uuid.UUID
	config     *ffmpeg.Config
	media      *media.Container
	target     *ffmpeg.Target
	outputPath string

	command      Command
	status       TranscodeTaskStatus
	lastProgress *ffmpeg.Progress
}

func NewTranscodeTask(outputPath string, m *media.Container, t *ffmpeg.Target) *TranscodeTask {
	out := fmt.Sprintf("%s/%s.%s", outputPath, t.Label, t.Ext)

	return &TranscodeTask{
		id:           uuid.New(),
		media:        m,
		target:       t,
		lastProgress: nil,
		outputPath:   out,
		command:      nil,
		status:       WAITING,
	}
}

func (task *TranscodeTask) Run(ctx context.Context, updateHandler func(*ffmpeg.Progress)) error {
	if task.command != nil {
		return errors.New("cannot start transcode task because a command is already set (conflict)")
	}

	task.command = ffmpeg.NewCmd(task.media.Source(), task.outputPath, task.config)
	defer func() {
		task.command = nil
		task.lastProgress = nil
	}()

	task.status = WORKING
	err := task.command.Run(ctx, task.target.FfmpegOptions, updateHandler)
	if err != nil {
		task.status = TROUBLED
		return fmt.Errorf("transcode task failed due to command error: %v", err)
	}

	if ctx.Err() != nil {
		// Task was stopped because the context was cancelled
		task.Cancel()
		return nil
	}

	task.status = COMPLETE
	return nil
}

// Cancel will interrupt any running transcode, cleaning up any partially transcoded output
// if applicable.
func (task *TranscodeTask) Cancel() {
	task.status = CANCELLED
	// TODO cleanup
}

// LastKnownProgress is an accessor function to the latest ffmpeg progress
// from the underlying ffmpeg command.
// If no last progress is available, nil will be returned.
func (task *TranscodeTask) LastProgress() *ffmpeg.Progress { return task.lastProgress }
func (task *TranscodeTask) Id() uuid.UUID                  { return task.id }
func (task *TranscodeTask) Media() *media.Container        { return task.media }
func (task *TranscodeTask) Target() *ffmpeg.Target         { return task.target }
func (task *TranscodeTask) Status() TranscodeTaskStatus    { return task.status }
func (task *TranscodeTask) Trouble() any                   { return nil }
func (task *TranscodeTask) String() string {
	return fmt.Sprintf("Task{ID=%s MediaID=%s TargetID=%s Status=%s}", task.id, task.media.Id(), task.target.ID, task.status)
}

func (s TranscodeTaskStatus) String() string {
	switch s {
	case WAITING:
		return fmt.Sprintf("WAITING[%d]", s)
	case WORKING:
		return fmt.Sprintf("WORKING[%d]", s)
	case SUSPENDED:
		return fmt.Sprintf("SUSPENDED[%d]", s)
	case TROUBLED:
		return fmt.Sprintf("TROUBLED[%d]", s)
	case CANCELLED:
		return fmt.Sprintf("CANCELLED[%d]", s)
	case COMPLETE:
		return fmt.Sprintf("COMPLETE[%d]", s)
	}

	return fmt.Sprintf("UNKNOWN[%d]", s)
}
