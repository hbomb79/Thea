package ws

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hbomb79/TPA/pkg"
)

var socketLogger = pkg.Log.GetLogger("WebSocket", pkg.CORE)

type SocketHandler func(*SocketHub, *SocketMessage) error

// SocketHub is the struct responsible for managing
// the websocket upgrading, connecting, pushing and
// receiving of messages.
type SocketHub struct {
	handlers           map[string]SocketHandler
	upgrader           *websocket.Upgrader
	clients            []*socketClient
	registerCh         chan *socketClient
	deregisterCh       chan *socketClient
	sendCh             chan *SocketMessage
	receiveCh          chan *SocketMessage
	doneCh             chan int
	connectionCallback func() map[string]interface{}
	running            bool
}

// Returns a new SocketHub with the channels,
// maps and slices initialised to sane starting
// values
func NewSocketHub() *SocketHub {
	return &SocketHub{
		handlers: make(map[string]SocketHandler),
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		running: false,
	}
}

// WithConnectionPayload sets a callback that will be executed each time a new client
// connects to this socketHub. This allows the client to be furnished with a payload
// of the servers current state, without having to wait for an UPDATE packet from the
// server (which may never come if the content does not change).
func (hub *SocketHub) WithConnectionCallback(callback func() map[string]interface{}) {
	hub.connectionCallback = callback
}

// Binds a provided particular command to a particular socker handler
func (hub *SocketHub) BindCommand(command string, handler SocketHandler) *SocketHub {
	hub.handlers[command] = handler
	return hub
}

// Start beings the socket hub by listening on all related channels
// for incoming clients and messages
func (hub *SocketHub) Start() {
	if hub.running {
		socketLogger.Emit(pkg.WARNING, "Attempting to start socketHub when already running! Ignoring request.\n")
		return
	}
	socketLogger.Emit(pkg.INFO, "Opening SocketHub!\n")

	// Open channels and make clients slice
	hub.sendCh = make(chan *SocketMessage)
	hub.receiveCh = make(chan *SocketMessage)
	hub.registerCh = make(chan *socketClient)
	hub.deregisterCh = make(chan *socketClient)
	hub.doneCh = make(chan int)
	hub.clients = make([]*socketClient, 0)
	hub.running = true

	defer hub.close()
loop:
	for {
		select {
		case message := <-hub.sendCh:
			// Send the message provided - either by broadcasting to all, or
			// sending to only the client with a UUID matching the message 'target'
			if message.Target != nil {
				if _, client := hub.findClient(message.Target); client != nil {
					if err := client.SendMessage(message); err != nil {
						socketLogger.Emit(pkg.ERROR, "Failed to send message to target {%v}: %v\n", message.Target, err.Error())
					}
				} else {
					socketLogger.Emit(pkg.WARNING, "Attempted to send message to target {%v}, but no matching client was found.\n", message.Target)
				}

				break
			}

			// No specific target
			hub.broadcastMessage(message)
		case message := <-hub.receiveCh:
			go hub.handleMessage(message)
		case client := <-hub.registerCh:
			// Register the client by pushing the received client in to the
			// 'clients' slice
			if idx, _ := hub.findClient(client.id); idx > -1 {
				socketLogger.Emit(pkg.ERROR, "Attempted to register client that is already registered (duplicate uuid)! Illegal!\n")
				client.Close()

				break
			}

			hub.clients = append(hub.clients, client)
			socketLogger.Emit(pkg.NEW, "Registered new client {%v}\n", client.id)
		case client := <-hub.deregisterCh:
			// Deregister the client by removing the received client and closing it's sockets
			// and channels
			if idx, _ := hub.findClient(client.id); idx != -1 {
				hub.clients = append(hub.clients[:idx], hub.clients[idx+1:]...)
				socketLogger.Emit(pkg.REMOVE, "Deregistered client {%v}\n", client.id)

				break
			}

			socketLogger.Emit(pkg.WARNING, "Attempted to deregister unknown client {%v}\n", client.id)
		case <-hub.doneCh:
			// Shutdown the socket hub, closing all clients and breaking this select loop
			socketLogger.Emit(pkg.REMOVE, "Shutting down socket hub! Closing all clients.\n")
			break loop
		}
	}
}

// Send accepts a socket message and will emit this message on
// the send channel - message is ignored if hub is not running (see Start())
// A message provided that has a Target will only be sent to the client with
// a matching ID
func (hub *SocketHub) Send(message *SocketMessage) {
	if !hub.running {
		socketLogger.Emit(pkg.WARNING, "Attempted to send message via socket hub, however the hub is offline. Ignoring message.\n")
		return
	}

	hub.sendCh <- message
}

