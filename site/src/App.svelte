<script lang="ts">
import Nav from './components/Nav.svelte'
import Queue from './components/Queue.svelte'

import { statusStream } from './commander'
import { SocketPacketType } from './store'
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

@media (min-width: 640px) {
    main {
        max-width: none;
    }
}
</style>

<Nav title="TPA Dashboard"/>
<main>
    {#if $statusStream == SocketPacketType.INIT}
        <div class="loading modal">
            <h2>Connecting to server...</h2>
        </div>
    {:else if $statusStream == SocketPacketType.OPEN}
        <Queue />
    {:else}
        <div class="err modal">
            <h2>Failed to connect to server.</h2>
            <p>Ensure the server is online, or try again later.</p>
        </div>
    {/if}
</main>

