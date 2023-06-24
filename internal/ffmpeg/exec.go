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
)

var log = logger.Get("FFmpeg")

type Config struct {
	FfmpegBinPath  string
	FfprobeBinPath string
}

type FfmpegProgress struct {
	FramesProcessed string
	CurrentTime     string
	CurrentBitrate  string
	Progress        float64
	Speed           string
}

type TranscodeCommand struct {
	inputPath       string
	outputPath      string
	transcodeConfig *Config
	runningCommand  *exec.Cmd
}

func NewCmd(input string, output string, config *Config) *TranscodeCommand {
	return &TranscodeCommand{input, output, config, nil}
}

func (cmd *TranscodeCommand) Run(ctx context.Context, ffmpegConfig transcoder.Options, updateHandler func(*FfmpegProgress)) error {
	transcoder := ffmpeg.
		New(&ffmpeg.Config{
			ProgressEnabled: true,
			FfmpegBinPath:   cmd.transcodeConfig.FfmpegBinPath,
			FfprobeBinPath:  cmd.transcodeConfig.FfprobeBinPath,
		}).
		Input(cmd.inputPath).
		Output(cmd.outputPath).
		WithContext(&ctx)

	os.MkdirAll(filepath.Dir(cmd.outputPath), os.ModeDir)

	progressChannel, err := transcoder.Start(ffmpegConfig)
	if err != nil {
		return parseFfmpegError(err)
	}

	cmd.runningCommand = transcoder.GetRunningCmdInstance()

	for {
		prog, ok := <-progressChannel
		if !ok {
			log.Emit(logger.DEBUG, "FFmpeg command has closed progress channel... closing report channel\n")
			return nil
		}

		updateHandler(&FfmpegProgress{
			FramesProcessed: prog.GetFramesProcessed(),
			CurrentTime:     prog.GetCurrentTime(),
			CurrentBitrate:  prog.GetCurrentBitrate(),
			Progress:        prog.GetProgress(),
			Speed:           prog.GetSpeed(),
		})
	}
}

func (cmd *TranscodeCommand) Suspend() {
	if cmd.runningCommand == nil {
		log.Emit(logger.ERROR, "Cannot suspend FFmpeg instance %v because command is not intialised\n", cmd)
		return
	}

	cmd.runningCommand.Process.Signal(syscall.SIGTSTP)
	log.Emit(logger.SUCCESS, "Suspended transcode %v\n", cmd)
}

func (cmd *TranscodeCommand) Continue() {
	if cmd.runningCommand == nil {
		log.Emit(logger.ERROR, "Cannot continue FFmpeg instance %v because command is not intialised\n", cmd)
		return
	}

	cmd.runningCommand.Process.Signal(syscall.SIGCONT)
	log.Emit(logger.SUCCESS, "Resumed transcode %v\n", cmd)
}

func (cmd *TranscodeCommand) RunningCommand() *exec.Cmd {
	return cmd.runningCommand
}

func (cmd *TranscodeCommand) InputPath() string {
	return cmd.inputPath
}

func (cmd *TranscodeCommand) OutputPath() string {
	return cmd.outputPath
}

func (cmd *TranscodeCommand) String() string {
	var pid int = -1
	if cmd.runningCommand != nil {
		pid = cmd.runningCommand.Process.Pid
	}

	return fmt.Sprintf("{ffmpeg pid=%d | in_path=%s | out_path = %s}", pid, cmd.inputPath, cmd.outputPath)
}

func parseFfmpegError(err error) error {
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
