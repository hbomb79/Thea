import { writable } from 'svelte/store'

const webstore = writable('')
const socket = new WebSocket("ws://localhost:8080/api/tpa/v0/ws")

const sendMessage = function(message:string) {
    if (socket.readyState <= 1) {
        socket.send(message)

        return true
    }

    return false
}

socket.addEventListener('open', function(event) {
    console.log("[Websocket] Connection established", event)
})

socket.addEventListener('message', function(event) {
    console.log("[Websocket] Message recieved: ", event.data)
    webstore.set(event.data)
})

socket.addEventListener('close', function(event) {
    console.error("[Websocket] Connection closed...", event)
})

export default {
    "webstore": {
        subscribe: webstore.subscribe,
        sendMessage
    }
}
