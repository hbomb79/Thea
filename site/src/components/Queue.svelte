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

    .subtitle {
        margin-top: 2rem;
        margin-left: 3rem;
        margin-bottom: -1rem;
        display: block;
        text-align: left;
        font-weight: 500;
        color: #8c91b9;
        text-transform: uppercase;
    }
}
</style>

<div>
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
