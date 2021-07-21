<script context="module" lang="ts">
export interface QueueTitleInfo {
    Title: string
    Episodic: boolean
    Episode: number
    Season: number
    Year: number
    Resolution: string
}

export interface QueueOmdbInfo {
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

export interface QueueTroubleInfo {
    message: string
}

export interface QueueDetails extends QueueItem {
    title_info: QueueTitleInfo
    omdb_info:  QueueOmdbInfo
    trouble: QueueTroubleInfo
}
</script>

<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../commander";
import { SocketMessageType } from "../store";
import type { SocketData } from "../store";
import type { QueueItem } from "./Queue.svelte";

import troubleIcon from '../assets/warning.svg';
import OverviewPanel from "./panels/OverviewPanel.svelte";
import TitlePanel from "./panels/TitlePanel.svelte";
import OmdbPanel from "./panels/OmdbPanel.svelte";
import FfmpegPanel from "./panels/FfmpegPanel.svelte";
import DatabasePanel from "./panels/DatabasePanel.svelte";

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
    TROUBLE
}


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

const handleStatClick = function() {
    if(details.trouble) {
        page = ComponentPage.TROUBLE
    }
}

const handleSpinnerClick = function() {
    if(details.trouble) {
        page = ComponentPage.TROUBLE;
        return
    }

    page = details.stage as ComponentPage
}

$:stat = function() {
    return getStageStr(details.stage) + ": " + getStatusStr(details.status)
}

$:isStatActive = function() {
    return details.trouble && page == ComponentPage.TROUBLE
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

            <div class="status" on:click="{handleStatClick}" class:active="{isStatActive()}">
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
            {#if page == ComponentPage.OVERVIEW}
                <OverviewPanel details={details} on:spinner-click="{handleSpinnerClick}"/>
            {:else if page == ComponentPage.TITLE}
                <TitlePanel/>
            {:else if page == ComponentPage.OMDB}
                <OmdbPanel/>
            {:else if page == ComponentPage.FFMPEG}
                <FfmpegPanel/>
            {:else if page == ComponentPage.DB}
                <DatabasePanel/>
            {:else if page == ComponentPage.TROUBLE}
                <span><b>Trouble: </b>{details.trouble.message}</span>
            {/if}
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
