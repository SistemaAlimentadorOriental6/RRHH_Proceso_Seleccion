<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { WorkflowSquare10Icon } from '@hugeicons/core-free-icons';
  import type { MigaDePan } from '../../configuracion/navegacion.config';

  interface Propiedades {
    seccion?: string;
    titulo?: string;
    migasDePan?: MigaDePan[];
    estadoMotor?: string;
    totalEventosLog?: number;
    alNavegarMiga?: (ruta: string) => void;
  }

  let {
    seccion = 'Candidatos',
    titulo = 'Validación de Candidatos / Logs',
    migasDePan = [],
    estadoMotor = 'Motor Orquestador Activo',
    totalEventosLog = 0,
    alNavegarMiga
  }: Propiedades = $props();
</script>

<header class="encabezado-global">
  <!-- Migas de Pan Interactivas (Breadcrumbs con Slugs) y Título Dinámico -->
  <div class="bloque-breadcrumbs">
    <nav class="migas-de-pan" aria-label="Migas de pan">
      {#if migasDePan.length > 0}
        {#each migasDePan as miga, indice (indice)}
          {@const esUltima = indice === migasDePan.length - 1}
          {#if indice > 0}
            <span class="separador-miga" aria-hidden="true">/</span>
          {/if}

          {#if miga.ruta && !esUltima}
            <button
              type="button"
              class="miga-enlace"
              onclick={() => miga.ruta && alNavegarMiga?.(miga.ruta)}
            >
              {miga.etiqueta}
            </button>
          {:else}
            <span
              class="miga-texto"
              class:miga-actual={esUltima}
              aria-current={esUltima ? 'page' : undefined}
            >
              {miga.etiqueta}
            </span>
          {/if}
        {/each}
      {:else}
        <span class="miga-texto">DASHBOARD</span>
        <span class="separador-miga" aria-hidden="true">/</span>
        <span class="miga-texto">{seccion}</span>
        <span class="separador-miga" aria-hidden="true">/</span>
        <span class="miga-actual" aria-current="page">{titulo}</span>
      {/if}
    </nav>

    <div class="fila-titulo">
      <h1 class="titulo-pagina">{titulo}</h1>
      <span class="insignia-n8n">
        <HugeiconsIcon icon={WorkflowSquare10Icon} size={13} strokeWidth={2.2} />
        <span>Flujo Orquestado n8n</span>
      </span>
    </div>
  </div>


</header>

<style>
  .encabezado-global {
    width: 100%;
    height: 56px;
    background-color: #ffffff;
    border-bottom: 1px solid var(--color-fondo-soft);
    padding: 0 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-shrink: 0;
    position: sticky;
    top: 0;
    z-index: 100;
    box-shadow: 0 1px 3px rgba(15, 23, 42, 0.02);
  }

  .bloque-breadcrumbs {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 1px;
    min-width: 0;
  }

  .migas-de-pan {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    font-size: 8.5px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-texto-grey);
  }

  .miga-texto {
    color: var(--color-texto-grey);
  }

  .miga-enlace {
    padding: 0;
    background: transparent;
    color: var(--color-texto-grey);
    font-size: 8.5px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
    transition: color 0.2s ease;
  }

  .miga-enlace:hover {
    color: var(--color-primario);
  }

  .separador-miga {
    color: #cbd5e1;
  }

  .miga-actual {
    color: var(--color-primario);
    font-weight: 800;
  }

  .fila-titulo {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .titulo-pagina {
    font-size: 14px;
    font-weight: 800;
    color: var(--color-texto-dark);
    letter-spacing: -0.02em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .insignia-n8n {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 9px;
    font-weight: 700;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    flex-shrink: 0;
  }

</style>
