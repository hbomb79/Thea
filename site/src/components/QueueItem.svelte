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

$:stat = function() {
    return getStageStr(details.stage) + ": " + getStatusStr(details.status)
}

onMount(getDetails)
</script>

<style lang="scss">
.item {
    background: white;
    box-shadow: 0px 0px 6px -3px #4b494c;
    max-width: 950px;
    border-radius: 2px;
    width: 80%;
    margin: 2rem auto;
    overflow: hidden;

    .header {
        /* Ocupy all remaining width of the flexbox */
        width: 100%;
        text-align: left;
        overflow: hidden;
        box-shadow: 0px 0px 5px -3px black;
        background: #eee;

        display: flex;
        justify-content: space-between;

        .id {
            color: #a0a0a0;
            align-self: center;
            padding-left: 11px;
            font-size: 0.9rem;
            font-style: italic;
        }

        h2 {
            margin: 0;
            font-size: 1.2rem;
            color: #5E5E5E;
            display: inline-block;
            padding: 0.8rem;
            padding-left: 7px;
            flex: auto;
        }

        .status {
            display: inline-block;
            height: fit-content;
            align-self: center;
            padding-right: 1rem;
            font-style: italic;
            color: #6d6d6d;
            font-size: 0.8rem;

            span {
                height: 24px;
                display: inline-block;
                line-height: 24px;
            }

            :global(svg) {
                width: 21px;
                height: 21px;
                float: right;
                margin-left: 8px;
                fill: red;
            }
        }
    }

    .panel {
        background: #f5f5f5;
        padding: 0 1rem;
        text-align: left;

        span {
            padding: 8px 16px;
            background-color: #cee0ff;
            display: inline-block;
            margin: 9px 5px;
            border-radius: 15px;
            cursor: pointer;

            transition: background-color 100ms ease-in-out, border-color 100ms ease-in-out;
            border: 2px solid #f5f5f5;

            &:hover {
                background-color: #d0c3f2;
            }

            &.active {
                border-color: red;
            }
        }
    }

    main {
        padding: 1rem 0 1rem 0;

        span {
            font-style: italic;
            color: #868686;
        }

        .stages {
            display: flex;
            justify-content: space-around;
            padding: 2rem;

            .stage {
                display: flex;
                flex-direction: column-reverse;

                .caption {
                    display: block;
                    margin-top: 6px;
                }

                :global(svg) {
                    width: 3rem;
                    height: 3rem;
                }
            }
        }
    }
}
</style>

<!-- Template -->
{#if state == ComponentState.LOADING}
    <span>Loading...</span>
{:else if state == ComponentState.COMPLETE}
    <div class="item" class:trouble="{details.trouble}">
        <div class="header">
            <span class="id">#{details.id}</span>
            {#if details.omdb_info}
                <h2>{details.omdb_info.Title}</h2>
            {:else}
                <h2>{details.name}</h2>
            {/if}

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
                <div class="stage import"><span class="caption">Import</span>{@html importIcon}</div>
                <div class="stage title"><span class="caption">Title</span>{@html titleIcon}</div>
                <div class="stage omdb"><span class="caption">OMDB</span>{@html omdbIcon}</div>
                <div class="stage ffmpeg"><span class="caption">Ffmpeg</span>{@html ffmpegIcon}</div>
                <div class="stage db"><span class="caption">DB</span>{@html dbIcon}</div>
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
