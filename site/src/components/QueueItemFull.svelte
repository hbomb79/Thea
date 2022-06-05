<script lang="ts">
    import type { QueueDetails } from "../queue";
    import { QueueStatus } from "../queue";

    import wavesSvg from "../assets/waves.svg";
    import OverviewPanel from "./panels/OverviewPanel.svelte";
    import TitlePanel from "./panels/TitlePanel.svelte";
    import OmdbPanel from "./panels/OmdbPanel.svelte";
    import FfmpegPanel from "./panels/FfmpegPanel.svelte";
    import DatabasePanel from "./panels/DatabasePanel.svelte";
    import QueueItemControls, { Action } from "./QueueItemControls.svelte";
    import StageIcon from "./StageIcon.svelte";
    import QueueStagePanel from "./panels/QueueStagePanel.svelte";
    import { commander } from "../commander";
    import { SocketMessageType } from "../store";
    import type { SocketData } from "../store";
    import { fade } from "svelte/transition";
    import { getContext } from "svelte";
    import ConfirmationPopup from "./modals/ConfirmationPopup.svelte";

    const { open } = getContext("simple-modal");

    export let details: QueueDetails = null;
    const stages = [
        ["Importer", "import", null],
        ["Title Parser", "title", TitlePanel],
        ["OMDB Queryer", "omdb", OmdbPanel],
        ["FFmpeg Transcoder", "ffmpeg", FfmpegPanel],
        ["Database Committer", "db", DatabasePanel],
    ];

    const openStages: boolean[] = new Array(stages.length);

    $: detailsChanged(details);

    // Called automatically by Svelte when the details provided to
    // this component change.
    const detailsChanged = (newDetails: QueueDetails) => {
        console.log(newDetails);
    };

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
                        onOkay: () => cancelItem,
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
        <div class="splash" in:fade={{ duration: 150, delay: 50 }}>
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
                {#key details}
                    <OverviewPanel {details} />
                {/key}
            </div>

            <h2 class="tile-title">Stage Details</h2>
            {#each stages as [display, tag, component], k (tag)}
                <div
                    class={`item stage ${tag}`}
                    class:content-open={openStages[k]}
                    in:fade={{ duration: 150, delay: 50 + k * 50 }}
                >
                    <div class="header" on:click={() => (openStages[k] = !openStages[k])}>
                        <h2>{display}</h2>
                        {#key details}
                            <div class="check" in:fade={{ duration: 300, delay: 100 + k * 50 }}>
                                <StageIcon {details} stageIndex={k} />
                            </div>
                        {/key}
                    </div>

                    {#if openStages[k]}
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

        .splash {
            height: 250px;
            position: sticky;
            top: 0;
            overflow: hidden;
            background: linear-gradient(28deg, #6ca8ff, #ef8dff);
            box-shadow: 0 -5px 9px 0px #0000003d;
            z-index: 100;

            .waves {
                position: absolute;
                bottom: -350px;
                width: 1850px;
                z-index: 1;
                opacity: 0.4;
            }

            .content {
                position: absolute;
                width: 100%;
                height: 100%;
                z-index: 2;

                .title {
                    padding: 0 3rem;
                    color: white;
                    font-size: 2rem;
                    margin-bottom: 0;

                    .id {
                        font-size: 1rem;
                        font-weight: 400;
                        color: #ffffff7a;
                        margin-left: -2px;
                        font-style: italic;
                    }
                }

                .sub {
                    padding: 0 3rem;
                    color: #bfd9ff;
                    margin-top: 0;
                }
            }
        }

        .main {
            padding: 1rem 2rem;
            max-width: 1100px;
            margin: 0 auto;

            .tile-title {
                font-size: 1rem;
                color: #9e94c5;
                font-weight: 500;
            }

            @import "../styles/queueItem.scss";
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
