<script lang="ts">
import { createEventDispatcher } from "svelte";


export let fields: Object
export let title: string

export let cb: (arg0:Object) => void

const dispatcher = createEventDispatcher();

setTimeout(() => {
    cb({
        foo: "bar",
        ...fields,
        title: title
    })
    dispatcher("close")
}, 1000)
</script>

<style lang="scss">

</style>

<h2>TEST</h2>
