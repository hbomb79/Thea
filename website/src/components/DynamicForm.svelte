<script lang="ts">
    import { createEventDispatcher } from "svelte";

    export let fields: Object;

    const dispatch = createEventDispatcher();
    const defaultValue = (type: string, val: any): any => {
        switch (type) {
            case "bool":
                return val || false;
            case "int":
                return parseInt(val) || -1;
            case "string":
                return val || "";
            default:
                return "";
        }
    };

    const onFormSubmit = (event: Event) => {
        const result = {};
        Object.entries(fields).forEach((val: Object) => {
            const target = event.target[val[0]];
            if (target === undefined) return;

            result[val[0]] = defaultValue(val[1], target.type == "checkbox" ? target.checked : target.value);
        });

        event.stopPropagation();
        event.preventDefault();

        dispatch("submitted", result);
    };
</script>

<div class="dynamic-form">
    <form action="#" on:submit={onFormSubmit}>
        {#each Object.entries(fields) as [name, type]}
            <div class="field">
                {#if type == "int"}
                    <span>{name} <i>({type})</i></span>
                    <input type="number" {name} />
                {:else if type == "bool"}
                    <span>{name} <i>({type})</i></span>
                    <input type="checkbox" {name} />
                {:else if type == "string"}
                    <span>{name} <i>({type})</i></span>
                    <input type="text" {name} />
                {/if}
            </div>
        {/each}

        <input type="submit" value="Submit" />
    </form>
</div>

<style lang="scss">
</style>
