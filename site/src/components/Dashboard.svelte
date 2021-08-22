<script lang="ts">
import { onMount } from "svelte";
import Queue from "./Queue.svelte";
// import ProgressBar from 'progressbar.js'

import healthSvg from '../assets/health.svg';
import { QueueManager } from "../queue";
import type { QueueDetails, QueueItem } from "../queue";
import QueueItemMini from "./QueueItemMini.svelte";

const comp = {
    optionElements: new Array(3),
    domCompleted: null,
    options: ["Home", "Queue", "Settings"],
    selectionOption: 0,
}

let queue = new QueueManager()

let details: QueueDetails[]
let index: QueueItem[]
queue.itemDetails.subscribe((v) => details = v)
queue.itemIndex.subscribe((v) => index = v)

onMount(() => {
    comp.optionElements.forEach((item: HTMLElement, index) => {
        item.addEventListener("click", () => comp.selectionOption = index)
    })
})
</script>


<style lang="scss">
@use "../styles/dashboard.scss";
</style>

<div class="dashboard">
    <div class="wrapper">
        <div class="options">
            {#each comp.options as title, k}
                <div class="option" class:active={comp.selectionOption == k} bind:this={comp.optionElements[k]}>{title}</div>
            {/each}
        </div>

        <div class="tiles">
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
                <Queue queueIndex={index} queueDetails={details}/>
            {/if}
        </div>
    </div>
</div>
