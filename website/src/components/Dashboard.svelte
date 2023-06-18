<script lang="ts">
    import { fade } from "svelte/transition";
    import Modal from "svelte-simple-modal";

    import { writable } from "svelte/store";
    import { itemIndex, queueState } from "stores/queue";
    import { selectedQueueItem } from "stores/item";
    import { contentManager } from "queue";
    import { QueueState } from "queueOrderManager";

    import QueueItemMini from "components/QueueItemMini.svelte";
    import QueueItemFull from "components/queue_item/QueueItemFull.svelte";
    import QueueList from "components/queue_list/QueueList.svelte";
    import ServerSettings from "pages/ServerSettings.svelte";

    import healthSvg from "assets/health.svg";

    type DashboardPage = "Home" | "Queue" | "Settings";

    const dashboardOptions: DashboardPage[] = ["Home", "Queue", "Settings"];
    const selectedDashboardPage = writable<DashboardPage>("Home");

    function miniItemClick(itemId: number) {
        selectedQueueItem.set(itemId);
        selectedDashboardPage.set("Queue");
    }
</script>

<Modal>
    <div class="dashboard tiled-layout">
        <div class="wrapper" class:sidebar-open={$selectedDashboardPage === "Queue"}>
            <div class="overflow-wrapper">
                <div class="options">
                    {#each dashboardOptions as label}
                        <div
                            class="option"
                            class:active={$selectedDashboardPage == label}
                            on:click={() => selectedDashboardPage.set(label)}
                        >
                            {label}
                        </div>
                    {/each}
                </div>

                <div class="respect-overflow">
                    <div class="sidebar">
                        {#if $selectedDashboardPage == "Queue"}
                            <h2 class="header">Items</h2>
                            {#if $queueState == QueueState.REORDERING || $queueState == QueueState.SYNCING}
                                <button
                                    id="queue-reorder-commit"
                                    on:click={() => contentManager.queueOrderManager.commitReorder()}
                                >
                                    Save Order
                                </button>
                            {/if}

                            <div class="queue-items" style="width: 350px;">
                                <QueueList
                                    on:index-reorder={(event) =>
                                        contentManager.queueOrderManager.handleReorder(event.detail)}
                                />
                            </div>
                        {/if}
                    </div>

                    <div class="tiles" class:full-size={$selectedDashboardPage == "Queue"}>
                        {#if $selectedDashboardPage == "Home"}
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
                                                <div class="progress" />
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
                                        {#if $itemIndex !== undefined}
                                            {#each $itemIndex as item, k}
                                                <div in:fade={{ duration: 120, delay: 120 + k * 100 }}>
                                                    <QueueItemMini
                                                        on:click={() => miniItemClick(item.id)}
                                                        queueItemID={item.id}
                                                    />
                                                </div>
                                            {/each}
                                        {/if}
                                    </div>
                                </div>
                            </div>
                        {:else if $selectedDashboardPage == "Queue"}
                            {#if $selectedQueueItem != -1}
                                <QueueItemFull />
                            {:else}
                                <h2>Select an item</h2>
                            {/if}
                        {:else if $selectedDashboardPage == "Settings"}
                            <ServerSettings />
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
