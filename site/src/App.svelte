<script lang="ts">
import Nav from './components/Nav.svelte'
import Queue from './components/Queue.svelte'
import StatusPanel from './components/StatusPanel.svelte'
import rippleHtml from './assets/html/ripple.html'

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
    display: flex;
    align-items: flex-start;

    .modal, :global(.queue) {
        margin: 0 auto;
    }

    :global(.queue) {
        flex: 1 auto;
    }

    .modal {
        background: #ffffff85;
        display: inline-block;
        width: 450px;
        align-self: center;
        padding: 2rem;
        border: solid 1px #c0c0c3;
        border-radius: 6px;
        color: #83769c;
        box-shadow: 0px 0px 5px -3px black;
        flex: 0 auto;
    }


}
</style>

<Nav title="TPA Dashboard"/>
<StatusPanel/>
<main>
    {#if $statusStream == SocketPacketType.INIT}
        <div class="loading modal">
            <h2>Connecting to server...</h2>
            {@html rippleHtml}
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

