<script lang="ts">
import { createEventDispatcher } from "svelte";
import DynamicForm from "../../DynamicForm.svelte";
import type { QueueDetails } from "../../QueueItem.svelte";

export let queueDetails: QueueDetails

const dispatcher = createEventDispatcher()

let currentResolver = ""
const validResolvers = [
    ["Manual", 'struct'],
]

function resolveTitle(titleInfo: Object) {
    dispatcher("try-resolve", { args: titleInfo })
}

export function getHeader(): string {
    return `Title Parser Trouble`
}

export function getBody(): string {
    return "TITLE_FAILURE"
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

{#if currentResolver == "struct"}
    <h2>TitleInfo Struct</h2>
    <p class="sub">Provide TitleInfo manually</p>

    <p>
        The processor was unable to parse information from this filename<br>
        <code>{queueDetails.name}</code>
        <br><br>
        Fill out the form below to resolve this problem, <i>you can leave a field blank if it's not applicable (e.g. episode number for a movie)</i>
    </p>

    <DynamicForm on:submitted={(event) => resolveTitle(event.detail)} fields={queueDetails.trouble.expected_args}></DynamicForm>
{/if}
