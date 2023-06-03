<script lang="ts">
    import type { TranscodeProfile, TranscodeTarget } from "../../../queue";
    import CancelIcon from "../../../assets/cancel.svg";
    import { createEventDispatcher, getContext } from "svelte";
    import ProfileTargetTile from "./ProfileTargetTile.svelte";
    import { commander } from "../../../commander";
    import { SocketMessageType } from "../../../stores/socket";
    import type { SocketData } from "../../../stores/socket";
    import ConfirmationPopup from "../../modals/ConfirmationPopup.svelte";
    import MatchConditionBuilder from "./MatchConditionBuilder.svelte";

    const { open } = getContext<any>("simple-modal");

    export let profile: TranscodeProfile = null;

    const confirmRemoveProfile = () => {
        open(
            ConfirmationPopup,
            {
                title: "Delete Profile",
                body: `Are you sure you want to delete profile <b>${profile.tag}</b>?<br />This action <i>cannot be reversed!</i><br/><br/>Please note that items already using this transcoder profile/targets will be unaffected by this change.`,
                onOkay: () => dispatch("remove", profile.tag),
            },
            { closeButton: false }
        );
    };

    const updateCommand = (profile: TranscodeProfile, command: any) => {
        commander.sendMessage(
            {
                title: "PROFILE_UPDATE_COMMAND",
                type: SocketMessageType.COMMAND,
                arguments: {
                    profileTag: profile.tag,
                    command: command,
                },
            },
            (response: SocketData): boolean => {
                if (response.type == SocketMessageType.ERR_RESPONSE) {
                    alert(`Failed to update profile ${profile.tag} command to ${command}: ${response.arguments.error}`);
                }

                return false;
            }
        );
    };

    const dispatch = createEventDispatcher();
    let isOpen: boolean = false;
</script>

<li class="profile" class:open={isOpen}>
    <div class="header" on:click|stopPropagation={() => (isOpen = !isOpen)}>
        <div class="stat">
            <span class="tag">{profile.tag} <span class="apply-stat"> - applied to <b>???</b> items</span></span>
        </div>

        <div class="controls">
            <div class="remove control" on:click|stopPropagation={confirmRemoveProfile}>
                {@html CancelIcon}
            </div>
        </div>
    </div>

    {#if isOpen}
        <div class="main">
            <div class="settings">
                <div>
                    <b>Output Path</b>
                    <input type="text" placeholder="<default>" />
                </div>

                <div>
                    <b>Match Criteria</b>
                    <MatchConditionBuilder {profile} matchComponents={profile.matchCriteria} />
                </div>

                <div>
                    <b>FFmpeg Command</b>
                    <ProfileTargetTile
                        {profile}
                        on:propertiesChanged={(event) => updateCommand(profile, event.detail)}
                    />
                </div>
            </div>
        </div>
    {/if}
</li>

<style lang="scss">
    .profile {
        margin-bottom: 1rem;
        border: solid 1px #d4d6e1;
        border-radius: 5px;
        background: #f3f5fe;
        overflow: hidden;
        transition: box-shadow 500ms ease-out;
        box-shadow: none;

        .header {
            padding: 1.3rem 2rem;
            cursor: pointer;
            display: flex;
            transition: all 200ms ease-out;
            transition-property: background, border-color, color;

            &:hover {
                background: white;
            }

            .controls {
                flex: 0 auto;
                margin-left: 2rem;
                display: flex;
                align-items: center;

                :global(svg) {
                    width: 1.5rem;
                    height: 1.5rem;
                    fill: #9aa1d3;
                    padding: 8px;
                }

                .control {
                    display: inline-block;
                    background: transparent;
                    border-radius: 8px;
                    transition: all 150ms ease-in;
                    transition-property: background box-shadow;

                    &:hover {
                        background: #c0d7fd8a;
                        box-shadow: 0px 0px 3px #e2d9f6;
                    }
                }
            }

            .stat {
                flex: 1 auto;

                .tag {
                    font-weight: 600;
                }

                .apply-stat {
                    font-weight: 300;
                    font-style: italic;
                }

                .target-stat {
                    display: block;
                    font-weight: 400;
                    color: #9aa1d3;
                }
            }
        }

        .main {
            display: none;
            padding: 1.3rem 2rem;
            background: #f3f5fe;
        }

        &.open {
            box-shadow: 0px 0px 6px #d7d7d7;
            .header {
                background: white;
            }

            .main {
                display: block;
            }
        }
    }
</style>
