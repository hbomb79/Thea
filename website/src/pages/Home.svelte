<script lang="ts">
    import { fade } from "svelte/transition";
    import healthSvg from "assets/health.svg";
    import { itemIndex } from "stores/queue";
    import QueueItemMini from "components/QueueItemMini.svelte";
    import { selectedQueueItem } from "stores/item";
    import { navigate } from "svelte-routing";

    function miniItemClick(itemId: number) {
        selectedQueueItem.set(itemId);
        navigate('/queue');
    }
</script>

<div class="tiles">
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
</div>

<style lang="scss">
.overview {
    margin-bottom: 2rem;
    .content {
        height: 180px;
        background: linear-gradient(28deg, #6ca8ff, #ef8dff)!important;
        position: relative;
        box-shadow: 0px 2px 6px -1px rgba(0,0,0,0.2);

        h2 {
            padding: 2rem 0 0 2rem;
            color: white;
            margin: 0;
            font-size: 2rem;
        }

        p {
            margin: 0 0 0 2rem;
            color: #bfd9ff;
            font-size: 1.1rem;
        }

        :global(svg) {
            height: 100px;
            fill: white;
            position: absolute;
            right: 2.8rem;
            top: 2.4rem;
            width: auto;
        }
    }
}

.status {
    margin-bottom: 2rem;

    .content {
        background: none!important;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        box-shadow: none!important;

        .mini-tile {
            width: 40%;
            height: 6rem;
            padding: 1rem;
            display: flex;
            flex-direction: row;

            .main {
                width: 35%;
                height: 100%;
                flex-grow: 0;
                display: flex;
                flex-direction: column;
                justify-content: center;

                .progress {
                    position: relative;
                    flex-grow: 0;
                }
            }

            .tag {
                flex-grow: 1;
                align-self: center;
                text-align: center;
            }
        }
    }
}

.tile.queue {
    display: flex;
    flex-direction: column;

    .content{
        background: none !important;
        box-shadow: none !important;
        padding: 0;
    }
}
</style>