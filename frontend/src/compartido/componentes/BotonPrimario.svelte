<script lang="ts">
  import type { Snippet } from 'svelte';
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { Loading03Icon } from '@hugeicons/core-free-icons';

  interface Propiedades {
    tipo?: 'button' | 'submit' | 'reset';
    cargando?: boolean;
    deshabilitado?: boolean;
    textoCargando?: string;
    /** Para botones de tipo 'button'; los de tipo 'submit' se atienden con el envío del formulario. */
    alClick?: () => void;
    children: Snippet;
  }

  let {
    tipo = 'button',
    cargando = false,
    deshabilitado = false,
    textoCargando = 'Cargando...',
    alClick = undefined,
    children
  }: Propiedades = $props();
</script>

<button
  type={tipo}
  disabled={deshabilitado || cargando}
  onclick={alClick}
  class="boton-accion boton-premium"
>
  {#if cargando}
    <span class="icono-giratorio" aria-hidden="true">
      <HugeiconsIcon icon={Loading03Icon} size={20} strokeWidth={2} />
    </span>
    <span>{textoCargando}</span>
  {:else}
    {@render children()}
  {/if}
</button>

<style>
  .boton-accion {
    width: 100%;
    padding: 16px 24px;
    border-radius: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .icono-giratorio {
    display: inline-flex;
    animation: girar 0.8s linear infinite;
  }

  @keyframes girar {
    to {
      transform: rotate(360deg);
    }
  }
</style>
