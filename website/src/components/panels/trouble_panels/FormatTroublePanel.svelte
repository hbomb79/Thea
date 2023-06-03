<script lang="ts">
import { createEventDispatcher } from "svelte";

const dispatcher = createEventDispatcher()

export let currentResolver = ""
const validResolvers = [
    ["Retry", "retry"]
]

function retryFormat() {
    dispatcher("try-resolve", {})
}

export function getHeader(): string {
    return `FFmpeg formatter experienced a problem`
}

export function getBody(): string {
    return "FFMPEG_FAILURE"
}

export function listResolvers() {
    return validResolvers
}

export function selectResolver(resolver: string) {
    const idx = validResolvers.findIndex(([_, key]) => key == resolver)

    currentResolver = idx > -1 ? resolver : ""
    dispatcher("selection-change")
}

export function selectedResolver(): string {
    return currentResolver
}

</script>

<style lang="scss">
@use "../../../styles/trouble.scss";
</style>

{#if currentResolver == "retry"}
    <h2>Retry Stage</h2>
    <p class="trouble">Usually this type of error indicates an error with the source file. If you believe you've fixed the issue and would like to have the processor try again, click the button below.</p>
    <button on:click={retryFormat}>Retry</button>
{/if}
