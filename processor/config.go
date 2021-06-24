package processor

import (
	"fmt"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Skeleton processor configuration struct that
// is filled in via the YAML config file provided
// by the USER
type ProcessorConfig struct {
	Format   FormatConfig   `yaml:"formatter"`
	Database DatabaseConfig `yaml:"database"`
}

type FormatConfig struct {
	ImportPath   string `yaml:"import_path"`
	OutputPath   string `yaml:"output_path"`
	CacheFile    string `yaml:"cache_file"`
	TargetFormat string `yaml:"target_format"`
}

type DatabaseConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
}

func (config *ProcessorConfig) LoadConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine user's home directory - %v\n", err.Error())
	}

	configPath := fmt.Sprintf("%v/.config/tpa/config.yaml", homeDir)
	err = cleanenv.ReadConfig(configPath, config)
	if err != nil {
		log.Fatalf("Cannot load configuration for ProcessorConfig -  %v\n", err.Error())
	}
}
