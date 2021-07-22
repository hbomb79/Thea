<script lang="ts">
import type { QueueDetails } from '../QueueItem.svelte';
import StageIcon from '../StageIcon.svelte';
export let details:QueueDetails;

</script>

<style lang="scss">
/*TODO: Move duplicate styling for panels in to SCSS @use*/
.title-info {
    display: flex;
    flex-direction: row;
    padding: 0 !important;

    .status {
        background-color: white;
        padding: 1rem;
        display: flex;
        flex-direction: column-reverse;
        margin: 1rem 1rem 1rem 2rem;

        :global(svg) {
            width: 4.5rem;
            height: 4.5rem;
            margin-bottom: 1rem;
            fill: #abc3fb;
            flex: 1 auto;
        }
    }

    .view {
        flex: 1 auto;
        padding: 1rem 2rem 1rem 1rem;
        flex-direction: row;
        min-width: 0;
        flex-basis: auto;

        .props {
            display: flex;
            flex-wrap: wrap;
            height: 100%;
            justify-content: space-around;
            text-align: left;

            .prop {
                width: 33%;
                min-width: 170px;
                align-self: center;

                overflow: hidden;
                text-overflow: ellipsis;
                padding: 0.5rem 1rem;

                .name {
                    display: block;
                }
            }
        }
    }
}

</style>
<div class="title-info tile" class:troubled={details.trouble && details.stage == 2}>
    <section class="status">
        <StageIcon details={details} stageIndex={2}/>
    </section>
    <section class="view">
        <div class="props">
            {#each Object.entries(details.omdb_info) as value}
                <div class="prop">
                    <span class="name"><b>{value[0]}</b></span>
                    <span>{value[1]}</span>
                </div>
            {/each}
        </div>
    </section>
</div>
