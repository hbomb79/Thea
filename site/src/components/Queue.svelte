<script lang="ts">
import {onMount} from 'svelte'
import store from '../store'

const webstore = store.webstore

function handleMessage(dataStr:string) {
}

onMount(() => {
    // When loaded, query for the queue from the websocket
    webstore.sendMessage(JSON.stringify({
        title: "QUEUE_INDEX",
        type: 1,
        id: 42
    }))

    webstore.subscribe((data:string) => {
        console.log("Queue received data", data)
    })
})

</script>
<h1>Queue NYI</h1>
