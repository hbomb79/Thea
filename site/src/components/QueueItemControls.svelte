<script lang="ts" context="module">
export enum Action {
    NONE,
    PAUSE,
    CANCEL,
    PROMOTE,
}
</script>
<script lang="ts">
import pauseSvg from '../assets/pause.svg';
import advanceSvg from '../assets/advance.svg';
import cancelSvg from '../assets/cancel.svg';
import { createEventDispatcher, onMount } from 'svelte';

const dispatch = createEventDispatcher()

let controlSpans = new Array(3)
let controlItems = new Array(3)
let currentControl: Action = Action.NONE

onMount(() => {
    controlItems.forEach((item: HTMLElement) => {
        item.addEventListener("mouseleave", resetSelection)
    })
})

export function resetSelection() {
    currentControl = Action.NONE
    controlItems.forEach((item) => item.style.width = `1.3rem`)
}

const onSelect = (ev:MouseEvent) => {
    const target = ev.currentTarget as HTMLElement
    const action = Number(target.dataset.action)

    if(action == NaN) {
        console.warn("QueueControl action invalid")
        return
    }

    if(currentControl && currentControl == action) {
        dispatch("queue-control", action)
    }

    currentControl = action
    controlItems.forEach((item) => {
        if(item == controlItems[currentControl]) {
            const span = controlSpans[action]

            item.style.width = `${span.offsetWidth + 26}px`

            span.style.transitionProperty = 'opacity'
            span.style.right = `-${span.offsetWidth/2}px`

            setTimeout(() => {
                span.style.transitionProperty = 'right,opacity'
                span.style.right = "8px"
            }, 20)
        } else {
            item.style.width = `1.3rem`
        }
    })

    ev.stopPropagation()
    ev.preventDefault()
}
</script>

<style lang="scss">
.controls {
    position: absolute;
    right: 1rem;
    top: 1rem;
    font-size: 0;
    display: flex;
    flex-direction: row-reverse;
    background: #f3f5fe;
    padding: 5px 3px;
    border-radius: 4px;
    transition: opacity 250ms ease-in-out;

    span.control {
        display: inline-block;
        padding: 6px;
        margin: 3px 4px;
        border-radius: 4px;
        overflow: hidden;
        cursor: pointer;
        transition: all 250ms ease-in-out;
        transition-property: width;
        width: 1.3rem;
        position: relative;

        &.active span {
            display: inline-block;
            opacity: 1;
        }

        span {
            opacity: 0;
            transition: all 250ms ease-in-out;
            transition-property: opacity;
            font-size: 14px;
            color: #767676;

            position: absolute;
            right: 8px;
        }

        :global(svg) {
            width: 0.8rem;
            height: 0.8rem;
            padding: 4px;
            fill: grey;
            display: inline-block;
        }

        &.promote :global(svg) {
            transform: rotate(-90deg);
        }

        &:hover {
            background: #cecece;
        }
    }
}
</style>

<div class="controls">
    <span class="pause control" bind:this={controlItems[Action.PAUSE]} data-action={Action.PAUSE} class:active={currentControl == Action.PAUSE} on:click={onSelect}>
        {@html pauseSvg}
        <span bind:this={controlSpans[Action.PAUSE]}>Pause</span>
    </span>
    <span class="cancel control" bind:this={controlItems[Action.CANCEL]} data-action={Action.CANCEL} class:active={currentControl == Action.CANCEL} on:click={onSelect}>
        {@html cancelSvg}
        <span bind:this={controlSpans[Action.CANCEL]}>Cancel</span>
    </span>
    <span class="promote control" bind:this={controlItems[Action.PROMOTE]} data-action={Action.PROMOTE} class:active={currentControl == Action.PROMOTE} on:click={onSelect}>
        {@html advanceSvg}
        <span bind:this={controlSpans[Action.PROMOTE]}>Promote</span>
    </span>
</div>
