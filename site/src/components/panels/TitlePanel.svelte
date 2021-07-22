<script lang="ts">
import type { QueueDetails } from '../QueueItem.svelte';
import StageIcon from '../StageIcon.svelte';
export let details:QueueDetails;

</script>

<style lang="scss">
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
        /*background: linear-gradient( 139deg , #d0c3f2b5, #c1d6fd66);
        border: solid 1px #eeeeee;
        border-radius: 3px;
        box-shadow: 0px 0px 3px -2px black;*/

        :global(svg) {
            width: 4.5rem;
            height: 4.5rem;
            margin-bottom: 1rem;
            fill: #abc3fb;
            flex: 1 auto;
        }
    }

    .view {
        flex: 1;
        padding: 1rem 2rem 1rem 1rem;
        flex-direction: row;

        .props {
            display: flex;
            flex-wrap: wrap;
            height: 100%;

            .prop {
                width: 33%;
                min-width: 170px;
                align-self: center;
            }
        }
    }
}
</style>

<div class="title-info tile">
    <section class="status">
        <StageIcon details={details} stageIndex={1}/>
    </section>
    <section class="view">
        <div class="props">
            {#each Object.entries(details.title_info) as value}
                <div class="prop">
                    <span class="name"><b>{value[0]}</b></span>
                    <span>{value[1]}</span>
                </div>
            {/each}
        </div>
    </section>
</div>
