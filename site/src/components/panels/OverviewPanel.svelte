<script lang="ts">
import importIcon from '../../assets/import-stage.svg';
import titleIcon from '../../assets/title-stage.svg';
import omdbIcon from '../../assets/omdb-stage.svg';
import ffmpegIcon from '../../assets/ffmpeg-stage.svg';
import dbIcon from '../../assets/db-stage.svg';
import ellipsisHtml from '../../assets/html/ellipsis.html';
import workingHtml from '../../assets/html/dual-ring.html';
import errHtml from '../../assets/err.svg';
import { onMount } from "svelte";
import type { QueueDetails } from '../QueueItem.svelte';

export let details:QueueDetails;
const els:HTMLElement[] = new Array(4);

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
</script>

<style lang="scss">
@use '../../styles/global.scss';
@use '../../styles/overviewPanel.scss';
</style>


<div class="stages">
    <div bind:this={els[0]} class="stage import" class:hidden="{0 < details.stage}" class:active="{details.stage == 0}"><span class="caption">Import</span>{@html importIcon}</div>
    <div bind:this={els[1]} class="stage title" class:hidden="{1 < details.stage}" class:active="{details.stage == 1}"><span class="caption">Title</span>{@html titleIcon}</div>
    <div bind:this={els[2]} class="stage omdb" class:hidden="{details.stage == 0 || details.stage > 2}" class:active="{details.stage == 2}"><span class="caption">OMDB</span>{@html omdbIcon}</div>
    <div bind:this={els[3]} class="stage ffmpeg" class:hidden="{details.stage > 1 && details.stage < 2}" class:active="{details.stage == 3}"><span class="caption">Ffmpeg</span>{@html ffmpegIcon}</div>
    <div bind:this={els[4]} class="stage db" class:hidden="{details.stage < 3}" class:active="{details.stage == 4}"><span class="caption">DB</span>{@html dbIcon}</div>
    <div bind:this={els[5]} class="loading">
        {#if details.trouble}
            {@html errHtml}
        {:else if details.status == 0}
            {@html ellipsisHtml}
        {:else if details.status == 1}
            {@html workingHtml}
        {/if}
    </div>
</div>
