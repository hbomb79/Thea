<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../commander";
import { SocketMessageType, SocketPacketType } from "../store";
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

const getStageStr = function(stage:number): string {
    switch(stage) {
        case 0: return "IO Poller"
        case 1: return "Title Format"
        case 2: return "OMDB Querying"
        case 3: return "Formatter"
        case 4: return "Finished"
    }
}

const getStatusStr = function(status:number): string {
    switch(status) {
        case 0: return "Pending"
        case 1: return "Working"
        case 2: return "Completed"
        case 3: return "Troubled"
    }
}

$:stat = function() {
    return getStageStr(details.stage) + ": " + getStatusStr(details.status)
}

onMount(getDetails)
</script>

<style lang="scss">
.item {
    background: white;
    box-shadow: 0px 0px 6px -3px #4b494c;
    max-width: 950px;
    border-radius: 2px;
    width: 80%;
    margin: 2rem auto;
    overflow: hidden;

    .header {
        /* Ocupy all remaining width of the flexbox */
        width: 100%;
        text-align: left;
        overflow: hidden;
        box-shadow: 0px 0px 5px -3px black;
        background: #eee;

        display: flex;
        justify-content: space-between;

        .id {
            color: #a0a0a0;
            align-self: center;
            padding-left: 11px;
            font-size: 0.9rem;
            font-style: italic;
        }

        h2 {
            margin: 0;
            font-size: 1.2rem;
            color: #5E5E5E;
            display: inline-block;
            padding: 0.8rem;
            padding-left: 7px;
            flex: auto;
        }

        .status {
            display: inline-block;
            height: fit-content;
            align-self: center;
            padding-right: 1rem;
            font-style: italic;
            color: #6d6d6d;
            font-size: 0.8rem;
        }
    }

    main {
        padding: 1rem 0 1rem 0;
    }
}
</style>

<!-- Template -->
{#if state == ComponentState.LOADING}
    <span>Loading...</span>
{:else if state == ComponentState.COMPLETE}
    <div class="item" class:trouble="{details.trouble}">
        <div class="header">
            <span class="id">#{details.id}</span>
            {#if details.omdb_info}
                <h2>{details.omdb_info.Title}</h2>
            {:else}
                <h2>{details.name}</h2>
            {/if}

            <span class="status">{stat()}</span>
        </div>
        <main>
            <p>queue item status line goes here...</p>
        </main>
    </div>
{:else}
    <div class="item">
        <div class="header">
            <h2>Failed to load</h2>
            <span>Our request to the server failed. Please check the console for details.</span>
        </div>
    </div>
{/if}
