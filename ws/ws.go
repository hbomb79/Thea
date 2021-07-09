package ws

import (
	"fmt"

	"github.com/gorilla/websocket"
)

type SocketHandler func(*SocketHub, *SocketCommand) error

// SocketCommandArgument is a struct that encapsulates the
// argument value AND type - this allows the receiver to
// properly convert to the string value to the type it's
// intended to be. (i.e. "9" can be converted to int confidently
// and an error can be thrown if an invalid value is given for the type)
type SocketCommandArgument struct {
	Type  string
	Value string
}

// SocketCommand is a struct that allows us to define the
// command that has been passed through the web socket.
// The Identifier is used so that the client can detect replies
// from the server that are pertinent to it's request. For example,
// if the client asks the server to complete an action, the client can
// assign an Identifier, and the server can reply with a success/error
// using the same Identifier so the client can wait for the reply without
// blocking.
type SocketCommand struct {
	Name       string
	Arguments  []string
	Identifier int
}

type HubOptions struct {
}

// SocketHub is the struct responsible for managing
// the websocket upgrading, connecting, pushing and
// receiving of messages.
type SocketHub struct {
	socketHandlers map[string]SocketHandler
	websocket      *websocket.Conn
}

func NewSocketHub() *SocketHub {
	return &SocketHub{
		socketHandlers: make(map[string]SocketHandler),
	}
}

// Initialise the SocketHub by opening the web socket
func (hub *SocketHub) Start(opts *HubOptions) {

}

// Binds a provided particular command to a particular socker handler
func (hub *SocketHub) BindCommand(command string, handler SocketHandler) *SocketHub {
	hub.socketHandlers[command] = handler
	return hub
}

// Push a new message on the web socket bound to
// this SocketHub
func (hub *SocketHub) Send(command SocketCommand) {

}

// handleCommand is an internal method that accepts a command
// and wil forward the command to the bound handler if one
// exists. If none exists, a warning is printed to the console
func (hub *SocketHub) handleCommand(command *SocketCommand) {
	if handler, ok := hub.socketHandlers[command.Name]; ok {
		if err := handler(hub, command); err != nil {
			fmt.Printf("[Websocket] (!!) Handler for command '%v' returned error - %v\n", command.Name, err.Error())
		} else {
			fmt.Printf("[Websocket] Handler for command '%v' executed successfully\n", command.Name)
		}

		return
	}

	fmt.Printf("[Websocket] No handler found for command '%v'\n", command.Name)
}
