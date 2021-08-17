<script lang="ts">
import { onMount } from "svelte";
import { commander, dataStream } from "../../commander";
import type { CommandCallback } from "../../commander";
import { SocketMessageType } from "../../store";
import type { SocketDataArguments } from "../../store";

import type { SocketData } from "../../store";
import { QueueTroubleType } from "../QueueItem.svelte";
import type { QueueDetails, QueueTroubleDetails } from "../QueueItem.svelte";

import rippleHtml from '../../assets/html/ripple.html';

import OmdbTroublePanel from "./trouble_panels/OmdbTroublePanel.svelte";

enum ComponentState {
    INIT,
    LOADED,
    RESOLVING,
    ERR
}

export let details:QueueDetails
let state = ComponentState.INIT
let troubleDetails:QueueTroubleDetails

interface EmbeddedTroublePanel {
    updateState(arg0:QueueDetails):void
}
let embeddedPanel:EmbeddedTroublePanel

const getTroubleDetails = () => {
    commander.sendMessage({
        title: "TROUBLE_DETAILS",
        type: SocketMessageType.COMMAND,
        arguments: { id: details.id }
    }, (data:SocketData): boolean => {
        // Wait for reply to message by using a callback.
        if(data.type == SocketMessageType.RESPONSE) {
            troubleDetails = data.arguments.payload
            state = ComponentState.LOADED
        } else {
            state = ComponentState.ERR
        }

        return true;
    })
}

onMount(() => {
    getTroubleDetails()

    dataStream.subscribe((data:SocketData) => {
        if(data.type == SocketMessageType.UPDATE) {
            const updateContext = data.arguments.context
            if(updateContext && updateContext.QueueItem.id == details.id) {
                // Update received for this item, refetch trouble details
                state = ComponentState.INIT
                getTroubleDetails()
            }
        }
    })
})

// sendResolution will attempt to send a trouble resolution command to the server
// by appending the given data to the message using the spread syntax.
// A callback must be provided, and is passed to the send command to enable
// feedback from the server.
function sendResolution(data: SocketDataArguments, cb: CommandCallback) {
    commander.sendMessage({
        title: "TROUBLE_RESOLVE",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: details.id,
            ...data
        }
    }, cb)
}

function tryResolve(packet:CustomEvent) {
    sendResolution(packet.detail.args, packet.detail.cb)
}
</script>

<style lang="scss">
@use "../../styles/global.scss";

.tile.trouble {
    padding: 1rem;

    h2 {
        margin: 0;
        color: #5e5e5e;
    }
}
</style>

<!-- Template -->
<div class="tile trouble">
    {#if state == ComponentState.LOADED}
        {#if troubleDetails.type == QueueTroubleType.TITLE_FAILURE}
            <!-- A title failure means we need to provide the arguments back to the server that we need to
                 make a new TitleInfo struct -->
            <p>NYI</p>
        {:else if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
            <OmdbTroublePanel bind:this={embeddedPanel} troubleDetails={troubleDetails} queueDetails={details} on:try-resolve={tryResolve}/>
        {:else if troubleDetails.type == QueueTroubleType.FFMPEG_FAILURE}
            <h2>FFMPEG Troubled</h2>
            <p>NYI</p>
        {:else}
            <h2>Unknown trouble</h2>
            <p>We don't have a known resolution for this trouble case. Please check server logs for guidance.</p>
        {/if}
    {:else if state == ComponentState.INIT}
        <p>Fetching trouble resolution</p>
        {@html rippleHtml}
    {:else}
        <span class="err">Failed to fetch trouble resolution</span>
    {/if}
</div>
