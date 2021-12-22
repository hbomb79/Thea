<script lang="ts">
    import type { QueueDetails, TranscodeProfile } from "../../../queue";
    import CancelIcon from "../../../assets/cancel.svg";
    import EditIcon from "../../../assets/edit.svg";
    import { createEventDispatcher } from "svelte";

    export let profile: TranscodeProfile = null;
    export let usages: Number = 0;

    const dispatch = createEventDispatcher();
</script>

<li class="profile" on:click|stopPropagation={() => dispatch("select", profile.tag)}>
    <div class="stat">
        <span class="tag">{profile.tag}</span>
        <span class="apply-stat">Applied to <span class="count">{usages}</span> items</span>
        <span class="target-stat">{profile.targets.length} target{profile.targets.length == 1 ? "" : "s"}</span>
    </div>

    <div class="controls">
        <div class="remove control" on:click|stopPropagation={() => dispatch("remove", profile.tag)}>
            {@html CancelIcon}
        </div>
    </div>
</li>

<style lang="scss">
    .profile {
        padding: 1.3rem 2rem;
        margin-bottom: 1rem;
        border: solid 1px #eee;
        transition: all 200ms ease-out;
        transition-property: background, border-color, color;
        cursor: pointer;

        display: flex;

        &:not(.create),
        &:hover {
            background: white;
            border-radius: 8px;
        }

        .controls {
            flex: 0 auto;
            margin-left: 2rem;

            :global(svg) {
                width: 1.5rem;
                height: 1.5rem;
            }

            .control {
                display: inline-block;
            }
        }

        .stat {
            flex: 1 auto;

            .tag {
                font-weight: 600;
            }

            .apply-stat {
                float: right;
            }

            .target-stat {
                display: block;
            }
        }
    }
</style>
