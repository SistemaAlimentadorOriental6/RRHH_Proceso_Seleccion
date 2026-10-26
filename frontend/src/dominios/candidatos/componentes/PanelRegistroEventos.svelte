<script lang="ts">
  import { tick } from 'svelte';
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { Cancel01Icon } from '@hugeicons/core-free-icons';
  import {
    ETIQUETA_ETAPA,
    type IdEtapa,
    type LineaRegistro
  } from '../modelos/analisisEnVivo';

  interface Propiedades {
    lineas: LineaRegistro[];
    nombreCandidato: string;
    enVivo: boolean;
    alCerrar: () => void;
  }

  let { lineas, nombreCandidato, enVivo, alCerrar }: Propiedades = $props();

  let contenedor = $state<HTMLDivElement | null>(null);
  let seguirFinal = $state(true);

  // Mientras se está al final de la lista, cada línea nueva la mantiene a la vista.
  $effect(() => {
    void lineas.length;
    if (!seguirFinal) return;
    void tick().then(() => {
      if (contenedor) contenedor.scrollTop = contenedor.scrollHeight;
    });
  });

  function alDesplazar(): void {
    if (!contenedor) return;
    const distancia = contenedor.scrollHeight - contenedor.scrollTop - contenedor.clientHeight;
    seguirFinal = distancia < 24;
  }

  function etiquetaEtapa(etapa: LineaRegistro['etapa']): string {
    return etapa === 'general' ? 'general' : ETIQUETA_ETAPA[etapa as IdEtapa];
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<aside
  class="panel-registro"
  onpointerdown={(e) => e.stopPropagation()}
  onwheel={(e) => e.stopPropagation()}
>
  <header class="cabecera-registro">
    <div class="titulos-registro">
      <strong>Registro de eventos</strong>
      <span>{nombreCandidato}</span>
    </div>
    {#if enVivo}
      <span class="insignia-vivo"><i></i> En vivo</span>
    {/if}
    <button type="button" class="boton-cerrar" onclick={alCerrar} title="Ocultar el registro">
      <HugeiconsIcon icon={Cancel01Icon} size={14} strokeWidth={2.2} />
    </button>
  </header>

  <div class="cuerpo-registro" bind:this={contenedor} onscroll={alDesplazar}>
    {#each lineas as linea (linea.id)}
      <div class="linea nivel-{linea.nivel}">
        <time>{linea.hora}</time>
        <span class="etapa">{etiquetaEtapa(linea.etapa)}</span>
        <p>{linea.texto}</p>
      </div>
    {:else}
      <p class="vacio">
        Todavía no hay eventos para esta persona. Aparecen aquí cuando el análisis llega a ella.
      </p>
    {/each}
  </div>
</aside>

<style>
  .panel-registro {
    position: absolute;
    top: 96px;
    right: 20px;
    bottom: 84px;
    width: min(380px, calc(100% - 40px));
    display: flex;
    flex-direction: column;
    background: rgba(255, 255, 255, 0.96);
    backdrop-filter: blur(8px);
    border-radius: 22px;
    box-shadow:
      0 25px 50px -12px rgba(0, 0, 0, 0.18),
      0 15px 30px -5px rgba(44, 62, 80, 0.1);
    overflow: hidden;
    cursor: default;
    z-index: 5;
  }

  .cabecera-registro {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 16px 12px;
    border-bottom: 1px solid var(--color-fondo-soft);
  }

  .titulos-registro {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .titulos-registro strong {
    font-size: 13px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .titulos-registro span {
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .insignia-vivo {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 10px;
    font-weight: 700;
  }

  .insignia-vivo i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-primario);
    animation: pulso 1.2s ease-in-out infinite;
  }

  @keyframes pulso {
    50% {
      opacity: 0.35;
      transform: scale(0.8);
    }
  }

  .boton-cerrar {
    width: 28px;
    height: 28px;
    display: grid;
    place-items: center;
    border-radius: 10px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-cerrar:hover {
    color: var(--color-texto-dark);
  }

  .cuerpo-registro {
    flex: 1;
    overflow-y: auto;
    padding: 10px 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .linea {
    display: grid;
    grid-template-columns: 58px 1fr;
    column-gap: 8px;
    row-gap: 1px;
    padding: 7px 10px;
    border-radius: 12px;
    border-left: 3px solid transparent;
    background: var(--color-fondo-soft);
  }

  .linea time {
    grid-row: 1 / span 2;
    font-size: 10px;
    font-weight: 600;
    color: var(--color-texto-grey);
    font-variant-numeric: tabular-nums;
    padding-top: 2px;
  }

  .linea .etapa {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--color-texto-grey);
  }

  .linea p {
    font-size: 12px;
    font-weight: 500;
    line-height: 1.4;
    color: var(--color-texto-dark);
    overflow-wrap: anywhere;
  }

  .nivel-ok {
    border-left-color: var(--color-primario);
  }

  .nivel-warn {
    border-left-color: #f59e0b;
    background: rgba(245, 158, 11, 0.08);
  }

  .nivel-error {
    border-left-color: #ef4444;
    background: rgba(239, 68, 68, 0.07);
  }

  .vacio {
    font-size: 12px;
    color: var(--color-texto-grey);
    padding: 18px 10px;
    line-height: 1.5;
  }
</style>
