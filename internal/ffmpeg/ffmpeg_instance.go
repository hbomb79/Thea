package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/internal/profile"
	"github.com/hbomb79/TPA/internal/queue"
	"github.com/hbomb79/TPA/pkg/logger"
)

var ffmpegLogger = logger.Get("FFMPEG")

const DEFAULT_THREADS_REQUIRED int = 1

type ffmpegProgress struct {
	Frames   string
	Elapsed  string
	Bitrate  string
	Progress float64
	Speed    string
}

type ffmpegInstance struct {
	execCmd             *exec.Cmd
	status              CommanderTaskStatus
	progress            *ffmpegProgress
	important           bool
	trouble             queue.Trouble
	cancelChan          chan bool
	troubleResolvedChan chan bool
	item                *queue.QueueItem
	profile             profile.Profile
	config              FormatterConfig
	paused              bool
}

// Start manages this ffmpeg instance by capturing any errors, handling troubled states, and
// directly executing the ffmpeg transcode.
func (ffmpegI *ffmpegInstance) Start(config FormatterConfig) {
	ffmpegI.config = config

	ffmpegLogger.Emit(logger.INFO, "Starting instance %s\n", ffmpegI)
	for {
		if ffmpegI.trouble == nil {
			err := ffmpegI.beginTranscode()
			if err != nil {
				ffmpegLogger.Emit(logger.ERROR, "FFMPEG instance (%s) error detected: %s\n", ffmpegI, err.Error())
				ffmpegI.raiseTrouble(&queue.FormatTaskError{BaseTaskError: queue.NewBaseTaskError(err.Error(), ffmpegI.item, queue.FFMPEG_FAILURE) /*, ffmpegI*/})
			} else {
				// Success or cancelled
				return
			}
		} else {
			// Wait for trouble to be resolved
			ffmpegLogger.Emit(logger.WARNING, "FFMPEG instance (%s) waiting for trouble resolution\n", ffmpegI)
			_, ok := <-ffmpegI.troubleResolvedChan
			if !ok {
				return
			}
		}
	}
}

func (ffmpegI *ffmpegInstance) String() string {
	return fmt.Sprintf("{pid=%v itemID=%v status=%v profileTag=%v}",
		ffmpegI.getProcessID(),
		ffmpegI.item.ID,
		ffmpegI.status,
		ffmpegI.profile)
}

func (ffmpegI *ffmpegInstance) ThreadsRequired() int {
	profile := ffmpegI.profile
	if profile == nil || profile.Command().Threads == nil {
		return DEFAULT_THREADS_REQUIRED
	} else {
		return *profile.Command().Threads
	}
}

func (ffmpegI *ffmpegInstance) Stop() {
	if ffmpegI.status == CANCELLED {
		ffmpegLogger.Emit(logger.WARNING, "Ignoring request to cancel FFmpeg instance %s as it's already status is already CANCELLED!\n", ffmpegI)

		return
	}

	close(ffmpegI.troubleResolvedChan)
	close(ffmpegI.cancelChan)

	ffmpegI.SetStatus(CANCELLED)
	ffmpegLogger.Emit(logger.STOP, "FFmpeg instance %s cancelled\n", ffmpegI)
}

var FFMPEG_COMMAND_SUBSTITUTIONS []string = []string{
	"DEFAULT_TARGET_EXTENSION",
	"DEFAULT_THREAD_COUNT",
	"DEFAULT_OUTPUT_DIR",
	"TITLE",
	"RESOLUTION",
	"HOME_DIRECTORY",
	"SEASON_NUMBER",
	"EPISODE_NUMBER",
	"SOURCE_PATH",
	"SOURCE_TITLE",
	"SOURCE_FILENAME",
	"OUTPUT_PATH",
}

func (ffmpegI *ffmpegInstance) composeCommandArguments(sourceCommand string) string {
	getVal := func(command string) string {
		item := ffmpegI.item
		switch command {
		case "DEFAULT_TARGET_EXTENSION":
			return "mp4"
		case "DEFAULT_THREAD_COUNT":
			return "1"
		case "DEFAULT_OUTPUT_DIR":
			return "/"
		case "TITLE":
			return item.OmdbInfo.Title
		case "RESOLUTION":
			return item.TitleInfo.Resolution
		case "HOME_DIRECTORY":
			return ""
		case "SEASON_NUMBER":
			return fmt.Sprint(item.TitleInfo.Season)
		case "EPISODE_NUMBER":
			return fmt.Sprint(item.TitleInfo.Episode)
		case "SOURCE_PATH":
			return item.Path
		case "SOURCE_TITLE":
			return item.TitleInfo.Title
		case "SOURCE_FILENAME":
			return item.Name
		case "OUTPUT_PATH":
			return item.TitleInfo.OutputPath()
		default:
			ffmpegLogger.Emit(logger.WARNING, "Encountered unknown command substitution '%s' in source command '%s'\n", command, sourceCommand)
			return command
		}
	}

	for _, commandSub := range FFMPEG_COMMAND_SUBSTITUTIONS {
		sourceCommand = strings.ReplaceAll(
			sourceCommand,
			fmt.Sprintf("%%%s%%", commandSub),
			getVal(commandSub),
		)
	}

	return sourceCommand
}

