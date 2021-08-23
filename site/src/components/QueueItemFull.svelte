<script lang="ts">
import type { QueueDetails } from "../queue";

export let details: QueueDetails = null;
</script>


{#if details}
    <h2>{details.name}</h2>
{:else}
{/if}
