<script lang="ts">
    import dualRing from "assets/html/dual-ring.html";
    import { itemDetails } from "stores/queue";
    import { selectedQueueItem } from "stores/item";

    $: details = $itemDetails.get($selectedQueueItem);
</script>

{#if details.omdb_info && details.omdb_info.Response}
    <div class="title-info tile" class:troubled={details.trouble && details.stage == 2}>
        <section class="view">
            <div class="side">
                <img src={details.omdb_info.poster} alt="{details.omdb_info.Title} IMDB poster image" />
            </div>
            <div class="main">
                <div class="header">
                    <div class="title">
                        <h2>{details.omdb_info.Title}</h2>
                        {#if details.title_info.Episodic}
                            <span>S{details.title_info.Season}E{details.title_info.Episode}</span>
                        {/if}
                        <a class="id" target="_new" href="https://www.imdb.com/title/{details.omdb_info.ImdbId}/"
                            >{details.omdb_info.ImdbId}</a
                        >
                    </div>
                    <div class="stats">
                        <div class="runtime">
                            <span class="label">Runtime:</span>
                            <span class="value">{details.omdb_info.Runtime}</span>
                        </div>
                        <div class="type">
                            <span class="value">{details.omdb_info.Type}</span>
                        </div>
                    </div>
                </div>
                <div class="plot">
                    <p><b>Plot:</b></p>
                    <span>{details.omdb_info.plot}</span>
                </div>
                <div class="genres">
                    <span><b>Genres:</b></span>
                    {#each details.omdb_info.Genre as genre}
                        <span class="genre">{genre}</span>
                    {/each}
                </div>
            </div>
        </section>
    </div>
{:else if details.stage == 2}
    <div class="pending-tile">
        <h2>This stage is in progress</h2>
        <span
            >Hang tight while this stage completes it's computation. The results will be displayed here when they're
            available</span
        >
        {@html dualRing}
    </div>
{:else}
    <span class="error">OMDB info not found. Consult server logs for more information.</span>
{/if}

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
            text-align: left;
            display: flex;

            .side {
                padding: 1rem;
                min-width: 180px;
                width: 0;
                overflow: hidden;

                img {
                    width: 100%;
                    box-shadow: 0px 0px 3px 2px #00000052;
                    border: solid 1px black;
                }
            }

            .main {
                padding: 1rem;
                flex: 1;

                .header {
                    display: flex;
                    align-items: center;
                    border-bottom: solid 1px #eeeeee;
                    margin-bottom: 2rem;
                    padding-bottom: 1rem;

                    .title {
                        flex: 1 auto;
                        text-align: left;
                        color: #5e5e5e;

                        h2 {
                            margin: 0;
                            display: inline-block;
                        }

                        .id {
                            display: block;
                            font-weight: 300;
                            font-style: italic;
                            color: #5e5e5e;
                        }
                    }

                    .stats {
                        display: flex;
                        flex-direction: column;
                        text-align: right;

                        .value {
                            color: grey;
                            font-style: italic;
                        }
                    }
                }

                .genres {
                    margin-top: 2rem;

                    .genre {
                        padding: 5px 10px;
                        margin: 5px;
                        background-color: #f5f2f2;
                        color: #868686;
                        font-size: 0.8rem;
                        display: inline-block;
                        border-radius: 3px;
                    }
                }
            }
        }

        /*
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
    */
    }
</style>
