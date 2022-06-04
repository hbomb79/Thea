<script lang="ts">
    import { commander, profileMatchValidTypes } from "../../../commander";

    import { MatchKey, MatchType, ModifierType } from "../../../queue";
    import type { ProfileMatchCriterion, TranscodeProfile } from "../../../queue";
    import type { SocketData } from "../../../store";
    import { SocketMessageType } from "../../../store";

    export let profile: TranscodeProfile;
    export let matchComponents: ProfileMatchCriterion[] = [];

    let syncing: boolean = false;
    let validTypes = $profileMatchValidTypes;

    const syncMatchComponents = () => {
        syncing = true;
        commander.sendMessage(
            {
                title: "PROFILE_SET_MATCH_CONDITIONS",
                type: SocketMessageType.COMMAND,
                arguments: { profileTag: profile.tag, matchConditions: matchComponents },
            },
            (response: SocketData): boolean => {
                syncing = false;
                if (response.type == SocketMessageType.ERR_RESPONSE) {
                    alert(`Failed to set match conditions: ${response.arguments.error}`);
                    matchComponents = profile.matchCriteria;
                }

                return false;
            }
        );
    };

    const matchTargetInputChange = (match: ProfileMatchCriterion, e: any) => {
        match.matchTarget = e.target.value;
        syncMatchComponents();
    };

    const matchTypeInputChange = (match: ProfileMatchCriterion, e: any) => {
        match.matchType = new Number(e.target.value) as MatchType;
        syncMatchComponents();
    };

    const matchKeyInputChange = (match: ProfileMatchCriterion, e: any) => {
        match.key = new Number(e.target.value) as MatchKey;

        // Test if the current match type is valid for the new type - if not, replace with the default (IS_PRESENT).
        if (!validTypes[match.key].includes(match.matchType)) {
            match.matchType = MatchType.IS_PRESENT;
            match.matchTarget = "";
        }
        syncMatchComponents();
    };

    const appendNewMatchComponent = () => {
        if (!matchComponents) {
            matchComponents = [];
        }
        matchComponents.push({
            key: MatchKey.TITLE,
            matchType: MatchType.IS_PRESENT,
            modifier: ModifierType.AND,
            matchTarget: "",
        });

        matchComponents = matchComponents;
        syncMatchComponents();
    };
</script>

<ul class="criteria">
    {#if matchComponents}
        {#each matchComponents as match, index}
            <li class="match">
                <div class="components">
                    <!-- svelte-ignore a11y-no-onchange -->
                    <select on:change={(e) => matchKeyInputChange(match, e)} bind:value={match.key} disabled={syncing}>
                        {#each Object.keys(MatchKey).filter((v) => v.length > 1) as t}
                            <option value={MatchKey[t]}>{t}</option>
                        {/each}
                    </select>
                    <!-- svelte-ignore a11y-no-onchange -->
                    <select
                        name="type"
                        on:change={(e) => matchTypeInputChange(match, e)}
                        bind:value={match.matchType}
                        disabled={syncing}
                    >
                        {#each Object.keys(MatchType).filter((v) => v.length > 1) as t}
                            {#if validTypes[match.key].includes(MatchType[t])}
                                <option value={MatchType[t]}>{t}</option>
                            {/if}
                        {/each}
                    </select>
                    <input
                        name="target"
                        type="text"
                        on:change={(e) => matchTargetInputChange(match, e)}
                        bind:value={match.matchTarget}
                        disabled={syncing || match.matchType >= MatchType.IS_PRESENT}
                    />
                    <button
                        class="del"
                        on:click|preventDefault={() => {
                            matchComponents.splice(index, 1);
                            syncMatchComponents();
                        }}>Remove</button
                    >
                </div>

                {#if index + 1 < matchComponents.length}
                    <div class="modifier">
                        <button
                            class="match-mod"
                            disabled={syncing}
                            on:click={() => (match.modifier = match.modifier ? 0 : 1)}
                            >{match.modifier ? "OR" : "AND"}</button
                        >
                    </div>
                {/if}
            </li>
        {/each}
    {:else}
        <b>Something has gone wrong.</b>
    {/if}
    <div class="new">
        <button disabled={syncing} on:click|preventDefault={appendNewMatchComponent}>Add Condtion +</button>
    </div>
</ul>

<style lang="scss">
    ul {
        list-style: none;
        margin: 0;
    }

    .components {
        padding: 1rem 0;
    }

    select,
    input,
    button {
        background: white;
        border-color: #e3dff9;
        color: #9487c6;
        margin: 0px 8px;
    }

    .del {
        float: right;
    }

    .modifier,
    .new {
        text-align: center;
    }
</style>
