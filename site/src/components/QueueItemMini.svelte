<script lang="ts">
import { QueueStatus } from "../queue";
import type { QueueDetails } from "../queue";

export let queueDetails:QueueDetails
const getStatusClass = () => {
    switch(queueDetails.status) {
        case QueueStatus.PENDING:
            return "pending"
        case QueueStatus.PROCESSING:
            return "processing"
        case QueueStatus.CANCELLING:
            return "cancelling"
        case QueueStatus.CANCELLED:
            return "cancelled"
        case QueueStatus.NEEDS_RESOLVING:
            return "troubled"
    }
}
</script>

<style lang="scss">
@use '../styles/global.scss';

$pendingColour: #cabdff;
$processingColour: #39d3fd96;
$cancelledColour: #ffc297;
$troubleColour: #f76c6c;

@mixin pulse-keyframe($color) {
    @keyframes pulse {
        0% {
            transform: scale(0.9);
            box-shadow: 0 0 0 $color;
        }
        70% {
            transform: scale(1);
            box-shadow: 0 0 10px rgba($color: $color, $alpha: 0.6);
        }
        90% {
            box-shadow: 0 0 15px rgba($color: $color, $alpha: 0);
        }
        100% {
            transform: scale(0.9);
        }
    }
}

p {
    padding: 1rem;
    border-bottom: solid 1px #cec9e7;
    margin: 0rem;
    color: #615a7c;

    .status {
        background: #39d3fd96;
        height: 12px;
        width: 12px;
        display: inline-block;
        border-radius: 100%;
        margin-right: 8px;

        &.pending {
            background: $pendingColour;
        }
        &.processing {
            background: $processingColour;
        }
        &.cancelling, &.cancelled {
            background: $cancelledColour;
            @include pulse-keyframe($cancelledColour);
            animation: pulse infinite 2s;
        }
        &.troubled {
            background: $troubleColour;
            @include pulse-keyframe($troubleColour);
            animation: pulse infinite 2s;
        }

    }
}
</style>

<!-- Template -->
<div>
    {#if queueDetails}
        <p>
            <span class={`status ${getStatusClass()}`}></span>
            <span class="name">{queueDetails.omdb_info?.Title || queueDetails.title_info?.Title || queueDetails.name}</span>
        </p>
    {/if}
</div>
