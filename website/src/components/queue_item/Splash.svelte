<script lang="ts">
    import { fade } from "svelte/transition";

    import type { QueueDetails } from "queue";
    import { QueueStatus } from "queue";

    import QueueItemControls from "components/queue_item/QueueItemControls.svelte";

    import wavesSvg from "assets/waves.svg";

    export let details: QueueDetails;
    export let queueControlCallback: (ev: CustomEvent) => void;

    $: if (details) shuffleWaveComponents();

    let waveComponent: HTMLElement;

    function shuffleWaveComponents() {
        if (waveComponent === undefined) return;

        const components = waveComponent.getElementsByTagName("path");

        for (let i = 0; i < components.length; i++) {
            const item = components.item(i);

            // Apply random offset to wave X-axis.
            const random = Math.random() * 200;
            item.setAttribute("transform", `translate(-${random}, 0)`);
        }
    }
</script>

{#if details}
    <div
        class="splash"
        class:trouble={details.status == QueueStatus.NEEDS_ATTENTION || details.status == QueueStatus.NEEDS_RESOLVING}
        in:fade={{ duration: 150, delay: 50 }}
    >
        <div class="waves" bind:this={waveComponent}>{@html wavesSvg}</div>
        <div class="content">
            <h2 class="title">
                {details.omdb_info?.Title || details.title_info?.Title || details.name || "UNNAMED"}
                <span class="id">#{details.id}</span>
            </h2>
            <p class="sub">Item Status</p>

            <QueueItemControls on:queue-control={queueControlCallback} />
        </div>
    </div>
{/if}

<style lang="scss">
    @import "../../styles/waves.scss";
</style>
