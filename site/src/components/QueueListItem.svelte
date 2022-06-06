<script lang="ts">
    import { QueueStatus } from "../queue";
    import type { QueueDetails } from "../queue";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    export let selectedItem: number = null;
    export let details: QueueDetails = null;
    $: getStatusClass = () => {
        switch (details?.status) {
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

{#if details}
    <div class="item" on:click={() => dispatch("selected", details.id)} class:active={selectedItem == details?.id}>
        <span class={`status ${getStatusClass()}`} />

        {#if details.omdb_info?.Title}
            {details.omdb_info.Title}
        {:else if details.title_info?.Title}
            {details.title_info.Title}
        {:else}
            {details.name}
        {/if}

        {#if details.title_info && details.title_info.Episodic}
            <span class="season">S{details.title_info.Season}E{details.title_info.Episode}</span>
        {/if}
    </div>
{/if}

<style lang="scss">
    @use "../styles/global.scss";
    .item {
        padding: 1rem;
        color: #9696a5;
        cursor: pointer;
        text-align: left;
        border-radius: 7px;
        background: #e9eaef54;
        margin: 6px 1rem;
        border: solid 1px #8c91b938;
        transition: all 200ms;
        transition-property: background, border, box-shadow, color;

        &:hover {
            background: #ffffff85;
        }

        &.active {
            background: white;
            box-shadow: 0px 0px 7px -5px black;
            color: #8e82bf;
        }

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
