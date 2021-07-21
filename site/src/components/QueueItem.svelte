<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../commander";
import { SocketMessageType, SocketPacketType } from "../store";
import type { SocketData } from "../store";
import type { QueueItem } from "./Queue.svelte";

import importIcon from '../assets/import-stage.svg';
import titleIcon from '../assets/title-stage.svg';
import omdbIcon from '../assets/omdb-stage.svg';
import ffmpegIcon from '../assets/ffmpeg-stage.svg';
import dbIcon from '../assets/db-stage.svg';
import troubleIcon from '../assets/warning.svg';

import ellipsisHtml from '../assets/html/ellipsis.html';
import workingHtml from '../assets/html/dual-ring.html';
import errHtml from '../assets/err.svg';

enum ComponentState {
    LOADING,
    ERR,
    COMPLETE
}

enum ComponentPage {
    OVERVIEW,
    TITLE,
    OMDB,
    FFMPEG,
    DB,
}

interface QueueTitleInfo {
    Title: string
    Episodic: boolean
    Episode: number
    Season: number
    Year: number
    Resolution: string
}

interface QueueOmdbInfo {
    Title: string
    plot: string
    ReleaseYear: number
    Runtime: string
    Type: string
    poster: string
    ImdbId: string
    Response: boolean
    Error: string
    Genre: string[]
}

interface QueueTroubleInfo {
    message: string
}

interface QueueDetails extends QueueItem {
    title_info: QueueTitleInfo
    omdb_info:  QueueOmdbInfo
    trouble: QueueTroubleInfo
}

const els:HTMLElement[] = new Array(4);
export let queueInfo:QueueItem

let state = ComponentState.LOADING
let page = ComponentPage.OVERVIEW
let details:QueueDetails = null
const getDetails = function() {
    // Get enhanced details of the queue item
    commander.sendMessage({
        type: SocketMessageType.COMMAND,
        title: "QUEUE_DETAILS",
        arguments: {id: queueInfo.id}
    }, (response:SocketData): boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            details = response.arguments.payload
            state = ComponentState.COMPLETE

            requestAnimationFrame(updateEllipsis);

            return true
        }

        details = null
        state = ComponentState.ERR

        return false
    })
}

const getStageStr = function(stage:number): string {
    switch(stage) {
        case 0: return "IO Poller"
        case 1: return "Title Format"
        case 2: return "OMDB Querying"
        case 3: return "Formatter"
        case 4: return "Finished"
    }
}

const getStatusStr = function(status:number): string {
    switch(status) {
        case 0: return "Pending"
        case 1: return "Working"
        case 2: return "Completed"
        case 3: return "Troubled"
    }
}

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

$:stat = function() {
    return getStageStr(details.stage) + ": " + getStatusStr(details.status)
}

onMount(getDetails)
</script>

<style lang="scss">
@use '../styles/global.scss';
@use '../styles/queueItem.scss';
</style>

<!-- Template -->
{#if state == ComponentState.LOADING}
    <span>Loading...</span>
{:else if state == ComponentState.COMPLETE}
    <div class="item" class:trouble="{details.trouble}">
        <div class="header">
            <span class="id">#{details.id}</span>
            <h2>
                {#if details.omdb_info} {details.omdb_info.Title}
                {:else if details.title_info} {details.title_info.Title}
                {/if}

                {#if details.title_info && details.title_info.Episodic}
                    <span class="season">S{details.title_info.Season}E{details.title_info.Episode}</span>
                {/if}
            </h2>

            <div class="status">
                <span>{stat()}</span>
                {#if details.trouble} {@html troubleIcon} {/if}
            </div>
        </div>
        <div class="panel">
            <span class:active="{page == ComponentPage.OVERVIEW}" on:click="{() => page = ComponentPage.OVERVIEW}">Overview</span>
            <span class:active="{page == ComponentPage.TITLE}" on:click="{() => page = ComponentPage.TITLE}">Title</span>
            <span class:active="{page == ComponentPage.OMDB}" on:click="{() => page = ComponentPage.OMDB}">OMDB</span>
            <span class:active="{page == ComponentPage.FFMPEG}" on:click="{() => page = ComponentPage.FFMPEG}">FFmpeg</span>
            <span class:active="{page == ComponentPage.DB}" on:click="{() => page = ComponentPage.DB}">DB</span>
        </div>
        <main>
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
        </main>
    </div>
{:else}
    <div class="item">
        <div class="header">
            <h2>Failed to load</h2>
            <span>Our request to the server failed. Please check the console for details.</span>
        </div>
    </div>
{/if}
