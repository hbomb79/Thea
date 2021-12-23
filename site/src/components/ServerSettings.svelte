<!--
    Server Settings is a Svelte component that is used
    to display a dashboard-like (screen made up of tiles) view
    of Server related settings (profile, targets, cache, security, etc).

    Each sub-tile should display some basic HTML that will be styled
    by this component
-->
<script lang="ts" context="module">
    enum SettingsState {
        MAIN,
        PROFILE,
        CACHE,
        CONFIG,
    }
</script>

<script lang="ts">
    import ServerProfiles from "./tiles/ServerProfiles.svelte";
    import ServerCache from "./tiles/ServerCache.svelte";
    import type { QueueDetails, TranscodeProfile, QueueItem } from "../queue";
    import ServerProfileDetail from "./ServerProfileDetail.svelte";
    import { fade } from "svelte/transition";

    export let profiles: TranscodeProfile[] = [];
    export let index: QueueItem[] = [];
    export let details: Map<number, QueueDetails> = new Map();

    let state: SettingsState = SettingsState.MAIN;
    let selectedProfile: string = null;

    // Change the state of the Settings component to the newly-provided state
    const changeState = (newState: SettingsState) => {
        console.log("Settings state-change: ", newState);
        state = newState;
        selectedProfile = null;
    };

    const selectProfile = (profileTag: string) => {
        changeState(SettingsState.PROFILE);
        selectedProfile = profileTag;
    };
</script>

<!-- Template Start -->
{#if state == SettingsState.MAIN}
    <div class="column main">
        <div class="tile profiles" in:fade={{ duration: 150, delay: 100 }}>
            <h2 class="header">Profiles</h2>
            <div class="content trans">
                <ServerProfiles
                    {profiles}
                    {details}
                    on:select={(ev) => {
                        selectProfile(ev.detail);
                    }}
                />
            </div>
        </div>
    </div>
    <div class="column">
        <div class="tile misc" in:fade={{ duration: 150, delay: 150 }}>
            <h2 class="header">Config</h2>
            <div class="content trans" />
        </div>
        <div class="tile cache" in:fade={{ duration: 150, delay: 250 }}>
            <h2 class="header">Cache</h2>
            <div class="content trans">
                <ServerCache />
            </div>
        </div>
    </div>
{:else if state == SettingsState.PROFILE}
    <div>
        <ServerProfileDetail
            {profiles}
            profileTag={selectedProfile}
            on:deselect={() => changeState(SettingsState.MAIN)}
        />
    </div>
{/if}

<!-- Template End -->
<style lang="scss">
    .column {
        .tile.misc,
        .tile.cache {
            flex: 1 auto;
        }
    }
</style>
