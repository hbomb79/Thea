<script lang="ts">
    import { queueState } from "stores/queue";
    import { QueueState } from "queueOrderManager";
    import { contentManager } from "../queue";
    import { selectedQueueItem } from "stores/item";
    import QueueList from "components/queue_list/QueueList.svelte";
    import QueueItemFull from "components/queue_item/QueueItemFull.svelte";
</script>

<div class="queue-page">
    <div class="sidebar">
        <h2 class="header">Items</h2>
        {#if $queueState == QueueState.REORDERING || $queueState == QueueState.SYNCING}
            <button
                id="queue-reorder-commit"
                on:click={() => contentManager.queueOrderManager.commitReorder()}
            >
                Save Order
            </button>
        {/if}

        <div class="queue-items">
            <QueueList
                on:index-reorder={(event) =>
                    contentManager.queueOrderManager.handleReorder(event.detail)}
            />
        </div>
    </div>

    <div class="tiles">
        {#if $selectedQueueItem != -1}
            <QueueItemFull />
        {:else}
            <h2 class="select-message">Select an item</h2>
        {/if}
    </div>
</div>

<style lang="scss">
$sidebarWidth: 350px;

.queue-page {
    display: flex;
    height: 100%;
}

.sidebar {
    width: $sidebarWidth;
    height: 100%;
    box-shadow: -4px 0px 8px #0000004d;
    background: #ffffff3f;
    border-right: solid 1px #bfbfbf5f;
    overflow: hidden;

    transition: all 150ms ease-in-out;
    transition-property: opacity, width;

    h2 {
        color: #9184c5;
        text-align: left;
        padding: 2rem 0 0.4rem 2rem;
        margin: 0;
        font-weight: 400;
    }

    button {
        position: absolute;
        right: 1.3rem;
        top: 2rem;
    }
}

.tiles {
    padding: 0;
}

.select-message {
    padding: 1rem;
}
</style>