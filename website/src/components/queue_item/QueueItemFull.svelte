<script lang="ts">
    import { fade } from "svelte/transition";
    import { getContext } from "svelte";

    import { QueueStatus } from "queue";

    import { writable } from "svelte/store";
    import { selectedQueueItem } from "stores/item";
    import { itemDetails } from "stores/queue";
    import { commander } from "commander";
    import { SocketMessageType } from "stores/socket";
    import type { SocketData } from "stores/socket";

    import StageIcon from "components/StageIcon.svelte";
    import ConfirmationPopup from "components/modals/ConfirmationPopup.svelte";
    import QueueStagePanel from "components/queue_item/QueueStagePanel.svelte";
    import FfmpegPanel from "components/queue_item/stage_panels/FfmpegPanel.svelte";
    import DatabasePanel from "components/queue_item/stage_panels/DatabasePanel.svelte";
    import OverviewPanel from "components/queue_item/stage_panels/OverviewPanel.svelte";
    import TitlePanel from "components/queue_item/stage_panels/TitlePanel.svelte";
    import OmdbPanel from "components/queue_item/stage_panels/OmdbPanel.svelte";
    import QueueItemControls, { Action } from "components/queue_item/QueueItemControls.svelte";

    import wavesSvg from "assets/waves.svg";

    const { open } = getContext<any>("simple-modal");

    interface Stage {
        label: string;
        tag: string;
        component: any;
    }

    /**
     * Component is responsible for rendering the full details of an in-progress
     * Thea item. Selected item is pulled from global 'selectedQueueItem' state, which
     * the Dashboard component mutates when the user item selection changes.
     *
     * The details of the selected item is pulled from the details store from within
     * the ContentManager
     */

    $: details = $itemDetails.get($selectedQueueItem);

    const openedDetailPanels = writable<Map<string, boolean>>(new Map());
    const detailPanels: Stage[] = [
        { label: "Importer", tag: "import", component: null },
        { label: "Title", tag: "title", component: TitlePanel },
        { label: "OMDB", tag: "omdb", component: OmdbPanel },
        { label: "FFmpeg", tag: "ffmpeg", component: FfmpegPanel },
        { label: "DB", tag: "db", component: DatabasePanel },
    ];

    function toggleDetailPanel(stage: string) {
        openedDetailPanels.update((m) => {
            const current = m.get(stage);
            return m.set(stage, !current);
        });
    }

    function sendCommand(
        command: string,
        successCallback: (arg0: SocketData) => void,
        errorCallback: (arg0: SocketData) => void
    ) {
        commander.sendMessage(
            {
                type: SocketMessageType.COMMAND,
                title: command,
                arguments: { id: details.id },
            },
            (reply: SocketData): boolean => {
                if (reply.type == SocketMessageType.ERR_RESPONSE) {
                    errorCallback(reply);
                } else {
                    successCallback(reply);
                }

                return false;
            }
        );
    }

    function promoteItem() {
        sendCommand(
            "PROMOTE_ITEM",
            (successData) => {
                console.log("Promotion success!");
            },
            (errData) => {
                alert(`Failed to promote item: ${errData.title}: ${errData.arguments.error}`);
            }
        );
    }

    function pauseItem() {
        sendCommand(
            "PAUSE_ITEM",
            (successData) => {
                console.log("Pause success!");
            },
            (errData) => {
                alert(`Failed to pause item: ${errData.title}: ${errData.arguments.error}`);
            }
        );
    }

    function cancelItem() {
        console.log("CANCELLING ITEM");
        sendCommand(
            "CANCEL_ITEM",
            (successData) => {
                console.log("Cancellation success!");
            },
            (errData) => {
                alert(`Failed to cancel item: ${errData.title}: ${errData.arguments.error}`);
            }
        );
    }

    function handleItemAction(event: CustomEvent) {
        console.log(event);
        const action = event.detail as Action;
        switch (action) {
            case Action.PROMOTE:
                promoteItem();
                break;
            case Action.PAUSE:
                pauseItem();
                break;
            case Action.CANCEL:
                open(
                    ConfirmationPopup,
                    {
                        title: "Cancel Item",
                        body: "Are you sure you wish to cancel this item?<br/><br/><b>All progress will be lost and the item will be removed from the queue.</b><br/><br/><i>This action cannot be reversed, however if you later wish to process this item, remove it from the server cache (go to Settings > Cache > Edit Cache).</i>",
                        onOkay: cancelItem,
                    },
                    { closeButton: false }
                );
                break;
            case Action.NONE:
            default:
                console.warn(`Unknown item action ${action}`);
        }
    }
