<script lang="ts">
    import importIcon from "../../assets/import-stage.svg";
    import titleIcon from "../../assets/title-stage.svg";
    import omdbIcon from "../../assets/omdb-stage.svg";
    import ffmpegIcon from "../../assets/ffmpeg-stage.svg";
    import dbIcon from "../../assets/db-stage.svg";
    import StageIcon from "../StageIcon.svelte";
    import { createEventDispatcher, onMount } from "svelte";
    import type { QueueDetails } from "../../queue";
    import { fade } from "svelte/transition";

    // details is the QueueDetails we're showing in this component,
    // passed in from the parent component
    export let details: QueueDetails;

    // els and checkEls are lists of HTMLElements that are
    // bound dynamically by svelte after mounting
    const els: HTMLElement[] = new Array(4);

    // The main container for our stages that is used to adjust
    // the 3d perspective for our tilt animations (see handleMouseOver).
    // Populated by Svelte dynamically during mount
    let perspectiveContainer: HTMLElement;

    // Event dispatcher from Svelte
    const dispatch = createEventDispatcher();

    // onMount is used here to bind mousemove and mouseleave event
    // listeners to the elements in the 'els' array. This array
    // is populated automatically by Svelte, which is why this logic
    // is inside the onMount.
    onMount(() => {
        els.forEach((el) => {
            el.addEventListener("mousemove", (e) => handleMouseOver(el, e));
            el.addEventListener("mouseleave", (_) => handleMouseLeave(el));
        });
    });

    // handleMouseLeave is called when the mouse leaves an element
    // that exists in the 'els' array. This function resets the 3d
    // rotation and perspectives back to 0.
    function handleMouseLeave(el: HTMLElement) {
        el.style.transform = `rotate3d(0,0,0,0)`;
        perspectiveContainer.style.perspective = "";
        perspectiveContainer.style.perspectiveOrigin = "";
    }

    // handleMouseOver is used to generate a 3d tilt/rotation effect
    // on any of the elements stored inside the 'els' array.
    function handleMouseOver(el: HTMLElement, ev: MouseEvent) {
        const { offsetX, offsetY } = ev;
        const midX = el.offsetWidth / 2;
        const midY = el.offsetHeight / 2;
        const vecX = ((midX - offsetX) / el.offsetWidth) * 2;
        const vecY = ((midY - offsetY) / el.offsetHeight) * 2;

        // Center the camera over the div we're rotating
        perspectiveContainer.style.perspectiveOrigin = `${el.offsetLeft + midX}px center`;
        perspectiveContainer.style.perspective = "400px";

        el.style.transform = `rotate3d(${-vecY}, ${vecX}, 0, -10deg)`;
    }

    // handleStageClick will take a MouseEvent
    // and find the target of the mouse event. If it's
    // a click event, a 'stage-click' event is emitted from
    // this component, passing the index (i, representing
    // the stage).
    function handleStageClick(e: MouseEvent) {
        const t = e.target as HTMLElement;
        els.every((el, i) => {
            if (el.isSameNode(t)) {
                dispatch("stage-click", i);
                return false;
            }

            return true;
        });
    }

    // onCheckClick will take the index of the check button
    // that was clicked (this checkIndex maps 1:1 to our pipeline
    // stages) and if the check clicked was the one representing
    // a stage, where we are troubled, we will dispatch a 'spinner-click' event
    // which tells any listeners that we want to see the trouble situation.
    function onCheckClick(checkIndex: number) {
        if (checkIndex == details.stage && details.trouble) {
            dispatch("spinner-click");
        }
    }

    // stages is used to specify the stages we're presenting in this overlay.
    // defined here as an array to allow our HTML to be generated using a loop.
    const stages = [
        { caption: "import", icon: importIcon },
        { caption: "title", icon: titleIcon },
        { caption: "omdb", icon: omdbIcon },
        { caption: "ffmpeg", icon: ffmpegIcon },
        { caption: "db", icon: dbIcon },
    ];
</script>

<div class="stages tile" bind:this={perspectiveContainer}>
    {#key details}
        {#each stages as { caption, icon }, index (caption)}
            <div
                bind:this={els[index]}
                class="stage {caption}"
                class:active={details.stage == index}
                on:click={handleStageClick}
                in:fade={{ duration: 150, delay: index * 125 }}
            >
                <span class="caption">{caption.toUpperCase()}</span>
                {@html icon}
            </div>

            {#if index < stages.length - 1}
                <div on:click={() => onCheckClick(index)} class="check-wrapper">
                    <StageIcon drawLines={true} {details} stageIndex={index} stagger={true} />
                </div>
            {/if}
        {/each}
    {/key}
</div>

<style lang="scss">
    @use "../../styles/global.scss";
    @use "../../styles/overviewPanel.scss";
</style>
