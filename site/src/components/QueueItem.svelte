<script context="module" lang="ts">
// This module just exports some interfaces that we want
// to be able to use in other components.
export enum QueueStatus {
    PENDING,
    PROCESSING,
    COMPLETED,
    NEEDS_RESOLVING,
    CANCELLING,
    CANCELLED,
}

export enum QueueStage {
    IMPORT,
    TITLE,
    OMDB,
    FFMPEG,
    DB,
    FINISH
}

export enum QueueTroubleType {
	TITLE_FAILURE,
	OMDB_NO_RESULT_FAILURE,
	OMDB_MULTIPLE_RESULT_FAILURE,
	OMDB_REQUEST_FAILURE,
	FFMPEG_FAILURE,
}

export interface QueueItem {
    id: number
    name: string
    stage: QueueStage
    status: QueueStatus
    statusLine: string
}

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

export interface QueueTroubleDetails {
    message: string
    expected_args: any
    type: QueueTroubleType
    payload: any
    item_id: number
}

// QueueDetails is a single interface that extends the definition
// given by QueueItem, by appending the three above interfaces to it.
export interface QueueDetails extends QueueItem {
    title_info: QueueTitleInfo
    omdb_info:  QueueOmdbInfo
    trouble: QueueTroubleDetails
}
</script>

<script lang="ts">
import { onMount } from "svelte";
import { commander, dataStream } from "../commander";
import { SocketMessageType } from "../store";
import type { SocketData } from "../store";

import rippleHtml from '../assets/html/ripple.html';
import pendingHtml from '../assets/html/ellipsis.html';

import OverviewPanel from "./panels/OverviewPanel.svelte";
import TitlePanel from "./panels/TitlePanel.svelte";
import OmdbPanel from "./panels/OmdbPanel.svelte";
import FfmpegPanel from "./panels/FfmpegPanel.svelte";
import DatabasePanel from "./panels/DatabasePanel.svelte";
import TroublePanel from './panels/TroublePanel.svelte';
import QueueItemControls, { Action } from './QueueItemControls.svelte';

// The queueInfo we're wanting to display from the parent component.
export let queueInfo:QueueItem

// The enhanced version of the above queueInfo, populated after the component
// has been mounted by commander (QUEUE_DETAILS websocket command)
let queueDetails:QueueDetails = null

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

// The initial state/page of the component
let state = ComponentState.LOADING
let page = QueueStage.IMPORT
let controlsPanel:QueueItemControls = null
let troubleModal:TroublePanel = null
let domItemBack:HTMLElement = null
let domItemFront:HTMLElement = null
let domItem:HTMLElement = null

const getQueueDetails = () => {
    commander.sendMessage({
        type: SocketMessageType.COMMAND,
        title: "QUEUE_DETAILS",
        arguments: {id: queueInfo.id}
    }, (response:SocketData): boolean => {
        if(response.type == SocketMessageType.RESPONSE) {
            queueDetails = response.arguments.payload
            state = ComponentState.COMPLETE

            return true
        }

        queueDetails = null
        state = ComponentState.ERR

        return false
    })
}

// getStageStr returns a string representing the current stage of this item
function getStageStr(stage:number): string {
    switch(stage) {
        case 0: return "IO Poller"
        case 1: return "Title Formatter"
        case 2: return "OMDB Querying"
        case 3: return "Formatter"
        case 4: return "DB Committer"
        case 5: return "Finished"
        default: return "UNKNOWN"
    }
}

// getStatusStr returns a string representing the current status of this item
function getStatusStr(status:number): string {
    switch(status) {
        case 0: return "Pending"
        case 1: return "Working"
        case 2: return "Completed"
        case 3: return "<b>Troubled</b>"
        case 4: return "<i>Cancelling</i>"
        case 5: return "Cancelled"
        default: return "UNKNOWN"
    }
}

function sendCommand(command: string, successCallback: (arg0: SocketData) => void, errorCallback: (arg0: SocketData) => void) {
    commander.sendMessage({
        type: SocketMessageType.COMMAND,
        title: command,
        arguments: { id: queueDetails.id }
    }, (reply: SocketData): boolean => {
        if(reply.type == SocketMessageType.ERR_RESPONSE) {
            errorCallback(reply)
        } else {
            successCallback(reply)
        }

        return false
    })
}

