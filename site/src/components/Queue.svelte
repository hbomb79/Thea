<script lang="ts">
import { onMount } from 'svelte'
import { commander } from '../commander';
import { SocketMessageType } from '../store';
import type { SocketData } from '../store';

enum QueueState {
    INDEXING,
    COMPLETE
}

let state = QueueState.INDEXING
let items = []
onMount(() => {
    commander.sendMessage({
        title: "QUEUE_INDEX",
        type: 1,
        id: 42
    }, (response:SocketData):boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            state = QueueState.COMPLETE
            items = response.arguments.payload.items
        }

        return false
    });
})

</script>

<div>
    {#if state == QueueState.INDEXING}
        <div>
            <span>Spinning up...</span>
        </div>
    {:else if state == QueueState.COMPLETE}
        {#each items as item}
            <div>
                <h2>{item.title}</h2>
                <p>Status: {item.status}</p>
                <p>Stage: {item.stage}</p>
            </div>
        {/each}
    {:else}
        <div><span>Fail</span></div>
    {/if}
</div>
