<script lang="ts">
import { onMount } from 'svelte'

import store from './store'

import Nav from './components/Nav.svelte'
import Queue from './components/Queue.svelte'

const enum STATE {
    LOADING,
    CONNECTED,
    ERR
}

const socketStore:any = store.webstore
let appState:STATE = STATE.LOADING

function handleMessage(dataStr:string) {
    // Consume the initial writeable empty string that is emitted on creation
    if(dataStr == "") return

    // Try to convert to JSON
    try {
        let dataObj = JSON.parse(dataStr)
        if(appState == STATE.LOADING) {
            if(dataObj && dataObj.title == "CONNECTION_ESTABLISHED") {
                appState = STATE.CONNECTED
                console.info("Connection success!")

                return
            }

            console.error("Unexpected message received from websocket", dataStr)
        }
    } catch(e:any) {
        console.warn("Connection to websocket seems to have failed, or response is unexpected: ", e)
    }
}

onMount(() => {
    socketStore.subscribe((data:string) => {
        // Handle event
        handleMessage(data)
    })
})

</script>

<style lang="scss">
@use "./styles/global.scss";

main {
    text-align: center;
    padding: 1em;
    max-width: 240px;
    margin: 0 auto;
    margin-top: global.$navHeight + 1rem;
}

.subtitle {
    color: #ff3e00;
    text-transform: uppercase;
    font-weight: 100;
}

@media (min-width: 640px) {
    main {
        max-width: none;
    }
}
</style>

<Nav title="TPA Dashboard"/>
<main>
    {#if appState == STATE.LOADING}
        <div class="loading modal">
            <h2>Connecting to server...</h2>
        </div>
    {:else if appState == STATE.CONNECTED}
        <Queue />
    {:else}
        <div class="err modal">
            <h2>Failed to connect to server.</h2>
            <p>Ensure the server is online, or try again later.</p>
        </div>
    {/if}
    <span class="subtitle">Queue</span>
</main>