</script>

{#if details}
    <div class="queue-item">
        <div
            class="splash"
            class:trouble={details.status == QueueStatus.NEEDS_ATTENTION ||
                details.status == QueueStatus.NEEDS_RESOLVING}
            in:fade={{ duration: 150, delay: 50 }}
        >
            <div class="waves">{@html wavesSvg}</div>
            <div class="content">
                <h2 class="title">
                    {details.omdb_info?.Title || details.title_info?.Title || details.name || "UNNAMED"}
                    <span class="id">#{details.id}</span>
                </h2>
                <p class="sub">Item Status</p>

                <QueueItemControls on:queue-control={handleItemAction} />
            </div>
        </div>

        <div class="main">
            <h2 class="tile-title">Pipeline</h2>
            <div class="item pipeline">
                {#key details.id}
                    <OverviewPanel />
                {/key}
            </div>

            <h2 class="tile-title">Stage Details</h2>
            {#each detailPanels as { tag, label, component }, k (tag)}
                <div
                    class={`item stage ${tag}`}
                    class:content-open={$openedDetailPanels.get(tag)}
                    in:fade={{ duration: 150, delay: 50 + k * 50 }}
                >
                    <div class="header" on:click={() => toggleDetailPanel(tag)}>
                        <h2>{label}</h2>
                        {#key details.id}
                            <div class="check" in:fade={{ duration: 300, delay: 100 + k * 50 }}>
                                <StageIcon {details} stageIndex={k} />
                            </div>
                        {/key}
                    </div>

                    {#if $openedDetailPanels.get(tag)}
                        <div
                            class="content"
                            class:troubled={details.stage == k &&
                                (details.status == QueueStatus.NEEDS_RESOLVING ||
                                    details.status == QueueStatus.NEEDS_ATTENTION)}
                        >
                            <QueueStagePanel queueDetails={details} stageIndex={k} stagePanel={component} />
                        </div>
                    {/if}
                </div>
            {/each}
        </div>
    </div>
{/if}

<style lang="scss">
    .queue-item {
        flex: 1;
        text-align: left;

        @import "../../styles/waves.scss";

        .main {
            padding: 1rem 2rem;
            max-width: 1100px;
            margin: 0 auto;

            .tile-title {
                font-size: 1rem;
                color: #9e94c5;
                font-weight: 500;
            }

            @import "../../styles/queueItem.scss";
            .item {
                background: #f3f5fe;
                border-radius: 5px;
                border: solid 1px #d4d6e1;
                margin: 0 0 1.2rem 0;
                max-width: none;

                &.pipeline {
                    text-align: center;
                    padding: 1.4rem;
                }

                &.stage {
                    .header {
                        padding: 4px 1rem;
                        background: none;
                        cursor: pointer;

                        transition: all 200ms ease-in-out;
                        transition-property: background border-bottom box-shadow;

                        width: unset;
                        display: flex;
                        flex-direction: row;
                        align-items: center;
                        position: relative;

                        &:hover {
                            background: white;
                        }

                        .check {
                            position: absolute;
                            right: 1rem;
                            width: 40px;
                        }
                    }

                    .content:not(.troubled) {
                        border-top: solid 1px #e0e3fc;
                    }

                    &.content-open .header {
                        background: white;
                    }
                }
            }
        }
    }
</style>
