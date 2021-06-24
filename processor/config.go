package processor

import (
	"fmt"
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type TPAConfig struct {
	Format   formatterConfig `yaml:"formatter"`
	Database databaseConfig  `yaml:"database"`
}

type formatterConfig struct {
	ImportPath   string `yaml:"import_path"`
	OutputPath   string `yaml:"output_path"`
	CacheFile    string `yaml:"cache_file"`
	TargetFormat string `yaml:"target_format"`
}

type databaseConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
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
