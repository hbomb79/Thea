<script lang="ts">
import type { QueueDetails } from '../../queue';
import StageIcon from '../StageIcon.svelte';

export let details:QueueDetails;
</script>

<style lang="scss">
.title-info {
    display: flex;
    flex-direction: row;
    padding: 0 !important;

    .view {
        flex: 1 auto;
        padding: 1rem 2rem 1rem 1rem;
        flex-direction: row;
        min-width: 0;
        flex-basis: auto;

        .source {
            text-align: center;

            p {
                display: inline-block;
                padding: 14px 2rem;
                color: #786da5;
                background: white;
                border-radius: 6px;
                margin-bottom: 0;
                border: solid 1px #dce1f9;
            }
        }

        .result {
            display: flex;
            justify-content: space-around;

            .main {
                width: 60%;
                margin-right: 1rem;
            }

            .prop {
                margin: 1.3rem 0;

                .title {
                    color: #9184c5;
                    font-weight: 400;
                    margin: 0;
                    font-size: 1.3rem;
                }

                span {
                    color: #8c91b9;
                    font-style: italic;
                }
            }
        }
    }
}
</style>

{#if details.title_info}
    <div class="title-info tile">
        <section class="view">
            <div class="source">
                <p>{details.name}</p>
            </div>
            <div class="result">
                <div class="main">
                    <div class="prop">
                        <h2 class="title">Title</h2>
                        <span>{details.title_info.Title}</span>
                    </div>
                    <div class="prop">
                        <h2 class="title">Year</h2>
                        <span>{details.title_info.Year}</span>
                    </div>
                    <div class="prop">
                        <h2 class="title">Resolution</h2>
                        <span>{details.title_info.Resolution}</span>
                    </div>
                </div>
                <div class="episode">
                    <div class="prop">
                        <h2 class="title">Episodic?</h2>
                        <span>{details.title_info.Episodic}</span>
                    </div>
                    <div class="prop">
                        <h2 class="title">Season</h2>
                        <span>{details.title_info.Season}</span>
                    </div>
                    <div class="prop">
                        <h2 class="title">Episode</h2>
                        <span>{details.title_info.Episode}</span>
                    </div>
                </div>
            </div>
        </section>
    </div>
{/if}
