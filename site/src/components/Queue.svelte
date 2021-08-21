<script context="module" lang="ts">
// Export the types that we want to be able to reference
// from any QueueItem child components
export type QueueList = QueueItem[]
</script>

<script lang="ts">
import { onMount } from 'svelte'
import { commander, dataStream } from '../commander';
import { SocketMessageType } from '../store';
import Item from './QueueItem.svelte';
import type { QueueDetails } from './QueueItem.svelte';
import type { SocketData } from '../store';
import type { QueueItem } from './QueueItem.svelte';
import QueueItemMini from './QueueItemMini.svelte';

enum ComponentState {
    INDEXING,
    ERR,
    COMPLETE
}

let state = ComponentState.INDEXING
let items:QueueList = []
export let minified = false
const getQueueIndex = () => {
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
}

onMount(() => {
    getQueueIndex()

    dataStream.subscribe(data => {
        if(data.type == SocketMessageType.UPDATE) {
            const update = data.arguments.context
            const newItem = update.QueueItem as QueueDetails

            const idx = items.findIndex(item => item.id == update.ItemId)
            if( update.ItemPosition < 0 || !newItem ) {
                // Item has been removed from queue! Find the item
                // in the queue with the ID that matches the one removed
                // and pull it from the list
                if(idx < 0) {
                    console.warn("Failed to find item inside of list for removal. Forcing refresh!")
                    getQueueIndex()

                    return
                }

                items.splice(idx, 1)
                // Svelte reactivity requires re-assignment of an array if it's modified using
                // a mutating method like 'splice'
                items = items
            } else if(idx != update.ItemPosition) {
                // The position for this item has changed.. likely due to a item promotion.
                // Update the order of the queue - to do this we should
                // simply re-query the server for an up-to-date queue index.
                getQueueIndex()
            } else {
                if(idx < 0) {
                    // New item
                    items.push(newItem)
                } else {
                    // An existing item has had an in-place update.
                    items[idx] = newItem
                }
            }
        }
    })
})

</script>

<style lang="scss">

.wrapper {
    width: 90%;
    min-width: 780px;
    margin: 0 auto;
    max-width: 950px;
}

</style>


<div class="queue">
    {#if state == ComponentState.INDEXING}
        <div>
            <span>Spinning up...</span>
        </div>
    {:else if state == ComponentState.COMPLETE}
        {#if minified}
            {#each items as queueInfo (queueInfo.id)}
                <QueueItemMini {queueInfo} />
            {/each}
        {:else}
            <div class="wrapper">
                {#each items as queueInfo (queueInfo.id)}
                    <Item {queueInfo} />
                {/each}
            </div>
        {/if}
    {:else}
        <div><span>Fail</span></div>
    {/if}
</div>
