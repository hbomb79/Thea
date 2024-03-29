<script lang="ts">
    import { createEventDispatcher } from "svelte";

    import { QueueStatus } from "queue";

    import { selectedQueueItem } from "stores/item";
    import { itemDetails } from "stores/queue";

    const dispatch = createEventDispatcher();

    export let itemID: number = undefined;
    $: details = $itemDetails.get(itemID);

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
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div class="item" on:click={() => dispatch("selected", details.id)} class:active={$selectedQueueItem == details.id}>
        <span class={`status ${getStatusClass()}`} />

        <span class="title">
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
        </span>
    </div>
{/if}

<style lang="scss">
    @use "../../styles/global.scss";
    .item {
        padding: 1rem;
        color: #9696a5;
        cursor: pointer;
        text-align: left;
        border-radius: 5px;
        background: #e9eaef54;
        margin: 6px 1rem;
        border: solid 1px #8c91b938;
        transition: all 200ms;
        transition-property: background, border, box-shadow, color;

        overflow: hidden;
        text-overflow: ellipsis;

        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;
        align-items: center;

        &:hover {
            background: #ffffff85;
        }

        &.active {
            background: white;
            box-shadow: 0px 0px 5px -3px rgba(0, 0, 0, 0.2);
            font-weight: 700;
            color: #8e82bf;
            border-color: #8e82bf57;
        }

        .title {
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .status {
            background: #39d3fd96;
            height: 12px;
            width: 12px;
            display: inline-block;
            border-radius: 100%;
            margin-right: 12px;
            flex: none;

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
