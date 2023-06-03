<script lang="ts">
    import type { QueueDetails } from "../../queue";
    import { QueueStatus, QueueStage } from "../../queue";
    import TroublePanel from "./TroublePanel.svelte";

    import pendingHtml from "../../assets/html/ellipsis.html";
    import rippleHtml from "../../assets/html/ripple.html";

    export let queueDetails: QueueDetails;
    export let stagePanel = null;
    export let stageIndex;
</script>

{#if queueDetails.stage == stageIndex && queueDetails.status != QueueStatus.COMPLETED && queueDetails.status != QueueStatus.PROCESSING}
    <!-- We're viewing the page representing the current stage -->
    {#if queueDetails.status == QueueStatus.NEEDS_RESOLVING && queueDetails.stage != QueueStage.FFMPEG}
        <!-- Stage is troubled. -->
        <TroublePanel {queueDetails} />
    {:else if queueDetails.status == QueueStatus.PENDING}
        <div class="pending tile">
            <div class="main">
                {#if queueDetails.trouble}
                    <div class="resolving">
                        <p class="sub">
                            This item has trouble resolution data attached and is waiting for confirmation of item
                            progress
                        </p>
                    </div>
                {/if}
                <h2>This stage is queued</h2>
                <span>
                    All workers for this stage are busy with other items - progress will appear here once a worker is
                    available.
                </span>
            </div>
            <div class="sub">
                {@html rippleHtml}
            </div>
        </div>
    {:else}
        <svelte:component this={stagePanel} />
    {/if}
{:else if queueDetails.stage >= stageIndex || queueDetails.status == QueueStatus.PROCESSING}
    {#if stagePanel}
        <svelte:component this={stagePanel} />
    {:else}
        <h2>Error: No stage panel defined for stageIndex {stageIndex}</h2>
    {/if}
{:else}
    <div class="pending tile">
        <div class="main">
            <h2>This stage is scheduled</h2>
            <span>
                We're waiting on previous stages of the pipeline to succeed before we start this stage. Check the
                'Overview' to track progress.
            </span>
        </div>
        <div class="sub">
            {@html rippleHtml}
        </div>
    </div>
{/if}

<style lang="scss">
    .resolving {
        width: 80%;
        margin: 0 auto 2rem auto;
        background: linear-gradient(326deg, #d9b6ea5f, #bfd9ff5f);
        border: solid 1px #a99dfb;
        border-radius: 3px;
        padding: 0rem;
        color: #5e6495;
    }

    .pending.tile {
        display: flex;
        flex-direction: row-reverse;
        padding: 2rem;

        .main {
            flex: 1;
            text-align: center;

            h2 {
                margin-top: 0;
                color: #8c91b9;
            }

            span {
                font-style: italic;
            }
        }

        .sub {
            color: #96b4fd;
        }
    }
</style>
