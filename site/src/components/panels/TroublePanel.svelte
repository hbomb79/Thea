<script context="module" lang="ts">
export interface QueueTroubleDetails extends QueueTroubleInfo {

}
</script>

<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../../commander";
import { SocketMessageType } from "../../store";
import type { SocketData } from "../../store";

import type { QueueDetails, QueueTroubleInfo } from "../QueueItem.svelte";

enum ComponentState {
    LOADING,
    COMPLETE,
    ERR
}

export let details:QueueDetails
let state = ComponentState.LOADING
let troubleDetails:QueueTroubleInfo

onMount(() => {
    commander.sendMessage({
        title: "TROUBLE_DETAILS",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: details.id
        }
    }, (data:SocketData): boolean => {
        // Wait for reply to message by using a callback.
        if(data.title == "COMMAND_SUCCESS") {
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

</style>

<!-- Template -->
<div class="tile trouble">
    <h2>This stage is troubled</h2>
    <span>While processing this stage, we experienced an error</span>
    <span class="trouble">{details.trouble.message}</span>

    {#if state == ComponentState.COMPLETE}
        <span>Trouble resolution fetched</span>
    {:else if state == ComponentState.LOADING}
        <span>Fetching trouble resolution</span>
    {:else}
        <span class="err">Failed to fetch trouble resolution</span>
    {/if}
</div>
