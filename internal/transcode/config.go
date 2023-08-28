package transcode

type Config struct {
	OutputPath               string `toml:"default_output_dir" env:"FORMAT_DEFAULT_OUTPUT_DIR" env-required:"true"`
	FfmpegBinaryPath         string `toml:"ffmpeg_binary_path" env:"FORMAT_FFMPEG_BINARY_PATH" env-default:"/usr/bin/ffmpeg"`
	FfprobeBinaryPath        string `toml:"ffprobe_binary_path" env:"FORMAT_FFPROBE_BINARY_PATH" env-default:"/usr/bin/ffprobe"`
	MaximumThreadConsumption int    `toml:"max_thread_consumption" env-default:"8"`
}
