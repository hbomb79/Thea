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

    export let profiles: TranscodeProfile[] = [];
    export let index: QueueItem[] = [];
    export let details: Map<number, QueueDetails> = new Map();

    let state: SettingsState = SettingsState.MAIN;

    // Change the state of the Settings component to the newly-provided state
    const changeState = (state: SettingsState) => {
        console.log("Settings state-change: ", state);
    };
</script>

<!-- Template Start -->
{#if state == SettingsState.MAIN}
    <div class="column main">
        <div class="tile profiles">
            <h2 class="header">Profiles</h2>
            <div class="content trans">
                <ServerProfiles {profiles} {details} on:select={(customEvent) => {}} />
            </div>
        </div>
    </div>
    <div class="column">
        <div class="tile misc">
            <h2 class="header">Config</h2>
            <div class="content trans" />
        </div>
        <div class="tile cache">
            <h2 class="header">Cache</h2>
            <div class="content trans">
                <ServerCache />
            </div>
        </div>
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
