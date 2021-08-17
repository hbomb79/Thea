package ws

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SocketHandler func(*SocketHub, *SocketMessage) error

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
	receiveCh    chan *SocketMessage
	doneCh       chan int
	running      bool
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

// Binds a provided particular command to a particular socker handler
func (hub *SocketHub) BindCommand(command string, handler SocketHandler) *SocketHub {
	hub.handlers[command] = handler
	return hub
}

// Start beings the socket hub by listening on all related channels
// for incoming clients and messages
func (hub *SocketHub) Start() {
	if hub.running {
		fmt.Printf("[Websocket] (!) Attempting to start socketHub when already running! Ignoring request.\n")
		return
	}
	fmt.Printf("[Websocket] (O) Opening SocketHub!\n")

	// Open channels and make clients slice
	hub.sendCh = make(chan *SocketMessage)
	hub.receiveCh = make(chan *SocketMessage)
	hub.registerCh = make(chan *socketClient)
	hub.deregisterCh = make(chan *socketClient)
	hub.doneCh = make(chan int)
	hub.clients = make([]*socketClient, 0)
	hub.running = true

	defer hub.Close()
loop:
	for {
		select {
		case message := <-hub.sendCh:
			// Send the message provided - either by broadcasting to all, or
			// sending to only the client with a UUID matching the message 'target'
			if message.Target != nil {
				if _, client := hub.findClient(message.Target); client != nil {
					if err := client.SendMessage(message); err != nil {
						fmt.Printf("[Websocket] (!!) Failed to send message to target {%v}: %v\n", message.Target, err.Error())
					}
				} else {
					fmt.Printf("[Websocket] (!) Attempted to send message to target {%v}, but no matching client was found.\n", message.Target)
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
				fmt.Printf("[Websocket] (!!) Attempted to register client that is already registered (duplicate uuid)! Illegal!\n")
				client.Close()

				break
			}

			hub.clients = append(hub.clients, client)
			fmt.Printf("[Websocket] (+) Registered new client {%v}\n", client.id)
		case client := <-hub.deregisterCh:
			// Deregister the client by removing the received client and closing it's sockets
			// and channels
			if idx, _ := hub.findClient(client.id); idx != -1 {
				hub.clients = append(hub.clients[:idx], hub.clients[idx+1:]...)
				fmt.Printf("[Websocket] (-) Deregistered client {%v}\n", client.id)

				break
			}

			fmt.Printf("[Websocket] (!) Attempted to deregister unknown client {%v}\n", client.id)
		case <-hub.doneCh:
			// Shutdown the socket hub, closing all clients and breaking this select loop
			fmt.Printf("[Websocket] (-) Shutting down socket hub! Closing all clients.\n")
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
		fmt.Printf("[Websocket] (!) Attempted to send message via socket hub, however the hub is offline. Ignoring message.\n")
		return
	}

	hub.sendCh <- message
}

// Upgrades a given HTTP request to a websocket and adds the new clients to the hub
func (hub *SocketHub) UpgradeToSocket(w http.ResponseWriter, r *http.Request) {
	if !hub.running {
		fmt.Printf("[Websocket] (!!) Failed to upgrade incoming HTTP request to a websocket: SocketHub has not been started!\n")
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

	client := &socketClient{
		id:     &id,
		socket: sock,
	}

	// Register the client and open the read loop
	hub.registerCh <- client

	// Send welcome message to this client
	hub.Send(&SocketMessage{
		Title:  "CONNECTION_ESTABLISHED",
		Body:   map[string]interface{}{"client": id},
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
		fmt.Printf("[Websocket] (!) Client {%v} closed, error: %v\n", client.id, err.Error())
	}
}

// Closes the sockethub by deregistering and closing all
// connected clients and sockets
func (hub *SocketHub) Close() {
	if !hub.running {
		fmt.Printf("[Websocket] (!) Attempted to close a socket hub that is not running!\n")
		return
	}

	// Send done notification to the hub
	// We do this non-blocking because if the
	// hub closes, it calls this function to close
	// the channels and therefore nothing it receiving on doneCh
	select {
	case hub.doneCh <- 1:
	default:
	}

	// Close all the clients
	for _, client := range hub.clients {
		client.Close()
	}

	// Close all the channels
	close(hub.deregisterCh)
	close(hub.registerCh)
	close(hub.receiveCh)
	close(hub.sendCh)
	close(hub.doneCh)

	// Reset the clients slice
	hub.clients = nil
	hub.running = false
	fmt.Printf("[Websocket] (X) Socket hub is now closed!\n")
}

// handleMessage is an internal method that accepts a message
// and wil forward the command to the bound handler if one
// exists. If none exists, a warning is printed to the console
func (hub *SocketHub) handleMessage(command *SocketMessage) {
	if command.Type != Command {
		fmt.Printf("[Websocket] (!) SocketHub received a message from client {%v} of type {%v} - this type is not allowed, only commands can be sent to the server!\n", command.Origin, command.Type)
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
			fmt.Printf("[Websocket] (!!) Handler for command '%v' returned error - %v\n", command.Title, err.Error())
			replyWithError(err.Error())
		} else {
			fmt.Printf("[Websocket] Handler for command '%v' executed successfully\n", command.Title)
		}

		return
	}

	replyWithError("Unknown command")
	fmt.Printf("[Websocket] (!) No handler found for command '%v'\n", command.Title)
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
