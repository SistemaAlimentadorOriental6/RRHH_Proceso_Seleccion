<script lang="ts">
  import type { Snippet } from 'svelte';
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { Tick02Icon } from '@hugeicons/core-free-icons';

  interface Propiedades {
    marcada?: boolean;
    children: Snippet;
  }

  let { marcada = $bindable(false), children }: Propiedades = $props();
</script>

<label class="casilla" class:marcada>
  <input type="checkbox" bind:checked={marcada} />
  <span class="caja" aria-hidden="true">
    <HugeiconsIcon icon={Tick02Icon} size={12} strokeWidth={3.2} />
  </span>
  <span class="texto">{@render children()}</span>
</label>

<style>
  .casilla {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    user-select: none;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
  }

  /* El input real queda accesible por teclado pero invisible. */
  input {
    position: absolute;
    opacity: 0;
    width: 0;
    height: 0;
  }

  .caja {
    display: grid;
    place-items: center;
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    border-radius: 7px;
    background: #ffffff;
    border: 2px solid #dfe5ea;
    color: transparent;
    transition: var(--transicion-vuesax);
  }

  .casilla:hover .caja {
    border-color: var(--color-primario);
  }

  .marcada .caja {
    background: var(--color-primario);
    border-color: var(--color-primario);
    color: #ffffff;
    box-shadow: 0px 6px 14px -6px rgba(var(--vs-primary-raw), 0.6);
  }

  input:focus-visible + .caja {
    box-shadow: 0 0 0 3px rgba(var(--vs-primary-raw), 0.25);
  }
</style>
