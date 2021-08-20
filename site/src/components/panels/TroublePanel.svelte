<script lang="ts">
import { onMount } from "svelte";
import { commander, dataStream } from "../../commander";
import { SocketMessageType } from "../../store";

import type { SocketData } from "../../store";
import { QueueTroubleType } from "../QueueItem.svelte";
import type { QueueDetails, QueueTroubleDetails } from "../QueueItem.svelte";

import rippleHtml from '../../assets/html/ripple.html';

import OmdbTroublePanel from "./trouble_panels/OmdbTroublePanel.svelte";
import { createEventDispatcher } from "svelte/internal";
import TitleTroublePanel from "./trouble_panels/TitleTroublePanel.svelte";
import FormatTroublePanel from "./trouble_panels/FormatTroublePanel.svelte";

interface EmbeddedPanel {
    selectResolver(arg0: string): void
    selectedResolver(): string
    listResolvers(): string[][]
    getHeader(): string
    getBody(): string
}

enum ComponentState {
    MAIN,
    RESOLVING,
    CONFIRMING,
    RESOLVED,
    FAILURE,
    PERSISTS,
    ERR
}

export let queueDetails: QueueDetails

const dispatch = createEventDispatcher()

let state = ComponentState.MAIN
let troubleDetails: QueueTroubleDetails = queueDetails.trouble
let failureDetails = ""

let embeddedPanel: EmbeddedPanel = null
let embeddedPanelResolver = ""

// sendResolution will attempt to send a trouble resolution command to the server
// by appending the given data to the message using the spread syntax.
// A callback must be provided, and is passed to the send command to enable
// feedback from the server.
function sendResolution(packet:CustomEvent) {
    const args = packet.detail.args

    state = ComponentState.RESOLVING
    commander.sendMessage({
        title: "TROUBLE_RESOLVE",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: queueDetails.id,
            ...args
        }
    }, function(data: SocketData) {
        if(data.type == SocketMessageType.RESPONSE) {
            state = ComponentState.CONFIRMING
        } else {
            state = ComponentState.FAILURE
            failureDetails = `Server rejected resolution with error: <b>${data.arguments.error}</b>`
        }
            
        return true
    })
}

function updateEmbeddedPanelResolver() {
    embeddedPanelResolver = embeddedPanel.selectedResolver()
}

function resetPanel() {
    state = ComponentState.MAIN

    // requestAnimationFrame because 'embeddedPanel' will
    // not exist as it's only present when state is LOADED.
    requestAnimationFrame(() => {
        if(embeddedPanel) embeddedPanel.selectResolver(embeddedPanelResolver)
    })
}

onMount(() => {
    dataStream.subscribe((data) => {
        if(data.type == SocketMessageType.UPDATE) {
            // TODO
        }
    })
})

</script>

<style lang="scss">
@use "../../styles/global.scss";
@use "../../styles/modal.scss";

.modal.trouble {
    width: 70%;
    overflow: hidden;
    border-color: red;
    border-width: 1px;

    .header {
        background: #f75e5e;

        h2 {
            color: #890101;
        }
    }

    main {
        padding: 1rem 2rem;

        @import "../../styles/trouble.scss";
    }
}

</style>

<!-- Template -->
<div class="modal-backdrop" on:click="{() => dispatch('close')}"></div>
<div class="item modal trouble">
    <div class="header">
        <h2>Trouble Diagnostics</h2>
    </div>

    {#if embeddedPanel}
        <div class="panel">
            <span class="panel-item" on:click={() => embeddedPanel.selectResolver("")} class:active={embeddedPanelResolver == ""}>Details</span>
            {#each embeddedPanel.listResolvers() as [display, key]}
                <span class="panel-item" class:active={embeddedPanelResolver == key} on:click="{() => embeddedPanel.selectResolver(key)}">{display}</span>
            {/each}
        </div>
    {/if}
    <main>
        {#if state == ComponentState.MAIN}
            {#if troubleDetails.type == QueueTroubleType.TITLE_FAILURE}
                <TitleTroublePanel bind:this={embeddedPanel} queueDetails={queueDetails} on:try-resolve={sendResolution} on:selection-change={updateEmbeddedPanelResolver}/>
            {:else if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
                <OmdbTroublePanel bind:this={embeddedPanel} queueDetails={queueDetails} on:try-resolve={sendResolution} on:selection-change={updateEmbeddedPanelResolver}/>
            {:else if troubleDetails.type == QueueTroubleType.FFMPEG_FAILURE}
                <FormatTroublePanel bind:this={embeddedPanel} on:try-resolve={sendResolution} on:selection-change={updateEmbeddedPanelResolver}/>
            {:else}
                <h2>Unknown trouble</h2>
                <p>We don't have a known resolution for this trouble case. Please check server logs for guidance.</p>
            {/if}

            {#if embeddedPanelResolver == "" && embeddedPanel}
                <h2>{embeddedPanel.getHeader()}</h2>
                <p class="sub">{@html embeddedPanel.getBody()}</p>

                <p><code><b>Error: </b>{troubleDetails.message}</code><br><br><i>Select an option above to begin resolving</i></p>
            {/if}
        {:else if state == ComponentState.RESOLVING || state == ComponentState.CONFIRMING}
            <h2>Resolving trouble</h2>
            <p class="sub">
                {#if state == ComponentState.RESOLVING}
                Waiting for server
                {:else}
                Verifying item progression
                {/if}
            </p>
            <p>
                Please wait while we process that request. This could take a few seconds.
            </p>
        {:else if state == ComponentState.RESOLVED}
            <h2>Trouble Resolved</h2>
            <p>This trouble has been resolved. You can now close this modal.</p>

            <button on:click|preventDefault={() => dispatch("close")}>Close</button>
        {:else if state == ComponentState.PERSISTS}
            <h2>Trouble Persists</h2>
            <p>
                The server accepted our resolution data, however the trouble was re-raised by the server.<br>
                <i>Check server logs for more information, or contact server administrator for further assistance</i>
            </p>

            <button on:click|preventDefault={resetPanel}>Back</button>
        {:else if state == ComponentState.FAILURE}
            <h2>Trouble Resolution Rejected</h2>
            <p>{@html failureDetails}</p>

            <button on:click|preventDefault={resetPanel}>Back</button>
        {:else}
            <span class="err">Failed to fetch trouble resolution</span>
        {/if}
    </main>
</div>

