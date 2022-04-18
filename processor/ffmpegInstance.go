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
	"github.com/hbomb79/TPA/profile"
)

const DEFAULT_THREADS_REQUIRED int = 1

type ffmpegProgress struct {
	Frames   string
	Elapsed  string
	Bitrate  string
	Progress float64
	Speed    string
}

type ffmpegInstance struct {
	pid         int
	status      CommanderTaskStatus
	progress    *ffmpegProgress
	important   bool
	trouble     Trouble
	cancelChan  chan int
	item        *QueueItem
	profileTag  string
	targetLabel string
}

func (ffmpegI *ffmpegInstance) Start(proc *Processor) error {
	queueItem := ffmpegI.item

	ffmpegCfg := &ffmpeg.Config{
		ProgressEnabled: true,
		FfmpegBinPath:   proc.Config.Format.FfmpegBinaryPath,
		FfprobeBinPath:  proc.Config.Format.FfprobeBinaryPath,
	}

	pIdx, p := proc.Profiles.FindProfileByTag(ffmpegI.profileTag)
	if pIdx == -1 {
		p = proc.Profiles.Profiles()[0]
	}

	t := p.Targets()[0]

	cmdContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	progress, err := ffmpeg.
		New(ffmpegCfg).
		Input(queueItem.Path).
		Output(ffmpegI.GetOutputPath()).
		WithContext(&cmdContext).
		Start(t.FFmpegOptions)

	if err != nil {
		// Try and pick out some relevant information from the HUGE
		// output log from ffmpeg. The error we get contains lots of information
		// about how the binary was compiled... this is useless info, we just
		// want the 'message' JSON that is encoded inside.
		messageMatcher := regexp.MustCompile(`(?s)message: ({.*})`)
		groups := messageMatcher.FindStringSubmatch(err.Error())
		if messageMatcher == nil {
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

	// Progress listener. Automatically cancels the above context
	// when ffmpeg execution is complete
	go func() {
		for v := range progress {
			ffmpegI.progress = &ffmpegProgress{
				v.GetCurrentBitrate(),
				v.GetCurrentTime(),
				v.GetCurrentBitrate(),
				v.GetProgress(),
				v.GetSpeed(),
			}
		}

		// FFmpeg completed execution
		ffmpegI.Stop()
	}()

	// Wait for cancel signal either due to completion
	// of ffmpeg, or manual cancellation from the user
	// Cancellation of the context is deferred to function
	// return
	<-ffmpegI.cancelChan

	return nil
}

func (ffmpegI *ffmpegInstance) ThreadsRequired() int {
	threads := ffmpegI.getTargetInstance().FFmpegOptions.Threads
	if threads == nil {
		return DEFAULT_THREADS_REQUIRED
	} else {
		return *threads
	}
}

func (ffmpegI *ffmpegInstance) Stop() {
	if ffmpegI.status != WORKING {
		// Can't cancel something that isn't happening. We should
		// ignore this request.
		fmt.Printf("[Commander] (!) Ignoring request to cancel ffmpeg instance %v\ninstance has incorrect state {%v}\n", ffmpegI, ffmpegI.status)
		return
	}

	// Non-blocking cancel request. We use a non-blocking select here
	// because the instance may be cancelled already when we call this
	// if we're unlucky enough to experience a race condition to cancel
	select {
	case ffmpegI.cancelChan <- 1:
		fmt.Printf("[Commander] (X) Cancelled ffmpeg instance %v\n", ffmpegI)
	default:
		fmt.Printf("[Commander] (!) Failed to cancel ffmpeg instance %v\nInstance may already be closed\n", ffmpegI)
	}
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
	"OUTPUT_PATH",
}

func (ffmpegI *ffmpegInstance) composeCommandArguments(sourceCommand string) string {
	getVal := func(command string) string {
		item := ffmpegI.item
		switch command {
		case "%DEFAULT_TARGET_EXTENSION%":
			return "mp4"
		case "%DEFAULT_THREAD_COUNT%":
			return "1"
		case "%DEFAULT_OUTPUT_DIR%":
			return "/"
		case "%TITLE%":
			return item.OmdbInfo.Title
		case "%RESOLUTION%":
			return item.TitleInfo.Resolution
		case "%HOME_DIRECTORY%":
			return ""
		case "%SEASON_NUMBER%":
			return fmt.Sprint(item.TitleInfo.Season)
		case "%EPISODE_NUMBER%":
			return fmt.Sprint(item.TitleInfo.Episode)
		case "%SOURCE_PATH%":
			return item.Path
		case "%OUTPUT_PATH%":
			return item.TitleInfo.OutputPath()
		default:
			fmt.Printf("[Commander] (!) Encountered unknown command substitution '%s' in source command '%s'\n", command, sourceCommand)
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
		Progress    *ffmpegProgress
		Status      CommanderTaskStatus
		Trouble     Trouble
		ItemId      int
		ProfileTag  string
		TargetLabel string
	}{
		ffmpegI.progress,
		ffmpegI.status,
		ffmpegI.trouble,
		ffmpegI.item.ItemID,
		ffmpegI.profileTag,
		ffmpegI.targetLabel,
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

func (ffmpegI *ffmpegInstance) ProfileTag() string {
	return ffmpegI.profileTag
}

func (ffmpegI *ffmpegInstance) TargetLabel() string {
	return ffmpegI.targetLabel
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
	targetInstance := ffmpegI.getTargetInstance()
	var itemOutputPath string
	if targetInstance == nil {
		itemOutputPath = fmt.Sprintf("%s.%s", ffmpegI.item.TitleInfo.OutputPath(), outputFormat)
		itemOutputPath = filepath.Join(ffmpegI.item.processor.Config.Format.OutputPath, itemOutputPath)
	} else {
		itemOutputPath = targetInstance.OutputPath
	}

	itemOutputPath = ffmpegI.composeCommandArguments(itemOutputPath)
	return itemOutputPath
}

func (ffmpegI *ffmpegInstance) getProfileInstance() profile.Profile {
	_, profile := ffmpegI.item.processor.Profiles.FindProfileByTag(ffmpegI.profileTag)

	return profile
}

func (ffmpegI *ffmpegInstance) getTargetInstance() *profile.Target {
	profile := ffmpegI.getProfileInstance()
	if profile == nil {
		return nil
	}

	return profile.FindTarget(ffmpegI.targetLabel)
}

func newFfmpegInstance(item *QueueItem, profileTag string, targetLabel string) CommanderTask {
	return &ffmpegInstance{
		item:        item,
		profileTag:  profileTag,
		targetLabel: targetLabel,
		pid:         -1,
		status:      PENDING,
	}
}
