<script context="module" lang="ts">
// Export the types that we want to be able to reference
// from any QueueItem child components
export type QueueList = QueueItem[]
export interface QueueItem {
    id: number
    name: string
    stage: number
    status: number
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
        type: 1
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

<div>
    {#if state == ComponentState.INDEXING}
        <div>
            <span>Spinning up...</span>
        </div>
    {:else if state == ComponentState.COMPLETE}
        {#each items as item}
            <!-- QueueItem is aliased to Item to avoid naming conflict -->
            <Item queueDetails={item} />
        {/each}
    {:else}
        <div><span>Fail</span></div>
    {/if}
</div>
