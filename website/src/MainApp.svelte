<script lang="ts">
    import { selectedQueueItem } from "stores/item";

    import { Router, Route, Link } from "svelte-routing";

    import ServerSettings from "components/settings/ServerSettings.svelte";
    import Home from "pages/Home.svelte";
    import Queue from "pages/QueuePage.svelte";

    function miniItemClick(itemId: number) {
        selectedQueueItem.set(itemId);
    }
</script>

<div id="app-wrapper" class="tiled-layout">
    <Router>
        <nav>
            <Link to="/" let:active>
                <div class="nav-item" class:active={active}>Home</div>
            </Link>
            <Link to="/queue" let:active>
                <div class="nav-item" class:active={active}>Queue</div>
            </Link>
            <Link to="/settings" let:active>
                <div class="nav-item" class:active={active}>Settings</div>
            </Link>
        </nav>

        <main>
            <Route path="/" component={Home} />
            <Route path="/queue" component={Queue} />
            <Route path="/settings" component={ServerSettings} />
        </main>

        <footer>
            Made with ♥ by <a target="_new" href="https://github.com/hbomb79">hbomb79</a>
        </footer>
    </Router>
</div>

<svelte:head>
    <style lang="scss" global>
        @use "./styles/tiled-layout.scss";
    </style>
</svelte:head>

<style lang="scss">
    #app-wrapper {
        width: 100%;
        height: 100%;
        display: flex;
        flex-direction: column;
    }
    nav {
        width: 100%;
        flex: 0 1 auto;
        margin: 5rem 4rem;
        margin-bottom: 0;
        text-align: left;

        .nav-item {
            display: inline-block;
            padding: 0.5rem 1.2rem;
            margin: 0.4rem;
            text-align: left;
            border-radius: 8px;
            cursor: pointer;
            background: #ffffff00;
            color: #9aa1d3;
            transition: all 150ms ease-in-out;
            transition-property: background, color; 

            &:hover {
                background: #ffffff55;
            }

            &.active {
                background: white;
                font-weight: 500;
                color: #9285c5;
                box-shadow: 0px 0px 3px #00000030;
            }

            &:last-child {
                margin-left: auto;
            }
        }
    }

    main {
        width: calc(100% - 8rem);
        margin: 0 auto;
        flex: 1 1 auto;
        min-height: 0;
        overflow: hidden;
        background: linear-gradient(122deg, #ffffffc7, #ffffff45);
        border: solid 1px #c4b8db;
        border-radius: 10px;
        box-shadow: 0px 0px 3px #0000000f;
    }

    footer {
        width: 100%;
        flex: 0 1 auto;
        color: #9285c5;
        padding: 20px;
    }
</style>