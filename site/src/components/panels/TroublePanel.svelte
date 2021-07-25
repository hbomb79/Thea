<script context="module" lang="ts">
export enum QueueTroubleType {
	TITLE_FAILURE,
	OMDB_NO_RESULT_FAILURE,
	OMDB_MULTIPLE_RESULT_FAILURE,
	OMDB_REQUEST_FAILURE,
	FFMPEG_FAILURE,
}

export interface QueueTroubleDetails {
    trouble:QueueTroubleInfo,
    type:QueueTroubleType,
    expectedArgs:Object,
    [key:string]:any
}

</script>

<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../../commander";
import { SocketMessageType } from "../../store";
import { QueueStage } from "../QueueItem.svelte";

import type { SocketData } from "../../store";
import type { QueueDetails, QueueTroubleInfo } from "../QueueItem.svelte";

enum ComponentState {
    LOADING,
    COMPLETE,
    ERR
}

export let details:QueueDetails
let state = ComponentState.LOADING
let troubleDetails:QueueTroubleDetails

onMount(() => {
    commander.sendMessage({
        title: "TROUBLE_DETAILS",
        type: SocketMessageType.COMMAND,
        arguments: { id: details.id }
    }, (data:SocketData): boolean => {
        // Wait for reply to message by using a callback.
        if(data.title == "COMMAND_SUCCESS" && data.type == SocketMessageType.RESPONSE) {
            troubleDetails = data.arguments.payload
            state = ComponentState.COMPLETE
        } else {
            state = ComponentState.ERR
        }

        return true;
    })
})
</script>

<style lang="scss">
.tile.trouble {
    padding: 1rem;
}
</style>

<!-- Template -->
<div class="tile trouble">
    <h2>This stage is troubled</h2>
    <span>While processing this stage, we experienced an error</span>
    <span class="trouble">{details.trouble.message}</span>

    {#if state == ComponentState.COMPLETE}
        {#if details.stage == QueueStage.TITLE}
            <span></span>
        {:else if details.stage == QueueStage.OMDB}
            <!-- Figure out what kind of trouble we're dealing with -->
        {:else if details.stage == QueueStage.FFMPEG}
            <span>FFMPEG trouble resolution</span>
        {:else if details.stage == QueueStage.DB}
            <span>Database trouble</span>
        {:else}
            <span>This stage has no known trouble resolution methods. Please consult the server logs for more information.</span>
        {/if}
    {:else if state == ComponentState.LOADING}
        <span>Fetching trouble resolution</span>
    {:else}
        <span class="err">Failed to fetch trouble resolution</span>
    {/if}
</div>
