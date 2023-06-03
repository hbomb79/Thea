<script lang="ts">
    import { createEventDispatcher, getContext } from "svelte";
    import type { TranscodeProfile, TranscodeTarget } from "../../../queue";
    import TargetProps from "../../modals/TargetProps.svelte";

    export let profile: TranscodeProfile = null;
    const dispatch = createEventDispatcher();

    const { open } = getContext("simple-modal");

    const modifiedProps = (com: Object): number => {
        let count = 0;
        Object.keys(com).forEach((key) => {
            if (com[key]) count++;
        });

        return count;
    };

    $: modified = modifiedProps(profile.command);

    const openPropDialog = () => {
        open(
            TargetProps,
            {
                onOkay: (modifiedMap: Map<string, any>) => {
                    console.log("Dialog SAVED", modifiedMap, profile.command);
                    Object.entries(modifiedMap).forEach((v) => {
                        const [key, value] = v;
                        profile.command[key] = value;
                    });

                    dispatch("propertiesChanged", modifiedMap);
                },
                onCancel: () => {
                    console.log("Dialog cancelled");
                },
                availableProperties: { ...profile.command },
            },
            {
                closeButton: false,
            }
        );
    };
</script>

<div class="ffmpeg-opts">
    <div class="command">
        <button on:click|preventDefault={openPropDialog}>
            <i><b>{modified}</b> modified options</i>
            <br />
            <b>Modify</b>
        </button>
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
