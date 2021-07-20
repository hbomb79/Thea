<script lang="typescript">
import statusIcon from '../assets/status-circle.svg';

function getStatus(): void {
    alert("NYI");
}
</script>

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

<a href="/#" class="nav-svg" on:click="{getStatus}">
    {@html statusIcon}
</a>
