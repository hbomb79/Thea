package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
)

type Tpa struct {
	proc        *processor.Processor
	socketHub   *ws.SocketHub
	wsGateway   *api.WsGateway
	httpGateway *api.HttpGateway
	httpRouter  *api.Router
}

func NewTpa() *Tpa {
	proc := processor.NewProcessor()

	return &Tpa{
		proc:        proc,
		httpRouter:  api.NewRouter(),
		httpGateway: api.NewHttpGateway(proc),
		socketHub:   ws.NewSocketHub(),
		wsGateway:   api.NewWsGateway(proc),
	}
}

func (tpa *Tpa) Start() {
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
func (tpa *Tpa) setupRoutes() {
	tpa.httpRouter.CreateRoute("v0/queue", "GET", tpa.httpGateway.HttpQueueIndex)
	tpa.httpRouter.CreateRoute("v0/queue/{id}", "GET", tpa.httpGateway.HttpQueueGet)
	tpa.httpRouter.CreateRoute("v0/queue/promote/{id}", "POST", tpa.httpGateway.HttpQueueUpdate)
	tpa.httpRouter.CreateRoute("v0/ws", "GET", tpa.socketHub.UpgradeToSocket)

	tpa.socketHub.BindCommand("QUEUE_INDEX", tpa.wsGateway.WsQueueIndex)
	tpa.socketHub.BindCommand("QUEUE_DETAILS", tpa.wsGateway.WsQueueDetails)
	tpa.socketHub.BindCommand("QUEUE_PROMOTE", tpa.wsGateway.WsQueuePromote)
	tpa.socketHub.BindCommand("TROUBLE_RESOLVE", tpa.wsGateway.WsTroubleResolve)
	tpa.socketHub.BindCommand("TROUBLE_DETAILS", tpa.wsGateway.WsTroubleDetails)
}

func (tpa *Tpa) OnProcessorUpdate(update *processor.ProcessorUpdate) {
	tpa.socketHub.Send(&ws.SocketMessage{
		Title: "UPDATE",
		Body: map[string]interface{}{
			"context": update.Context,
		},
		Type: ws.Update,
	})
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

	procCfg := new(processor.TPAConfig)
	procCfg.LoadFromFile(filepath.Join(homeDir, ".config/tpa/config.yaml"))

	tpa := NewTpa()
	tpa.proc.WithConfig(procCfg)

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
