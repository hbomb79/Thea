<script lang="ts">
import { createEventDispatcher } from "svelte";

export let fields: Object
export let title: string
export let description: string
export let cb: (arg0:Object) => void

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

    cb(result)
    dispatch("close")
}

</script>

<style lang="scss">
.modal, .modal-backdrop {
    position: fixed;
}

.modal {
    z-index: 101;
    top: 50%;
    left: 50%;

    transform: translate(-50%, -50%);
    background-color: white;
    padding: 2rem;
    border-radius: 2px;
    border: solid 2px black;

    h2 {
        margin-top: 0;
    }

    .field {
        display: flex;
        justify-content: space-between;
        align-items: center;
        min-height: 3rem;
        margin: 1rem 0;

        span i {
            color: grey;
        }

        input {
            margin: 0;
        }
    }
}

.modal-backdrop {
    z-index: 100;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;

    background-color: rgba(0, 0, 0, 0.6);
}
</style>

<div class="modal-backdrop" on:click="{() => dispatch('close')}"></div>
<div class="modal">
    <h2 class="title">{title}</h2>
    {@html description}

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
