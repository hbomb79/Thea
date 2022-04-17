<script lang="ts">
    import { createEventDispatcher, getContext } from "svelte";
    import type { TranscodeTarget } from "../../../queue";
    import TargetProps from "../../modals/TargetProps.svelte";

    export let target: TranscodeTarget = null;
    const dispatch = createEventDispatcher();

    const { open } = getContext("simple-modal");

    const modifiedProps = (com: Object): number => {
        let count = 0;
        Object.keys(com).forEach((key) => {
            if (com[key]) count++;
        });

        return count;
    };

    $: modified = modifiedProps(target.command);

    const openPropDialog = () => {
        open(
            TargetProps,
            {
                onOkay: (modifiedMap: Map<string, any>) => {
                    console.log("Dialog SAVED", modifiedMap, target.command);
                    Object.entries(modifiedMap).forEach((v) => {
                        const [key, value] = v;
                        target.command[key] = value;
                    });

                    dispatch("propertiesChanged", modifiedMap);
                },
                onCancel: () => {
                    console.log("Dialog cancelled");
                },
                availableProperties: { ...target.command },
            },
            {
                closeButton: false,
            }
        );
    };
</script>

<div class="target">
    <h2 class="label">{target.label}</h2>

    <div class="command">
        <button on:click|preventDefault={openPropDialog}>
            <i><b>{modified}</b> modified options</i>
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
