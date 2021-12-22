<script lang="ts">
    import type { TranscodeProfile, QueueDetails } from "../../queue";
    import CreateIcon from "../../assets/create-icon.svg";
    import ServerProfile from "./profile/ServerProfile.svelte";
    import { commander } from "../../commander";
    import { SocketMessageType } from "../../store";
    import type { SocketData } from "../../store";

    export let profiles: TranscodeProfile[] = [];
    export let details: Map<number, QueueDetails> = null;

    const countUse = function (tag: string): number {
        let count = 0;
        details.forEach((val: QueueDetails) => {
            if (val.profile_tag == tag) count++;
        });

        return count;
    };

    const removeProfile = (profileTag: string) => {
        console.log("Removing profile:", profileTag);

        commander.sendMessage(
            {
                title: "PROFILE_REMOVE",
                type: SocketMessageType.COMMAND,
                arguments: {
                    tag: profileTag,
                },
            },
            (data: SocketData): boolean => {
                return false;
            }
        );
    };

    const selectProfile = (profileTag: string) => {
        console.log("Selecting profile:", profileTag);
    };
</script>

<ul class="profiles">
    {#each profiles as profile}
        <ServerProfile
            {profile}
            usages={countUse(profile.tag)}
            on:select={(ev) => selectProfile(ev.detail)}
            on:remove={(ev) => removeProfile(ev.detail)}
        />
    {/each}

    <li class="profile create">
        {@html CreateIcon}
        <span>Create Profile</span>
    </li>
</ul>

<style lang="scss">
    .profiles {
        list-style: none;
        padding: 0;

        .profile.create {
            display: flex;
            justify-content: center;
            align-items: center;
            border-color: transparent;
            color: #707070;

            &:hover {
                color: black;

                :global(svg) {
                    fill: black;
                }
            }

            :global(svg) {
                margin: 0 1rem;
                width: 2rem;
                height: 2rem;
                fill: #707070;
                transition: fill 200ms ease-out;
            }
        }
    }
</style>
