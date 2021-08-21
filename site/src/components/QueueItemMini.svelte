<script lang="ts">
import { onMount } from "svelte";
import { commander, dataStream } from "../commander";
import { SocketMessageType } from "../store";
import type { SocketData } from "../store";

import rippleHtml from '../assets/html/ripple.html';
import type { QueueItem, QueueDetails } from "./QueueItem.svelte";
import { QueueStatus } from "./QueueItem.svelte";

// The queueInfo we're wanting to display from the parent component.
export let queueInfo:QueueItem
let queueDetails:QueueDetails

// The state of this component, affected by the websocket
// packets that we're receiving from commander.
// This enum is used to control what HTML content we're displaying
enum ComponentState {
    LOADING,
    ERR,
    COMPLETE
}

// The page/panel we're wanting to view inside this component,
// affected by user interaction (e.g. on:click events)

// The initial state/page of the component
let state = ComponentState.LOADING
const getQueueDetails = () => {
    commander.sendMessage({
        type: SocketMessageType.COMMAND,
        title: "QUEUE_DETAILS",
        arguments: {id: queueInfo.id}
    }, (response:SocketData): boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            queueDetails = response.arguments.payload
            state = ComponentState.COMPLETE

            return true
        }

        queueDetails = null
        state = ComponentState.ERR

        return false
    })
}

const getStatusClass = () => {
    switch(queueDetails.status) {
        case QueueStatus.PENDING:
            return "pending"
        case QueueStatus.PROCESSING:
            return "processing"
        case QueueStatus.CANCELLING:
            return "cancelling"
        case QueueStatus.CANCELLED:
            return "cancelled"
        case QueueStatus.NEEDS_RESOLVING:
            return "troubled"
    }
}

// Get enhanced details of the queue item. If this information changes
// we'll be notified by the server via an 'UPDATE' packet, which we
// can use for all the information
onMount(() => {
    getQueueDetails()

    dataStream.subscribe((data:SocketData) => {
        if(data.type == SocketMessageType.UPDATE) {
            const updateContext = data.arguments.context
            if(updateContext && updateContext.QueueItem && updateContext.QueueItem.id == queueDetails.id) {
                queueDetails = updateContext.QueueItem
            }
        }
    })
})

</script>

<style lang="scss">
@use '../styles/global.scss';

$pendingColour: #cabdff;
$processingColour: #39d3fd96;
$cancelledColour: #ffc297;
$troubleColour: #f76c6c;

@mixin pulse-keyframe($color) {
    @keyframes pulse {
        0% {
            transform: scale(0.9);
            box-shadow: 0 0 0 $color;
        }
        70% {
            transform: scale(1);
            box-shadow: 0 0 10px rgba($color: $color, $alpha: 0.6);
        }
        90% {
            box-shadow: 0 0 15px rgba($color: $color, $alpha: 0);
        }
        100% {
            transform: scale(0.9);
        }
    }
}

p {
    padding: 1rem;
    border-bottom: solid 1px #cec9e7;
    margin: 0rem;
    color: #615a7c;

    .status {
        background: #39d3fd96;
        height: 12px;
        width: 12px;
        display: inline-block;
        border-radius: 100%;
        margin-right: 8px;

        &.pending {
            background: $pendingColour;
        }
        &.processing {
            background: $processingColour;
        }
        &.cancelling, &.cancelled {
            background: $cancelledColour;
            @include pulse-keyframe($cancelledColour);
            animation: pulse infinite 2s;
        }
        &.troubled {
            background: $troubleColour;
            @include pulse-keyframe($troubleColour);
            animation: pulse infinite 2s;
        }

    }
}
</style>

<!-- Template -->
<div>
    {#if state == ComponentState.LOADING}
        <main>
            <h2>Loading</h2>
            {@html rippleHtml}
        </main>
    {:else if state == ComponentState.COMPLETE}
        <p>
            <span class={`status ${getStatusClass()}`}></span>
            <span class="name">{queueDetails.omdb_info?.Title || queueDetails.title_info?.Title || queueDetails.name}</span>
        </p>
    {:else}
        <div class="header">
            <h2>Failed to load</h2>
        </div>
    {/if}
</div>