func (ffmpegI *ffmpegInstance) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Progress   *ffmpegProgress
		Status     CommanderTaskStatus
		Trouble    queue.Trouble
		ItemId     int
		ProfileTag string
	}{
		ffmpegI.progress,
		ffmpegI.status,
		ffmpegI.trouble,
		ffmpegI.item.ItemID,
		ffmpegI.profile.Tag(),
	})
}

func (ffmpegI *ffmpegInstance) Important() bool {
	return ffmpegI.important
}

func (ffmpegI *ffmpegInstance) Item() *queue.QueueItem {
	return ffmpegI.item
}

func (ffmpegI *ffmpegInstance) Trouble() queue.Trouble {
	return ffmpegI.trouble
}

func (ffmpegI *ffmpegInstance) ResolveTrouble(args map[string]interface{}) error {
	const ERR_FMT = "unable to resolve FFmpeg task error - %v"
	tr := ffmpegI.trouble
	if _, ok := tr.(*queue.FormatTaskError); !ok {
		return fmt.Errorf("cannot resolve trouble %v: trouble expected to be a FormatTaskError, got %T", tr, tr)
	}

	if v, ok := args["profileTag"]; ok {
		ffmpegI.SetProfileTag(v.(string))
	} else if v, ok := args["action"]; ok {
		val := v.(string)
		if val == "retry" {
			// Do nothing, a retry will occur if execution reaches the end of this function.
		} else if val == "pause" {
			ffmpegI.SetPaused(!ffmpegI.paused)
		} else if val == "cancel" {
			return fmt.Errorf(ERR_FMT, "'cancel' action not yet implemented!")
		} else {
			return fmt.Errorf(ERR_FMT, "'action' accepts one of [retry, cancel, pause] as it's value")
		}
	} else {
		return fmt.Errorf(ERR_FMT, "no valid resolution was found, expected profileTag, or an action containing one of [retry, cancel, pause]")
	}

	// Unset the trouble as the resolution above didn't find any problems with the payload
	ffmpegI.trouble = nil

	select {
	case ffmpegI.troubleResolvedChan <- true:
	default:
		ffmpegLogger.Emit(logger.WARNING, "Trouble resolution channel send on ffmpeg instance %s was blocked/ignored!\n", ffmpegI)
	}

	return nil
}

func (ffmpegI *ffmpegInstance) ProfileTag() string {
	return ffmpegI.profile.Tag()
}

func (ffmpegI *ffmpegInstance) Progress() interface{} {
	return ffmpegI.progress
}

func (ffmpegI *ffmpegInstance) Status() CommanderTaskStatus {
	if ffmpegI.paused {
		return PAUSED
	}

	return ffmpegI.status
}

func (ffmpegI *ffmpegInstance) SetPaused(paused bool) {
	if paused == ffmpegI.paused {
		ffmpegLogger.Emit(logger.WARNING, "Ignoring request to change instance %s PAUSED=%v because instance is already in this paused state", ffmpegI, paused)
		return
	}
	ffmpegI.paused = paused

	// If formatting has begun, suspend the process.
	if paused {
		if ffmpegI.status == WORKING {
			ffmpegI.suspendTranscode()
			ffmpegI.status = PAUSED
		}
	} else {
		if ffmpegI.status == PAUSED {
			ffmpegI.resumeTranscode()
			ffmpegI.status = WORKING
		}
	}

}

func (ffmpegI *ffmpegInstance) SetStatus(s CommanderTaskStatus) {
	ffmpegI.status = s
}

func (ffmpegI *ffmpegInstance) SetProfileTag(newProfile string) {
	panic("TODO")
	// ffmpegI.profileTag = newProfile
}

func (ffmpegI *ffmpegInstance) GetOutputPath() string {
	outputFormat := ffmpegI.config.TargetFormat
	var itemOutputPath string

	profile := ffmpegI.profile
	if profile == nil || profile.Output() == "" {
		itemOutputPath = fmt.Sprintf("%s.%s", ffmpegI.item.TitleInfo.OutputPath(), outputFormat)
		itemOutputPath = filepath.Join(ffmpegI.config.OutputPath, itemOutputPath)
	} else {
		itemOutputPath = profile.Output()
	}

	itemOutputPath = ffmpegI.composeCommandArguments(itemOutputPath)
	return itemOutputPath
}

