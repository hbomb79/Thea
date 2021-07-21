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

(window as any).els = els;
const dispatch = createEventDispatcher();
onMount(() => {
    requestAnimationFrame(updateEllipsis);
})

const updateEllipsis = function() {
    const left = els[details.stage] as HTMLElement
    const right = els[details.stage + 1] as HTMLElement
    const spinner = els[5];

    if (!left || !right || !spinner) {
        return
    }

    const mid = ((left.offsetLeft + left.offsetWidth) + (right.offsetLeft))/2
    spinner.setAttribute("style", "left: " + (mid - spinner.offsetWidth / 2) + "px;")

    requestAnimationFrame(updateEllipsis)
}

const handleSpinnerClick = function() {
    dispatch('spinner-click')
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
</script>

<style lang="scss">
@use '../../styles/global.scss';
@use '../../styles/overviewPanel.scss';
</style>


<div class="stages">
    <div bind:this={els[0]} 
        on:click="{handleStageClick}"
        class="stage import"
        class:hidden="{0 < details.stage}"
        class:active="{details.stage == 0}">
            <span class="caption">Import</span>
            {@html importIcon}
    </div>

    <div bind:this={checkEls[0]} class="check {getCheckClass(0)}">{@html getCheckContent(0)}</div>

    <div bind:this={els[1]}
        on:click="{handleStageClick}"
        class="stage title"
        class:hidden="{1 < details.stage}"
        class:active="{details.stage == 1}">
            <span class="caption">Title</span>
            {@html titleIcon}
    </div>

    <div bind:this={checkEls[1]} class="check {getCheckClass(1)}">{@html getCheckContent(1)}</div>

    <div bind:this={els[2]}
        on:click="{handleStageClick}"
        class="stage omdb"
        class:hidden="{details.stage == 0 || details.stage > 2}"
        class:active="{details.stage == 2}">
            <span class="caption">OMDB</span>
            {@html omdbIcon}
    </div>

    <div bind:this={checkEls[2]} class="check {getCheckClass(2)}">{@html getCheckContent(2)}</div>

    <div bind:this={els[3]}
        on:click="{handleStageClick}"
        class="stage ffmpeg"
        class:hidden="{details.stage > 1 && details.stage < 2}"
        class:active="{details.stage == 3}">
            <span class="caption">Ffmpeg</span>
            {@html ffmpegIcon}
    </div>


    <div bind:this={checkEls[3]} class="check {getCheckClass(3)}">{@html getCheckContent(3)}</div>

    <div bind:this={els[4]}
        on:click="{handleStageClick}"
        class="stage db"
        class:hidden="{details.stage < 3}"
        class:active="{details.stage == 4}">
            <span class="caption">DB</span>
            {@html dbIcon}
    </div>
</div>
