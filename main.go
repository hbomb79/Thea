package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hbomb79/TPA/api"
	"github.com/hbomb79/TPA/processor"
	"github.com/hbomb79/TPA/ws"
	"github.com/liip/sheriff"
)

type TPA struct {
	proc       *processor.Processor
	socketHub  *ws.SocketHub
	httpRouter *api.Router
}

func (tpa *TPA) Initialise(cfgPath string) *TPA {
	// Creates a new Processor struct, filling in the configuration
	procCfg := new(processor.TPAConfig)
	procCfg.LoadFromFile(cfgPath)

	tpa.httpRouter, tpa.socketHub = api.NewRouter(), ws.NewSocketHub()
	tpa.proc = processor.New().
		WithConfig(procCfg).
		WithNegotiator(tpa)

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

func (tpa *TPA) OnProcessorUpdate(update *processor.ProcessorUpdate) {
	tpa.socketHub.Send(&ws.SocketMessage{
		Title: "UPDATE",
		Arguments: map[string]interface{}{
			"context": update.Context,
		},
		Type: ws.Update,
	})
}

func (tpa *TPA) wsQueueIndex(hub *ws.SocketHub, message *ws.SocketMessage) error   { return nil }
func (tpa *TPA) wsQueueDetails(hub *ws.SocketHub, message *ws.SocketMessage) error { return nil }
func (tpa *TPA) wsResolveTrouble(hub *ws.SocketHub, message *ws.SocketMessage) error {
	const ERR_FMT = "failed to resolve trouble for queue item %v - %v"

	stringId, ok := message.Arguments["id"]
	if !ok {
		return errors.New(fmt.Sprintf(ERR_FMT, "?", "no 'id' argument provided"))
	}

	queueItemId, err := strconv.Atoi(fmt.Sprintf("%v", stringId))
	if err != nil {
		return errors.New(fmt.Sprintf(ERR_FMT, stringId, err.Error()))
	}

	if item := tpa.proc.Queue.FindById(queueItemId); item != nil {
		if err = item.Trouble.Resolve(message.Arguments); err != nil {
			return errors.New(fmt.Sprintf(ERR_FMT, stringId, err.Error()))
		}

		return nil
	}

	return errors.New(fmt.Sprintf(ERR_FMT, stringId, "item could not be found"))
}

// HttpQueueIndex returns the current processor queue with some information
// omitted. Full information for each item can be found via HttpQueueGet
func (tpa *TPA) HttpQueueIndex(w http.ResponseWriter, r *http.Request) {
	data, err := sheriffApiMarshal(tpa.proc.Queue, []string{"api"})
	if err != nil {
		api.JsonMessage(w, err.Error(), http.StatusInternalServerError)

		return
	}

	api.JsonMarshal(w, data)
}

// HttpQueueGet returns full details for a queue item at the index {id} inside the queue
func (tpa *TPA) HttpQueueGet(w http.ResponseWriter, r *http.Request) {
	queue, stringId := tpa.proc.Queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem := queue.FindById(id)
	if queueItem == nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' cannot be found", http.StatusBadRequest)
		return
	}

	api.JsonMarshal(w, queueItem)
}

// HttpQueueUpdate pushes an update to the processor dictating the new
// positioning of a certain queue item. This allows the user to
// reorder the queue by sending an item to the top of the
// queue, therefore priorisiting it - similar to the Steam library
func (tpa *TPA) HttpQueueUpdate(w http.ResponseWriter, r *http.Request) {
	queue, stringId := tpa.proc.Queue, mux.Vars(r)["id"]

	id, err := strconv.Atoi(stringId)
	if err != nil {
		api.JsonMessage(w, "QueueItem ID '"+stringId+"' not acceptable - "+err.Error(), http.StatusNotAcceptable)
		return
	}

	queueItem := queue.FindById(id)
	if queueItem == nil {
		api.JsonMessage(w, "QueueItem with ID "+fmt.Sprint(id)+" not found", http.StatusNotFound)
	} else if queue.PromoteItem(queueItem) != nil {
		api.JsonMessage(w, "Failed to promote QueueItem #"+stringId+": "+err.Error(), http.StatusInternalServerError)
	} else {
		api.JsonMessage(w, "Queue item promoted successfully", http.StatusOK)
	}
}

// setupRoutes initialises the routes and commands for the HTTP
// REST router, and the websocket hub
func (tpa *TPA) setupRoutes() {
	tpa.httpRouter.CreateRoute("v0/queue", "GET", tpa.HttpQueueIndex)
	tpa.httpRouter.CreateRoute("v0/queue/{id}", "GET", tpa.HttpQueueGet)
	tpa.httpRouter.CreateRoute("v0/queue/promote/{id}", "POST", tpa.HttpQueueUpdate)
	tpa.httpRouter.CreateRoute("v0/ws", "GET", tpa.socketHub.UpgradeToSocket)

	tpa.socketHub.BindCommand("resolveTrouble", tpa.wsResolveTrouble)
	tpa.socketHub.BindCommand("queueIndex", tpa.wsQueueIndex)
	tpa.socketHub.BindCommand("queueDetails", tpa.wsQueueDetails)
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
