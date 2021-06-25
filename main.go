package main

import (
	"log"

	"gitlab.com/hbomb79/TPA/processor"
)

// main() is the entry point to the program, from here will
// we load the users TPA configuration from their home directory,
// merging the configuration with the default config
func main() {
	// Creates a new Processor struct, filling in the configuration
	t := processor.New()

	// Start the program
	err := t.Begin()
	if err != nil {
		log.Panicf("Failed to initialise Processer - %v\n", err.Error())
	}
}
