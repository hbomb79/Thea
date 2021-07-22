<script lang="ts">
import importIcon from '../../assets/import-stage.svg';
import titleIcon from '../../assets/title-stage.svg';
import omdbIcon from '../../assets/omdb-stage.svg';
import ffmpegIcon from '../../assets/ffmpeg-stage.svg';
import dbIcon from '../../assets/db-stage.svg';
import ellipsisHtml from '../../assets/html/ellipsis.html';
import workingHtml from '../../assets/html/dual-ring.html';
import errHtml from '../../assets/err.svg';
import checkHtml from '../../assets/check-mark.svg';
import pendingHtml from '../../assets/pending.svg';
import { createEventDispatcher, onMount } from "svelte";
import type { QueueDetails } from '../QueueItem.svelte';

export let details:QueueDetails;
const els:HTMLElement[] = new Array(4);
const checkEls:HTMLElement[] = new Array(4);

let perspectiveContainer:HTMLElement;
const dispatch = createEventDispatcher();
onMount(() => {
    els.forEach(el => {
        el.addEventListener('mousemove', (e) => handleMouseOver(el, e))
        el.addEventListener('mouseleave', (e) => handleMouseLeave(el))
    })
})

const handleMouseLeave = function(el:HTMLElement) {
    el.style.transform = `rotate3d(0,0,0,0)`
    perspectiveContainer.style.perspective = ''
    perspectiveContainer.style.perspectiveOrigin = ''
}

const handleMouseOver = function(el:HTMLElement, ev:MouseEvent) {
    // transform: rotate3d(1, 1, 0, 21deg)
    // First, convert the mouse position to a value
    // between 1 and 0 relative to the coords of the
    // element
    const {offsetX, offsetY} = ev
    const midX = el.offsetWidth / 2
    const midY = el.offsetHeight / 2

    // Center the camera over the div we're rotating
    perspectiveContainer.style.perspectiveOrigin = `${el.offsetLeft + midX}px center`
    perspectiveContainer.style.perspective = "400px";

    const vecX = (midX - offsetX) / el.offsetWidth * 2
    const vecY = (midY - offsetY) / el.offsetHeight * 2

    el.style.transform = `rotate3d(${-vecY}, ${vecX}, 0, -10deg)`
    // console.log(rot)
}

const handleStageClick = function(e:Event) {
    const t = e.target as HTMLElement;
    els.every((el, i) => {
        if(el.isSameNode(t)) {
            dispatch('stage-click', i)
            return false
        }

        return true
    })
}

$:getCheckClass = function(checkIndex:number):string {
    if(checkIndex < details.stage) {
        return 'complete'
    } else if(checkIndex == details.stage) {
        return details.trouble ? 'trouble' : (details.status == 0 ? 'pending' : 'working')
    } else {
        return 'queued'
    }
}

$:getCheckContent = function(checkIndex:number):string {
    if(checkIndex < details.stage) {
        return checkHtml
    } else if(checkIndex == details.stage) {
        if(details.trouble) {
            return errHtml
        } else if(details.status == 0) {
            return ellipsisHtml
        }

        return workingHtml
    } else if(checkIndex > details.stage) {
        return pendingHtml
    }
}

const onCheckClick = function(checkIndex:number) {
    if(checkIndex == details.stage && details.trouble) {
        dispatch('spinner-click')
    }
}

const stages = [
    {caption: "import", icon: importIcon},
    {caption: "title", icon: titleIcon},
    {caption: "omdb", icon: omdbIcon},
    {caption: "ffmpeg", icon: ffmpegIcon},
    {caption: "db", icon: dbIcon}
]
</script>

<style lang="scss">
@use '../../styles/global.scss';
@use '../../styles/overviewPanel.scss';
</style>


<div class="stages" bind:this={perspectiveContainer}>
    {#each stages as {caption, icon}, index (caption)}
        <div bind:this={els[index]} class="stage {caption}" class:active="{details.stage == index}" on:click="{handleStageClick}">
            <span class="caption">{caption.toUpperCase()}</span>
            {@html icon}
        </div>

        {#if index < stages.length - 1}
            <div bind:this={checkEls[index]} class="check {getCheckClass(index)}" on:click="{() => onCheckClick(index)}">{@html getCheckContent(index)}</div>
        {/if}
    {/each}
</div>
