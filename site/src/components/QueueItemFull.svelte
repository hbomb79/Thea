<script lang="ts">
import type { QueueDetails } from "../queue";
import { QueueStatus } from "../queue";

import wavesSvg from '../assets/waves.svg';
import OverviewPanel from "./panels/OverviewPanel.svelte";
import TitlePanel from "./panels/TitlePanel.svelte";
import OmdbPanel from "./panels/OmdbPanel.svelte";
import FfmpegPanel from "./panels/FfmpegPanel.svelte";
import DatabasePanel from "./panels/DatabasePanel.svelte";
import QueueItemControls from "./QueueItemControls.svelte";
import StageIcon from "./StageIcon.svelte";
import TroublePanel from "./panels/TroublePanel.svelte";
import QueueStagePanel from "./panels/QueueStagePanel.svelte";

export let details: QueueDetails = null;
const stages = [
    ["Importer", "import", null],
    ["Title Parser", "title", TitlePanel],
    ["OMDB Queryer", "omdb", OmdbPanel],
    ["FFmpeg Transcoder", "ffmpeg", FfmpegPanel],
    ["Database Committer", "db", DatabasePanel],
]

const openStages: boolean[] = new Array(stages.length);

$:detailsChanged(details)

// Called automatically by Svelte when the details provided to
// this component change.
const detailsChanged = (newDetails: QueueDetails) => {
    console.log(newDetails)
}
</script>

<style lang="scss">
.queue-item {
    flex: 1;
    text-align: left;

    .splash {
        height: 250px;
        position: sticky;
        top: 0;
        overflow: hidden;
        background: linear-gradient(28deg, #6ca8ff, #ef8dff);
        box-shadow: 0 -5px 9px 0px #0000003d;
        z-index: 100;

        .waves {
            position: absolute;
            width: 100%;
            bottom: -65%;
            min-width: 1050px;
            z-index: 1;
            opacity: 0.4;
        }

        .content {
            position: absolute;
            width: 100%;
            height: 100%;
            z-index: 2;

            .title {
                padding: 0 3rem;
                color: white;
                font-size: 2rem;
                margin-bottom: 0;

                .id {
                    font-size: 1rem;
                    font-weight: 400;
                    color: #ffffff7a;
                    margin-left: -2px;
                    font-style: italic;
                }
            }

            .sub {
                padding: 0 3rem;
                color: #bfd9ff;
                margin-top: 0;
            }
        }
    }

    .main {
        padding: 1rem 2rem;
        .tile-title {
            font-size: 1rem;
            color: #9e94c5;
            font-weight: 500;
        }

        @import "../styles/queueItem.scss";
        .item {
            background: #f3f5fe;
            border-radius: 5px;
            border: solid 1px #d4d6e1;
            margin: 0 0 1.2rem 0;
            max-width: none;

            &.pipeline {
                text-align: center;
                padding: 1.4rem;
            }

            &.stage {
                .header {
                    padding: 4px 1rem;
                    background: none;
                    cursor: pointer;

                    transition: all 200ms ease-in-out;
                    transition-property: background border-bottom box-shadow;

                    width: unset;
                    display: flex;
                    flex-direction: row;
                    align-items: center;

                    &:hover {
                        background: white;
                    }

                    .stage-icon, .stage-icon :global(svg) {
                        width: 1.5rem;
                        height: 1.5rem;
                    }
                }

                &.content-open .header {
                    background: white;
                }
            }
        }
    }
}
</style>


{#if details}
    <div class="queue-item">
        <div class="splash">
            <div class="waves">{@html wavesSvg}</div>
            <div class="content">
                <h2 class="title">
                    {#if details.omdb_info}
                        {details.omdb_info.Title}
                    {:else if details.title_info}
                        {details.title_info.Title}
                    {:else}
                        {details.name}
                    {/if}
                    <span class="id">#{details.id}</span>
                </h2>
                <p class="sub">Item Status</p>

                <QueueItemControls/>
            </div>
        </div>

        <div class="main">
            <h2 class="tile-title">Pipeline</h2>
            <div class="item pipeline">
                <OverviewPanel details={details}/>
            </div>

            <h2 class="tile-title">Stage Details</h2>
            {#each stages as [display, tag, component], k (tag)}
                <div class={`item stage ${tag}`} class:content-open={openStages[k]}>
                    <div class="header" on:click={() => openStages[k] = !openStages[k]}>
                        <h2>{display}</h2>
                        <div class="check">
                            <StageIcon details={details} stageIndex={k}/>
                        </div>
                    </div>

                    {#if openStages[k]}
                        <div class="content">
                            <QueueStagePanel queueDetails={details} stageIndex={k} stagePanel={component}/>
                        </div>
                    {/if}
                </div>
            {/each}
        </div>
    </div>
{/if}
