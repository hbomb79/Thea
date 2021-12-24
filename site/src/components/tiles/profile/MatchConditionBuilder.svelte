<script lang="ts">
    import { commander, ffmpegMatchKeysStream } from "../../../commander";

    import { MatchType, ModifierType } from "../../../queue";
    import type { ProfileMatchCriterion, TranscodeProfile } from "../../../queue";
    import type { SocketData } from "../../../store";
    import { SocketMessageType } from "../../../store";

    export let profile: TranscodeProfile;
    export let matchComponents: ProfileMatchCriterion[] = [];

    let matchKeys = $ffmpegMatchKeysStream;
    let syncing: boolean = false;

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
        match.key = e.target.value;
        syncMatchComponents();
    };

    const appendNewMatchComponent = () => {
        if (!matchComponents) {
            matchComponents = [];
        }
        matchComponents.push({
            key: matchKeys[0],
            matchType: MatchType.MATCHES,
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
                <div class="content">
                    <!-- svelte-ignore a11y-no-onchange -->
                    <select on:change={(e) => matchKeyInputChange(match, e)} bind:value={match.key} disabled={syncing}>
                        {#each matchKeys as matchKey}
                            <option value={matchKey}>{matchKey}</option>
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
                            <option value={MatchType[t]}>{t}</option>
                        {/each}
                    </select>
                    <input
                        name="target"
                        type="text"
                        on:change={(e) => matchTargetInputChange(match, e)}
                        bind:value={match.matchTarget}
                        disabled={syncing}
                    />
                    <button
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
    <button disabled={syncing} on:click|preventDefault={appendNewMatchComponent}>Add Condtion +</button>
</ul>
