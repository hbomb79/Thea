<script lang="ts">
import { onMount } from "svelte";
import { commander, dataStream } from "../../commander";
import { SocketMessageType } from "../../store";

import type { SocketData } from "../../store";
import { QueueStatus, QueueTroubleType } from "../QueueItem.svelte";
import type { QueueDetails, QueueTroubleDetails } from "../QueueItem.svelte";

import closeSvg from '../../assets/close.svg';

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
    LONG_CONFIRMATION,
    RESOLVED,
    FAILURE,
    PERSISTS
}

export let queueDetails: QueueDetails

const dispatch = createEventDispatcher()

let state = ComponentState.MAIN
let troubleDetails: QueueTroubleDetails = queueDetails.trouble
let failureDetails = ""

let embeddedPanel: EmbeddedPanel = null
let embeddedPanelResolver = ""

let confirmationTimeout = null

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
            if(state == ComponentState.RESOLVING) state = ComponentState.CONFIRMING
        } else {
            state = ComponentState.FAILURE
            failureDetails = `Server rejected resolution with error:<br><code><b>${data.arguments.error}</b></code>`
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

function startConfirmationTimeout() {
    if(confirmationTimeout) return

    confirmationTimeout = setTimeout(() => {
        if(state == ComponentState.CONFIRMING) state = ComponentState.LONG_CONFIRMATION
        confirmationTimeout = null
    }, 3000)
}

onMount(() => {
    dataStream.subscribe((data) => {
        if(data.type == SocketMessageType.UPDATE) {
            const updateContext = data.arguments.context
            if(!(updateContext && updateContext.QueueItem && updateContext.QueueItem.id == queueDetails.id)) {
                return
            }

            // Depending on the status of this panel, we'll need to perform some
            // comparisions between this new data, and the data we're currently holding.
            const newDetails = updateContext.QueueItem
            switch(state) {
                case ComponentState.RESOLVING:
                case ComponentState.LONG_CONFIRMATION:
                case ComponentState.CONFIRMING:
                    if(queueDetails.stage != newDetails.stage || !newDetails.trouble || newDetails.taskFeedback) {
                        // Our item has either gone to the next stage, cleared it's trouble state, or the task has
                        // provided some real feedback. This is the earliest indication our resolution likely
                        // worked
                        state = ComponentState.RESOLVED
                        break
                    }

                    // If the stage is the same, there's a trouble, AND no task feedback we must test to see if the trouble
                    // is the same as what we have
                    if(queueDetails.trouble.type == newDetails.trouble.type) {
                        // If the new details given still show the item as NEEDS_RESOLVING,
                        // then it means the resolution didn't work. This is because
                        // as soon as a resolution is accepted by the server, the item is marked as PENDING.
                        if(newDetails.status == QueueStatus.NEEDS_RESOLVING) {
                            // The item still needs attention
                            state = ComponentState.PERSISTS
                        } else if(newDetails.status == QueueStatus.PENDING){
                            startConfirmationTimeout()
                        }

                        break
                    }

                    break
                case ComponentState.PERSISTS:
                    // If while we're notifying the user that the trouble
                    // did not resolve... it resolves, display the resolved panel
                    if(!newDetails.trouble) state = ComponentState.RESOLVED
                    break
            }

            queueDetails = newDetails
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
    top: 10%;
    margin: 0;
    max-height: 85%;
    display: flex;
    flex-direction: column;
    transform: translate(-50%, 0);

    .header {
        background: #f75e5e;
        align-items: center;
        flex-shrink: 0;

        h2 {
            color: #890101;
            padding-left: 1rem;
        }

        .close {
            padding: 0.5rem;
            margin-right: 0.5rem;
            background: #f75e5e;
            transition: background 150ms;
            border-radius: 4px;
            font-size: 0;
            cursor: pointer;
            height: fit-content;

            &:hover {
                background: #f74242;
            }

            :global(svg) {
                width: 1rem;
                height: 1rem;
                fill: #890101;
            }
        }
    }

    main {
        padding: 1rem 2rem;
        overflow-y: auto;

        @import "../../styles/trouble.scss";
    }
}

</style>

<!-- Template -->
<div class="modal-backdrop" on:click="{() => dispatch('close')}"></div>
<div class="item modal trouble">
    <div class="header">
        <h2>Trouble Diagnostics</h2>
        <span class="close" on:click={() => dispatch('close')}>{@html closeSvg}</span>
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
                <h2>Cannot Resolve</h2>
                <p class="sub">Unknown trouble type</p>
                <p>We don't have a known resolution for this trouble case. Please check server logs for guidance.</p>
            {/if}

            {#if embeddedPanelResolver == "" && embeddedPanel}
                <h2>{embeddedPanel.getHeader()}</h2>
                <p class="sub">{@html embeddedPanel.getBody()}</p>

                <p><code><b>Error: </b>{troubleDetails.message}</code><br><br><i>Select an option above to begin resolving</i></p>
            {/if}
        {:else if state == ComponentState.RESOLVING || state == ComponentState.CONFIRMING || state == ComponentState.LONG_CONFIRMATION}
            <h2>Resolving trouble</h2>
            <p class="sub">
                {#if state == ComponentState.RESOLVING}
                Waiting for server
                {:else}
                Verifying item progression
                {/if}
            </p>
            {#if state == ComponentState.LONG_CONFIRMATION}
                <p>
                    This might take longer than anticipated... we need a worker in order to confirm that this resolution solved the problem, but they're all busy at the moment.
                    <br><br>
                    <i>Feel free to close this modal - your resolution data is saved and will be used once a worker is available</i>
                </p>
            {:else}
            <p>
                Please wait while we confirm that your resolution data solved the problem. This could take a few seconds...
            </p>
            {/if}
        {:else if state == ComponentState.RESOLVED}
            <h2>Trouble Resolved</h2>
            <p>This trouble has been resolved. You can now close this modal.</p>

            <button on:click|preventDefault={() => dispatch("close")}>Close</button>
        {:else if state == ComponentState.PERSISTS}
            <h2>Trouble Resolution Failed</h2>
            <p class="sub">Persistent trouble type</p>
            <p>
                The server accepted our resolution data, however the trouble was re-raised by the server.<br>
                <i>Check server logs for more information, or contact server administrator for further assistance</i>
            </p>

            <button on:click|preventDefault={resetPanel}>Back</button>
        {:else if state == ComponentState.FAILURE}
            <h2>Trouble Resolution Failed</h2>
            <p class="sub">Resolution rejected</p>

            <p>{@html failureDetails}</p>

            <button on:click|preventDefault={resetPanel}>Back</button>
        {:else}
            <h2>Unknown Error</h2>
            <p class="sub">Component state error</p>
            <span class="err">This component has an inner state that is out-of-bounds for normal operation.<br>Please try closing and re-opening this modal</span>
            <p><i>Contact server administrator if issue persists</i></p>
        {/if}
    </main>
</div>

