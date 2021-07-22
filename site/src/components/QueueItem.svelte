<script context="module" lang="ts">
// This module just exports some interfaces that we want
// to be able to use in other components.

// QueueTroubleInfo represents the data we receive
// from the Go server regarding the Title info for
// our queue item. This information will be
// unavailable until the title formatting
// has been completed.
export interface QueueTitleInfo {
    Title: string
    Episodic: boolean
    Episode: number
    Season: number
    Year: number
    Resolution: string
}

// QueueOmdbInfo represents the information about a queue item
// from the go server - this data will be unavailable until
// a worker on the go server has queried OMDB for this information
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

// QueueTroubleInfo is the information regarding a trouble
// state from the Go server
export interface QueueTroubleInfo {
    message: string
}

// QueueDetails is a single interface that extends the definition
// given by QueueItem, by appending the three above interfaces to it.
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

import OverviewPanel from "./panels/OverviewPanel.svelte";
import TitlePanel from "./panels/TitlePanel.svelte";
import OmdbPanel from "./panels/OmdbPanel.svelte";
import FfmpegPanel from "./panels/FfmpegPanel.svelte";
import DatabasePanel from "./panels/DatabasePanel.svelte";

// The queueInfo we're wanting to display from the parent component.
export let queueInfo:QueueItem

// The enhanced version of the above queueInfo, populated after the component
// has been mounted by commander (QUEUE_DETAILS websocket command)
let details:QueueDetails = null

// The state of this component, affected by the websocket
// packets that we're receiving from commander.
// This enum is used to control what HTML content we're displaying
enum ComponentState {
    LOADING,
    ERR,
    COMPLETE
}

// The page/panel we're wanting to view inside this component,
// affected by user interaction (e.g. on:click events)
enum ComponentPage {
    OVERVIEW,
    TITLE,
    OMDB,
    FFMPEG,
    DB,
    TROUBLE
}

// The initial state/page of the component
let state = ComponentState.LOADING
let page = ComponentPage.OVERVIEW


// Get enhanced details of the queue item
onMount(() => {
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

    // TODO REMOVE THIS, for simulating data changing ONLY
    setTimeout(() => {
        details.stage = 0;
        const int = setInterval(() => {
            details.stage++;
            if(details.stage == 2) clearInterval(int)
        }, 500);
    }, 2000)
})

// getStageStr returns a string representing the current stage of this item
function getStageStr(stage:number): string {
    switch(stage) {
        case 0: return "IO Poller"
        case 1: return "Title Format"
        case 2: return "OMDB Querying"
        case 3: return "Formatter"
        case 4: return "Finished"
    }
}

// getStatusStr returns a string representing the current status of this item
function getStatusStr(status:number): string {
    switch(status) {
        case 0: return "Pending"
        case 1: return "Working"
        case 2: return "Completed"
        case 3: return "<b>Troubled</b>"
    }
}

// handleStatClick will switch the component page to the TROUBLE
// page IF the queue item is currently troubled.
// If it's not troubled, the page is set to the page for the current stage
function handleStatClick():void {
    if(details.trouble) {
        page = ComponentPage.TROUBLE

        return
    }

    page = details.stage as ComponentPage
}

// handleStageClick will set the page to the event detail
// passed to this method - this function is called when
// we receive the 'stage-click' custom event from a child component
function handleStageClick(event:CustomEvent) {
    page = event.detail as ComponentPage
}

// stat is a dynamic binding method for Svelte that will live update
// the status text based on the stage and status of the item
$:stat = function() {
    return getStageStr(details.stage) + ": " + getStatusStr(details.status)
}

// isStatActive is a dynamic svelte binding to test if the status
// button for this component should be marked 'active'. Is active
// if the stage is troubled AND we're viewing the trouble.
$:isStatActive = function() {
    return details.trouble && page == ComponentPage.TROUBLE
}
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
                <span>{@html stat()}</span>
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
                <OverviewPanel details={details} on:spinner-click="{handleStatClick}" on:stage-click="{handleStageClick}"/>
            {:else if page == ComponentPage.TITLE}
                <TitlePanel details={details}/>
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
        </div>
        <main>
            <span>Our request to the server failed. Please check the console for details.</span>
        </main>
    </div>
{/if}
