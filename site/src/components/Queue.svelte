<script context="module" lang="ts">
// Export the types that we want to be able to reference
// from any QueueItem child components
export type QueueList = QueueItem[]


//TODO Defining these enums here causes a circular dependency
// They're better placed in QueueItem as this component is already
// having to import QueueItem in order to use the component.
export enum QueueStatus {
    PENDING,
    PROCESSING,
    COMPLETED,
    TROUBLED
}

export enum QueueStage {
    IMPORT,
    TITLE,
    OMDB,
    FFMPEG,
    DB, //TODO Implement in Go server
    FINISH
}

export interface QueueItem {
    id: number
    name: string
    stage: QueueStage
    status: QueueStatus
    statusLine: string
}
</script>

<script lang="ts">
import { onMount } from 'svelte'
import { commander, dataStream } from '../commander';
import { SocketMessageType } from '../store';
import type { SocketData } from '../store';
import Item from './QueueItem.svelte';

enum ComponentState {
    INDEXING,
    ERR,
    COMPLETE
}

let state = ComponentState.INDEXING
let items:QueueList = []
onMount(() => {
    // As soon as this 
    commander.sendMessage({
        title: "QUEUE_INDEX",
        type: SocketMessageType.COMMAND
    }, (response:SocketData):boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            state = ComponentState.COMPLETE
            items = response.arguments.payload.items

            return true
        }

        state = ComponentState.ERR
        items = []

        return false
    });

    dataStream.subscribe(data => {
        if(data.type == SocketMessageType.UPDATE) {
            //TODO
        }
    })
})

</script>

<style lang="scss">
.wrapper {
    width: 80%;
    min-width: 780px;
    margin: 0 auto;
    max-width: 950px;

    .subtitle {
        margin: 2rem auto -1rem auto;
        padding-left: 1rem;
        display: block;
        text-align: left;
        font-weight: 500;
        color: #8c91b9;
        text-transform: uppercase;
    }
}

</style>


<div class="queue">
    {#if state == ComponentState.INDEXING}
        <div>
            <span>Spinning up...</span>
        </div>
    {:else if state == ComponentState.COMPLETE}
        <div class="wrapper">
            <span class="subtitle">Queue <span style="font-weight: 300;">({items.length})</span></span>
            {#each items as item (item.id)}
                <!-- QueueItem is aliased to Item to avoid naming conflict -->
                <Item queueInfo={item} />
            {/each}
        </div>
    {:else}
        <div><span>Fail</span></div>
    {/if}
</div>
