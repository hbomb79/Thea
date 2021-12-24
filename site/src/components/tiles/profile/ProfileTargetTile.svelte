<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import type { TranscodeTarget } from "../../../queue";

    export let target: TranscodeTarget = null;
    const dispatch = createEventDispatcher();

    const modifiedProps = (): number => {
        let count = 0;
        Object.keys(target.command).forEach((key) => {
            if (target.command[key]) count++;
        });

        return count;
    };
</script>

<div class="target">
    <h2 class="label">{target.label}</h2>

    <div class="command">
        <button>
            <i><b>{modifiedProps()}</b> modified options</i>
            <br />
            <b>Modify</b>
        </button>
    </div>
    <div class="controls">
        <div class="up" on:click={() => dispatch("move-up")}>UP</div>
        <div class="down" on:click={() => dispatch("move-down")}>DN</div>
    </div>
</div>

<style lang="scss">
    .target {
        background: white;
        border: solid 1px #eee;
        border-radius: 5px;
        margin: 0.5rem 0;
        display: flex;
        position: relative;
        align-items: center;
        overflow: hidden;

        .label {
            margin: 0;
            font-size: 1rem;
            font-weight: 400;
            flex: 1;
            padding: 8px 2rem;
        }

        .command {
            margin-right: 1rem;
        }

        .controls {
            text-align: center;
            width: 60px;
            height: 80px;
            line-height: 40px;
            border-left: solid 1px #cacbf7;

            > * {
                height: calc(50%);
                width: 100%;
                transition: background 200ms ease-in-out;
                background: #e4e3fa;
                cursor: pointer;

                &:hover {
                    background: #c2c1df;
                }

                &:nth-child(1) {
                    border-bottom: solid 1px #cacbf7;
                }
            }
        }
    }
</style>
