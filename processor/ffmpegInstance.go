package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/TPA/pkg"
	"github.com/hbomb79/TPA/profile"
)

var ffmpegLogger = pkg.Log.GetLogger("FFMPEG", pkg.CORE)

const DEFAULT_THREADS_REQUIRED int = 1

type ffmpegProgress struct {
	Frames   string
	Elapsed  string
	Bitrate  string
	Progress float64
	Speed    string
}

type ffmpegInstance struct {
	pid                 int
	status              CommanderTaskStatus
	progress            *ffmpegProgress
	important           bool
	trouble             Trouble
	cancelChan          chan bool
	troubleResolvedChan chan bool
	item                *QueueItem
	profileTag          string
}

// Start manages this ffmpeg instance by capturing any errors, handling troubled states, and
// directly executing the ffmpeg transcode.
func (ffmpegI *ffmpegInstance) Start(proc *Processor) {
	ffmpegLogger.Emit(pkg.INFO, "Starting instance %s\n", ffmpegI)
	for {
		if ffmpegI.trouble == nil {
			err := ffmpegI.beginTranscode()
			if err != nil {
				ffmpegLogger.Emit(pkg.ERROR, "FFMPEG instance (%s) error detected: %s\n", ffmpegI, err.Error())
				ffmpegI.raiseTrouble(&FormatTaskError{NewBaseTaskError(err.Error(), ffmpegI.item, FFMPEG_FAILURE), ffmpegI})
			} else {
				// Success or cancelled
				return
			}
		} else {
			// Wait for trouble to be resolved
			ffmpegLogger.Emit(pkg.WARNING, "FFMPEG instance (%s) waiting for trouble resolution\n", ffmpegI)
			_, ok := <-ffmpegI.troubleResolvedChan
			if !ok {
				return
			}
		}
	}
}

func (ffmpegI *ffmpegInstance) String() string {
	return fmt.Sprintf("{pid=%v itemID=%v status=%v profileTag=%v trouble=%v}", ffmpegI.pid, ffmpegI.item.ID, ffmpegI.status, ffmpegI.profileTag, ffmpegI.trouble)
}

func (ffmpegI *ffmpegInstance) ThreadsRequired() int {
	threads := ffmpegI.getProfileInstance().Command().Threads
	if threads == nil {
		return DEFAULT_THREADS_REQUIRED
	} else {
		return *threads
	}
}

func (ffmpegI *ffmpegInstance) Stop() {
	if ffmpegI.status == CANCELLED {
		ffmpegLogger.Emit(pkg.WARNING, "Ignoring request to cancel FFmpeg instance %s as it's already status is already CANCELLED!", ffmpegI)

		return
	}

	close(ffmpegI.troubleResolvedChan)
	close(ffmpegI.cancelChan)

	ffmpegI.SetStatus(CANCELLED)
	ffmpegLogger.Emit(pkg.STOP, "FFmpeg instance %s cancelled", ffmpegI)
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
			ffmpegLogger.Emit(pkg.WARNING, "Encountered unknown command substitution '%s' in source command '%s'\n", command, sourceCommand)
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
		Trouble    Trouble
		ItemId     int
		ProfileTag string
	}{
		ffmpegI.progress,
		ffmpegI.status,
		ffmpegI.trouble,
		ffmpegI.item.ItemID,
		ffmpegI.profileTag,
	})
}

func (ffmpegI *ffmpegInstance) Important() bool {
	return ffmpegI.important
}

func (ffmpegI *ffmpegInstance) Item() *QueueItem {
	return ffmpegI.item
}

func (ffmpegI *ffmpegInstance) Trouble() Trouble {
	return ffmpegI.trouble
}

func (ffmpegI *ffmpegInstance) ResolveTrouble(args map[string]interface{}) error {
	tr := ffmpegI.trouble
	if _, ok := tr.(*FormatTaskError); !ok {
		return fmt.Errorf("cannot resolve trouble %v: trouble expected to be a FormatTaskError, got %T", tr, tr)
	}

	if err := tr.Resolve(args); err != nil {
		return fmt.Errorf("cannot resolve trouble %v: %s", tr, err.Error())
	}

	// The trouble resolved! Apply the content of it's resolution context to this instance and then signal
	// the instance that is's okay to continue working.
	res := tr.ResolutionContext()
	if v, ok := res["profileTag"]; v != nil && ok {
		ffmpegI.profileTag = v.(string)
	}

	select {
	case ffmpegI.troubleResolvedChan <- true:
	default:
	}

	return nil
}

