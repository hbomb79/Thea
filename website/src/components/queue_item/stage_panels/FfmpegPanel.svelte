<script lang="ts">
    import { CommanderTaskStatus, FfmpegProgress } from "queue";
    import type { CommanderTask as FfmpegInstance } from "queue";
    import workingHtml from "assets/html/dual-ring.html";
    import troubleSvg from "assets/err.svg";
    import checkSvg from "assets/check-mark.svg";
    import cancelSvg from "assets/cancel.svg";
    import scheduledSvg from "assets/pending.svg";
    import { SocketMessageType } from "stores/socket";
    import type { SocketData } from "stores/socket";
    import { commander } from "commander";
    import { itemFfmpegInstances } from "stores/queue";
    import { selectedQueueItem } from "stores/item";

    $: ffmpegInstances = $itemFfmpegInstances.get($selectedQueueItem) || [];

    function resolveTrouble(instance: FfmpegInstance, resolution: any) {
        commander.sendMessage(
            {
                type: SocketMessageType.COMMAND,
                title: "TROUBLE_RESOLVE",
                arguments: { id: $selectedQueueItem, instanceId: instance.id, ...resolution },
            },
            (reply: SocketData): boolean => {
                if (reply.type == SocketMessageType.ERR_RESPONSE) {
                    alert(`Failed to resolve ffmpeg instance trouble: ${reply.title}: ${reply.arguments.error}`);
                } else {
                    console.log("Resolution success!");
                }

                return false;
            }
        );
    }

    const retryHandler = (instance: FfmpegInstance) => {
        resolveTrouble(instance, { action: "retry" });
    };

    const pauseHandler = (instance: FfmpegInstance) => {
        resolveTrouble(instance, { action: "pause" });
    };

    const cancelHandler = (instance: FfmpegInstance) => {
        resolveTrouble(instance, { action: "cancel" });
    };

    const troubleResolvers: [string, (instance: FfmpegInstance) => void][] = [
        ["Retry", retryHandler],
        // ["Specify Profile", specifyProfileHandler],
        ["Pause", pauseHandler],
        ["Cancel", cancelHandler],
    ];

    const wrapSpinner = (spinner: string) => `<div class="spinner-wrap">${spinner}</div>`;
    $: getStageIcon = function (instance: FfmpegInstance): string {
        switch (instance.status) {
            case CommanderTaskStatus.WAITING:
                return scheduledSvg;
            case CommanderTaskStatus.WORKING:
                return wrapSpinner(workingHtml);
            case CommanderTaskStatus.SUSPENDED:
                return scheduledSvg;
            case CommanderTaskStatus.TROUBLED:
                return troubleSvg;
            case CommanderTaskStatus.CANCELLED:
                return cancelSvg;
            case CommanderTaskStatus.COMPLETE:
                return checkSvg;
            default:
                return "?";
        }
    };

    $: getCheckClass = function (instance: FfmpegInstance): string {
        switch (instance.status) {
            case CommanderTaskStatus.WAITING:
                return "pending";
            case CommanderTaskStatus.WORKING:
                return "working";
            case CommanderTaskStatus.TROUBLED:
                return "trouble";
            case CommanderTaskStatus.CANCELLED:
                return "cancelled";
            case CommanderTaskStatus.COMPLETE:
                return "complete";
            default:
                return "unknown";
        }
    };

    $: commanderStatusToText = function (status: CommanderTaskStatus): string {
        switch (status) {
            case CommanderTaskStatus.WAITING:
                return "Waiting for Resources";
            case CommanderTaskStatus.WORKING:
                return "Transcoding";
            case CommanderTaskStatus.SUSPENDED:
                return "Transcode Paused";
            case CommanderTaskStatus.TROUBLED:
                return "Troubled";
            case CommanderTaskStatus.COMPLETE:
                return "Transcode Complete";
            case CommanderTaskStatus.CANCELLED:
                return "Transcode Cancelled";
            default:
                return "Unknown Status";
        }
    };

    const dummyProgress: FfmpegProgress = {
        Progress: 40.008,
        Speed: "300Kb/s",
        Elapsed: "908",
        Frames: "100",
        Bitrate: "300",
    };
</script>

{#if ffmpegInstances.length == 0}
    <h2>Warming Up...</h2>
    <p>Thea is allocating some resources to this item. Won't be long...</p>
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
                        <h2>{instance.id}</h2>
                        <span>{commanderStatusToText(instance.status)}</span>
                    </div>
                    <div class="body" class:trouble={instance.trouble}>
                        {#if instance.trouble}
                            <h2 class="title">{instance.trouble.message}</h2>
                            <div class="controls">
                                {#each troubleResolvers as [display, handler] (display)}
                                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                                    <span class="button" on:click={() => handler(instance)}>{display}</span>
                                {/each}
                            </div>
                        {:else if instance.status == CommanderTaskStatus.WORKING && instance.progress}
                            <div class="ffmpeg-prog">
                                <div class="meta">
                                    <span class="elapsed">Elapsed {instance.progress.Elapsed}</span>
                                    <span class="speed">{instance.progress.Speed}</span>
                                </div>
                                <div class="percentage">
                                    <div class="fill-bg" style="width: {instance.progress.Progress}%" />
                                    <span>{Math.floor(instance.progress.Progress)}%</span>
                                </div>
                            </div>
                        {:else if instance.status == CommanderTaskStatus.COMPLETE}
                            <p>Item Completed</p>
                        {/if}
                    </div>
                </div>
                <div class="controls" />
            </li>
        {/each}
    </ul>
{/if}

<style lang="scss">
    @use "../../../styles/stageIcon.scss";

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

                    .ffmpeg-prog {
                        display: flex;
                        flex-direction: column;
                        width: 100%;

                        .percentage {
                            width: 100%;
                            display: flex;
                            align-items: center;
                            justify-content: center;
                            position: relative;
                            background: #bbbbbb30;

                            span {
                                font-size: 0.9rem;
                                color: #565287;
                                z-index: 100;
                            }

                            .fill-bg {
                                width: 0%;
                                position: absolute;
                                left: 0;
                                bottom: 0;
                                height: 100%;
                                transition: width 3s ease-in-out;
                                background-color: #aca8dd;
                            }
                        }
                    }

                    .controls {
                        flex: 0 0 auto;

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
