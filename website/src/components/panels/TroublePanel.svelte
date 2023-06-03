<script lang="ts">
    import { commander } from "../../commander";
    import { SocketMessageType } from "../../stores/socket";

    import type { SocketData } from "../../stores/socket";
    import { QueueTroubleType } from "../../queue";
    import type { QueueDetails } from "../../queue";

    import OmdbTroublePanel from "./trouble_panels/OmdbTroublePanel.svelte";
    import TitleTroublePanel from "./trouble_panels/TitleTroublePanel.svelte";
    import FormatTroublePanel from "./trouble_panels/FormatTroublePanel.svelte";

    interface EmbeddedPanel {
        selectResolver(arg0: string): void;
        selectedResolver(): string;
        listResolvers(): string[][];
        getHeader(): string;
        getBody(): string;
    }

    enum ComponentState {
        MAIN,
        RESOLVING,
        CONFIRMING,
        FAILURE,
    }

    export let queueDetails: QueueDetails;

    let state = ComponentState.MAIN;
    $: troubleDetails = queueDetails?.trouble;
    let failureDetails = "";

    let embeddedPanel: EmbeddedPanel = null;
    let embeddedPanelResolver = "";

    // sendResolution will attempt to send a trouble resolution command to the server
    // by appending the given data to the message using the spread syntax.
    // A callback must be provided, and is passed to the send command to enable
    // feedback from the server.
    function sendResolution(packet: CustomEvent) {
        const args = packet.detail.args;

        state = ComponentState.RESOLVING;
        commander.sendMessage(
            {
                title: "TROUBLE_RESOLVE",
                type: SocketMessageType.COMMAND,
                arguments: {
                    id: queueDetails.id,
                    ...args,
                },
            },
            function (data: SocketData) {
                if (data.type == SocketMessageType.RESPONSE) {
                    if (state == ComponentState.RESOLVING) state = ComponentState.CONFIRMING;
                } else {
                    state = ComponentState.FAILURE;
                    failureDetails = `Server rejected resolution with error:<br><code><b>${data.arguments.error}</b></code>`;
                }

                return true;
            }
        );
    }

    function updateEmbeddedPanelResolver() {
        embeddedPanelResolver = embeddedPanel.selectedResolver();
    }

    function resetPanel() {
        state = ComponentState.MAIN;

        // requestAnimationFrame because 'embeddedPanel' will
        // not exist as it's only present when state is LOADED.
        requestAnimationFrame(() => {
            if (embeddedPanel) embeddedPanel.selectResolver(embeddedPanelResolver);
        });
    }
</script>

<!-- Template -->
<div class="item modal trouble">
    {#if embeddedPanel}
        <div class="panel">
            <span
                class="panel-item"
                on:click={() => embeddedPanel.selectResolver("")}
                class:active={embeddedPanelResolver == ""}>Details</span
            >
            {#each embeddedPanel.listResolvers() as [display, key]}
                <span
                    class="panel-item"
                    class:active={embeddedPanelResolver == key}
                    on:click={() => embeddedPanel.selectResolver(key)}>{display}</span
                >
            {/each}
        </div>
    {/if}
    <main>
        {#if state == ComponentState.MAIN}
            {#if troubleDetails}
                {#if troubleDetails.type == QueueTroubleType.TITLE_FAILURE}
                    <TitleTroublePanel
                        bind:this={embeddedPanel}
                        {queueDetails}
                        on:try-resolve={sendResolution}
                        on:selection-change={updateEmbeddedPanelResolver}
                    />
                {:else if troubleDetails.type == QueueTroubleType.OMDB_MULTIPLE_RESULT_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_REQUEST_FAILURE || troubleDetails.type == QueueTroubleType.OMDB_NO_RESULT_FAILURE}
                    <OmdbTroublePanel
                        bind:this={embeddedPanel}
                        {queueDetails}
                        on:try-resolve={sendResolution}
                        on:selection-change={updateEmbeddedPanelResolver}
                    />
                {:else if troubleDetails.type == QueueTroubleType.FFMPEG_FAILURE}
                    <FormatTroublePanel
                        bind:this={embeddedPanel}
                        on:try-resolve={sendResolution}
                        on:selection-change={updateEmbeddedPanelResolver}
                    />
                {:else}
                    <h2>Cannot Resolve</h2>
                    <p class="sub">Unknown trouble type</p>
                    <p>
                        We don't have a known resolution for this trouble case. Please check server logs for guidance.
                    </p>
                {/if}

                {#if embeddedPanelResolver == "" && embeddedPanel}
                    <h2>{embeddedPanel.getHeader()}</h2>
                    <p class="sub">{@html embeddedPanel.getBody()}</p>

                    <p>
                        <code><b>Error: </b>{troubleDetails.message}</code><br /><br /><i
                            >Select an option above to begin resolving</i
                        >
                    </p>
                {/if}
            {:else}
                <h2>Trouble Information Unavailable</h2>
                <p class="sub">TROUBLE_DETAILS_MISSING</p>
                <p>
                    The item has a troubled status (NEEDS_RESOLVING or NEEDS_ATTENTION) however no trouble information
                    is available. This is a bug, please report to the server administrator
                </p>
            {/if}
        {:else if state == ComponentState.RESOLVING || state == ComponentState.CONFIRMING}
            <h2>Resolving trouble</h2>
            <p class="sub">
                {#if state == ComponentState.RESOLVING}
                    Waiting for server
                {:else}
                    Verifying item progression
                {/if}
            </p>
            <p>
                Please wait while we confirm that your resolution data solved the problem. This could take a few
                seconds...
            </p>
        {:else if state == ComponentState.FAILURE}
            <h2>Trouble Resolution Failed</h2>
            <p class="sub">Resolution rejected</p>

            <p>{@html failureDetails}</p>

            <button on:click|preventDefault={resetPanel}>Back</button>
        {:else}
            <h2>Unknown Error</h2>
            <p class="sub">Component state error</p>
            <span class="err"
                >This component has an inner state that is out-of-bounds for normal operation.<br />Please try closing
                and re-opening this modal</span
            >
            <p><i>Contact server administrator if issue persists</i></p>
        {/if}
    </main>
</div>

<style lang="scss">
    @use "../../styles/global.scss";
    @use "../../styles/tile.scss";

    .modal.trouble {
        margin: 0;
        flex-direction: column;
        position: initial;
        max-width: none;

        main {
            padding: 1rem 2rem;

            @import "../../styles/trouble.scss";
        }
    }
</style>
