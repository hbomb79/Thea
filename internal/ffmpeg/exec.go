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
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/mitchellh/go-homedir"
)

var log = logger.Get("FFmpeg")

type Config struct {
	FfmpegBinPath       string
	FfprobeBinPath      string
	OutputBaseDirectory string
}

func (config *Config) GetOutputBaseDirectory() string {
	out, err := homedir.Expand(config.OutputBaseDirectory)
	if err != nil {
		log.Errorf("Failed to expand transcode base output path (%s): %v {will use provided path un-expanded}\n", config.OutputBaseDirectory, err)
		return config.OutputBaseDirectory
	}

	return out
}

type Progress struct {
	FramesProcessed string
	CurrentTime     string
	CurrentBitrate  string
	Progress        float64
	Speed           string
}

type TranscodeCmd struct {
	inputPath       string
	outputPath      string
	transcodeConfig Config
	runningCommand  *exec.Cmd
}

func NewCmd(input string, output string, config Config) *TranscodeCmd {
	return &TranscodeCmd{input, output, config, nil}
}

func (cmd *TranscodeCmd) Run(ctx context.Context, ffmpegConfig transcoder.Options, updateHandler func(*Progress)) error {
	transcoder := ffmpeg.
		New(&ffmpeg.Config{
			ProgressEnabled: true,
			FfmpegBinPath:   cmd.transcodeConfig.FfmpegBinPath,
			FfprobeBinPath:  cmd.transcodeConfig.FfprobeBinPath,
		}).
		Input(cmd.inputPath).
		Output(cmd.outputPath).
		WithContext(&ctx)

	if err := os.MkdirAll(filepath.Dir(cmd.outputPath), os.ModeDir); err != nil {
		return err
	}

	progressChannel, err := transcoder.Start(ffmpegConfig)
	if err != nil {
		return ParseFfmpegError(err)
	}

	cmd.runningCommand = transcoder.GetRunningCmdInstance()

	for {
		prog, ok := <-progressChannel
		if !ok {
			log.Emit(logger.DEBUG, "FFmpeg command has closed progress channel... closing report channel\n")
			return nil
		}

		updateHandler(&Progress{
			FramesProcessed: prog.GetFramesProcessed(),
			CurrentTime:     prog.GetCurrentTime(),
			CurrentBitrate:  prog.GetCurrentBitrate(),
			Progress:        prog.GetProgress(),
			Speed:           prog.GetSpeed(),
		})
	}
}

func (cmd *TranscodeCmd) Suspend() error {
	if cmd.runningCommand == nil {
		return fmt.Errorf("cannot suspend FFmpeg instance %v because command is not intialised", cmd)
	}

	return cmd.runningCommand.Process.Signal(syscall.SIGTSTP)
}

func (cmd *TranscodeCmd) Continue() error {
	if cmd.runningCommand == nil {
		return fmt.Errorf("cannot continue FFmpeg instance %v because command is not initialised", cmd)
	}

	return cmd.runningCommand.Process.Signal(syscall.SIGCONT)
}

func (cmd *TranscodeCmd) RunningCommand() *exec.Cmd {
	return cmd.runningCommand
}

func (cmd *TranscodeCmd) InputPath() string {
	return cmd.inputPath
}

func (cmd *TranscodeCmd) OutputPath() string {
	return cmd.outputPath
}

func (cmd *TranscodeCmd) String() string {
	pid := -1
	if cmd.runningCommand != nil {
		pid = cmd.runningCommand.Process.Pid
	}

	return fmt.Sprintf("{ffmpeg pid=%d | in_path=%s | out_path = %s}", pid, cmd.inputPath, cmd.outputPath)
}

func ParseFfmpegError(err error) error {
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
	errMap, ok := out["error"].(map[string]interface{})
	if !ok {
		return errors.New(groups[1])
	}

	errMsg, ok := errMap["string"].(string)
	if !ok {
		return errors.New(groups[1])
	}

	return fmt.Errorf("FFmpeg transcoding failed: %s", errMsg)
}
