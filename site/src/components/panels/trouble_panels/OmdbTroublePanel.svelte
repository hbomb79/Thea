<script lang="ts">
import { createEventDispatcher, onMount } from "svelte";
import { SocketMessageType } from "../../../store";
import type { SocketData } from "../../../store";

import type { QueueDetails, QueueTroubleDetails } from "../../QueueItem.svelte";
import { QueueTroubleType } from "../../QueueItem.svelte";

export let troubleDetails:QueueTroubleDetails
export let queueDetails:QueueDetails

const dispatcher = createEventDispatcher()
enum ComponentState {
    INIT,
    READY,
    RESOLVING,
    CONFIRMING,
    TROUBLE_PERSISTS,
    FAILURE
}

let state = ComponentState.READY
let err:string = ''
export function updateState(item:QueueDetails) {
    if(state != ComponentState.CONFIRMING || item.id != queueDetails.id) return

    //TODO the logic here needs some work. Need to figure out a nice
    // way to handle a new trouble type (i.e. a trouble exists, but it's
    // a different type.). Should we tell the user, or maybe detect if it's
    // 'our' problem, or a trouble from the next stage and react accordingly.
    if(item.trouble && item.trouble.type == queueDetails.trouble.type) {
        // The data we received is a trouble state that matches our
        // current trouble state. Our attempt to resolve didn't work!
        state = ComponentState.TROUBLE_PERSISTS
    } else {
        state = ComponentState.READY
    }
}

function resolveChoice(choiceId:number) {
    state = ComponentState.RESOLVING
    dispatcher("try-resolve", {
        args: {choice: choiceId},
        cb: (data:SocketData): boolean => {
            if(data.type == SocketMessageType.RESPONSE) {
                console.log("Successfully resolved trouble state. Waiting for confirmation")
                state = ComponentState.CONFIRMING

                return true
            }

            console.warn("Failed to resolve trouble state")
            state = ComponentState.FAILURE
            err = `${data.title}: ${data.arguments.error}`

            return false
        }
    })
}
</script>

<style lang="scss">
.choices {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    margin-top: 1rem;
    flex-wrap: wrap;
    padding: 0 2rem;

    .choice {
        flex: 1;
        max-width: 33%;
        min-width: 33%;
        height: fit-content;
        padding: 1rem;
        cursor: pointer;

        background: whitesmoke;
        box-shadow: 0px 0px 6px -5px black;
        border: solid 1px #e4e3e3;

        transition: all 200ms ease-out;
        transition-property: background, box-shadow, border;

        margin: 2rem;

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
</style>

{#if state == ComponentState.READY}
    <h2>OMDB Troubled</h2>
    <p class="trouble">{troubleDetails.trouble.message}</p>

    {#if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE}
        <div class="choices">
            {#each troubleDetails.trouble.choices as {Title, Year, imdbId, Type}, i}
                <div class="choice choice-{i}" on:click="{() => resolveChoice(i)}">
                    <h2 class="title">{Title}<span class="id">{imdbId}</span></h2>
                    <p>{Type} from {Year}</p>
                </div>
            {/each}
        </div>
    {:else if troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
        <!--TODO-->
    {:else if troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE}
        <!--TODO-->
    {:else}
        <!--TODO-->
    {/if}
{:else if state == ComponentState.RESOLVING}
    <span>Sending resolution to server</span>
{:else if state == ComponentState.CONFIRMING}
    <span>Confirming the trouble is resolved</span>
{:else if state == ComponentState.TROUBLE_PERSISTS}
    <span>The resolution was accepted by the server, however the same trouble has occurred again.</span>
    <span>Please try again later, or check the server logs for guidance</span>
{:else if state == ComponentState.FAILURE}
    <span>Trouble resolution has failed - please check server logs for assistance</span>
    <p>{err}</p>
{/if}
