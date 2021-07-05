package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hbomb79/TPA/processor"
)

// main() is the entry point to the program, from here will
// we load the users TPA configuration from their home directory,
// merging the configuration with the default config
func main() {
	// Redirect log output to file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf(err.Error())
	}

	fh, err := os.OpenFile(filepath.Join(homeDir, "tpa.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf(err.Error())
	}

	log.SetOutput(fh)

	// Creates a new Processor struct, filling in the configuration
	cfg := &processor.TPAConfig{}
	cfg.LoadFromFile(filepath.Join(homeDir, ".config/tpa/config.yaml"))

	err = processor.New().
		WithConfig(cfg).
		Start()

	if err != nil {
		log.Panicf(fmt.Sprintf("Failed to initialise Processer - %v\n", err.Error()))
	}
}
