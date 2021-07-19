<script lang="ts">
    export let title:string
    console.log("Test")
</script>

<style lang="scss">
    nav {
        position: fixed;
        top: 0;
        background-color: #eee;
        padding: 1rem;
        left: 0;
        right: 0;
        box-shadow: 0px -8px 10px 3px black;

        .title {
            float: left;
            font-weight: 500;
        }
    }
</style>

<nav>
    <span class="title">{title}</span>
</nav>
