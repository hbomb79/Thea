<script lang="ts">
import Item from './QueueItem.svelte';
import type { QueueDetails, QueueItem } from '../queue';

export let queueIndex: QueueItem[] = []
export let queueDetails: Map<number, QueueDetails> = null
</script>

<style lang="scss">

.queue {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;

    .wrapper {
        width: 90%;
        min-width: 780px;
        margin: 0 auto;
        max-width: 950px;
    }
}
</style>

<div class="queue">
    <div class="wrapper">
        {#each queueIndex as item (item.id)}
            <Item queueDetails={queueDetails[item.id]} />
        {/each}
    </div>
</div>
