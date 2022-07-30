<script lang="ts">
    import { getContext, onMount } from "svelte";
    export let title: string = "Modify FFmpeg Command";
    export let okayText: string = "Save";
    export let cancelText: string = "Discard";
    export let onCancel = () => {};
    export let onOkay: (v: Map<string, any>) => void = () => {};
    export let availableProperties: Map<string, any> = new Map();

    const { close } = getContext("simple-modal");

    let inputElement: HTMLInputElement = null;

    function _onCancel() {
        onCancel();
        close();
    }

    function _onOkay() {
        onOkay(availableProperties);
        close();
    }

    onMount(() => {
        inputElement?.focus();
    });
</script>

<h2>{title}</h2>
<p>Please modify the available FFmpeg arguments as you see fit. Empty fields will be ignored.</p>
<ul>
    {#each Object.keys(availableProperties) as key}
        <li>
            <span>{key}</span>
            <input type="text" bind:value={availableProperties[key]} />
        </li>
    {/each}
</ul>

<div class="buttons">
    <button on:click={_onCancel}> {cancelText} </button>
    <button on:click={_onOkay}> {okayText} </button>
</div>

<style lang="scss">
    h2 {
        font-size: 2rem;
        text-align: center;
        margin: 0.3rem 0 0.9rem 0;
        font-weight: 300;
    }

    input {
        width: 70%;
        outline-color: #9285c5;
    }

    .buttons {
        display: flex;
        justify-content: center;

        button {
            margin: 0.3rem 1rem 0 1rem;
        }
    }
</style>
