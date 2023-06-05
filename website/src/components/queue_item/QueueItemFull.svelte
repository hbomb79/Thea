<script lang="ts">
    import { fade } from "svelte/transition";

    import { QueueStatus } from "queue";

    import { writable } from "svelte/store";
    import { selectedQueueItem } from "stores/item";
    import { itemDetails } from "stores/queue";
    import { commander } from "commander";
    import { SocketMessageType } from "stores/socket";
    import type { SocketData } from "stores/socket";

    import StageIcon from "components/StageIcon.svelte";
    import QueueStagePanel from "components/queue_item/QueueStagePanel.svelte";
    import FfmpegPanel from "components/queue_item/stage_panels/FfmpegPanel.svelte";
    import DatabasePanel from "components/queue_item/stage_panels/DatabasePanel.svelte";
    import OverviewPanel from "components/queue_item/stage_panels/OverviewPanel.svelte";
    import TitlePanel from "components/queue_item/stage_panels/TitlePanel.svelte";
    import OmdbPanel from "components/queue_item/stage_panels/OmdbPanel.svelte";
    import { Action } from "components/queue_item/QueueItemControls.svelte";

    import Splash from "./Splash.svelte";
    import InfoModal from "components/modal/InfoModal.svelte";

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

    let cancelConfirmModal: InfoModal;
    let showCancelConfirmModal = false;

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
        if (cancelConfirmModal) cancelConfirmModal.close();

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
                showCancelConfirmModal = true;
                break;
            case Action.NONE:
            default:
                console.warn(`Unknown item action ${action}`);
        }
    }
</script>

{#if details}
    <div class="queue-item">
        <Splash {details} queueControlCallback={handleItemAction} />
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
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
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

<InfoModal bind:this={cancelConfirmModal} bind:showModal={showCancelConfirmModal}>
    <span slot="header">Confirm Cancellation of Item <em>#{details.id}</em></span>

    <p>Upon cancellation, Thea will terminate any on-going transcodes and cleanup partially transcoded files.</p>
    <p>
        If you later decide you wish to process this item, you'll need to remove it from the list of 'Ignored Files' in
        Thea settings.
    </p>

    <button on:click={cancelItem}>Cancel Item</button>
</InfoModal>

<style lang="scss">
    .queue-item {
        flex: 1;
        text-align: left;
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
