package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
	"github.com/liip/sheriff"
)

type TPA struct {
	proc       *processor.Processor
	socketHub  *ws.SocketHub
	httpRouter *api.Router
	negotiator *ApiNegotiator
}

func (tpa *TPA) Initialise(cfgPath string) *TPA {
	// Creates a new Processor struct, filling in the configuration
	procCfg := new(processor.TPAConfig)
	procCfg.LoadFromFile(cfgPath)

	tpa.httpRouter, tpa.socketHub = api.NewRouter(), ws.NewSocketHub()
	tpa.negotiator = &ApiNegotiator{tpa}
	tpa.proc = processor.New().
		WithConfig(procCfg).
		WithNegotiator(tpa.negotiator)

	return tpa
}

func (tpa *TPA) Start() {
	// Start websocket, router and processor
	tpa.setupRoutes()

	go tpa.socketHub.Start()
	go tpa.httpRouter.Start(&api.RouterOptions{
		ApiPort: 8080,
		ApiHost: "localhost",
		ApiRoot: "/api/tpa",
	})

	if err := tpa.proc.Start(); err != nil {
		log.Panicf(fmt.Sprintf("Failed to initialise Processor - %v\n", err.Error()))
	}
}

// setupRoutes initialises the routes and commands for the HTTP
// REST router, and the websocket hub
func (tpa *TPA) setupRoutes() {
	negotiator := tpa.negotiator
	tpa.httpRouter.CreateRoute("v0/queue", "GET", negotiator.HttpQueueIndex)
	tpa.httpRouter.CreateRoute("v0/queue/{id}", "GET", negotiator.HttpQueueGet)
	tpa.httpRouter.CreateRoute("v0/queue/promote/{id}", "POST", negotiator.HttpQueueUpdate)
	tpa.httpRouter.CreateRoute("v0/ws", "GET", tpa.socketHub.UpgradeToSocket)

	tpa.socketHub.BindCommand("TROUBLE_RESOLVE", negotiator.wsTroubleResolve)
	tpa.socketHub.BindCommand("TROUBLE_DETAILS", negotiator.wsTroubleDetails)
	tpa.socketHub.BindCommand("QUEUE_INDEX", negotiator.wsQueueIndex)
	tpa.socketHub.BindCommand("QUEUE_DETAILS", negotiator.wsQueueDetails)
	tpa.socketHub.BindCommand("QUEUE_PROMOTE", negotiator.wsQueuePromote)
}

// main() is the entry point to the program, from here will
// we load the users TPA configuration from their home directory,
// merging the configuration with the default config
func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Panicf(err.Error())
	}
	//redirectLogToFile(filepath.Join(homeDir, "tpa.log"))
	tpa := new(TPA).Initialise(filepath.Join(homeDir, ".config/tpa/config.yaml"))
	tpa.Start()
}

func redirectLogToFile(path string) {
	// Redirect log output to file

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf(err.Error())
	}

	log.SetOutput(fh)
}

// sheriffApiMarshal is a method that will marshal
// the provided argument using Sheriff to remove
// items from the struct that aren't exposed to the API (i.e. removes
// struct fields that lack the `groups:"api"` tag)
func sheriffApiMarshal(target interface{}, groups []string) (interface{}, error) {
	o := &sheriff.Options{Groups: groups}

	data, err := sheriff.Marshal(o, target)
	if err != nil {
		return nil, err
	}

	return data, nil
}
