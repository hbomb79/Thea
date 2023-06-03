<script lang="ts">
    import { QueueStatus } from "../queue";
    import type { QueueDetails } from "../queue";
    import { createEventDispatcher } from "svelte";
    import { itemDetails } from "../stores/queue";
    const dispatch = createEventDispatcher();

    export let queueItemID: number;

    $: queueDetails = $itemDetails.get(queueItemID);
    $: getStatusClass = () => {
        switch (queueDetails?.status) {
            case QueueStatus.PENDING:
                return "pending";
            case QueueStatus.PROCESSING:
                return "processing";
            case QueueStatus.CANCELLING:
                return "cancelling";
            case QueueStatus.CANCELLED:
                return "cancelled";
            case QueueStatus.NEEDS_RESOLVING:
                return "troubled";
        }
    };
</script>

<!-- Template -->
<div>
    {#if queueDetails}
        <p on:click|stopPropagation={() => dispatch("click")}>
            <span class={`status ${getStatusClass()}`} />
            <span class="name"
                >{queueDetails.omdb_info?.Title || queueDetails.title_info?.Title || queueDetails.name}</span
            >
        </p>
    {/if}
</div>

<style lang="scss">
    @use "../styles/global.scss";

    p {
        padding: 1rem;
        background: white;
        margin: 0rem;
        color: #615a7c;
        border-radius: 4px;
        margin-bottom: 0.7rem;
        box-shadow: 0px 0px 3px rgb(0 0 0 / 10%);
        cursor: pointer;

        .status {
            background: #39d3fd96;
            height: 12px;
            width: 12px;
            display: inline-block;
            border-radius: 100%;
            margin-right: 8px;

            &.pending {
                background: global.$pendingColour;
            }
            &.processing {
                background: global.$processingColour;
            }
            &.cancelling,
            &.cancelled {
                background: global.$cancelledColour;
                @include global.pulse-keyframe(global.$cancelledColour);
                animation: pulse infinite 2s;
            }
            &.troubled {
                background: global.$troubleColour;
                @include global.pulse-keyframe(global.$troubleColour);
                animation: pulse infinite 2s;
            }
        }
    }
</style>
