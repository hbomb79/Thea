package ws

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
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
// The Id field can be used when replying to this message
// so the receiving client is aware of which message this reply
// is for. Origin is much for the same - it allows us to
// send the reply to the websocket attached to the client
// with the matching UUID
type SocketMessage struct {
	Body      string            `json:"body"`
	Arguments []string          `json:"args"`
	Id        int               `json:"id"`
	Type      socketMessageType `json:"type"`
	Origin    uuid.UUID         `json:"-"`
	Target    uuid.UUID         `json:"-"`
}

// SocketHub is the struct responsible for managing
// the websocket upgrading, connecting, pushing and
// receiving of messages.
type SocketHub struct {
	handlers     map[string]SocketHandler
	upgrader     *websocket.Upgrader
	clients      []*socketClient
	registerCh   chan *socketClient
	deregisterCh chan *socketClient
	sendCh       chan *SocketMessage
	doneCh       chan int
	running      bool
}

type socketClient struct {
	Id     uuid.UUID
	Socket *websocket.Conn
}

func NewSocketHub() *SocketHub {
	return &SocketHub{
		handlers:     make(map[string]SocketHandler),
		upgrader:     &websocket.Upgrader{},
		sendCh:       make(chan *SocketMessage),
		registerCh:   make(chan *socketClient),
		deregisterCh: make(chan *socketClient),
		doneCh:       make(chan int),
		running:      false,
	}
}

// Binds a provided particular command to a particular socker handler
func (hub *SocketHub) BindCommand(command string, handler SocketHandler) *SocketHub {
	hub.handlers[command] = handler
	return hub
}

func (hub *SocketHub) Start() {
	if hub.running {
		return
	}

	for {
		select {
		case _ = <-hub.sendCh:
			// Send the message provided - either by broadcasting to all, or
			// sending to only the client with a UUID matching the message 'target'
		case _ = <-hub.registerCh:
			// Register the client by pushing the received client in to the
			// 'clients' slice
		case _ = <-hub.deregisterCh:
			// Deregister the client by removing the received client and closing it's sockets
			// and channels
		case <-hub.doneCh:
			// Shutdown the socket hub, closing all clients and breaking this select loop
		}
	}
}

func (hub *SocketHub) Send(command SocketMessage) {

}

// Upgrades a given HTTP request to a websocket and adds the new clients to the hub
func (hub *SocketHub) UpgradeToSocket(w http.ResponseWriter, r *http.Request) {
	if !hub.running {
		fmt.Printf("[Websocket] (!!) Failed to upgrade incoming HTTP request to a websocket: SocketHub has not been started!")
		return
	}

	// Try generate UUID first - if we do this later and it fails... we've already
	// upgraded the connection to a websocket.
	id, err := uuid.NewRandom()
	if err != nil {
		fmt.Printf("[Websocket] (!!) Failed to generate UUID for new connection - aborting!\n")
		return
	}

	// UUID success, upgrade the connection to a websocket
	sock, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("[Websocket] (!!) Failed to upgrade incoming HTTP request to a websocket: %v\n", err.Error())
		return
	}

	fmt.Printf("[Websocket] (+) Registering new client with the socket hub")
	hub.registerCh <- &socketClient{
		Id:     id,
		Socket: sock,
	}
}

func (hub *SocketHub) Close() {}

func (hub *SocketHub) registerClient() {}

func (hub *SocketHub) deregisterClient() {}

// handleCommand is an internal method that accepts a command
// and wil forward the command to the bound handler if one
// exists. If none exists, a warning is printed to the console
func (hub *SocketHub) handleCommand(command *SocketMessage) {
	if handler, ok := hub.handlers[command.Body]; ok {
		if err := handler(hub, command); err != nil {
			fmt.Printf("[Websocket] (!!) Handler for command '%v' returned error - %v\n", command.Body, err.Error())
		} else {
			fmt.Printf("[Websocket] Handler for command '%v' executed successfully\n", command.Body)
		}

		return
	}

	fmt.Printf("[Websocket] (!) No handler found for command '%v'\n", command.Body)
}
