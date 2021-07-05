package processor

import (
	"fmt"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type TPAConfig struct {
	Concurrent concurrentConfig `yaml:"concurrency"`
	Format     formatterConfig  `yaml:"formatter"`
	Database   databaseConfig   `yaml:"database"`
}

type concurrentConfig struct {
	Import int `yaml:"import_threads"`
	Title  int `yaml:"title_threads"`
	OMBD   int `yaml:"omdb_threads"`
	Format int `yaml:"ffmpeg_threads"`
}

type formatterConfig struct {
	ImportPath         string `yaml:"import_path"`
	OutputPath         string `yaml:"output_path"`
	CacheFile          string `yaml:"cache_file"`
	TargetFormat       string `yaml:"target_format"`
	ImportDirTickDelay int    `yaml:"import_polling_delay"`
	FfmpegBinaryPath   string `yaml:"ffmpeg_binary"`
	FfprobeBinaryPath  string `yaml:"ffprobe_binary"`
}

type databaseConfig struct {
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	Name       string `yaml:"name"`
	Host       string `yaml:"host"`
	Port       string `yaml:"port"`
	OmdbKey    string `yaml:"omdb_api_key"`
	OmdbApiUrl string `yaml:"omdb_api_url"`
}

func (config *TPAConfig) LoadConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf("Cannot determine user's home directory - %v\n", err.Error())
	}

	configPath := fmt.Sprintf("%v/.config/tpa/config.yaml", homeDir)
	err = cleanenv.ReadConfig(configPath, config)
	if err != nil {
		log.Panicf("Cannot load configuration for ProcessorConfig -  %v\n", err.Error())
	}
}
