<script lang="ts">
import type { QueueDetails } from "../queue";
import wavesSvg from '../assets/waves.svg';
import OverviewPanel from "./panels/OverviewPanel.svelte";
import TitlePanel from "./panels/TitlePanel.svelte";
import OmdbPanel from "./panels/OmdbPanel.svelte";
import FfmpegPanel from "./panels/FfmpegPanel.svelte";
import DatabasePanel from "./panels/DatabasePanel.svelte";
import QueueItemControls from "./QueueItemControls.svelte";

export let details: QueueDetails = null;

const stages = [
    ["Importer", "import", null],
    ["Title Parser", "title", TitlePanel],
    ["OMDB Queryer", "omdb", OmdbPanel],
    ["FFmpeg Trascoder", "ffmpeg", FfmpegPanel],
    ["Database Committer", "db", DatabasePanel],
]
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
            min-width: 900px;
            z-index: 1;
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
            margin: 0 0 2rem 0;
            max-width: none;

            &.pipeline {
                text-align: center;
                padding: 1.4rem;
            }

            &.stage {
                .content {
                    display: none;
                }

                &.content-open .content {
                    display: block;
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
                <h2 class="title">Item Title</h2>
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
            {#each stages as [display, tag, component] (tag)}
                <div class={`item stage ${tag} content-open`}>
                    <div class="header">
                        <h2>{display}</h2>
                    </div>
                    <div class="content">
                        {#if component}
                            <svelte:component this={component} details={details}/>
                        {:else}
                            <p>No component available for this stage</p>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
    </div>
{/if}
