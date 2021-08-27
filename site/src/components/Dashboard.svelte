<script lang="ts">
import { onMount } from "svelte";

import healthSvg from '../assets/health.svg';
import { QueueManager } from "../queue";
import type { QueueDetails, QueueItem } from "../queue";
import QueueItemMini from "./QueueItemMini.svelte";
import QueueListItem from "./QueueListItem.svelte";
import QueueItemFull from "./QueueItemFull.svelte";

const comp = {
    optionElements: new Array(3),
    domCompleted: null,
    options: ["Home", "Queue", "Settings"],
    selectionOption: 0,
}

let queue = new QueueManager()
let details: Map<number, QueueDetails> = null
let index: QueueItem[] = []
let selectedItem: number = null

onMount(() => {
    comp.optionElements.forEach((item: HTMLElement, index) => {
        item.addEventListener("click", () => comp.selectionOption = index)
    })

    queue.itemDetails.subscribe((v) => details = v)
    queue.itemIndex.subscribe((v) => index = v)
})
</script>


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
</style>

<div class="dashboard">
    <div class="wrapper" class:sidebar-open={comp.selectionOption == 1}>
        <div class="overflow-wrapper">
            <div class="options">
                {#each comp.options as title, k}
                    <div class="option" class:active={comp.selectionOption == k} bind:this={comp.optionElements[k]}>{title}</div>
                {/each}
            </div>

            <div class="respect-overflow">
                <div class="sidebar">
                    {#if comp.selectionOption == 1}
                        <h2 class="header">Items</h2>

                        <div class="queue-items">
                            {#each index as item (item.id)}
                                <QueueListItem selectedItem={selectedItem} on:selected={(event) => selectedItem = event.detail} details={details[item.id]}/>
                            {/each}
                        </div>
                    {/if}
                </div>

                <div class="tiles" class:full-size={comp.selectionOption == 1}>
                    {#if comp.selectionOption == 0}
                        <div class="column main">
                            <div class="tile overview">
                                <h2 class="header">Overview</h2>
                                <div class="content">
                                    <h2>System Health</h2>
                                    <p>All systems healthy</p>

                                    {@html healthSvg}
                                </div>
                            </div>
                            <div class="tile status">
                                <div class="content">
                                    <div class="mini-tile complete">
                                        <div class="main">
                                            <div class="progress" bind:this={comp.domCompleted}></div>
                                        </div>
                                        <p class="tag">Items Complete</p>
                                    </div>
                                    <div class="mini-tile trouble">
                                        <div class="main"></div>
                                        <p class="tag">Items Need Assistance</p>
                                    </div>
                                </div>
                            </div>
                            <div class="tile workers">
                                <h2 class="header">Workers</h2>
                                <div class="content" style="min-height:230px;">

                                </div>
                            </div>
                        </div>
                        <div class="column">
                            <div class="tile queue">
                                <h2 class="header">Queue</h2>
                                <div class="content">
                                    {#each index as item}
                                        <QueueItemMini queueDetails={details[item.id]}/>
                                    {/each}
                                </div>
                            </div>
                        </div>
                    {:else if comp.selectionOption == 1}
                        {#if details[selectedItem]}
                            <QueueItemFull details={details[selectedItem]}/>
                        {:else}
                            <h2>Select an item</h2>
                        {/if}
                    {/if}
                </div>
            </div>
        </div>
    </div>
</div>
