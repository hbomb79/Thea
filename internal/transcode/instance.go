package transcode

import (
	"encoding/json"
	"fmt"
	"time"

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
	Start(TranscodeConfig, ProgressChannel)

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
	provider          Thea
	trouble           queue.Trouble
	status            InstanceStatus
	message           string
	profileLabel      string
	itemID            int
	id                uuid.UUID
	command           FfmpegCmd
	retryChan         chan bool
	lastKnownProgress transcoder.Progress
	commandOutputPath string
}

func (instance *ffmpegInstance) Start(config TranscodeConfig, progressReportCallback ProgressChannel) {
	defer close(progressReportCallback)

	if instance.status != WAITING {
		log.Emit(logger.ERROR, "Cannot start instance %v due to invalid status %v, expected WAITING (%v)\n", instance, instance.status, WAITING)
		return
	}

	for {
		if instance.status == TROUBLED {
			// Wait on the trouble channel to emit before re-trying
			log.Emit(logger.INFO, "Instance %v now waiting for trouble resolution on retryChan\n", instance)
			<-instance.retryChan

			instance.status = WAITING
			progressReportCallback <- nil
			time.Sleep(time.Second * 2)
		}

		instance.startAndMonitorFfmpeg(config, progressReportCallback)
		if instance.status == CANCELLED || instance.status == COMPLETE {
			// Done and dusted
			return
		}
	}
}

func (instance *ffmpegInstance) startAndMonitorFfmpeg(config TranscodeConfig, progressChan ProgressChannel) {
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
	proxyReportChannel := instance.createProxyProgressChannel(progressChan)
	ffmpegErr := instance.command.Run(proxyReportChannel, config)
	instance.commandOutputPath = instance.command.GetOutputPath()
	log.Emit(logger.DEBUG, "FFmpeg instance %v has completed with error=%v\n", instance, ffmpegErr)
	if ffmpegErr != nil {
		instance.raiseTrouble(item, ffmpegErr)
		proxyReportChannel <- nil
	} else {
		log.Emit(logger.SUCCESS, "FFmpeg instance %v marked as COMPLETED", instance)
		instance.status = COMPLETE
	}

	// Cleanup command after completion
	instance.command = nil
	instance.lastKnownProgress = nil
}

func (instance *ffmpegInstance) Cancel() {
	log.Emit(logger.STOP, "Cancelling instance %v...\n", instance)
	if instance.command == nil {
		log.Emit(logger.DEBUG, "Cannot cancel instance %v as no command is initialized yet\n", instance)
		return
	}

	// TODO terminate command
	// TODO cleanup partially transcoded output
}

func (instance *ffmpegInstance) Pause()  {}
func (instance *ffmpegInstance) Resume() {}
func (instance *ffmpegInstance) RequiredThreads() (uint32, error) {
	// Load the profile specified by profileLabel
	profile := instance.provider.GetProfileByTag(instance.profileLabel)
	if profile == nil {
		instance.status = CANCELLED
		instance.message = "Automatically cancelled as profile was removed/could not be found"
		return 0, fmt.Errorf("cancelled instance %v because it's profile could not be found", instance)
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
		log.Emit(logger.VERBOSE, "Trouble resolution complete, sending wake-up to instance %v\n", instance)
	default:
		log.Emit(logger.WARNING, "Failed to send wake-up to instance %v after resolving trouble... channel blocked\n", instance)
	}

	return nil
}

func (instance *ffmpegInstance) raiseTrouble(item *queue.Item, err error) {
	instance.status = TROUBLED
	instance.trouble = &queue.FfmpegTaskError{BaseTaskError: queue.NewBaseTaskError(err.Error(), item, queue.FFMPEG_FAILURE)}

	log.Emit(logger.ERROR, "Raised trouble for instance %v - ERR: %s\n", instance, err.Error())
}

func (instance *ffmpegInstance) createProxyProgressChannel(source ProgressChannel) ProgressChannel {
	proxyReportChannel := make(ProgressChannel)
	go func(proxyChan ProgressChannel, forwardChan ProgressChannel) {
		for {
			progress, isOk := <-proxyChan
			if !isOk {
				log.Emit(logger.DEBUG, "FFmpeg instance %v has detected closure of progress channel for command, instance has either completed or crashed\n", instance)
				break
			}

			instance.lastKnownProgress = progress
			if progress != nil {
				// Progress updated, instance is working!
				instance.status = WORKING
			}

			select {
			case forwardChan <- progress:
				log.Emit(logger.VERBOSE, "FFmpeg instance %v forwarded FFmpeg command progress to commander\n", instance)
			default:
				log.Emit(logger.WARNING, "FFmpef instance %v FAILED to forward FFmpeg command progress to commander due to channel blockage\n", instance)
			}
		}
		log.Emit(logger.WARNING, "FFmpeg instance %v progress channel forwarding disconnected!\n", instance)
	}(proxyReportChannel, source)

	return proxyReportChannel
}

func (instance *ffmpegInstance) Status() InstanceStatus { return instance.status }
func (instance *ffmpegInstance) Id() uuid.UUID          { return instance.id }
func (instance *ffmpegInstance) ItemID() int            { return instance.itemID }
func (instance *ffmpegInstance) Profile() string        { return instance.profileLabel }
func (instance *ffmpegInstance) OutputPath() string     { return instance.commandOutputPath }
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
func (instance *ffmpegInstance) String() string {
	var pid int = -1
	if instance.command != nil {
		pid = instance.command.GetProcessID()
	}

	return fmt.Sprintf("{%v Profile=%s ItemID=%d PID=%d}", instance.id, instance.profileLabel, instance.itemID, pid)
}

func NewFfmpegInstance(itemID int, profileLabel string, provider Provider) FfmpegInstance {
	return &ffmpegInstance{
		provider:     provider,
		status:       WAITING,
		profileLabel: profileLabel,
		itemID:       itemID,
		id:           uuid.New(),
		command:      nil,
		retryChan:    make(chan bool),
	}
}
