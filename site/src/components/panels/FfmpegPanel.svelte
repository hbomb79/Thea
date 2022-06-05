<script lang="ts">
    import { CommanderTaskStatus, QueueStatus } from "../../queue";
    import type { CommanderTask, QueueDetails } from "../../queue";
    import spinnerHtml from "../../assets/html/hourglass.html";
    import workingHtml from "../../assets/html/dual-ring.html";
    import troubleSvg from "../../assets/err.svg";
    import checkSvg from "../../assets/check-mark.svg";
    import scheduledSvg from "../../assets/pending.svg";
    import TroublePanel from "./TroublePanel.svelte";
    export let details: QueueDetails;

    const retryHandler = () => false;
    const specifyProfileHandler = () => false;
    const pauseHandler = () => false;
    const cancelHandler = () => false;

    const troubleResolvers: [string, (instance: CommanderTask) => boolean][] = [
        ["Retry", retryHandler],
        ["Specify Profile", specifyProfileHandler],
        ["Pause", pauseHandler],
        ["Cancel", cancelHandler],
    ];

    $: ffmpegInstances = details.ffmpeg_instances;

    $: getStageIcon = function (instance: CommanderTask): string {
        switch (instance.Status) {
            case CommanderTaskStatus.PENDING:
                return scheduledSvg;
            case CommanderTaskStatus.WAITING:
                return scheduledSvg;
            case CommanderTaskStatus.WORKING:
                return workingHtml;
            case CommanderTaskStatus.TROUBLED:
                return troubleSvg;
            case CommanderTaskStatus.FINISHED:
                return checkSvg;
            default:
                return "?";
        }
    };

    // getCheckClass is a dynamic binding that is used to
    // get the HTML 'class' that must be applied to each
    // 'check' icon inbetween each pipeline stage in the Overview.
    // This class is used to adjust the color and connecting lines
    // to better reflect the situation (e.g. red with no line
    // after the icon to indicate an error)
    $: getCheckClass = function (instance: CommanderTask): string {
        switch (instance.Status) {
            case CommanderTaskStatus.PENDING:
                return "queued";
            case CommanderTaskStatus.WAITING:
                return "pending";
            case CommanderTaskStatus.WORKING:
                return "working";
            case CommanderTaskStatus.TROUBLED:
                return "trouble";
            case CommanderTaskStatus.FINISHED:
                return "complete";
            default:
                return "unknown";
        }
    };

    $: commanderStatusToText = function (status: CommanderTaskStatus): string {
        switch (status) {
            case CommanderTaskStatus.PENDING:
                return "Queued";
            case CommanderTaskStatus.WAITING:
                return "Waiting for Resources";
            case CommanderTaskStatus.WORKING:
                return "Transcoding";
            case CommanderTaskStatus.TROUBLED:
                return "Troubled";
            case CommanderTaskStatus.FINISHED:
                return "Transcode Complete";
            default:
                return "Unknown Status";
        }
    };
</script>

{#if ffmpegInstances.length == 0}
    <h2>No Instances</h2>
    <p>No FFmpeg transcoder profiles matched this item.</p>
{:else}
    <ul class="instances">
        {#each ffmpegInstances as instance}
            <li class="instance">
                <div class="icon">
                    <div class="check {getCheckClass(instance)}">
                        {@html getStageIcon(instance)}
                    </div>
                </div>
                <div class="info">
                    <div class="header">
                        <h2>{instance.ProfileTag}</h2>
                        <span>{commanderStatusToText(instance.Status)}</span>
                    </div>
                    <div class="body" class:trouble={instance.Trouble}>
                        {#if instance.Trouble}
                            <h2 class="title">{instance.Trouble.message}</h2>
                            <div class="controls">
                                {#each troubleResolvers as [display, handler] (display)}
                                    <span class="button" on:click={() => handler(instance)}>{display}</span>
                                {/each}
                            </div>
                        {:else}
                            <p>Info here...</p>
                        {/if}
                    </div>
                </div>
                <div class="controls" />
            </li>
        {/each}
    </ul>
{/if}

<style lang="scss">
    @use "../../styles/stageIcon.scss";

    .instances {
        padding: 0;
        margin: 0;

        .instance {
            padding: 1rem;
            list-style: none;
            border-bottom: solid 1px #d2d7ff;
            display: flex;

            .icon {
                padding-right: 1rem;
            }

            .info {
                flex: 1 auto;
                display: flex;

                .header {
                    padding-right: 1rem;
                }

                .body {
                    display: flex;
                    justify-content: space-between;
                    flex: 1 auto;

                    .controls {
                        .button {
                            padding: 0.6rem 0.6rem;
                            margin: 0 0.5rem;
                            background: #c3d4fc4a;
                            color: #9385cb;
                            border-radius: 7px;
                            border: solid 1px #a39ad585;
                            cursor: pointer;
                            display: inline-block;
                        }
                    }
                }

                h2 {
                    margin: 0;
                    font-size: 1.1rem;
                }

                span {
                    font-size: 0.9rem;
                    color: grey;
                }
            }
        }
    }
</style>
