<script lang="ts" context="module">
    export enum Action {
        NONE,
        PAUSE,
        CANCEL,
        PROMOTE,
    }
</script>

<script lang="ts">
    import pauseSvg from "assets/pause.svg";
    import advanceSvg from "assets/advance.svg";
    import cancelSvg from "assets/cancel.svg";
    import { createEventDispatcher } from "svelte";
    import { selectedQueueItem } from "stores/item";
    import { itemDetails } from "stores/queue";
    import { QueueStatus } from "queue";

    const dispatch = createEventDispatcher();

    let currentAction: Action = Action.NONE;

    $: currentItemStatus = $itemDetails.get($selectedQueueItem).status;
    $: currentItemTroubled =
        currentItemStatus == QueueStatus.NEEDS_ATTENTION || currentItemStatus == QueueStatus.NEEDS_RESOLVING;

    // The controls of this element, used to generate
    // HTML using Svelte 'each'. The item and span
    // elements begin 'undefined', but become populated
    // during component mounting using the Svelte bind directive.
    const controls: {
        label: String;
        action: Action;
        icon: any;
        itemElement?: HTMLElement;
        spanElement?: HTMLElement;
    }[] = [
        { label: "Pause", action: Action.PAUSE, icon: pauseSvg },
        { label: "Cancel", action: Action.CANCEL, icon: cancelSvg },
        { label: "Promote", action: Action.PROMOTE, icon: advanceSvg },
    ];

    // Resets the selected action to None, and clears
    // any styling that was applied via 'onClick' below.
    export function resetSelection() {
        currentAction = Action.NONE;
        controls.forEach((control) => (control.itemElement.style.width = `1.3rem`));
    }

    // Handles a click event for the button corresponding to the action
    // provided in the argument, requiring a 'double click' to fire the event
    // If this click is the first click, then the control is 'selected', expanding
    // it's content.
    // If this click is the second click on this element, then a 'queue-control'
    // event with the action as the payload is fired.
    const onClick = (action: Action) => {
        if (currentAction == action) {
            dispatch("queue-control", action);
            resetSelection();

            return;
        }

        currentAction = action;
        controls.forEach((control) => {
            if (control.action == currentAction) {
                const item = control.itemElement;
                const span = control.spanElement;

                item.style.width = `${span.offsetWidth + 26}px`;

                span.style.transitionProperty = "opacity";
                span.style.right = `-${span.offsetWidth / 2}px`;

                setTimeout(() => {
                    span.style.transitionProperty = "right,opacity";
                    span.style.right = "8px";
                }, 20);
            } else {
                control.itemElement.style.width = `1.3rem`;
            }
        });
    };
</script>

<div class="controls" class:troubled={currentItemTroubled} on:mouseleave={resetSelection}>
    {#each controls as { label, action, icon, itemElement, spanElement }}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <span
            class="control {label.toLowerCase()}"
            on:click|preventDefault={() => onClick(action)}
            bind:this={itemElement}
            class:active={currentAction == action}
        >
            {@html icon}
            <span bind:this={spanElement}>{label}</span>
        </span>
    {/each}
</div>

<style lang="scss">
    $okColor: #9385cb;
    $troubleColor: #b53e49;

    .controls {
        position: absolute;
        right: 1rem;
        top: 1rem;
        font-size: 0;
        display: flex;
        flex-direction: row-reverse;
        background: #ffffff5e;
        padding: 3px 3px;
        border-radius: 4px;
        border: solid 1px #00000030;
        transition: opacity 250ms ease-in-out;

        span.control {
            display: inline-block;
            padding: 6px;
            margin: 3px 4px;
            border-radius: 4px;
            overflow: hidden;
            cursor: pointer;
            transition: all 250ms ease-in-out;
            transition-property: width, background;
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
                color: $okColor;

                position: absolute;
                right: 8px;
            }

            :global(svg) {
                width: 0.8rem;
                height: 0.8rem;
                padding: 4px;
                fill: $okColor;
                display: inline-block;
            }

            &.promote :global(svg) {
                transform: rotate(-90deg);
            }

            &:hover {
                background: #00000014;
            }
        }

        &.troubled {
            span.control span {
                color: $troubleColor !important;
            }

            :global(svg) {
                fill: $troubleColor !important;
            }
        }
    }
</style>
