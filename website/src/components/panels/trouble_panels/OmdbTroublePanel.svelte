<script lang="ts">
import { createEventDispatcher, onMount } from "svelte";
import DynamicForm from "../../DynamicForm.svelte";
import type { QueueDetails } from "../../../queue";
import { QueueTroubleType } from "../../../queue";

export let queueDetails: QueueDetails
const dispatcher = createEventDispatcher()

let currentResolver = ""
const validResolvers = [
    ["IMDB ID", 'imdb'],
    ["Manual", 'struct'],
]

$: troubleDetails = queueDetails.trouble
onMount(() => {
    if(troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE) {
        validResolvers.unshift(["Choose", "choice"])
    }
})

function resolveChoice(choiceId:number) {
    dispatcher("try-resolve", {
        args: {choiceId: choiceId}
    })
}

function resolveWithForm(result: Object){
    dispatcher("try-resolve", {
        args: result
    })
}

export function getHeader(): string {
    return `OMDB API query result exception`
}

export function getBody(): string {
    switch(troubleDetails.type) {
        case QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE:
            return `OMDB_MULTIPLE_RESULT_FAILURE`
        case QueueTroubleType.OMDB_NO_RESULT_FAILURE:
            return "OMDB_NO_RESULT_FAILURE"
        case QueueTroubleType.OMDB_REQUEST_FAILURE:
        default:
            return "OMDB_REQUEST_FAILURE"
    }

}

export function listResolvers(): string[][] {
    return validResolvers
}

export function selectResolver(resolver: string) {
    const idx = listResolvers().findIndex(([_, key]) => key == resolver)

    currentResolver = idx > -1 ? resolver : ""
    dispatcher("selection-change")
}

export function selectedResolver(): string {
    return currentResolver
}
</script>

<style lang="scss">
@use "../../../styles/trouble.scss";

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

{#if currentResolver == "choice"}
    <h2>Choose Option</h2>
    <p class="trouble">Our search through OMDB resulted in multiple options. Pick the correct one below.<br>If none are correct, you can provide an IMDB id or provide the item details manually using the navigation above.</p>

    <div class="choices">
        {#each troubleDetails.payload.choices as {Title, Year, imdbId, Type}, i}
            <div class="choice choice-{i}" on:click="{() => resolveChoice(i)}">
                <h2 class="title">{Title}<span class="id">{imdbId}</span></h2>
                <p>{Type} from {Year}</p>
            </div>
        {/each}
    </div>
{:else if currentResolver == "imdb"}
    <h2>IMDB ID</h2>
    <p class="sub">Provide ImdbID</p>

    <p>
        Search IMDB for the entry that best describes this item and provide the ID here.<br>
        The processor will retry this item with the new ImdbID once a worker is available to do so.
    </p>

    <DynamicForm fields={{imdbId: "string"}} on:submitted={(event) => resolveWithForm(event.detail)}/>
{:else if currentResolver == "struct"}
    <h2>OmdbInfo Struct</h2>
    <p class="sub">Provide information manually</p>
    <p>
        If this item doesn't exist in OMDBs database yet, you can instead provide all the details manually below.
    </p>
    <DynamicForm fields={troubleDetails.expected_args} on:submitted={(event) => resolveWithForm({"replacementStruct": event.detail})}/>
{/if}