function promoteItem() {
    sendCommand(
        "PROMOTE_ITEM",
        (successData) => {
            console.log("Promotion success!")
        },
        (errData) => {
            alert(`Failed to promote item: ${errData.title}: ${errData.arguments.error}`)
        }
    )
}

function pauseItem() {
    sendCommand(
        "PAUSE_ITEM",
        (successData) => {
            console.log("Pause success!")
        },
        (errData) => {
            alert(`Failed to pause item: ${errData.title}: ${errData.arguments.error}`)
        }
    )
}

function cancelItem() {
    sendCommand(
        "CANCEL_ITEM",
        (successData) => {
            console.log("Cancellation success!")
        },
        (errData) => {
            alert(`Failed to cancel item: ${errData.title}: ${errData.arguments.error}`)
        }
    )
}

function handleItemAction(event: CustomEvent) {
    console.log(event)
    const action = event.detail as Action
    switch(action) {
        case Action.PROMOTE:
            promoteItem()
            break
        case Action.PAUSE:
            pauseItem()
            break
        case Action.CANCEL:
            cancelItem()
            break
        case Action.NONE:
        default:
            console.warn(`Unknown item action ${action}`)
    }
}

function openDiagnosticsPanel(event: MouseEvent) {
    event.stopPropagation()

    if(troubleModal) {
        troubleModal.$destroy()
    }

    // We need to attach this trouble modal to the 
    troubleModal = new TroublePanel({
        target: domItemBack,
        // NOTE this prop is NOT reactive... this is actually
        // a good thing as it allows the embedded panel to
        // catch changes in the dataStream (subscribing for updates)
        // and compare the new data with it's current data.
        props: { queueDetails: queueDetails }
    })

    troubleModal.$on("close", () => {
        troubleModal.$destroy()
        troubleModal = undefined
    })
}

// handleStatClick will switch the component page to the TROUBLE
// page IF the queue item is currently troubled.
// If it's not troubled, the page is set to the page for the current stage
function handleStatClick():void {
    page = queueDetails.stage
}

// handleStageClick will set the page to the event detail
// passed to this method - this function is called when
// we receive the 'stage-click' custom event from a child component
function handleStageClick(event:CustomEvent) {
    page = event.detail as QueueStage
}

// stat is a dynamic binding method for Svelte that will live update
// the status text based on the stage and status of the item
$:stat = function() {
    return getStageStr(queueDetails.stage) + ": " + getStatusStr(queueDetails.status)
}

// isStatActive is a dynamic svelte binding to test if the status
// button for this component should be marked 'active'. Is active
// if the stage is troubled AND we're viewing the trouble.
$:isStatActive = function() {
    return <number>page == queueDetails.stage
}

function resizeFlipContainer() {
    requestAnimationFrame(resizeFlipContainer)

    if(!domItem) return
    const newHeight = domItem.classList.contains("flipped") ? domItemBack.offsetHeight : domItemFront.offsetHeight
    domItem.style.height = `${newHeight}px`
}

// Get enhanced details of the queue item. If this information changes
// we'll be notified by the server via an 'UPDATE' packet, which we
// can use for all the information
onMount(() => {
    getQueueDetails()

    requestAnimationFrame(resizeFlipContainer)

    dataStream.subscribe((data:SocketData) => {
        if(data.type == SocketMessageType.UPDATE) {
            const updateContext = data.arguments.context
            if(updateContext && updateContext.QueueItem && updateContext.QueueItem.id == queueDetails.id) {
                queueDetails = updateContext.QueueItem
            }
        }
    })
})

</script>

<style lang="scss">
@use '../styles/global.scss';
@use '../styles/queueItem.scss';

