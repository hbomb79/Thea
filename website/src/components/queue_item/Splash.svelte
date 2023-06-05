<script lang="ts">
    import { fade } from "svelte/transition";

    import type { QueueDetails } from "queue";
    import { QueueStatus } from "queue";

    import InfoModal from "components/modal/InfoModal.svelte";
    import QueueItemControls from "components/queue_item/QueueItemControls.svelte";

    import wavesSvg from "assets/waves.svg";
    import questionMarkSvg from "assets/question-mark.svg";

    export let details: QueueDetails;
    export let queueControlCallback: (ev: CustomEvent) => void;

    let showInfoModal: boolean = false;

    // Only shuffle wave components if new details has a diff ID or status
    let currentItemID = details?.id;
    let currentItemStatus = details?.status;
    $: {
        //TODO: Stop treating NEEDS_RESOLVING and NEEDS_ATTENTION as the same
        // coloring at.. some point
        const normalizedStatus =
            details.status == QueueStatus.NEEDS_RESOLVING ? QueueStatus.NEEDS_ATTENTION : details.status;

        if (details.id != currentItemID || normalizedStatus != currentItemStatus) {
            shuffleWaveComponents();

            currentItemID = details.id;
            currentItemStatus = normalizedStatus;
        }
    }

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

    $: itemNameText = () => {
        return details.omdb_info?.Title || details.title_info?.Title || details.name || "UNNAMED";
    };

    $: itemStatusHelpText = () => {
        switch (details.status) {
            case QueueStatus.PENDING:
                return `Pending indicates that Thea has not started to work on this item in it's current stage.

                This is typically because Thea is waiting for CPU threads to become available while in the FFmpeg stage, 
                and the item transitioning to the 'Working' status may take some time depending on the progress of currently running transcodes.`;
            case QueueStatus.PROCESSING:
                return `Processing indicates that Thea is working on this item and hasn't encountered any issues.`;
            case QueueStatus.COMPLETED:
                return `Thea has completed this item and all it's associated FFmpeg instances.
                
                If new profiles are added that match this item, then Thea will NOT start any instances automatically as it usually would.
                Instead, you'll need to manually start a new FFmpeg transcoding task by navigating to the completed items via the main viewer.`;
            case QueueStatus.NEEDS_RESOLVING:
                return `All progress on this item has stopped because of one or multiple errors. This error can occur
                at many of the pipeline stages, and user input is required to remedy the problem.
                
                A common reason for this status is that no exact match could be found in OMDB, or all the FFmpeg instances
                for this item have encountered an error.
                
                Without remediation, this item will remain in this status indefinitely.`;
            case QueueStatus.CANCELLING:
                return `Thea is cancelling this item. Before this item transitions to 'Cancelled', Thea will need to wait for
                any uninteruptable tasks to complete (typically this is network related during the OMDB stage).
                
                If the item is in the FFmpeg stage, then the active instances (if any) will need to be stopped and any
                partial transcode outputs cleaned up from the server filesystem.`;
            case QueueStatus.CANCELLED:
                return `Thea has successfully cancelled this item and cleaned up any remaining artifacts. This item
                is now ignored by Thea and will not be present in the queue when Thea is next started.`;
            case QueueStatus.PAUSED:
                return `This item is paused and Thea will effectively 'ignore' it.

                If the item is in the FFmpeg stage, then all active ffmpeg processes
                will be suspended and no new processes will be spawned.`;
            case QueueStatus.NEEDS_ATTENTION:
                return `This item has encountered one or multiple errors during FFmpeg transcoding,
                however at least one of the processes is still working.
                
                If the working processes complete (meaning only the troubled instance remain), then
                the item will be transitioned to the 'Needs Resolving' status.`;
        }
    };

    $: itemStatusText = () => {
        switch (details.status) {
            case QueueStatus.PENDING:
                return "Waiting to Start";
            case QueueStatus.PROCESSING:
                return "Working";
            case QueueStatus.COMPLETED:
                return "Finished";
            case QueueStatus.NEEDS_RESOLVING:
                return "Needs Resolving";
            case QueueStatus.CANCELLING:
                return "Cancelling";
            case QueueStatus.CANCELLED:
                return "Cancelled";
            case QueueStatus.PAUSED:
                return "Paused";
            case QueueStatus.NEEDS_ATTENTION:
                return "Needs Attention";
        }
    };
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
                {itemNameText()}
                <span class="id">#{details.id}</span>
            </h2>
            <p class="sub">
                {itemStatusText()}
                <button class="status-help" on:click={() => (showInfoModal = true)}>{@html questionMarkSvg}</button>
            </p>

            <QueueItemControls on:queue-control={queueControlCallback} />
        </div>
    </div>
{/if}

<InfoModal bind:showModal={showInfoModal}>
    <span slot="header">Meaning of <em>{itemStatusText()}</em></span>

    <p>
        {@html itemStatusHelpText()
            .replaceAll(/ {2,}/g, "") // Strip multiple spaces
            .replaceAll(/\n\n/g, "<br/><br/>") // Convert two newlines in to breaks
            .replaceAll(/\n{1}/g, " ")}
    </p>
</InfoModal>

<style lang="scss">
    @import "../../styles/waves.scss";
</style>
