<script lang="ts">
import { onMount } from 'svelte'
import Nav from './components/Nav.svelte'
import store from './store'

const socketStore = store.webstore

function handleMessage(data:string) {

}

onMount(() => {
    socketStore.subscribe((data) => {
        // Handle event
        handleMessage(data)
    })
})

</script>

<style lang="scss">
@use "./styles/global.scss";

main {
    text-align: center;
    padding: 1em;
    max-width: 240px;
    margin: 0 auto;
    margin-top: global.$navHeight + 1rem;
}

.subtitle {
    color: #ff3e00;
    text-transform: uppercase;
    font-weight: 100;
}

@media (min-width: 640px) {
    main {
        max-width: none;
    }
}
</style>

<Nav title="TPA Dashboard"/>
<main>
    <span class="subtitle">Queue</span>
</main>