.resolving {
    width: 80%;
    margin: 0 auto 2rem auto;
    background: linear-gradient(326deg , #d9b6ea5f, #bfd9ff5f);
    border: solid 1px #a99dfb;
    border-radius: 3px;
    padding: 0rem;
    color: #5e6495;
}

.item-wrapper {
    perspective: 1000px;

    .item {
        position: relative;
        width: 100%;
        height: 300px;

        transition: transform 250ms ease-in-out;
        transform-style: preserve-3d;
        overflow: initial;

        &.flipped {
            transform: rotateX(180deg);
        }

        .item-front, .item-backside {
            position: absolute;
            width: 100%;
            backface-visibility: hidden;
            background: #f3f5fe;
            border-radius: 4px;
            overflow: hidden;
            border: solid 1px #9184c52e;
        }

        .item-front {
            z-index: -1;
        }

        .item-backside {
            transform: rotateX(180deg);
        }

        &:not(.flipped):hover :global(.controls) {
            opacity: 1;
        }
    }
}
</style>

<!-- Template -->
{#if state == ComponentState.LOADING}
    <div class="item">
        <main>
            <h2>Loading</h2>
            <span style="display:block;">Please wait while we fetch this queue item from the server</span>
            {@html rippleHtml}
        </main>
    </div>
{:else if state == ComponentState.COMPLETE}
    <div class="item-wrapper">
        <div class="item" class:flipped={troubleModal} bind:this={domItem} class:trouble="{queueDetails.trouble}">
            <QueueItemControls bind:this={controlsPanel} on:queue-control={handleItemAction}/>
            <div class="item-front" bind:this={domItemFront}>
                <div class="header">
                    <span class="id">#{queueDetails.id}</span>
                    <h2>
                        {#if queueDetails.omdb_info} {queueDetails.omdb_info.Title}
                        {:else if queueDetails.title_info} {queueDetails.title_info.Title}
                        {:else} {queueDetails.name}
                        {/if}

                        {#if queueDetails.title_info && queueDetails.title_info.Episodic}
                            <span class="season">S{queueDetails.title_info.Season}E{queueDetails.title_info.Episode}</span>
                        {/if}
                    </h2>
                </div>
                {#if page != QueueStage.IMPORT}
                    <div class="panel">
                        <span class="panel-item" on:click="{() => page = QueueStage.IMPORT}">Overview</span>
                        <span class:active="{page == QueueStage.TITLE}" class="panel-item" on:click="{() => page = QueueStage.TITLE}">Title</span>
                        <span class:active="{page == QueueStage.OMDB}" class="panel-item" on:click="{() => page = QueueStage.OMDB}">OMDB</span>
                        <span class:active="{page == QueueStage.FFMPEG}" class="panel-item" on:click="{() => page = QueueStage.FFMPEG}">FFmpeg</span>
                        <span class:active="{page == QueueStage.DB}" class="panel-item" on:click="{() => page = QueueStage.DB}">DB</span>
                    </div>
                {/if}
                <main>
                    {#if queueDetails.stage == page && queueDetails.status != QueueStatus.COMPLETED && queueDetails.status != QueueStatus.PROCESSING}
                        <!-- We're viewing the page representing the current stage -->
                        {#if queueDetails.status == QueueStatus.NEEDS_RESOLVING}
                            <!-- Stage is troubled. Show the trouble panel -->
                            <div class="troubled tile">
                                <h2>Stage Troubled</h2>
                                <p>This stage has experienced an error that can be resolved via the diagnostics panel.</p>
                                <button on:click|preventDefault={openDiagnosticsPanel}>Open Diagnostics Panel</button>
                            </div>
                        {:else if queueDetails.status == QueueStatus.PENDING}
                            <div class="pending tile">
                                {#if queueDetails.trouble}
                                    <div class="resolving">
                                        <p class="sub">This item has trouble resolution data attached and is waiting for confirmation of item progress</p>
                                    </div>
                                {/if}
                                <h2>This stage is queued</h2>
                                <span>All workers for this stage are busy with other items - progress will appear here once a worker is available.</span>
                                {@html pendingHtml}
                            </div>
                        {/if}
                    {:else if queueDetails.stage >= page || page == QueueStage.IMPORT || queueDetails.status == QueueStatus.PROCESSING}
                        {#if page == QueueStage.IMPORT}
                            <OverviewPanel details={queueDetails} on:spinner-click="{handleStatClick}" on:stage-click="{handleStageClick}"/>
                        {:else if page == QueueStage.TITLE}
                            <TitlePanel details={queueDetails}/>
                        {:else if page == QueueStage.OMDB}
                            <OmdbPanel details={queueDetails}/>
                        {:else if page == QueueStage.FFMPEG}
                            <FfmpegPanel/>
                        {:else if page == QueueStage.DB}
                            <DatabasePanel/>
                        {/if}
                    {:else}
                        <div class="pending tile">
                            <h2>This stage is scheduled</h2>
                            <span>We're waiting on previous stages of the pipeline to succeed before we start this stage. Check the 'Overview' to track progress.</span>
                            {@html rippleHtml}
                        </div>
                    {/if}
                </main>
            </div>
            <div class="item-backside" bind:this={domItemBack}>
                <!-- Svelte dynamic component here -->
            </div>
        </div>
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
