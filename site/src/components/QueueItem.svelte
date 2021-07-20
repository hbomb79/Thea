<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../commander";
import { SocketMessageType } from "../store";
import type { SocketData } from "../store";
import type { QueueItem } from "./Queue.svelte";

enum ComponentState {
    LOADING,
    ERR,
    COMPLETE
}

interface QueueTitleInfo {
    Title: string
    Episodic: boolean
    Episode: number
    Season: number
    Year: number
    Resolution: string
}

interface QueueOmdbInfo {
    Title: string
    plot: string
    ReleaseYear: number
    Runtime: string
    Type: string
    poster: string
    ImdbId: string
    Response: boolean
    Error: string
    Genre: string[]
}

interface QueueTroubleInfo {
    message: string
}

interface QueueDetails extends QueueItem {
    title_info: QueueTitleInfo
    omdb_info:  QueueOmdbInfo
    trouble: QueueTroubleInfo
}

export let queueInfo:QueueItem

let state = ComponentState.LOADING
let details:QueueDetails = null
const getDetails = function() {
    // Get enhanced details of the queue item
    commander.sendMessage({
        type: SocketMessageType.COMMAND,
        title: "QUEUE_DETAILS",
        arguments: {id: queueInfo.id}
    }, (response:SocketData): boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            details = response.arguments.payload
            state = ComponentState.COMPLETE

            return true
        }

        details = null
        state = ComponentState.ERR

        return false
    })
}

onMount(getDetails)
</script>

<style lang="sass">
.item
    background: #eee
    box-shadow: 0px 0px 6px -3px #4b494c
    border: solid 2px rgba(32,152,218,20%)
    border-radius: 3px
    max-width: 950px
    width: 80%
    margin: 1rem auto

    .status
        float: left

</style>

<!-- Template -->
{#if state == ComponentState.LOADING}
    <span>Loading...</span>
{:else if state == ComponentState.COMPLETE}
    <div class="item">
        <div class="header">
            {#if details.omdb_info}
                <h2>{details.omdb_info.Title}</h2>
            {:else}
                <h2>{details.name}</h2>
            {/if}
            <span>queue item status line goes here...</span>
        </div>
        <div class="status">
            <span>{queueInfo.id}</span>
            <p>{queueInfo.stage}: {queueInfo.status}</p>
        </div>
    </div>
{:else}
    <div class="item">
        <div class="header">
            <h2>Failed to load</h2>
            <span>Our request to the server failed. Please check the console for details.</span>
        </div>
    </div>
{/if}
