<script lang="ts">
import { createEventDispatcher } from "svelte";
import { SocketMessageType } from "../../../store";
import type { SocketData } from "../../../store";
import type {  QueueDetails, QueueTroubleDetails } from "../../QueueItem.svelte";

export let troubleDetails:QueueTroubleDetails
export let details:QueueDetails

const dispatcher = createEventDispatcher()
enum ComponentState {
    INIT,
    READY,
    RESOLVING,
    CONFIRMING,
    FAILURE
}

let state = ComponentState.READY
let err = ''

function retryFormat() {
    state = ComponentState.RESOLVING
    dispatcher("try-resolve", {
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

</style>

{#if state == ComponentState.READY}
    <h2>FFmpeg Formatter Troubled</h2>
    <p class="trouble">The ffmpeg formatter experienced an error while trying to process this item: <b>{troubleDetails.message}</b></p>
    <button on:click={retryFormat}>Retry</button>
{:else if state == ComponentState.RESOLVING}
    <span>Sending resolution to server</span>
{:else if state == ComponentState.CONFIRMING}
    <span>Confirming the trouble is resolved</span>
{:else if state == ComponentState.FAILURE}
    <span>Trouble resolution has failed - please check server logs for assistance</span>
    <p>{err}</p>
{/if}
