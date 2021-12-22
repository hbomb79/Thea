<script lang="ts">
    import { createEventDispatcher } from "svelte";

    import type { TranscodeProfile } from "../queue";

    export let profiles: TranscodeProfile[] = [];
    export let profileTag: string = null;

    const dispatch = createEventDispatcher();

    $: findProfile = (tag: string) => profiles.findIndex((v) => v.tag == tag);
    $: profileIndex = findProfile(profileTag);
    $: profile = profiles[profileIndex];
</script>

<button on:click|preventDefault={() => dispatch("deselect")}>&lt;- Back to Settings</button>
{#if profileIndex == -1}
    <b>Profile not found</b>
    <p>The profile ({profileTag}) cannot be found. It may have been deleted or renamed.</p>
{:else}
    <b>Profile Details</b>
    <p>{profile.tag}</p>
{/if}
