<script lang="ts">
import { createEventDispatcher } from "svelte";
import DraggableList from "svelte-dragdrop-list";

import type { QueueDetails, QueueItem } from "../queue";
import QueueListItem from "./QueueListItem.svelte";

const dispatch = createEventDispatcher()

export let index: QueueItem[] = null
export let details: Map<number, QueueDetails> = null
export let selectedItem: number = null

$:onChange(selectedItem)
const onChange = (item: number) => {
    dispatch("selection-change", item)
}

const reorderIndex = (event: CustomEvent) => {
    dispatch("index-reorder", event.detail)
}
</script>

{#if index && details}
<div class="queue-list">
    <DraggableList key={item => item.id} list={index} let:item on:reordered={reorderIndex}>
        <QueueListItem selectedItem={selectedItem} on:selected={(event) => selectedItem = event.detail} details={details.get(item.id)}/>
    </DraggableList>
</div>
{/if}