func (ffmpegI *ffmpegInstance) suspendTranscode() {
	ffmpegI.execCmd.Process.Signal(os.Interrupt)
}

func (ffmpegI *ffmpegInstance) resumeTranscode() {
	ffmpegI.execCmd.Process.Signal(syscall.SIGCONT)
}

func (ffmpegI *ffmpegInstance) beginTranscode() error {
	ffmpegI.SetStatus(WORKING)

	config := ffmpegI.config
	ffmpegCfg := &ffmpeg.Config{
		ProgressEnabled: true,
		FfmpegBinPath:   config.FfmpegBinaryPath,
		FfprobeBinPath:  config.FfprobeBinaryPath,
	}

	cmdContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	outputPath := ffmpegI.GetOutputPath()
	os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)

	transcoderInstance := ffmpeg.
		New(ffmpegCfg).
		Input(ffmpegI.item.Path).
		Output(outputPath).
		WithContext(&cmdContext)

	progressChannel, err := transcoderInstance.Start(ffmpegI.profile.Command())
	if err != nil {
		return ffmpegI.parseFfmpegError(err)
	}

	ffmpegI.execCmd = transcoderInstance.GetRunningCmdInstance()
	for {
		select {
		case v, ok := <-progressChannel:
			if !ok {
				// Progress channel has closed meaning that the
				// ffmpeg command has completed.
				ffmpegLogger.Emit(logger.SUCCESS, "FFmpeg instance (%s) FINISHED\n", ffmpegI)
				ffmpegI.SetStatus(FINISHED)
				return nil
			}

			ffmpegI.progress = &ffmpegProgress{
				v.GetCurrentBitrate(),
				v.GetCurrentTime(),
				v.GetCurrentBitrate(),
				v.GetProgress(),
				v.GetSpeed(),
			}

			ffmpegI.item.NotifyUpdate()
		case <-ffmpegI.cancelChan:
			// Cancel channel is closed/emitted on - doesn't matter, we cancel
			// by immediately returning from this method (which will cancel the
			// context and close the ffmpeg process)
			ffmpegLogger.Emit(logger.STOP, "FFmpeg instance (%s) has been CANCELLED/ABORTED\n", ffmpegI)
			ffmpegI.SetStatus(CANCELLED)
			return nil
		}
	}
}

func (ffmpegI *ffmpegInstance) parseFfmpegError(err error) error {
	// Try and pick out some relevant information from the HUGE
	// output log from ffmpeg. The error we get contains lots of information
	// about how the binary was compiled... this is useless info, we just
	// want the 'message' JSON that is encoded inside.
	messageMatcher := regexp.MustCompile(`(?s)message: ({.*})`)
	groups := messageMatcher.FindStringSubmatch(err.Error())
	if messageMatcher == nil || len(groups) == 0 {
		return err
	}

	// ffmpeg error is returned as a JSON encoded string. Unmarshal so we can extract the
	// error string..
	var out map[string]interface{}
	jsonErr := json.Unmarshal([]byte(groups[1]), &out)
	if jsonErr != nil {
		// We failed to extract the info.. just use the entire string as our error
		return errors.New(groups[1])
	}

	// Extract the exception from this result
	ffmpegException := out["error"].(map[string]interface{})
	return errors.New(ffmpegException["string"].(string))
}

func (ffmpegI *ffmpegInstance) raiseTrouble(t queue.Trouble) {
	ffmpegLogger.Emit(logger.WARNING, "Trouble raised {%v}!\n", t)
	if ffmpegI.trouble != nil {
		ffmpegLogger.Emit(logger.WARNING, "Instance is already troubled, new trouble instance will overwrite!\n")
	}

	ffmpegI.trouble = t
	ffmpegI.SetStatus(TROUBLED)
	ffmpegI.item.NotifyUpdate()
}

func (ffmpegI *ffmpegInstance) getProcessID() int {
	if ffmpegI.execCmd == nil || ffmpegI.execCmd.Process == nil {
		return -1
	}

	return ffmpegI.execCmd.Process.Pid
}

func newFfmpegInstance(item *queue.QueueItem, profile profile.Profile) CommanderTask {
	isPaused := false
	if item.Status == queue.Paused {
		isPaused = true
	}

	return &ffmpegInstance{
		item:                item,
		execCmd:             nil,
		status:              PENDING,
		cancelChan:          make(chan bool),
		troubleResolvedChan: make(chan bool),
		paused:              isPaused,
		profile:             profile,
	}
}
