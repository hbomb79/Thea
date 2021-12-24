/*
 * Commander is a exported object that allows components to send
 * and wait on replies from the websocket
 */
import { sendMessage as socketSend, SocketData, SocketMessageType, SocketPacketType, socketStream, SocketStreamPacket } from './store'
import { writable } from 'svelte/store'

// commandCallback is a function type alias that defines a callback
// for a socket command
export type CommandCallback = (arg0: SocketData) => boolean

// Below are the exported subscribables that
// allow components to be updated when messages
// are received, or when the socket state changes
export const dataStream = writable({} as SocketData)
export const statusStream = writable(SocketPacketType.INIT)
export const ffmpegOptionsStream = writable({})
export const ffmpegMatchKeysStream = writable([])

// Commander is a class that is intended to be used as a singleton
// instance for the entire application (see export below class def.)
// This class is responsible for sending and receiving data over the websocket
// connection to the go-server
class Commander {
    // A map of callbacks to be called when a message with a particular
    // ID is received.
    callbacks: Map<number, CommandCallback>

    // The last ID we've given to a message. Used to automatically
    // allocate IDs to messages being sent when a callback is provided
    // because the ID is required in order to listen for a reply from the server
    lastId: number

    // Initialises the class members and subscribes to the socketStream
    constructor() {
        this.lastId = 0;
        this.callbacks = new Map<number, CommandCallback>()

        socketStream.subscribe((data) => this.handlePacket(data))
        dataStream.subscribe(data => {
            if (data.type == SocketMessageType.WELCOME) {
                ffmpegOptionsStream.set(data.arguments.ffmpegOptions)
                ffmpegMatchKeysStream.set(data.arguments.ffmpegMatchKeys)
            }
        })
    }

    // Send a socket message (in the form SocketData).
    // Optionally, a callback can be provided which will be called
    // if/when we receive a reply from the server with the same
    // ID as the 'message' (message.id). If a message ID is not
    // found on the given message, one is allocated automatically
    //      
    // Note: The return value of the callback function will dictate
    // whether or not the received message passed to the callback will
    // be published on the dataStream - if true is returned, it will NOT
    // be published.
    sendMessage(message: SocketData, callback?: CommandCallback) {
        if (callback != undefined) {
            if (!message.id) {
                this.lastId++
                message.id = this.lastId
            }

            this.callbacks[message.id] = callback
        }

        const obj = JSON.stringify(message)
        socketSend(obj)
    }

    // handlePacket will examine a new packet from socketStream
    // and decide whether to publish it to the status or data stream
    handlePacket(packet: SocketStreamPacket): void {
        switch (packet.type) {
            case SocketPacketType.INIT:
            case SocketPacketType.OPEN:
                // Websocket has been opened! Forward this
                // new status on the dedicated channel
                statusStream.set(packet.type)
                break
            case SocketPacketType.MESSAGE:
                // We received a message from the server
                const ev = packet.ev as MessageEvent
                const obj = this.parseMessage(ev.data)
                if (obj) this.handleMessage(obj)

                break
            case SocketPacketType.CLOSE:
            case SocketPacketType.ERROR:
                console.warn("[Commander] SocketStream delivered CLOSE or ERROR packet: ", packet)
                statusStream.set(packet.type)

                break
            default:
                console.warn("[Commander] SocketStream delivered packet of unknown type: ", packet)
        }
    }

    // parseMessage is used to take the data of a socket message and
    // parse it from string -> JSON
    // null is returned on failure and a warning is emitted in the console
    parseMessage(message: string) {
        try {
            const obj = JSON.parse(message) as SocketData
            return obj
        } catch (e) {
            console.warn("[Commander] Failed to parse SocketStream data to valid JSON: ", e)

            return null
        }
    }

    // handleMessage will check to see if the provided SocketData message has
    // an ID, and if so, if a callback is assigned for that ID. If yes, the callback
    // is executed.
    // 
    // The message will then be published to the dataStream if:
    // - There is no callback for this message
    // - There is a callback and the return value of the callback is FALSE
    handleMessage(message: SocketData): void {
        if (message.id && this.callbacks[message.id]) {
            const cancelPropogate = this.callbacks[message.id](message)
            delete this.callbacks[message.id]

            if (cancelPropogate) return
        }

        dataStream.set(message)
    }
}

// Export an instance of the Commander to be used by the application
export const commander = new Commander();

