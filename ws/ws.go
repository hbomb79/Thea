package ws

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type SocketHandler func(*SocketHub, *SocketMessage) error

type socketMessageType int

const (
	Update socketMessageType = iota
	Command
	Response
	ErrorResponse
)

// SocketMessage is a struct that allows us to define the
// command that has been passed through the web socket.
type SocketMessage struct {
	Body       string            `json:"body"`
	Arguments  []string          `json:"args"`
	Identifier int               `json:"id"`
	Type       socketMessageType `json:"type"`
}

// SocketHub is the struct responsible for managing
// the websocket upgrading, connecting, pushing and
// receiving of messages.
type SocketHub struct {
	socketHandlers map[string]SocketHandler
	socketUpgrader *websocket.Upgrader
	websocket      *websocket.Conn
}

type SocketClient struct {
	Id     string
	Socket *websocket.Conn
}

func NewSocketHub() *SocketHub {
	return &SocketHub{
		socketHandlers: make(map[string]SocketHandler),
		socketUpgrader: &websocket.Upgrader{},
	}
}

// Binds a provided particular command to a particular socker handler
func (hub *SocketHub) BindCommand(command string, handler SocketHandler) *SocketHub {
	hub.socketHandlers[command] = handler
	return hub
}

func (hub *SocketHub) Start() {}

// Push a new message to all clients connected to this socket hub
func (hub *SocketHub) Broadcast(command SocketMessage) {

}

// Upgrades a given HTTP request to a websocket and opens the connection
func (hub *SocketHub) UpgradeToSocket(w http.ResponseWriter, r *http.Request) {
	_, err := hub.socketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("[Websocket] (!!) Failed to upgrade incoming HTTP request to a websocket: %v\n", err.Error())
		return
	}
}

func (hub *SocketHub) Close() {}

func (hub *SocketHub) registerClient() {}

func (hub *SocketHub) deregisterClient() {}

// handleCommand is an internal method that accepts a command
// and wil forward the command to the bound handler if one
// exists. If none exists, a warning is printed to the console
func (hub *SocketHub) handleCommand(command *SocketMessage) {
	if handler, ok := hub.socketHandlers[command.Body]; ok {
		if err := handler(hub, command); err != nil {
			fmt.Printf("[Websocket] (!!) Handler for command '%v' returned error - %v\n", command.Body, err.Error())
		} else {
			fmt.Printf("[Websocket] Handler for command '%v' executed successfully\n", command.Body)
		}

		return
	}

	fmt.Printf("[Websocket] (!) No handler found for command '%v'\n", command.Body)
}
