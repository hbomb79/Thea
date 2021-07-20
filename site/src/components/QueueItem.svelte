<script lang="ts">
import type { QueueItem } from "./Queue.svelte";


export let queueDetails:QueueItem
</script>

<style lang="scss">
</style>

<!-- Template -->
<div>
    <span>{queueDetails.id}</span>
    <h2>{queueDetails.name}</h2>
    <p>{queueDetails.stage}: {queueDetails.status}</p>
</div>
