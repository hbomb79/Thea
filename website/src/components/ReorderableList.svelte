<script>
    import { createEventDispatcher } from "svelte";
    import { flip } from "svelte/animate";
    import { quintInOut } from "svelte/easing";
    import { crossfade } from "svelte/transition";

    // animation
    const [send, receive] = crossfade({
        duration: (d) => Math.sqrt(d * 200),
        fallback(node, params) {
            const style = getComputedStyle(node);
            const transform = style.transform === "none" ? "" : style.transform;
            return {
                duration: 0,
                easing: quintInOut,
                css: (t) => `
          opacity: ${t}
        `,
            };
        },
    });

    // events

    let draggingId = null;
    let overId = null;

    const getDraggedParent = (node) =>
        node.dataset && node.dataset.index ? node.dataset : getDraggedParent(node.parentNode);

    const onDragstart = (ev) => {
        ev.effectAllowed = "move";
        const parent = getDraggedParent(ev.target);
        ev.dataTransfer.setData("source", parent.index);
        if (draggingId !== parent.id) draggingId = parent.id;
    };
    const onDragover = (ev) => {
        ev.preventDefault();
        ev.dropEffect = "move";
        const parent = getDraggedParent(ev.target);
        if (overId !== parent.id) overId = parent.id;
    };
    const onDragleave = (ev) => {
        const parent = getDraggedParent(ev.target);
        if (overId === parent.id) overId = null;
    };
    const onDrop = (ev) => {
        const parent = getDraggedParent(ev.target);
        const fromIdx = ev.dataTransfer.getData("source");
        if (!parent.id || !fromIdx) return;
        ev.preventDefault();
        overId = null;
        draggingId = null;
        const toIdx = parent.index;
        reorder({ fromIdx, toIdx });
    };

    // dispatch "reordered"
    const dispatch = createEventDispatcher();
    const reorder = ({ fromIdx, toIdx }) => {
        if (fromIdx === toIdx) return;
        const newList = [...list];
        const fromItem = list[fromIdx];
        newList.splice(fromIdx, 1);
        newList.splice(toIdx, 0, fromItem);
        list = newList;
        dispatch("reordered", newList);
    };

    export let list;
    export let key = (item) => item;
    const getKey = key;
</script>

{#if list && list.length}
    <ul>
        {#each list as item, index (getKey(item))}
            <li
                data-index={index}
                data-id={getKey(item)}
                draggable="true"
                on:dragstart={onDragstart}
                on:dragover={onDragover}
                on:dragleave={onDragleave}
                on:drop={onDrop}
                in:receive={{ key: getKey(item) }}
                out:send={{ key: getKey(item) }}
                animate:flip={{ duration: 300 }}
                class:dragging={getKey(item) === draggingId}
                class:over={getKey(item) === overId}
            >
                <slot {item} {index}>
                    <p>{getKey(item)}</p>
                </slot>
            </li>
        {/each}
    </ul>
{/if}

<style>
    ul {
        list-style: none;
        padding: 0;
    }
    li {
        border: 2px dotted transparent;
        transition: border 0.1s linear;
    }
    /* the element that is being dragged */
    .dragging {
        opacity: 0.4;
    }
    /* the element that the dragging cursor is over */
    .over {
        border-color: rgba(48, 12, 200, 0.2);
    }
</style>
