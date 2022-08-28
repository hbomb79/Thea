<script lang="ts" context="module">
    export enum QueueState {
        SYNCED,
        REORDERING,
        SYNCING,
        FAILURE,
    }
</script>

<script lang="ts">
    import { onMount } from "svelte";

    import healthSvg from "../assets/health.svg";
    import { ContentManager } from "../queue";
    import type { TranscodeProfile } from "../queue";
    import type { QueueDetails, QueueItem } from "../queue";
    import QueueItemMini from "./QueueItemMini.svelte";
    import QueueItemFull from "./QueueItemFull.svelte";
    import QueueList from "./QueueList.svelte";
    import { commander, ffmpegOptionsStream } from "../commander";
    import { SocketMessageType } from "../store";
    import type { SocketData } from "../store";
    import ServerSettings from "./ServerSettings.svelte";
    import { fade } from "svelte/transition";
    import Modal from "svelte-simple-modal";
    import Viewer from "./Viewer.svelte";

    const comp = {
        optionElements: new Array(3),
        domCompleted: null,
        options: ["Home", "Queue", "Settings"],
        selectionOption: 0,
    };

    let queueState = QueueState.SYNCED;

    const contentManager = new ContentManager();

    let details: Map<number, QueueDetails> = null;
    let index: QueueItem[] = [];
    let profiles: TranscodeProfile[] = [];
    let selectedItem: number = -1;

    ffmpegOptionsStream.subscribe((options) => {
        console.log("ffmpeg options changed!", options);
    });

    const handleQueueReorder = (event: CustomEvent) => {
        index = event.detail;
        queueState = QueueState.REORDERING;
    };

    const commitQueueReorder = () => {
        console.warn(
            "Syncing queue index with server - new index: ",
            index,
            "flattened: ",
            index.flatMap((item) => item.id)
        );
        queueState = QueueState.SYNCING;
        commander.sendMessage(
            {
                title: "QUEUE_REORDER",
                type: SocketMessageType.COMMAND,
                arguments: {
                    index: index.flatMap((item) => item.id),
                },
            },
            (replyData: SocketData): boolean => {
                if (replyData.type == SocketMessageType.ERR_RESPONSE) {
                    console.warn("Queue reordering failed - requesting up-to-date index from server", replyData);
                    alert(`Failed to reorder queue: ${replyData.arguments.error}`);

                    contentManager.requestQueueIndex();
                }

                queueState = QueueState.SYNCED;
                return false;
            }
        );
    };

    const miniItemClick = (itemId: number) => {
        selectedItem = itemId;
        comp.selectionOption = 1;
    };

    onMount(() => {
        comp.optionElements.forEach((item: HTMLElement, index) => {
            item.addEventListener("click", () => (comp.selectionOption = index));
        });

        contentManager.itemDetails.subscribe((v) => (details = v));
        contentManager.itemIndex.subscribe((v) => {
            if (queueState == QueueState.REORDERING) {
                console.warn("Queue index change from server was IGNORED as queue is being reordered!");
                return;
            }

            queueState = QueueState.SYNCED;
            index = v;
        });
        contentManager.serverProfiles.subscribe((v) => {
            profiles = v;
        });
    });
</script>

<Modal>
    <div class="dashboard tiled-layout">
        <div class="wrapper" class:sidebar-open={comp.selectionOption == 1}>
            <div class="overflow-wrapper">
                <div class="options">
                    {#each comp.options as title, k}
                        <div class="option" class:active={comp.selectionOption == k} bind:this={comp.optionElements[k]}>
                            {title}
                        </div>
                    {/each}
                </div>

                <div class="respect-overflow">
                    <div class="sidebar">
                        {#if comp.selectionOption == 1}
                            <h2 class="header">Items</h2>
                            {#if queueState == QueueState.REORDERING || queueState == QueueState.SYNCING}
                                <button id="queue-reorder-commit" on:click={commitQueueReorder}>Save Order</button>
                            {/if}

                            <div class="queue-items" style="width: 350px;">
                                <QueueList {index} {details} bind:selectedItem on:index-reorder={handleQueueReorder} />
                            </div>
                        {/if}
                    </div>

                    <div class="tiles" class:full-size={comp.selectionOption == 1}>
                        {#if comp.selectionOption == 0}
                            <!-- <Viewer /> -->
                            <div class="column main">
                                <div class="tile overview" in:fade={{ duration: 250 }}>
                                    <h2 class="header">Overview</h2>
                                    <div class="content">
                                        <h2>System Health</h2>
                                        <p>All systems healthy</p>

                                        {@html healthSvg}
                                    </div>
                                </div>
                                <div class="tile status" in:fade={{ duration: 250, delay: 100 }}>
                                    <div class="content">
                                        <div class="mini-tile complete">
                                            <div class="main">
                                                <div class="progress" bind:this={comp.domCompleted} />
                                            </div>
                                            <p class="tag">Items Complete</p>
                                        </div>
                                        <div class="mini-tile trouble">
                                            <div class="main" />
                                            <p class="tag">Items Need Assistance</p>
                                        </div>
                                    </div>
                                </div>
                                <div class="tile workers" in:fade={{ duration: 250, delay: 150 }}>
                                    <h2 class="header">Workers</h2>
                                    <div class="content" style="min-height:230px;" />
                                </div>
                            </div>
                            <div class="column" in:fade={{ duration: 250, delay: 175 }}>
                                <div class="tile queue">
                                    <h2 class="header">Queue</h2>
                                    <div class="content">
                                        {#each index as item, k}
                                            <div in:fade={{ duration: 120, delay: 120 + k * 100 }}>
                                                <QueueItemMini
                                                    on:click={() => miniItemClick(item.id)}
                                                    queueDetails={details.get(item.id)}
                                                />
                                            </div>
                                        {/each}
                                    </div>
                                </div>
                            </div>
                        {:else if comp.selectionOption == 1}
                            {#if selectedItem > -1 && details.has(selectedItem)}
                                <QueueItemFull details={details.get(selectedItem)} />
                            {:else}
                                <h2>Select an item</h2>
                            {/if}
                        {:else if comp.selectionOption == 2}
                            <ServerSettings {profiles} {index} {details} />
                        {/if}
                    </div>
                </div>
            </div>
        </div>

        <footer>
            Made with ♥ by <a target="_new" href="https://github.com/hbomb79">hbomb79</a>
        </footer>
    </div>
</Modal>

<svelte:head>
    <style lang="scss" global>
        @use "../styles/tiled-layout.scss";
    </style>
</svelte:head>

<style lang="scss">
    @use "../styles/dashboard.scss";

    .overflow-wrapper {
        position: relative;
        height: 100%;

        .respect-overflow {
            position: relative;
            width: 100%;
            height: 100%;
            overflow: hidden;
            background: linear-gradient(122deg, #ffffffc7, #ffffff45);
            border: solid 1px #c4b8db;
            border-radius: 10px;
            box-shadow: 0px 0px 3px #0000000f;
        }
    }

    footer {
        position: fixed;
        bottom: 0;
        left: 0;
        right: 0;
        padding: 8px;
        color: #9285c5;
    }
</style>
