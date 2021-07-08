package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/processor"
)

func redirectLogToFile(path string) {
	// Redirect log output to file

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf(err.Error())
	}

	log.SetOutput(fh)
}

// main() is the entry point to the program, from here will
// we load the users TPA configuration from their home directory,
// merging the configuration with the default config
func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf(err.Error())
	}
	redirectLogToFile(filepath.Join(homeDir, "tpa.log"))

	// Creates a new Processor struct, filling in the configuration
	proc, procCfg := processor.New(), new(processor.TPAConfig)
	procCfg.LoadFromFile(filepath.Join(homeDir, ".config/tpa/config.yaml"))
	proc.WithConfig(procCfg)

	// Spawn HTTP API in background
	setupApi(proc)

	// Run processor
	err = proc.Start()
	if err != nil {
		log.Panicf(fmt.Sprintf("Failed to initialise Processer - %v\n", err.Error()))
	}
}

func setupApi(proc *processor.Processor) *api.Router {
	router := api.NewRouter()

	// -- BEGIN API v0 routes -- //
	router.CreateRoute("v0", "GET", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Test")
	})
	router.CreateRoute("v0/queue", "GET", proc.Queue.ApiQueueIndex)
	router.CreateRoute("v0/queue/{id}", "GET", proc.Queue.ApiQueueGet)
	router.CreateRoute("v0/queue/{id}", "POST", proc.Queue.ApiQueueUpdate)

	// TODO
	// router.CreateRoute("v0/troubles/", "GET", apiTroubleIndex)
	// router.CreateRoute("v0/troubles/{trouble_id}", "GET", apiTroubleGet)
	// router.CreateRoute("v0/troubles/{trouble_id}", "PUSH", apiTroubleUpdate)
	// -- ENDOF API v0 routes -- //

	go router.Start(&api.RouterOptions{
		ApiPort: 8080,
		ApiHost: "localhost",
		ApiRoot: "/api/tpa/",
	})

	return router
}