func (ffmpegI *ffmpegInstance) ProfileTag() string {
	return ffmpegI.profileTag
}

func (ffmpegI *ffmpegInstance) Progress() interface{} {
	return ffmpegI.progress
}

func (ffmpegI *ffmpegInstance) Status() CommanderTaskStatus {
	return ffmpegI.status
}

func (ffmpegI *ffmpegInstance) SetStatus(s CommanderTaskStatus) {
	ffmpegI.status = s
}

func (ffmpegI *ffmpegInstance) SetProfileTag(string) {
	// ffmpegI.profileTag =
}

func (ffmpegI *ffmpegInstance) GetOutputPath() string {
	outputFormat := ffmpegI.item.processor.Config.Format.TargetFormat
	profile := ffmpegI.getProfileInstance()
	var itemOutputPath string
	if profile == nil || profile.Output() == "" {
		itemOutputPath = fmt.Sprintf("%s.%s", ffmpegI.item.TitleInfo.OutputPath(), outputFormat)
		itemOutputPath = filepath.Join(ffmpegI.item.processor.Config.Format.OutputPath, itemOutputPath)
	} else {
		itemOutputPath = profile.Output()
	}

	itemOutputPath = ffmpegI.composeCommandArguments(itemOutputPath)
	return itemOutputPath
}

func (ffmpegI *ffmpegInstance) getProfileInstance() profile.Profile {
	_, profile := ffmpegI.item.processor.Profiles.FindProfileByTag(ffmpegI.profileTag)

	return profile
}

func (ffmpegI *ffmpegInstance) beginTranscode() error {
	proc := ffmpegI.item.processor
	ffmpegCfg := &ffmpeg.Config{
		ProgressEnabled: true,
		FfmpegBinPath:   proc.Config.Format.FfmpegBinaryPath,
		FfprobeBinPath:  proc.Config.Format.FfprobeBinaryPath,
	}

	pIdx, p := proc.Profiles.FindProfileByTag(ffmpegI.profileTag)
	if pIdx == -1 {
		return fmt.Errorf("ffmpeg instance %s failed to start as the profile tag %s no longer exists", ffmpegI, ffmpegI.profileTag)
	}

	cmdContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	progressChannel, err := ffmpeg.
		New(ffmpegCfg).
		Input(ffmpegI.item.Path).
		Output(ffmpegI.GetOutputPath()).
		WithContext(&cmdContext).
		Start(p.Command())

	if err != nil {
		return ffmpegI.parseFfmpegError(err)
	}

	ffmpegI.SetStatus(WORKING)
	for {
		select {
		case v, ok := <-progressChannel:
			if !ok {
				// Progress channel has closed meaning that the
				// ffmpeg command has completed.
				ffmpegLogger.Emit(pkg.SUCCESS, "FFmpeg instance (%s) FINISHED\n", ffmpegI)
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
			ffmpegLogger.Emit(pkg.STOP, "FFmpeg instance (%s) has been CANCELLED/ABORTED\n", ffmpegI)
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

func (ffmpegI *ffmpegInstance) raiseTrouble(t Trouble) {
	ffmpegLogger.Emit(pkg.WARNING, "Trouble raised {%v}!\n", t)
	if ffmpegI.trouble != nil {
		ffmpegLogger.Emit(pkg.WARNING, "Instance is already troubled, new trouble instance will overwrite!\n")
	}

	ffmpegI.trouble = t
	ffmpegI.SetStatus(TROUBLED)
}

func newFfmpegInstance(item *QueueItem, profileTag string) CommanderTask {
	return &ffmpegInstance{
		item:                item,
		profileTag:          profileTag,
		pid:                 -1,
		status:              PENDING,
		cancelChan:          make(chan bool),
		troubleResolvedChan: make(chan bool),
	}
}
