package ingest

import (
	"time"

	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/mitchellh/go-homedir"
)

// Config contains configuration options that allow
// customization of how Thea detects files to auto-ingest.
type Config struct {
	// The IngestService uses a directory watcher, but a
	// 'force' sync can be performed on a regular interval
	// to protect against the watcher failing.
	ForceSyncSeconds int `toml:"force_sync_seconds" env-default:"500"`

	// The path to the directory the service should monitor
	// for new files
	IngestPath string `toml:"dir_path" env-required:"true"`

	// An array of regular expressions that can be used to RESTRICT
	// the files processed by this service. If any expression match
	// the name of the file, it is ignored.
	Blacklist []string `toml:"blacklist"`

	// When a new file is detected, it's likely to be an in-progress
	// download using an external software. As we cannot KNOW when the
	// download is complete, we instead wait for the 'modtime' of
	// the item to be at least this long in the past before processing
	RequiredModTimeAgeSeconds int `toml:"modtime_threshold_seconds" env-default:"120"`

	// Controls the number of workers that can perform ingestions. Reducing
	// to 1 means one ingestion at a time.
	// Caution should be taken to not increase this value too high, as ingestion
	// involves talking to external APIs which may impose rate limits
	IngestionParallelism int `toml:"parallelism" env-default:"2"`
}

func (config *Config) RequiredModTimeAgeDuration() time.Duration {
	return time.Duration(config.RequiredModTimeAgeSeconds) * time.Second
}

func (config *Config) GetIngestPath() string {
	out, err := homedir.Expand(config.IngestPath)
	if err != nil {
		logger.Get("Config").Emit(logger.ERROR, "Failed to expand ingestion path (%s): %v {will use provided path un-expanded}\n", config.IngestPath, err)
		return config.IngestPath
	}

	return out
}
