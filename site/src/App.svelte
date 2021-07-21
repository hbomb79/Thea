<script lang="ts">
import Nav from './components/Nav.svelte'
import Queue from './components/Queue.svelte'
import StatusPanel from './components/StatusPanel.svelte'

import { statusStream } from './commander'
import { SocketPacketType } from './store'
</script>

<style lang="scss">
@use "./styles/global.scss";

:global(body.status-panel-open) main {
    right: global.$statusPanelWidth;
}

main {
    text-align: center;

    position: fixed;
    left: 0;
    right: 0;
    top: 60px;
    bottom: 0;

    overflow-y: scroll;

    transition: right global.$statusPanelAnimTime ease-out;
}
</style>

<Nav title="TPA Dashboard"/>
<StatusPanel/>
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

