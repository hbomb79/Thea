<script lang="typescript">
import StatusButton from './StatusButton.svelte';

export let title:string;
</script>

<style lang="scss">
@use "../styles/global.scss";

nav {
    position: fixed;
    top: 0;
    background-color: #eee;
    padding: global.$navHeight / 2;
    left: 0;
    right: 0;
    box-shadow: 0px -8px 10px 3px black;

    .title {
        float: left;
        font-weight: 500;
    }

    .status-button {
        float: right;
    }
}
</style>

<nav>
    <span class="title">{title}</span>
    <div class="status-button">
        <StatusButton/>
    </div>
</nav>
