<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../../commander";
import { SocketMessageType } from "../../store";

import type { SocketData } from "../../store";
import { QueueTroubleType } from "../QueueItem.svelte";
import type { QueueDetails, QueueTroubleDetails } from "../QueueItem.svelte";

import rippleHtml from '../../assets/html/ripple.html';

import OmdbTroublePanel from "./trouble_panels/OmdbTroublePanel.svelte";
import type { SvelteComponent } from "svelte/internal";
import ResolutionModal from "../modals/ResolutionModal.svelte";
import TitleTroublePanel from "./trouble_panels/TitleTroublePanel.svelte";
import FormatTroublePanel from "./trouble_panels/FormatTroublePanel.svelte";

enum ComponentState {
    LOADING,
    LOADED,
    RESOLVING,
    ERR
}

export let details:QueueDetails
let state = ComponentState.LOADING
let troubleDetails:QueueTroubleDetails

let modal:SvelteComponent = null;

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

// sendResolution will attempt to send a trouble resolution command to the server
// by appending the given data to the message using the spread syntax.
// A callback must be provided, and is passed to the send command to enable
// feedback from the server.
function sendResolution(packet:CustomEvent) {
    const { args, cb } = packet.detail

    commander.sendMessage({
        title: "TROUBLE_RESOLVE",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: details.id,
            ...args
        }
    }, cb)
}

function spawnResolutionModal(packet:CustomEvent) {
    if(modal) {
        modal.$destroy()
        modal = undefined
    }

    modal = new ResolutionModal({
        target: document.body,
        props: { ...packet.detail }
    })

    modal.$on("close", () => {
        modal.$destroy()
        modal = undefined
    })
}

onMount( getTroubleDetails )
</script>

<style lang="scss">
@use "../../styles/global.scss";

.tile.trouble {
    padding: 1rem;

    :global(h2) {
        margin: 0;
        color: #5e5e5e;
    }
}
</style>

<!-- Template -->
<div class="tile trouble">
    {#if state == ComponentState.LOADED}
        {#if troubleDetails.type == QueueTroubleType.TITLE_FAILURE}
            <TitleTroublePanel details={details} troubleDetails={troubleDetails} on:display-modal={spawnResolutionModal} on:try-resolve={sendResolution}/>
        {:else if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
            <OmdbTroublePanel troubleDetails={troubleDetails} on:try-resolve={sendResolution} on:display-modal={spawnResolutionModal}/>
        {:else if troubleDetails.type == QueueTroubleType.FFMPEG_FAILURE}
            <FormatTroublePanel details={details} troubleDetails={troubleDetails} on:try-resolve={sendResolution}/>
        {:else}
            <h2>Unknown trouble</h2>
            <p>We don't have a known resolution for this trouble case. Please check server logs for guidance.</p>
        {/if}
    {:else if state == ComponentState.LOADING}
        <p>Fetching trouble resolution</p>
        {@html rippleHtml}
    {:else}
        <span class="err">Failed to fetch trouble resolution</span>
    {/if}
</div>