// Upgrades a given HTTP request to a websocket and adds the new clients to the hub
func (hub *SocketHub) UpgradeToSocket(w http.ResponseWriter, r *http.Request) {
	if !hub.running {
		socketLogger.Emit(pkg.ERROR, "Failed to upgrade incoming HTTP request to a websocket: SocketHub has not been started!\n")
		return
	}

	// Try generate UUID first - if we do this later and it fails... we've already
	// upgraded the connection to a websocket.
	id, err := uuid.NewRandom()
	if err != nil {
		socketLogger.Emit(pkg.ERROR, "Failed to generate UUID for new connection - aborting!\n")
		return
	}

	// UUID success, upgrade the connection to a websocket
	sock, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		socketLogger.Emit(pkg.ERROR, "Failed to upgrade incoming HTTP request to a websocket: %v\n", err.Error())
		return
	}

	client := &socketClient{
		id:     &id,
		socket: sock,
	}

	// Register the client and open the read loop
	hub.registerCh <- client

	// Send welcome message to this client with a composed
	// map of new-client properties.
	// These props can be used to supply the client with it's
	// initial state
	var body map[string]interface{}
	if hub.connectionCallback != nil {
		body = hub.connectionCallback()
	}
	body["client"] = id

	hub.Send(&SocketMessage{
		Title:  "CONNECTION_ESTABLISHED",
		Body:   body,
		Target: &id,
		Type:   Welcome,
	})

	// Ensure the client is deregistered once it's read loop closes
	// If client.Start finishes, it's either because the client disconnected
	// or an error occured - either way, we need to deregister it.
	defer func() {
		hub.deregisterCh <- client
		client.Close()
	}()

	// Start the read loop for the client
	if err := client.Read(hub.receiveCh); err != nil {
		socketLogger.Emit(pkg.WARNING, "Client {%v} closed, error: %v\n", client.id, err.Error())
	}
}

// Signals the SocketHub to close
func (hub *SocketHub) Close() {
	// Send done notification to the hub
	// We do this non-blocking because if the
	// hub closes, it calls this function to close
	// the channels and therefore nothing it receiving on doneCh
	select {
	case hub.doneCh <- 1:
	default:
	}
}

// Closes the sockethub by deregistering and closing all
// connected clients and sockets
func (hub *SocketHub) close() {
	if !hub.running {
		socketLogger.Emit(pkg.WARNING, "Attempted to close a socket hub that is not running!\n")
		return
	}

	// Close all the clients
	for _, client := range hub.clients {
		client.Close()
	}

	// Reset the clients slice
	hub.clients = nil
	hub.running = false
	socketLogger.Emit(pkg.STOP, "Socket hub is now closed!\n")
}

// handleMessage is an internal method that accepts a message
// and wil forward the command to the bound handler if one
// exists. If none exists, a warning is printed to the console
func (hub *SocketHub) handleMessage(command *SocketMessage) {
	if command.Type != Command {
		socketLogger.Emit(pkg.WARNING, "SocketHub received a message from client {%v} of type {%v} - this type is not allowed, only commands can be sent to the server!\n", command.Origin, command.Type)
		return
	}

	replyWithError := func(err string) {
		hub.Send(&SocketMessage{
			Title:  "COMMAND_FAILURE",
			Id:     command.Id,
			Target: command.Origin,
			Body:   map[string]interface{}{"command": command, "error": err},
			Type:   ErrorResponse,
		})
	}

	if handler, ok := hub.handlers[command.Title]; ok {
		if err := handler(hub, command); err != nil {
			socketLogger.Emit(pkg.ERROR, "Handler for command '%v' returned error - %v\n", command.Title, err.Error())
			replyWithError(err.Error())
		} else {
			socketLogger.Emit(pkg.SUCCESS, "Handler for command '%v' executed successfully\n", command.Title)
		}

		return
	}

	replyWithError("Unknown command")
	socketLogger.Emit(pkg.WARNING, "No handler found for command '%v'\n", command.Title)
}

// findClient returns a socketClient with the matching uuid if
// one can be found - if not, nil is returned. Additionally, the index
// of the client inside of the client list is returned as well.
func (hub *SocketHub) findClient(id *uuid.UUID) (int, *socketClient) {
	for idx, client := range hub.clients {
		if client.id == id {
			return idx, client
		}
	}

	return -1, nil
}

// broadcastMessage sends the provided message to every connected
// client - useful for pushing new state to all clients interested
func (hub *SocketHub) broadcastMessage(message *SocketMessage) error {
	for _, client := range hub.clients {
		if err := client.SendMessage(message); err != nil {
			return err
		}
	}

	return nil
}
