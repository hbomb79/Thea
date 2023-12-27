package transcode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/floostack/transcoder"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/ffmpeg"
	"github.com/hbomb79/Thea/internal/media"
	"github.com/hbomb79/Thea/pkg/logger"
)

var (
	ErrPathDirectoryCreation         = errors.New("failed to create required directories for FFmpeg specified output path")
	ErrMediaSourceNotFound           = errors.New("provided media references a file which cannot be found/accessed on the file system")
	ErrTargetExtensionInvalid        = errors.New("target provided has an invalid extension")
	ErrTranscodeFinishedWithNoOutput = errors.New("the ffmpeg transcoding seems to have completed, however no output can be found at the expected file path")
	ErrCancelled                     = errors.New("the ffmpeg transcoding was cancelled (via it's context)")
	ErrFfmpegProblem                 = errors.New("FFmpeg transcode failed")
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
	config     ffmpeg.Config
	media      *media.Container
	target     *ffmpeg.Target
	outputPath string

	command      Command
	status       TranscodeTaskStatus
	lastProgress *ffmpeg.Progress

	cancelHandle *context.CancelFunc
}

func NewTranscodeTask(outputPath string, m *media.Container, t *ffmpeg.Target, config ffmpeg.Config) (*TranscodeTask, error) {
	dir := filepath.Join(config.GetOutputBaseDirectory(), m.Id().String(), t.ID.String())
	if err := os.MkdirAll(filepath.Dir(dir), 0777); err != nil {
		log.Errorf("Failed to create required directories (%s) for transcoding output: %v\n", filepath.Dir(dir), err)
		return nil, ErrPathDirectoryCreation
	}

	//TODO: expand this to support other formats, but for now, let's keep it simple
	if t.Ext != "mp4" {
		return nil, ErrTargetExtensionInvalid
	}

	return &TranscodeTask{
		id:           uuid.New(),
		media:        m,
		target:       t,
		lastProgress: nil,
		outputPath:   fmt.Sprintf("%s.%s", dir, t.Ext),
		command:      nil,
		config:       config,
		status:       WAITING,
	}, nil
}

func (task *TranscodeTask) Run(parentCtx context.Context, updateHandler func(*ffmpeg.Progress)) error {
	log.Emit(logger.NEW, "Initializing transcoding pipeline for task %s\n", task)
	if task.command != nil {
		return errors.New("cannot start transcode task because a command is already set (conflict)")
	}

	if _, err := os.Stat(task.media.Source()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrMediaSourceNotFound
		} else {
			return fmt.Errorf("unexpected error when statting media %s source file: %w", task.media, err)
		}
	}

	if _, err := os.Stat(task.outputPath); err == nil {
		// Ensure we clear the output path if there's one present already. (if we're running this task then
		// previous checks to ensure a duplicate transcode entity have been done already, so a duplicate FILE
		// likely indicates some cleanup failed and this file should be considered unwelcome).
		log.Warnf("Transcode %s is expected to output to %s, however a file is already present. Removing file\n", task, task.outputPath)
		os.Remove(task.outputPath)
	}

	task.command = ffmpeg.NewCmd(task.media.Source(), task.outputPath, task.config)
	defer func() {
		task.command = nil
		task.lastProgress = nil
		task.cancelHandle = nil
	}()

	ctx, cancel := context.WithCancel(parentCtx)
	task.cancelHandle = &cancel

	task.status = WORKING
	err := task.command.Run(ctx, task.target.FfmpegOptions, updateHandler)
	if err != nil {
		task.status = TROUBLED
		return fmt.Errorf("%w: %v", ErrFfmpegProblem, err)
	}

	if ctx.Err() != nil {
		// Task was stopped because the context was cancelled,
		log.Infof("Transcode %s was interrupted due to context cancellation (%v). Cleaning up...\n", task, ctx.Err())
		task.status = CANCELLED
		task.cleanup()
		return ErrCancelled
	}

	log.Infof("Transcode %s closed/finished with no error, validating output...\n", task)
	// Before we blindly mark this transcode as completed, we should do some rudimentary checks
	// to ensure the transcode was ACTUALLY as we expected. For now, let's just check if a file exists and
	// is of non-zero size.
	// TODO: store the metadata scraped about this file in the DB, and expose it via the Media interface
	// such that we can assert the runtime of the output matches. This is much more rigorous, but will take
	// a fair bit of work so it's a later-me thing.
	if _, err := os.Stat(task.outputPath); err != nil {
		task.status = TROUBLED
		if errors.Is(err, fs.ErrNotExist) {
			return ErrTranscodeFinishedWithNoOutput
		} else {
			return fmt.Errorf("unexpected error occurred when validation ffmpeg transcode output (path = %s): %w", task.outputPath, err)
		}
	}

	task.status = COMPLETE
	return nil
}

// Cancel will interrupt any running transcode, cleaning up any partially transcoded output
// if applicable.
func (task *TranscodeTask) Cancel() error {
	if task.status != WORKING {
		return fmt.Errorf("only 'WORKING' tasks can be cancelled, this task is of status %s and thus cannot be cancelled", task.status)
	} else if task.cancelHandle == nil {
		return fmt.Errorf("task cannot be cancelled, no context cancel handle is available (this usually indicates the task is not running)")
	}

	(*task.cancelHandle)()
	return nil
}

func (task *TranscodeTask) cleanup() {
	if err := os.Remove(task.outputPath); err != nil {
		log.Errorf("failed to clean-up partially transcoded media after task %s cancellation: %v", task, err)
	}
}

// LastKnownProgress is an accessor function to the latest ffmpeg progress
// from the underlying ffmpeg command.
// If no last progress is available, nil will be returned.
func (task *TranscodeTask) LastProgress() *ffmpeg.Progress { return task.lastProgress }
func (task *TranscodeTask) Id() uuid.UUID                  { return task.id }
func (task *TranscodeTask) Media() *media.Container        { return task.media }
func (task *TranscodeTask) Target() *ffmpeg.Target         { return task.target }
func (task *TranscodeTask) OutputPath() string             { return task.outputPath }
func (task *TranscodeTask) Status() TranscodeTaskStatus    { return task.status }
func (task *TranscodeTask) Trouble() any                   { return nil }
func (task *TranscodeTask) String() string {
	return fmt.Sprintf("Task{ID=%s MediaID=%s TargetID=%s Status=%s OutputPath=%s}", task.id, task.media.Id(), task.target.ID, task.status, task.outputPath)
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
