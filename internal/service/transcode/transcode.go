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

func NewTranscodeTask(outputDir string, m *media.Container, t *ffmpeg.Target) *TranscodeTask {
	out := fmt.Sprintf("%s/%s.%s", outputDir, "", "")

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

	err := task.command.Run(ctx, task.target.TranscodeOptions(), updateHandler)
	if err != nil {
		return fmt.Errorf("transcode task failed due to command error: %s", err.Error())
	}

	return nil
}

// Cancel will interrupt any running transcode, returning true if it had to do so. False will
// be returned when the cancel request was a no-op (e.g., task was IDLE)
func (task *TranscodeTask) Cancel() bool {
	return false
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
