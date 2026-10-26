<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { Leaf01Icon } from '@hugeicons/core-free-icons';

  interface Propiedades {
    nombreMarca?: string;
    subtituloMarca?: string;
  }

  let {
    nombreMarca = 'Proceso de Selección y Contratación',
    subtituloMarca = ''
  }: Propiedades = $props();
</script>

<div class="contenedor-marca">
  <div class="insignia-hoja" aria-hidden="true">
    <HugeiconsIcon icon={Leaf01Icon} size={22} color="#ffffff" strokeWidth={2} />
  </div>
  <div class="textos-marca">
    <span class="texto-marca">{nombreMarca}</span>
    {#if subtituloMarca}
      <span class="subtitulo-marca">{subtituloMarca}</span>
    {/if}
  </div>
</div>

<style>
  .contenedor-marca {
    display: inline-flex;
    align-items: center;
    gap: 12px;
  }

  .insignia-hoja {
    width: 42px;
    height: 42px;
    border-radius: 12px;
    background: var(--color-primario);
    color: #ffffff;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    box-shadow: 0 8px 18px -4px rgba(var(--vs-primary-raw), 0.55);
    transition: var(--transicion-vuesax);
  }

  .contenedor-marca:hover .insignia-hoja {
    transform: translateY(-2px);
    box-shadow: 0 10px 22px -3px rgba(var(--vs-primary-raw), 0.65);
  }

  .textos-marca {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
  }

  .texto-marca {
    font-size: 17px;
    font-weight: 800;
    color: var(--color-texto-dark);
    letter-spacing: -0.01em;
  }

  .subtitulo-marca {
    font-size: 10.5px;
    font-weight: 700;
    color: var(--color-texto-grey);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    margin-top: 2px;
  }
</style>
