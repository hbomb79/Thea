<script context="module" lang="ts">
export enum QueueTroubleType {
	TITLE_FAILURE,
	OMDB_NO_RESULT_FAILURE,
	OMDB_MULTIPLE_RESULT_FAILURE,
	OMDB_REQUEST_FAILURE,
	FFMPEG_FAILURE,
}

export interface QueueTroubleDetails {
    trouble:QueueTroubleInfo,
    type:QueueTroubleType,
    expectedArgs:Object,
    [key:string]:any
}

</script>

<script lang="ts">
import { onMount } from "svelte";
import { commander } from "../../commander";
import { SocketMessageType } from "../../store";

import type { SocketData } from "../../store";
import type { QueueDetails, QueueTroubleInfo } from "../QueueItem.svelte";

import rippleHtml from '../../assets/html/ripple.html';

enum ComponentState {
    LOADING,
    COMPLETE,
    ERR
}

export let details:QueueDetails
let state = ComponentState.LOADING
let troubleDetails:QueueTroubleDetails

let omdbChoices: HTMLElement[] = []
onMount(() => {
    commander.sendMessage({
        title: "TROUBLE_DETAILS",
        type: SocketMessageType.COMMAND,
        arguments: { id: details.id }
    }, (data:SocketData): boolean => {
        // Wait for reply to message by using a callback.
        if(data.title == "COMMAND_SUCCESS" && data.type == SocketMessageType.RESPONSE) {
            troubleDetails = data.arguments.payload
            state = ComponentState.COMPLETE
        } else {
            state = ComponentState.ERR
        }

        return true;
    })
})

const retryFfmpeg = function(ev:MouseEvent) {
    const target = ev.currentTarget as HTMLButtonElement
    target.classList.add("spinning")
    target.disabled = true

    commander.sendMessage({
        title: "TROUBLE_RESOLVE",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: details.id
        }
    }, (data:SocketData): boolean => {
        target.classList.remove("spinning")
        target.disabled = false
        if(data.type == SocketMessageType.RESPONSE) {
            // Success
            console.log("Successfully resolved trouble state")
            return true
        }

        // Failure.
        console.warn("Failed to resolve trouble state")
        // TODO Window popup for errors.
        return false
    })
}

const onOmdbSelection = function(ev:MouseEvent) {
    const target = ev.currentTarget as HTMLElement
    let choiceId:number = -1;
    omdbChoices.every((el, idx) => {
        console.log("Testing isSameNode for ", target, el)
        el.classList.remove("working", "disabled")
        if(el.isSameNode(target)) {
            console.log("HIT")
            el.classList.add("working")
            choiceId = idx

            return true
        }

        el.classList.add("disabled")
        return true
    })

    if(choiceId < 0) {
        return
    }

    commander.sendMessage({
        title: "TROUBLE_RESOLVE",
        type: SocketMessageType.COMMAND,
        arguments: {
            id: details.id,
            choice: choiceId
        }
    }, (data:SocketData): boolean => {
        omdbChoices.every((el) => {
            el.classList.remove("working", "disabled")
            return true
        })

        if(data.type == SocketMessageType.RESPONSE) {
            // Success
            console.log("Successfully resolved trouble state")
            return true
        }

        // Failure.
        console.warn("Failed to resolve trouble state")
        // TODO Window popup for errors.
        return false
    })
}
</script>

<style lang="scss">
.spinner-button {
    position: relative;

    .spinner {
        position: absolute;
        top: -2px;
        right: -15px;

        width: 10px;
        height: 10px;

        transform: scale(0.4);
        opacity: 0;

        transition: opacity 100ms;
    }

    &.spinning :global(.spinner) {
        opacity: 1;
    }
}
.tile.trouble {
    padding: 1rem;

    h2 {
        margin: 0;
        color: #5e5e5e;
    }

    .choices {
        display: flex;
        flex-direction: row;
        justify-content: space-around;
        margin-top: 1rem;

        .choice {
            flex: 1;
            max-width: 40%;
            height: fit-content;
            padding: 1rem;
            cursor: pointer;

            background: whitesmoke;
            box-shadow: 0px 0px 6px -5px black;
            border: solid 1px #e4e3e3;

            transition: all 200ms ease-out;
            transition-property: background, box-shadow, border;

            .title {
                font-size: 1rem;

                .id {
                    font-size: 0.8rem;
                    font-style: italic;
                    font-weight: 400;

                    padding-left: 6px;
                }
            }

            p {
                color: #5e5e5e;
            }

            &:hover {
                background: #eeeeee;
                box-shadow: 0px 0px 6px -4px black;
            }
        }
    }
}
</style>

<!-- Template -->
<div class="tile trouble">
    {#if state == ComponentState.COMPLETE}
        {#if troubleDetails.type == QueueTroubleType.TITLE_FAILURE}
            <!-- A title failure means we need to provide the arguments back to the server that we need to
                 make a new TitleInfo struct -->
            <!--TODO -->
        {:else if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE && troubleDetails.trouble?.choices}
            <!-- This trouble means we have multiple choices from OMDB as to what movie/series this is.
                 Get the user to select it. -->
            <h2>OMDB Trouble</h2>
            <p class="trouble">{troubleDetails.trouble.message}</p>

            <div class="choices">
                {#each troubleDetails.trouble.choices as {Title, Year, imdbId, Type}, i}
                    <div class="choice choice-{i}" on:click="{onOmdbSelection}" bind:this="{omdbChoices[i]}">
                        <h2 class="title">{Title}<span class="id">{imdbId}</span></h2>
                        <p>{Type} from {Year}</p>
                    </div>
                {/each}
            </div>
        {:else if troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
            <!--TODO -->
        {:else if troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE}
            <!--TODO -->
        {:else if troubleDetails.type == QueueTroubleType.FFMPEG_FAILURE}
            <!--TODO -->
            <h2>FFMPEG Troubled</h2>
            <p class="trouble">{troubleDetails.trouble.message}</p>
            <button class="spinner-button" on:click="{retryFfmpeg}">Retry<div class="spinner">{@html rippleHtml}</div></button>
        {:else}
            <h2>Unknown trouble</h2>
            <p>We don't have a known resolution for this trouble case. Please check server logs for guidance.</p>
        {/if}
    {:else if state == ComponentState.LOADING}
        <p>Fetching trouble resolution</p>
        {@html rippleHtml}
    {:else}
        <span class="err">Failed to fetch trouble resolution</span>
    {/if}
</div>
