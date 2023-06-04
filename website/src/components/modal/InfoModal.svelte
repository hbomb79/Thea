<script lang="ts">
    export let showModal: Boolean;

    let dialog: HTMLDialogElement;

    $: if (dialog && showModal) dialog.showModal();
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<dialog bind:this={dialog} on:close={() => (showModal = false)} on:click|self={() => dialog.close()}>
    <div on:click|stopPropagation>
        <slot name="header" />
        <hr />
        <slot />
        <hr />
        <!-- svelte-ignore a11y-autofocus -->
        <button autofocus on:click={() => dialog.close()}>close modal</button>
    </div>
</dialog>

<style lang="scss">
    dialog {
        max-width: 32em;
        border-radius: 0.2em;
        border: none;
        padding: 0;

        &::backdrop {
            background: rgba(0, 0, 0, 0.3);
        }

        > div {
            padding: 1rem;
        }

        &[open] {
            animation: zoom 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);

            &::backdrop {
                animation: fade 0.2s ease-out;
            }
        }
    }

    @keyframes zoom {
        from {
            transform: scale(0.95);
        }
        to {
            transform: scale(1);
        }
    }

    @keyframes fade {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }
</style>
