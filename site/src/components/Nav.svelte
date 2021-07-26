<script lang="typescript">
import { statusStream } from '../commander';
import { SocketPacketType } from '../store';
import StatusButton from './StatusButton.svelte';

export let title:string;
</script>

<style lang="scss">
@use "../styles/global.scss";
@use 'sass:math';

nav {
    position: fixed;
    z-index: 100;
    top: 0;
    background-color: #f5f5f5;
    padding: math.div(global.$navHeight, 2);
    left: 0;
    right: 0;
    box-shadow: 0px -8px 10px 3px black;
    box-shadow: 0px -8px 10px 3px black;
    border-bottom: solid 2px red;
    border-image-source: linear-gradient(to left, #cd8cf7, #6e9df9);
    border-image-slice: 1;

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
    {#if $statusStream == SocketPacketType.OPEN}
        <div class="status-button">
            <StatusButton/>
        </div>
    {/if}
</nav>
