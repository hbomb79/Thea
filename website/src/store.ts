import { Writable, writable } from 'svelte/store'

// SocketDataArguments is an interface specifying the
// data members inside of a SocketData 'arguments' key.
export interface SocketDataArguments {
    payload?: any
    command?: any
    [key: string]: any
}

// socketData is an interface that speifies the data of
// a socket message dispatched from the go-server
export interface SocketData {
    id?: number
    title: string
    type: number
    arguments?: SocketDataArguments
}

// SocketPacketType is an enum that is used to tell what
// type of socket packet is being send downstream via the
// 'socketStream'. This is used by subscribers to help decide
// how to react to a socket update/open/close/error event
export enum SocketPacketType {
    INIT,
    OPEN,
    MESSAGE,
    CLOSE,
    ERROR
}

// SocketMessageType mirrors the message type enum
// specified in the go-server. This can therefore be used
// to tell which type of message has been sent to the client so
// we can use the payload correctly (e.g. update queue contents, or show
// a new trouble, etc)
export enum SocketMessageType {
    UPDATE,
    COMMAND,
    RESPONSE,
    ERR_RESPONSE,
    WELCOME
}

// This interface is used to specify the data members of a
// packet to be sent downstream via 'socketStream'.
export interface SocketStreamPacket {
    type: SocketPacketType
    ev: Event | MessageEvent
}

// socketStream is how low-level subscribers
// to listen for websocket events
export const socketStream = writable({
    type: SocketPacketType.INIT,
    ev: null
})

// socket is the underlying API for creating our websocket
// connection to the go-backend server. We export a sendMessage function
// that allows components to send messages directly via the websocket
// rather than through a Commander instance.
const config = SERVER_CONFIG
const socket = new WebSocket(`ws://${config.host}:${config.port}/api/thea/v0/ws`)
export function sendMessage(message: string) {
    if (socket.readyState <= 1) {
        socket.send(message)

        return true
    }

    return false
}

// open, message and close event listeners that send the
// new packet downstream via 'socketStream'
socket.addEventListener('open', function (event) {
    console.log("[Websocket] Connection established", event)
    socketStream.set({
        type: SocketPacketType.OPEN,
        ev: event
    })
})

socket.addEventListener('message', function (event) {
    console.log("[Websocket] Message recieved: ", event.data)
    socketStream.set({
        type: SocketPacketType.MESSAGE,
        ev: event
    })
})

socket.addEventListener('close', function (event) {
    console.warn("[Websocket] Connection closed...", event)
    socketStream.set({
        type: SocketPacketType.CLOSE,
        ev: event
    })
})
