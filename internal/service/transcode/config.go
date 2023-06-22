package transcode

type Config struct {
	ImportPath         string `yaml:"import_path" env:"FORMAT_IMPORT_PATH" env-required:"true"`
	OutputPath         string `yaml:"default_output_dir" env:"FORMAT_DEFAULT_OUTPUT_DIR" env-required:"true"`
	TargetFormat       string `yaml:"target_format" env:"FORMAT_TARGET_FORMAT" env-default:"mp4"`
	ImportDirTickDelay int    `yaml:"import_polling_delay" env:"FORMAT_IMPORT_POLLING_DELAY" env-default:"3600"`
	FfmpegBinaryPath   string `yaml:"ffmpeg_binary" env:"FORMAT_FFMPEG_BINARY_PATH" env-default:"/usr/bin/ffmpeg"`
	FfprobeBinaryPath  string `yaml:"ffprobe_binary" env:"FORMAT_FFPROBE_BINARY_PATH" env-default:"/usr/bin/ffprobe"`
}
