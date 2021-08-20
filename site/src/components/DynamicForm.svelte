<script lang="ts">
import { createEventDispatcher } from "svelte";

export let fields: Object

const dispatch = createEventDispatcher();
const defaultValue = (type:string, val:any): any => {
    switch(type) {
        case 'bool':
            return val || false
        case 'int':
            return val || -1
        case 'string':
            return val || ""
        default:
            return ''
    }
}

const onFormSubmit = (event:Event) => {
    const result = {}
    Object.entries(fields).forEach((val:Object) => {
        const target = event.target[val[0]]
        result[val[0]] = defaultValue(val[1], target.type == "checkbox" ? target.checked : target.value)
    })

    event.stopPropagation()
    event.preventDefault()

    dispatch("submitted", result)
}

</script>

<style lang="scss">
</style>

<div class="dynamic-form">
    <form action="#" on:submit={onFormSubmit}>
        {#each Object.entries(fields) as [name, type] }
            <div class="field">
                <span>{name} <i>({type})</i></span>

                {#if type == "int"}
                    <input type="number" name="{name}"/>
                {:else if type == "bool"}
                    <input type="checkbox" name="{name}"/>
                {:else if type == "string"}
                    <input type="text" name="{name}"/>
                {:else}
                    <b>Cannot create dynamic form element for {type} type</b>
                {/if}
            </div>
        {/each}

        <input type="submit" value="Submit"/>
    </form>
</div>
