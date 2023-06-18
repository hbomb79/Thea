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
    import { fade } from "svelte/transition";

    import ServerProfileDetail from "components/settings/ServerProfileDetail.svelte";
    import ServerProfiles from "components/settings/ServerProfiles.svelte";
    import ServerCache from "components/settings/ServerCache.svelte";
    import { ffmpegProfiles } from "stores/profiles";
    import Modal from "svelte-simple-modal";

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
<Modal>
    <div class="tiles">
    {#if state == SettingsState.MAIN}
        <div class="column main">
            <div class="tile profiles" in:fade={{ duration: 150, delay: 100 }}>
                <h2 class="header">Profiles</h2>
                <div class="content trans">
                    <ServerProfiles
                        profiles={$ffmpegProfiles}
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
                profiles={$ffmpegProfiles}
                profileTag={selectedProfile}
                on:deselect={() => changeState(SettingsState.MAIN)}
            />
        </div>
    {/if}
    </div>
</Modal>

<!-- Template End -->
<style lang="scss">
    .column {
        .tile.misc,
        .tile.cache {
            flex: 1 auto;
        }
    }
</style>
