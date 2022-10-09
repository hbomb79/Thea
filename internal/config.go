package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hbomb79/TPA/internal/db"
	"github.com/hbomb79/TPA/internal/ffmpeg"
	"github.com/ilyakaznacheev/cleanenv"
)

// TPAConfig is the struct used to contain the
// various user config supplied by file, or
// manually inside the code.
type TPAConfig struct {
	Concurrent    ConcurrentConfig       `yaml:"concurrency" env-required:"true"`
	Format        ffmpeg.FormatterConfig `yaml:"formatter"`
	Services      ServiceConfig          `yaml:"docker_services"`
	Database      db.DatabaseConfig      `yaml:"database" env-required:"true"`
	OmdbKey       string                 `yaml:"omdb_api_key" env:"OMDB_API_KEY" env-required:"true"`
	CacheDirPath  string                 `yaml:"cache_dir" env:"CACHE_DIR"`
	ConfigDirPath string                 `yaml:"config_dir" env:"CONFIG_DIR"`
	ApiHostAddr   string                 `yaml:"host" env:"HOST_ADDR" env-default:"0.0.0.0"`
	ApiHostPort   string                 `yaml:"port" env:"HOST_PORT" env-default:"8080"`
}

// ConcurrentConfig is a subset of the configuration that focuses
// only on the concurrency related configs (number of threads to use
// for each stage of the pipeline)
type ConcurrentConfig struct {
	Title  int `yaml:"title_threads" env:"CONCURRENCY_TITLE_THREADS" env-default:"1"`
	OMBD   int `yaml:"omdb_threads" env:"CONCURRENCY_OMDB_THREADS" env-default:"1"`
	Format int `yaml:"ffmpeg_threads" env:"CONCURRENCY_FFMPEG_THREADS" env-default:"8"`
}

// ServiceConfig is used to enable/disable the internal intialisation of
// supporting services for TPA. By default, these will be enabled so that TPA
// will initialise them automatically.
type ServiceConfig struct {
	EnablePostgres bool `yaml:"enable_postgres" env-default:"true"`
	EnablePgAdmin  bool `yaml:"enable_pg_admin" env-default:"true"`
	EnableFrontend bool `yaml:"enable_frontend" env-default:"true"`
}

// Loads a configuration file formatted in YAML in to a
// TPAConfig struct ready to be passed to Processor
func (config *TPAConfig) LoadFromFile(configPath string) error {
	err := cleanenv.ReadConfig(configPath, config)
	if err != nil {
		return fmt.Errorf("failed to load configuration for ProcessorConfig - %v", err.Error())
	}

	return nil
}

// getCachePath will return the path used for storing cache information. It will first look to
// in the config for a value, but if none is found, a default value will be returned. If the default
// cannot be derived due to an error, a panic will occur.
func (config *TPAConfig) getCachePath() string {
	if config.CacheDirPath != "" {
		return filepath.Join(config.CacheDirPath, TPA_CACHE_FILE_PATH)
	}

	// Derive default
	dir, err := os.UserCacheDir()
	if err != nil {
		panic(fmt.Sprintf("FAILURE to derive user cache dir %s", err))
	}

	return filepath.Join(dir, TPA_CACHE_FILE_PATH)
}

// getConfigPath will return the path used for storing config information. It will first look to
// in the config for a value, but if none is found, a default value will be returned
func (config *TPAConfig) getConfigPath() string {
	if config.CacheDirPath != "" {
		return filepath.Join(config.CacheDirPath, TPA_CONFIG_FILE_PATH)
	}

	// Derive default
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(fmt.Sprintf("FAILURE to derive user config dir %s", err))
	}

	return filepath.Join(dir, TPA_CONFIG_FILE_PATH)
}
