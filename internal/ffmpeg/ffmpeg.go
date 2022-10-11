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
	"syscall"

	"github.com/floostack/transcoder"
	"github.com/floostack/transcoder/ffmpeg"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/hbomb79/Thea/internal/queue"
)

type FfmpegCmd interface {
	// Run will attempt to construct and start an FFmpeg command
	// on the host machine. Each FFmpeg update detected from the
	// underlying command will be delivered to the callback.
	Run(ProgressCallback, FormatterConfig) error

	Suspend()
	Continue()
}

type cmd struct {
	item    *queue.QueueItem
	profile profile.Profile
	command *exec.Cmd
}

type ProgressCallback func(transcoder.Progress)

func (cmd *cmd) Run(progressCallback ProgressCallback, config FormatterConfig) error {
	ffmpegCfg := &ffmpeg.Config{
		ProgressEnabled: true,
		FfmpegBinPath:   config.FfmpegBinaryPath,
		FfprobeBinPath:  config.FfprobeBinaryPath,
	}

	cmdContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	outputPath := cmd.calculateOutputPath(config)
	os.MkdirAll(filepath.Dir(outputPath), os.ModePerm)

	transcoderInstance := ffmpeg.
		New(ffmpegCfg).
		Input(cmd.item.Path).
		Output(outputPath).
		WithContext(&cmdContext)

	progressChannel, err := transcoderInstance.Start(cmd.profile.Command())
	if err != nil {
		return cmd.parseFfmpegError(err)
	}

	// Store command instance to allow for suspension/resuming from other threads
	cmd.command = transcoderInstance.GetRunningCmdInstance()

	// Listen on progress channel and forward messages to the provided channel
	for {
		prog, ok := <-progressChannel
		if !ok {
			// Progress has closed?
			return nil
		}

		progressCallback(prog)
	}
}

func (cmd *cmd) Suspend() {
	if cmd.command == nil {
		return
	}

	cmd.command.Process.Signal(syscall.SIGTSTP)
}

func (cmd *cmd) Continue() {
	if cmd.command == nil {
		return
	}

	cmd.command.Process.Signal(syscall.SIGCONT)
}

func (cmd *cmd) calculateOutputPath(config FormatterConfig) string {
	outputFormat := config.TargetFormat
	var itemOutputPath string

	if cmd.profile == nil || cmd.profile.Output() == "" {
		itemOutputPath = fmt.Sprintf("%s.%s", cmd.item.TitleInfo.OutputPath(), outputFormat)
		itemOutputPath = filepath.Join(config.OutputPath, itemOutputPath)
	} else {
		itemOutputPath = cmd.profile.Output()
	}

	itemOutputPath = composeCommandArguments(itemOutputPath, cmd.item)
	return itemOutputPath
}

func (cmd *cmd) parseFfmpegError(err error) error {
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

func NewFfmpegCmd(item *queue.QueueItem, profile profile.Profile) FfmpegCmd {
	return &cmd{
		item:    item,
		profile: profile,
	}
}
