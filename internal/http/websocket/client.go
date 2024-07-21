package websocket

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type SocketClient struct {
	ID     uuid.UUID
	socket *websocket.Conn
}

func (client *SocketClient) SendMessage(message *SocketMessage) error {
	return client.socket.WriteJSON(message)
}

// Read starts a read-loop on the clients websocket connection, emitting
// all received messages on the channel provided. If the connection
// experiences an error, or the JSON marshalling fails, this error will be returned
// and consequently the read loop will close. It is the responsibility of the caller
// to de-register the client once the connection closes.
func (client *SocketClient) Read(receiveCh chan *SocketMessage) error {
	for {
		var recv SocketMessage
		if err := client.socket.ReadJSON(&recv); err != nil {
			return err
		}

		// Set the message origin to point to this clients uuid
		recv.Origin = &client.ID
		receiveCh <- &recv
	}
}

// Close will close this clients socket.
func (client *SocketClient) Close() {
	client.socket.Close()
}
