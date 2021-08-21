<script lang="ts">
import { onMount } from "svelte";
import Queue from "./Queue.svelte";
import ProgressBar from 'progressbar.js'

import healthSvg from '../assets/health.svg';

const optionElements = new Array(3)
let domCompleted:HTMLElement
const options = ["Home", "Queue", "Settings"]
let selectionOption = 0

onMount(() => {
    optionElements.forEach((item: HTMLElement, index) => {
        item.addEventListener("click", () => selectionOption = index)
    })

    var bar = new ProgressBar.Circle(domCompleted, {
      color: '#aaa',
      strokeWidth: 4,
      trailWidth: 1,
      easing: 'easeInOut',
      duration: 1400,
      text: {
        autoStyleContainer: false
      },
      from: { color: '#aaa', width: 1 },
      to: { color: '#333', width: 4 },
      // Set default step function for all animate calls
      step: function(state, circle) {
        circle.path.setAttribute('stroke', state.color);
        circle.path.setAttribute('stroke-width', state.width);

          circle.setText('7/10');
      }
    });

    bar.text.style.fontFamily = '"Raleway", Helvetica, sans-serif';
    bar.text.style.fontSize = '1rem';

    bar.animate(0.7);
})
</script>


<style lang="scss">
.dashboard {
    position: relative;
    width: 100%;
    height: 100%;

    .wrapper {
        position: absolute;
        top: 4rem;
        bottom: 4rem;
        left: 4rem;
        right: 4rem;
        background: linear-gradient(122deg, #ffffffc7, #ffffff45);
        border: solid 1px #c4b8db;
        border-radius: 10px;
        box-shadow: 0px 0px 3px #0000000f;
        overflow: hidden;
        max-width: 1400px;
        margin: 0 auto;

        .sidebar {
            width: 250px;
            height: 100%;
            background: #ffffff3f;
            border-right: solid 1px #bfbfbf5f;
            position: relative;

            h2 {
                color: #9184c5;
                text-align: left;
                padding: 2rem 0 1rem 2rem;
                margin: 0;
            }

            .options {
                .option {
                    padding: 1rem 2rem;
                    margin: 1rem;
                    text-align: left;
                    border-radius: 8px;
                    cursor: pointer;
                    background: #ffffff00;
                    color: #9aa1d3;

                    transition: all 150ms ease-in-out;
                    transition-property: background, border-top-right-radius, border-bottom-right-radius, margin-right;

                    &:hover {
                        background: #ffffff55;
                    }

                    &.active {
                        background: linear-gradient( 326deg , #d9b6ea52, #bfd9ff6b);
                        margin-right: 0;
                        border-top-right-radius: 0;
                        border-bottom-right-radius: 0;
                        font-weight: 500;
                        color: #9285c5;
                    }
                }
            }

            .footer {
                position: absolute;
                bottom: 0;
                padding: 8px;
                text-align: center;
                width: 100%;

                color: #988cc9;
            }
        }

        .tiles {
            position: absolute;
            left: 250px;
            right: 0;
            bottom: 0;
            top: 0;
            display: flex;
            flex-direction: row;
            padding: 3rem;
            align-items: flex-start;
            justify-content: space-between;
            overflow-y: auto;
            &::-webkit-scrollbar {
                width: 12px;
                background-color: #817d7d66;
                box-shadow: 0px 0px 8px -5px black;
            }

            &::-webkit-scrollbar-thumb {
                background-color: #ffffffb0;
                border-radius: 7px;
            }

            &::-webkit-scrollbar-thumb:hover {
                background-color: #ffffffef;
            }

            .column {
                height: 100%;
                text-align: left;
                width: 35%;
                display: flex;
                flex-direction: column;

                &.main {
                    width: 60%;
                }

                .tile {
                    width: 100%;

                    .header {
                        font-size: 1rem;
                        color: #9184c5;
                        padding-left: 1rem;
                    }

                    .content, .content .mini-tile {
                        background: #ffffff;
                        padding: 0;
                        border-radius: 4px;
                        box-shadow: 0px 0px 3px #0000000f;
                    }
                }

                .tile.overview {
                    margin-bottom: 2rem;

                    .content {
                        height: 180px;
                        background: linear-gradient(324deg, #ed17d3a1, #42c0dd);
                        position: relative;

                        h2 {
                            padding: 2rem 0 0 2rem;
                            color: white;
                            margin: 0;
                            font-size: 2rem;
                        }

                        p {
                            margin: 0 0 0 2rem;
                            color: #edf2fe;
                            font-size: 1.1rem;
                        }

                        :global(svg) {
                            height: 100px;
                            fill: white;
                            position: absolute;
                            right: 2.8rem;
                            top: 2.4rem;
                            width: auto;
                        }
                    }
                }

                .tile.status {
                    margin-bottom: 2rem;

                    .content {
                        background: none;
                        display: flex;
                        flex-direction: row;
                        justify-content: space-between;
                        box-shadow: none;

                        .mini-tile {
                            width: 40%;
                            height: 6rem;
                            padding: 1rem;
                            display: flex;
                            flex-direction: row;

                            .main {
                                width: 35%;
                                height: 100%;
                                flex-grow: 0;
                                display: flex;
                                flex-direction: column;
                                justify-content: center;

                                .progress {
                                    position: relative;
                                    flex-grow: 0;
                                }
                            }

                            .tag {
                                flex-grow: 1;
                                align-self: center;
                                text-align: center;
                            }
                        }
                    }
                }

                .tile.queue {
                    display: flex;
                    flex-direction: column;
                }
            }
        }
    }
}
</style>

<div class="dashboard">
    <div class="wrapper">
        <div class="sidebar">
            <h2 class="header">Dashboard</h2>

            <div class="options">
                {#each options as title, k}
                    <div class="option" class:active={selectionOption == k} bind:this={optionElements[k]}>{title}</div>
                {/each}
            </div>

            <div class="footer">
                <span>Made with &lt;3 on <a href="https://github.com/hbomb79/TPA">GitHub</a></span>
            </div>
        </div>

        <div class="tiles">
            {#if selectionOption == 0}
                <div class="column main">
                    <div class="tile overview">
                        <h2 class="header">Overview</h2>
                        <div class="content">
                            <h2>System Health</h2>
                            <p>All systems healthy</p>

                            {@html healthSvg}
                        </div>
                    </div>
                    <div class="tile status">
                        <div class="content">
                            <div class="mini-tile complete">
                                <div class="main">
                                    <div class="progress" bind:this={domCompleted}></div>
                                </div>
                                <p class="tag">Items Complete</p>
                            </div>
                            <div class="mini-tile trouble">
                                <div class="main"></div>
                                <p class="tag">Need Assistance</p>
                            </div>
                        </div>
                    </div>
                    <div class="tile workers">
                        <h2 class="header">Workers</h2>
                        <div class="content" style="min-height:230px;">

                        </div>
                    </div>
                </div>
                <div class="column">
                    <div class="tile queue">
                        <h2 class="header">Queue</h2>
                        <div class="content">
                            <Queue minified={true}/>
                        </div>
                    </div>
                </div>
            {:else if selectionOption == 1}
                <Queue minified={false}/>
            {/if}
        </div>
    </div>
</div>
