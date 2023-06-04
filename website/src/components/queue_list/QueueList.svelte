<script lang="ts">
    import { createEventDispatcher } from "svelte";

    import QueueListItem from "components/queue_list/QueueListItem.svelte";
    import ReorderableList from "components/ReorderableList.svelte";
    import { selectedQueueItem } from "stores/item";
    import { itemIndex } from "stores/queue";

    const dispatch = createEventDispatcher();

    const reorderIndex = (event: CustomEvent) => {
        dispatch("index-reorder", event.detail);
    };
</script>

{#if $itemIndex}
    <div class="queue-list">
        <ReorderableList key={(item) => item.id} list={$itemIndex} let:item on:reordered={reorderIndex}>
            <QueueListItem on:selected={(event) => selectedQueueItem.set(event.detail)} itemID={item.id} />
        </ReorderableList>
    </div>
{/if}
