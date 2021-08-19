<script lang="ts" context="module">
export enum Control {
    PAUSE,
    CANCEL,
    PROMOTE,
    NONE
}
</script>
<script lang="ts">
import pauseSvg from '../assets/pause.svg';
import advanceSvg from '../assets/advance.svg';
import cancelSvg from '../assets/cancel.svg';
import { createEventDispatcher } from 'svelte';

const dispatch = createEventDispatcher()
let controlSpans = new Array(3)
let controlItems = new Array(3)

let currentControl:Control = Control.NONE
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
            item.style.width = `${controlSpans[action].offsetWidth + 26}px`
        } else {
            item.style.width = `1.3rem`
        }
    })
}
</script>

<style lang="scss">
.controls {
    position: absolute;
    right: 4px;
    top: 50%;
    font-size: 0;
    transform: translate(0, -50%);

    span.control {
        display: inline-block;
        padding: 6px;
        background: #eee;
        margin: 0 4px;
        border-radius: 4px;
        overflow: hidden;
        cursor: pointer;
        border: solid 1px #e3e3e3;

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
    <span class="pause control" bind:this={controlItems[Control.PAUSE]} data-action={Control.PAUSE} class:active={currentControl == Control.PAUSE} on:click={onSelect}>
        {@html pauseSvg}
        <span bind:this={controlSpans[Control.PAUSE]}>Pause?</span>
    </span>
    <span class="cancel control" bind:this={controlItems[Control.CANCEL]} data-action={Control.CANCEL} class:active={currentControl == Control.CANCEL} on:click={onSelect}>
        {@html cancelSvg}
        <span bind:this={controlSpans[Control.CANCEL]}>Cancel?</span>
    </span>
    <span class="promote control" bind:this={controlItems[Control.PROMOTE]} data-action={Control.PROMOTE} class:active={currentControl == Control.PROMOTE} on:click={onSelect}>
        {@html advanceSvg}
        <span bind:this={controlSpans[Control.PROMOTE]}>Promote?</span>
    </span>
</div>
