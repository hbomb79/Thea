package transcode

import (
	"fmt"
	"strings"

	"github.com/hbomb79/Thea/internal/queue"
)

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

func composeCommandArguments(sourceCommand string, item *queue.Item) string {
	getVal := func(command string) string {
		item := item
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
