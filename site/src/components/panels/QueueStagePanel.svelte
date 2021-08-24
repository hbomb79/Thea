<script lang="ts">
import type { QueueDetails } from "../../queue";
import { QueueStatus, QueueStage } from "../../queue";
import TroublePanel from "./TroublePanel.svelte";

import pendingHtml from "../../assets/html/ellipsis.html";
import rippleHtml from "../../assets/html/ripple.html"


export let queueDetails: QueueDetails
export let stagePanel = null
export let stageIndex
</script>

{#if queueDetails.stage == stageIndex && queueDetails.status != QueueStatus.COMPLETED && queueDetails.status != QueueStatus.PROCESSING}
    <!-- We're viewing the page representing the current stage -->
    {#if queueDetails.status == QueueStatus.NEEDS_RESOLVING}
        <!-- Stage is troubled. Show the trouble panel -->
        <TroublePanel {queueDetails}/>
    {:else if queueDetails.status == QueueStatus.PENDING}
        <div class="pending tile">
            {#if queueDetails.trouble}
                <div class="resolving">
                    <p class="sub">This item has trouble resolution data attached and is waiting for confirmation of item progress</p>
                </div>
            {/if}
            <h2>This stage is queued</h2>
            <span>All workers for this stage are busy with other items - progress will appear here once a worker is available.</span>
            {@html pendingHtml}
        </div>
    {/if}
{:else if queueDetails.stage >= stageIndex || queueDetails.status == QueueStatus.PROCESSING}
    {#if stagePanel}
        <svelte:component this={stagePanel} details={queueDetails}/>
    {/if}
{:else}
    <div class="pending tile">
        <h2>This stage is scheduled</h2>
        <span>We're waiting on previous stages of the pipeline to succeed before we start this stage. Check the 'Overview' to track progress.</span>
        {@html rippleHtml}
    </div>
{/if}
