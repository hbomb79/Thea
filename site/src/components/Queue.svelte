<script lang="ts">
import Item from './QueueItem.svelte';
import type { QueueDetails, QueueItem } from '../queue';

export let queueIndex: QueueItem[] = []
export let queueDetails: QueueDetails[] = []
</script>

<style lang="scss">

.sidebar {
    width: 250px;
    height: 100%;
    background: #ffffff3f;
    border-right: solid 1px #bfbfbf5f;
    position: relative;

    h2 {
        color: #9184c5;
        text-align: left;
        padding: 2rem 0 1rem 2rem;
        margin: 0;
    }
}

.queue {
    position: absolute;
    top: 0;
    left: 250px;
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

<div class="sidebar">
    <h2 class="header">Items</h2>
</div>
<div class="queue">
    <div class="wrapper">
        {#each queueIndex as item (item.id)}
            <Item queueDetails={queueDetails[item.id]} />
        {/each}
    </div>
</div>
