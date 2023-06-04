<script lang="ts">
    import closeSvg from "assets/close.svg";

    export let showModal: Boolean;

    let dialog: HTMLDialogElement;

    $: if (dialog && showModal) dialog.showModal();
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<dialog bind:this={dialog} on:close={() => (showModal = false)} on:click|self={() => dialog.close()}>
    <div on:click|stopPropagation>
        <header>
            <slot name="header" />

            <!-- svelte-ignore a11y-autofocus -->
            <button autofocus on:click={() => dialog.close()}>{@html closeSvg}</button>
        </header>

        <main><slot /></main>
    </div>
</dialog>

<style lang="scss">
    dialog {
        max-width: 40%;
        border-radius: 0.4em;
        border: none;
        padding: 0;
        box-shadow: 0px 0px 6px 0px rgba(0, 0, 0, 0.2);

        &::backdrop {
            background: rgba(0, 0, 0, 0.5);
        }

        > div {
            background: #f3f5fe;

            header {
                padding: 1rem 1rem;
                background: #f8f9ff;
                border-bottom: solid 1px rgb(199 199 199 / 70%);
                display: flex;
                justify-content: space-between;
                margin: 0;
                font-size: 1.2rem;
                color: #8c91b9;
                align-items: center;
                font-weight: 500;
                box-shadow: 0px 0px 2px -1px black;

                button {
                    transition: fill 0.2s ease-out;
                    fill: #8d8c8c;
                    display: block;
                    overflow: hidden;
                    height: 1rem;
                    width: 1rem;
                    padding: 0;
                    line-height: 1rem;
                    margin: 0;
                    padding: 0;
                    border: none;
                    background: none;
                    cursor: pointer;

                    &:hover {
                        fill: rgb(46, 46, 46);
                    }
                }
            }

            main {
                padding: 0.5rem 1rem;
                font-style: italic;
                color: #666666;
            }
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
