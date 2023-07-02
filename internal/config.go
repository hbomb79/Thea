package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hbomb79/Thea/internal/api"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/internal/ingest"
	"github.com/hbomb79/Thea/internal/transcode"
	"github.com/ilyakaznacheev/cleanenv"
)

// TheaConfig is the struct used to contain the
// various user config supplied by file, or
// manually inside the code.
type TheaConfig struct {
	Format        transcode.Config        `toml:"transcode"`
	IngestService ingest.Config           `toml:"ingestion"`
	Services      DockerConfig            `toml:"docker"`
	Database      database.DatabaseConfig `toml:"database"`
	RestConfig    api.RestConfig          `toml:"api"`
	OmdbKey       string                  `toml:"omdb_api_key" env:"OMDB_API_KEY" env-required:"true"`
	CacheDirPath  string                  `toml:"cache_dir" env:"CACHE_DIR"`
	ConfigDirPath string                  `toml:"config_dir" env:"CONFIG_DIR"`
}

// DockerConfig is used to enable/disable the internal intialisation of
// supporting services for Thea. By default, these will be enabled so that Thea
// will initialise them automatically.
type DockerConfig struct {
	EnablePostgres bool `toml:"enable_postgres" env:"SERVICE_ENABLE_POSTGRES"`
	EnablePgAdmin  bool `toml:"enable_pg_admin" env:"SERVICE_ENABLE_PGADMIN"`
	EnableFrontend bool `toml:"enable_frontend" env:"SERVICE_ENABLE_UI"`
}

// Loads a configuration file formatted in TOML in to a
// TheaConfig struct ready to be passed to Processor
func (config *TheaConfig) LoadFromFile(configPath string) error {
	err := cleanenv.ReadConfig(configPath, config)
	if err != nil {
		return fmt.Errorf("failed to load configuration for ProcessorConfig - %v", err.Error())
	}

	return nil
}

// GetCacheDir will return the directory path used for storing cache information. It will first look to
// in the config for a value, but if none is found, a default value will be returned. If the default
// cannot be derived due to an error, a panic will occur.
func (config *TheaConfig) GetCacheDir() string {
	if config.CacheDirPath != "" {
		return filepath.Join(config.CacheDirPath, THEA_USER_DIR_SUFFIX)
	}

	// Derive default
	dir, err := os.UserCacheDir()
	if err != nil {
		panic(fmt.Sprintf("FAILURE to derive user cache dir %s", err))
	}

	return filepath.Join(dir, THEA_USER_DIR_SUFFIX)
}

// GetConfigDir will return the path used for storing config information. It will first look to
// in the config for a value, but if none is found, a default value will be returned
func (config *TheaConfig) GetConfigDir() string {
	if config.ConfigDirPath != "" {
		return filepath.Join(config.ConfigDirPath, THEA_USER_DIR_SUFFIX)
	}

	// Derive default
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(fmt.Sprintf("FAILURE to derive user config dir %s", err))
	}

	return filepath.Join(dir, THEA_USER_DIR_SUFFIX)
}
