package ffmpeg

import (
	"encoding/json"
	"fmt"

	"github.com/floostack/transcoder"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/queue"
	"github.com/hbomb79/Thea/pkg/logger"
)

type InstanceStatus int

const (
	WAITING = iota
	WORKING
	SUSPENDED
	TROUBLED
	CANCELLED
	COMPLETE
)

// FfmpegInstance is responsible for starting, monitoring, and stopping
// an FFmpeg command running on the host machine.
type FfmpegInstance interface {
	// Start begins the execution of this instance. FFmpeg progress updates will cause
	// this instance to send it's ID on the channel provided.
	Start(FormatterConfig, ProgressCallback)

	// Cancel stops the execution of the running FFmpeg task, cleaning up any partially
	// transcoded footage before exiting.
	Cancel()

	// Pause will suspend the background FFmpeg task (if any)
	Pause()

	// Resume will resume the suspended background FFmpeg task (if any)
	Resume()

	// Trouble returns the trouble for this instance (if any)
	Trouble() queue.Trouble

	// ResolveTrouble will attempt to resolve the instances trouble (if any) with the
	// provided payload.
	ResolveTrouble(map[string]any) error

	// Status returns the current status of this instance
	Status() InstanceStatus

	// Id returns the unique ID for this instance
	Id() uuid.UUID

	// ItemID returns the ID of the item that this instance is attached to
	ItemID() int

	// Profile returns the profile tag which will be used to compose the FFmpeg task command
	// when execution begins.
	Profile() string

	// RequiredThreads returns how many threads this FFmpeg task will consume based on the profile
	// configuration - will return an error if the profile specified for this instance
	// cannot be found (this will also cause the instance to become cancelled)
	RequiredThreads() (uint32, error)

	// OutputPath returns the intended output path for the transcoded footage
	OutputPath() string
}

type ffmpegInstance struct {
	provider          Provider
	trouble           queue.Trouble
	status            InstanceStatus
	message           string
	profileLabel      string
	itemID            int
	id                uuid.UUID
	command           FfmpegCmd
	retryChan         chan bool
	lastKnownProgress transcoder.Progress
}

func (instance *ffmpegInstance) Start(config FormatterConfig, progressReportCallback ProgressCallback) {
	for {
		if instance.status == TROUBLED {
			// Wait on the trouble channel to emit before re-trying
			<-instance.retryChan
		}

		instance.tryStart(config, progressReportCallback)
		if instance.status == CANCELLED || instance.status == COMPLETE {
			// Done and dusted
			return
		}

		progressReportCallback(nil)
	}
}

func (instance *ffmpegInstance) tryStart(config FormatterConfig, progressReportCallback ProgressCallback) {
	if instance.status != WAITING {
		return
	}

	// Load the profile specified by profileLabel
	profile := instance.provider.GetProfileByTag(instance.profileLabel)
	if profile == nil {
		instance.status = CANCELLED
		instance.message = "Automatically cancelled as profile was removed/could not be found"
		return
	}

	// Get the item we're working with
	item, err := instance.provider.GetItem(instance.itemID)
	if err != nil {
		instance.status = CANCELLED
		instance.message = "Automatically cancelled as item cannot be found"
		return
	}

	// Construct our FFmpeg command using this profile
	instance.command = NewFfmpegCmd(item, profile)

	// Start the command, providing a callback for progress notifications
	wrappedProgressReportCallback := func(prog transcoder.Progress) {
		instance.lastKnownProgress = prog
		progressReportCallback(prog)
	}

	ffmpegErr := instance.command.Run(wrappedProgressReportCallback, config)
	if ffmpegErr != nil {
		instance.raiseTrouble(item, ffmpegErr)
	} else {
		instance.status = COMPLETE
	}
}

func (instance *ffmpegInstance) Cancel() {
}

func (instance *ffmpegInstance) Pause()  {}
func (instance *ffmpegInstance) Resume() {}
func (instance *ffmpegInstance) RequiredThreads() (uint32, error) {
	// Load the profile specified by profileLabel
	profile := instance.provider.GetProfileByTag(instance.profileLabel)
	if profile == nil {
		instance.status = CANCELLED
		instance.message = "Automatically cancelled as profile was removed/could not be found"
		return 0, fmt.Errorf("cancelled instance %v because it's profile could not be found", instance.id)
	}

	if t := profile.Command().Threads; t != nil && *t >= 0 {
		return uint32(*t), nil
	}

	return DEFAULT_THREADS_REQUIRED, nil
}

func (instance *ffmpegInstance) ResolveTrouble(payload map[string]any) error {
	if instance.trouble == nil {
		return nil
	}

	if err := instance.trouble.Resolve(payload); err != nil {
		return err
	}

	resolution := instance.trouble.ResolutionContext()
	if v, ok := resolution["cancel"]; v == true && ok {
		instance.status = CANCELLED
	} else if v, ok := resolution["retry"]; v == true && ok {
		instance.status = WAITING
	}

	// Unblocking send to inform the Start event loop to retry
	// this instance now that it's been resolved
	select {
	case instance.retryChan <- true:
	default:
	}

	return nil
}

func (instance *ffmpegInstance) raiseTrouble(item *queue.QueueItem, err error) {
	instance.status = TROUBLED
	instance.trouble = &queue.FfmpegTaskError{BaseTaskError: queue.NewBaseTaskError(err.Error(), item, queue.FFMPEG_FAILURE)}

	commanderLogger.Emit(logger.ERROR, "Instance %v ERR: %s\n", instance.id, err.Error())
}

func (instance *ffmpegInstance) Status() InstanceStatus { return instance.status }
func (instance *ffmpegInstance) Id() uuid.UUID          { return instance.id }
func (instance *ffmpegInstance) ItemID() int            { return instance.itemID }
func (instance *ffmpegInstance) Profile() string        { return instance.profileLabel }
func (instance *ffmpegInstance) OutputPath() string     { return "" }
func (instance *ffmpegInstance) Trouble() queue.Trouble { return instance.trouble }

func (instance *ffmpegInstance) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Id       uuid.UUID         `json:"id"`
		Progress *InstanceProgress `json:"progress"`
		Status   InstanceStatus    `json:"status"`
		Trouble  queue.Trouble     `json:"trouble"`
	}{
		instance.id,
		instance.GetLastKnownProgressForInstance(),
		instance.status,
		instance.trouble,
	})
}

type InstanceProgress struct {
	Frames   string
	Elapsed  string
	Bitrate  string
	Progress float64
	Speed    string
}

func (instance *ffmpegInstance) GetLastKnownProgressForInstance() *InstanceProgress {
	if instance.lastKnownProgress != nil {
		return &InstanceProgress{
			instance.lastKnownProgress.GetFramesProcessed(),
			instance.lastKnownProgress.GetCurrentTime(),
			instance.lastKnownProgress.GetCurrentBitrate(),
			instance.lastKnownProgress.GetProgress(),
			instance.lastKnownProgress.GetSpeed(),
		}
	} else {
		return nil
	}
}

func NewFfmpegInstance(itemID int, profileLabel string, provider Provider) FfmpegInstance {
	return &ffmpegInstance{
		provider:     provider,
		status:       WAITING,
		profileLabel: profileLabel,
		itemID:       itemID,
		id:           uuid.New(),
		command:      nil,
	}
}
