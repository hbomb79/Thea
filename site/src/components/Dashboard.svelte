<script lang="ts">
import { onMount } from "svelte";
import Queue from "./Queue.svelte";


const optionElements = new Array(3)
const options = ["Home", "Queue", "Settings"]
let selectionOption = 0

onMount(() => {
    optionElements.forEach((item: HTMLElement, index) => {
        item.addEventListener("click", () => selectionOption = index)
    })
})
</script>


<style lang="scss">
:root(body) {
    padding: 0;
}

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
        background: #ffffff3f;
        border: solid 1px #c4b8db;
        border-radius: 4px;
        box-shadow: 0px 0px 3px #0000000f;
        .sidebar {
            width: 250px;
            height: 100%;
            background: #ffffff3f;
            border-right: solid 1px #bfbfbf5f;
            position: relative;

            h2 {
                color: #9184c5;
                text-align: left;
                padding: 2rem 0 2rem 2rem;
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

                    transition: all 150ms ease-in-out;
                    transition-property: background, border-top-right-radius, border-bottom-right-radius, margin-right;

                    &:hover {
                        background: #ffffff55;
                    }

                    &.active {
                        background: linear-gradient( 326deg , #d9b6ea52, #bfd9ff6b);
                        margin-right:0;
                        border-top-right-radius: 0;
                        border-bottom-right-radius: 0;
                    }
                }
            }

            .footer {
                position: absolute;
                bottom: 0;
                padding: 8px;
                text-align: center;
                width: 100%;

                color: #5e5e5e;
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
            align-items: center;
            justify-content: space-around;
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
                height: 90%;
                text-align: left;
                width: 30%;
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
                        background: #ffffff94;
                        padding: 0;
                        border-radius: 4px;
                        box-shadow: 0px 0px 3px #0000000f;
                    }
                }

                .tile.status .content {
                    background: none;
                    display: flex;
                    flex-direction: row;
                    justify-content: space-between;
                    box-shadow: none;

                    .mini-tile {
                        width: 8rem;
                        height: 8rem;
                        padding: 1rem;
                    }
                }

                .tile.queue {
                    display: flex;
                    flex-direction: column;

                    .content {
                        flex-grow: 1;
                    }
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
                    <div class="tile status">
                        <h2 class="header">Status</h2>
                        <div class="content">
                            <div class="mini-tile format"></div>
                            <div class="mini-tile complete"></div>
                            <div class="mini-tile trouble"></div>
                        </div>
                    </div>
                    <div class="tile workers">
                        <h2 class="header">Workers</h2>
                        <div class="content">

                        </div>
                    </div>
                </div>
                <div class="column">
                    <div class="tile queue">
                        <h2 class="header">Queue</h2>
                        <div class="content">
                            <div class="data">
                                <p>Joker</p>
                                <p>1917</p>
                                <p>WandaVision</p>
                                <p>Rick and Morty</p>
                            </div>
                        </div>
                    </div>
                </div>
            {:else if selectionOption == 1}
                <Queue/>
            {/if}
        </div>
    </div>
</div>
