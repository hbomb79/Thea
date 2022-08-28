<script lang="ts">
    import { getContext, onMount } from "svelte";
    export let message;
    export let hasForm = false;
    export let onCancel = () => {};
    export let onOkay: (arg0: any) => void = () => {};

    const { close } = getContext("simple-modal");

    let value;
    let inputElement: HTMLInputElement = null;

    function _onCancel() {
        onCancel();
        close();
    }

    function _onOkay() {
        onOkay(value);
        close();
    }

    onMount(() => {
        inputElement?.focus();
    });
</script>

<h2>{message}</h2>

{#if hasForm}
    <input type="text" bind:this={inputElement} bind:value on:keydown={(e) => e.which === 13 && _onOkay()} />
{/if}

<div class="buttons">
    <button on:click={_onCancel}> Cancel </button>
    <button on:click={_onOkay}> Okay </button>
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
