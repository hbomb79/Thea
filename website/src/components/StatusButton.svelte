<script lang="typescript">
    import statusIcon from "assets/status-circle.svg";

    function getStatus(): void {
        const el = document.getElementsByTagName("body");
        if (!el || el.length != 1) {
            console.warn("Failed to get body tag from DOM...");
            return;
        }

        const body = el[0];
        body.classList.toggle("status-panel-open");
    }
</script>

<a href="/#" class="nav-svg" on:click={getStatus}>
    {@html statusIcon}
</a>

<style lang="scss">
    a {
        display: block;
        height: 1rem;
        padding: 5px 5px 6px 5px;
        border-radius: 100%;

        transition: background-color 300ms ease-in-out;

        &:hover {
            background-color: #cec4f2;
        }

        :global(svg) {
            width: 1rem;
            height: 1rem;
            fill: #9d5ffb;

            :global(path:nth-child(2)) {
                fill: #5883ef;
            }
        }
    }
</style>
