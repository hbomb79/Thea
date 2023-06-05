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
	"github.com/hbomb79/Thea/pkg/logger"
)

type FfmpegCmd interface {
	// Run will attempt to construct and start an FFmpeg command
	// on the host machine. Each FFmpeg update detected from the
	// underlying command will be delivered to the callback.
	Run(ProgressChannel, FormatterConfig) error

	Suspend()
	Continue()

	GetProcessID() int
}

type cmd struct {
	item       *queue.Item
	profile    profile.Profile
	command    *exec.Cmd
	outputPath string
}

type ProgressChannel chan transcoder.Progress

func (cmd *cmd) Run(progressReportChannel ProgressChannel, config FormatterConfig) error {
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
	cmd.outputPath = outputPath

	// Listen on progress channel and forward messages to the provided channel
	for {
		prog, ok := <-progressChannel
		if !ok {
			// FFmpeg instance has closed, shut the channel
			log.Emit(logger.DEBUG, "FFmpeg command has closed progress channel... closing report channel\n")
			close(progressReportChannel)
			return nil
		}

		progressReportChannel <- prog
	}
}

func (cmd *cmd) Suspend() {
	if cmd.command == nil {
		log.Emit(logger.ERROR, "Cannot suspend FFmpeg instance (for item %v and profile %v) because command is not intialised\n", cmd.item, cmd.profile)
		return
	}

	log.Emit(logger.DEBUG, "Suspending FFmpeg process %v for item %v...\n", cmd.command.Process.Pid, cmd.item)
	cmd.command.Process.Signal(syscall.SIGTSTP)
	log.Emit(logger.SUCCESS, "Suspended FFmpeg process %v for item %v...\n", cmd.command.Process.Pid, cmd.item)
}

func (cmd *cmd) Continue() {
	if cmd.command == nil {
		log.Emit(logger.ERROR, "Cannot continue FFmpeg instance (for item %v and profile %v) because command is not intialised\n", cmd.item, cmd.profile)
		return
	}

	log.Emit(logger.DEBUG, "Resuming FFmpeg process %v for item %v...\n", cmd.command.Process.Pid, cmd.item)
	cmd.command.Process.Signal(syscall.SIGCONT)
	log.Emit(logger.SUCCESS, "Resuming FFmpeg process %v for item %v...\n", cmd.command.Process.Pid, cmd.item)
}

func (cmd *cmd) GetProcessID() int {
	if cmd.command == nil {
		return -1
	}

	return cmd.command.Process.Pid
}

func (cmd *cmd) calculateOutputPath(config FormatterConfig) string {
	outputFormat := config.TargetFormat
	var itemOutputPath string

	if cmd.profile == nil || cmd.profile.Output() == "" {
		itemOutputPath = fmt.Sprintf("%s.%s", cmd.item.TitleInfo.OutputPath(), outputFormat)
		itemOutputPath = filepath.Join(config.OutputPath, itemOutputPath)
	} else {
		itemOutputPath = fmt.Sprintf("%s.%s", cmd.profile.Output(), outputFormat)
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

func NewFfmpegCmd(item *queue.Item, profile profile.Profile) FfmpegCmd {
	return &cmd{
		item:    item,
		profile: profile,
	}
}
