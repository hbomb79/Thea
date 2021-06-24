package main

import (
	"fmt"

	"gitlab.com/hbomb79/TPA/processor"
)

/**
 * Main function is the entry point to the program, from here will
 * we load the users TPA configuration from their home directory,
 * merging the configuration with the default config
 */
func main() {
	t := processor.New()

	fmt.Printf("Test: %v (%T)\n", t, t)
}
