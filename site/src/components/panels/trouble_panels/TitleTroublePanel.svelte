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

function titleResolutionModalSpawn() {
    dispatcher("display-modal", {
        title: "Title Info",
        description: `<p>We failed to parse the title <b>'${details.name}</b><br>Please manually enter the data below (leave blank if not applicable).</p>`,
        fields: troubleDetails.expectedArgs,
        cb: (result:Object) => {
            state = ComponentState.RESOLVING
            dispatcher("try-resolve", {
                args: result,
                cb: (reply:SocketData): boolean => {
                    if(reply.type == SocketMessageType.RESPONSE) {
                        console.log("Successfully resolved trouble state. Waiting for confirmation")
                        state = ComponentState.CONFIRMING

                        return true
                    }

                    console.warn("Failed to resolve trouble state")
                    state = ComponentState.FAILURE
                    err = `${reply.title}: ${reply.arguments.error}`

                    return false
                }
            })
        }
    })
}
</script>

<style lang="scss">

</style>

{#if state == ComponentState.READY}
    <h2>Title Formatter Troubled</h2>
    <p class="trouble">{troubleDetails.message}</p>
    <button on:click={titleResolutionModalSpawn}>Provide Title Info</button>
{:else if state == ComponentState.RESOLVING}
    <span>Sending resolution to server</span>
{:else if state == ComponentState.CONFIRMING}
    <span>Confirming the trouble is resolved</span>
{:else if state == ComponentState.FAILURE}
    <span>Trouble resolution has failed - please check server logs for assistance</span>
    <p>{err}</p>
{/if}
