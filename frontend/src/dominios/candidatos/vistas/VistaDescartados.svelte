<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import { fade, fly } from "svelte/transition";
  import { cubicInOut, cubicOut } from "svelte/easing";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import { Cancel01Icon, WorkflowSquare10Icon, File02Icon, Download01Icon } from "@hugeicons/core-free-icons";
  import type { CandidatoEnVivo } from "../modelos/analisisEnVivo";
  import { almacenRrhh, veredictoDe } from "../servicios/almacenAnalisis.svelte";
  import { descargarZip, urlExportarCandidato } from "../servicios/clienteRrhh";

  interface Propiedades {
    alAbrirFlujoCandidato: (idVacante: string, slugCandidato: string) => void;
    alAbrirResultadosCandidato: (idVacante: string, slugCandidato: string) => void;
    alIrAReclutamiento: () => void;
  }
  let { alAbrirFlujoCandidato, alAbrirResultadosCandidato, alIrAReclutamiento }: Propiedades = $props();

  let textoBusqueda = $state("");

  onMount(() => {
    void almacenRrhh.asegurarVacantes();
  });

  // Cada vacante (o el grupo Operador / Conductor) lee lo ya guardado de sus candidatos.
  $effect(() => {
    if (almacenRrhh.vacantes.length === 0) return;
    for (const v of almacenRrhh.vacantes) void almacenRrhh.sesion(v.id).cargarGuardados();
  });

  interface Descartado {
    vacanteId: string;
    idsVacantes: string[];
    vacanteTitulo: string;
    candidato: CandidatoEnVivo;
  }

  const descartados = $derived.by((): Descartado[] => {
    const lista: Descartado[] = [];
    for (const v of almacenRrhh.vacantes) {
      for (const candidato of almacenRrhh.sesion(v.id).candidatos) {
        if (veredictoDe(candidato) === "no_apto") {
          lista.push({ vacanteId: v.id, idsVacantes: v.ids_miembros ?? [v.id], vacanteTitulo: v.titulo, candidato });
        }
      }
    }
    // Los últimos en llegar primero; los que vienen de antes, por su orden.
    return lista.sort(
      (a, b) => (b.candidato.terminadoEn ?? 0) - (a.candidato.terminadoEn ?? 0) || a.candidato.indice - b.candidato.indice,
    );
  });

  const visibles = $derived.by(() => {
    const termino = textoBusqueda.trim().toLowerCase();
    if (!termino) return descartados;
    return descartados.filter(
      ({ candidato: c }) => c.nombre.toLowerCase().includes(termino) || c.cedula.includes(termino),
    );
  });

  const cargando = $derived(almacenRrhh.cargando || almacenRrhh.vacantes.some((v) => almacenRrhh.sesion(v.id).cargandoGuardados));

  function motivos(c: CandidatoEnVivo): string {
    const lista = c.resultado?.evaluacion_idoneidad?.motivos_descarte;
    return lista && lista.length > 0 ? lista.join(" · ") : "No cumple los requisitos de la vacante.";
  }
</script>

<div class="contenedor">
  <section class="panel-superior">
    <div class="titulo">
      <span class="icono"><HugeiconsIcon icon={Cancel01Icon} size={20} strokeWidth={2.2} /></span>
      <div>
        <h2>Descartados</h2>
        <p>
          <strong>{descartados.length}</strong> hojas de vida rechazadas por el análisis. Las nuevas llegan aquí solas.
        </p>
      </div>
    </div>
    <input
      class="busqueda"
      type="search"
      placeholder="Buscar por nombre o cédula"
      bind:value={textoBusqueda}
    />
  </section>

  {#if visibles.length === 0}
    <div class="vacio" in:fade={{ duration: 250 }}>
      {#if cargando}
        <span>Cargando descartados…</span>
      {:else if descartados.length === 0}
        <span>Todavía no hay descartados.</span>
        <button type="button" class="enlace" onclick={alIrAReclutamiento}>Ir a Reclutamiento</button>
      {:else}
        <span>Ningún descartado coincide con la búsqueda.</span>
      {/if}
    </div>
  {:else}
    <div class="grilla">
      {#each visibles as { vacanteId, idsVacantes, vacanteTitulo, candidato } (`${vacanteId}|${candidato.slug}`)}
        <article
          class="tarjeta"
          animate:flip={{ duration: 600, easing: cubicInOut }}
          in:fly={{ y: 24, duration: 450, easing: cubicOut }}
        >
          <div class="cabecera">
            <img class="foto" src={candidato.fotoUrl} alt={candidato.nombre} loading="lazy" />
            <div class="identidad">
              <h3 title={candidato.nombre.toUpperCase()}>{candidato.nombre.toUpperCase()}</h3>
              <span class="cedula">{candidato.cedula ? `C.C. ${candidato.cedula}` : "Sin cédula en perfil"}</span>
            </div>
          </div>
          <div class="chips">
            <span class="chip">{candidato.urlDetalle.startsWith("correo:") ? "Correo" : "Computrabajo"}</span>
            <span class="chip suave" title={vacanteTitulo}>{vacanteTitulo}</span>
          </div>
          <p class="motivo">{motivos(candidato)}</p>
          <div class="acciones">
            <button type="button" class="boton" onclick={() => alAbrirFlujoCandidato(vacanteId, candidato.slug)}>
              <HugeiconsIcon icon={WorkflowSquare10Icon} size={14} strokeWidth={2.2} />
              <span>Ver flujo</span>
            </button>
            <button type="button" class="boton secundario" onclick={() => alAbrirResultadosCandidato(vacanteId, candidato.slug)}>
              <HugeiconsIcon icon={File02Icon} size={14} strokeWidth={2} />
              <span>Ver resultados</span>
            </button>
            {#if candidato.urlDetalle}
              <button
                type="button"
                class="descargar"
                onclick={() => descargarZip(urlExportarCandidato(idsVacantes, candidato.urlDetalle), `${candidato.nombre}.zip`)}
                title="Descargar un .zip con su información (datos.txt y sus hojas de vida)"
                aria-label="Descargar la información de {candidato.nombre}"
              >
                <HugeiconsIcon icon={Download01Icon} size={15} strokeWidth={2.2} />
              </button>
            {/if}
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>

<style>
  .contenedor {
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 4px;
  }

  .panel-superior {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 22px 26px;
    border-radius: 26px;
    background: #ffffff;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .titulo {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .icono {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: 14px;
    background: #fdecec;
    color: #d64545;
  }

  h2 {
    margin: 0;
    font-size: 20px;
    color: var(--color-texto-dark);
  }

  p {
    margin: 2px 0 0;
    font-size: 13px;
    color: var(--color-texto-suave, #6b7a89);
  }

  .busqueda {
    min-width: 240px;
    padding: 10px 16px;
    border: 0;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    font: inherit;
    font-size: 13px;
  }

  .grilla {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 16px;
  }

  .tarjeta {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 18px;
    border-radius: 22px;
    background: var(--color-fondo-soft);
    border: 2px solid transparent;
    transition: var(--transicion-vuesax);
  }

  .tarjeta:hover {
    background: #ffffff;
    border-color: #f1c4c4;
    box-shadow: 0 10px 28px -12px rgba(214, 69, 69, 0.3);
  }

  .cabecera {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .foto {
    width: 52px;
    height: 52px;
    border-radius: 16px;
    object-fit: cover;
    flex-shrink: 0;
  }

  .identidad {
    min-width: 0;
  }

  h3 {
    margin: 0;
    font-size: 14px;
    color: var(--color-texto-dark);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cedula {
    font-size: 12px;
    color: #8a97a4;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .chip {
    padding: 3px 9px;
    border-radius: 999px;
    background: #e6f6e9;
    color: #1f7a33;
    font-size: 11px;
    font-weight: 700;
  }

  .chip.suave {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    background: #ffffff;
    color: #6b7a89;
  }

  .motivo {
    margin: 0;
    padding: 10px 12px;
    border-radius: 14px;
    background: #fdecec;
    color: #a42a2a;
    font-size: 12.5px;
    line-height: 1.4;
  }

  .acciones {
    display: flex;
    gap: 8px;
    margin-top: auto;
  }

  .boton {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 9px 12px;
    border: 0;
    border-radius: 14px;
    background: var(--color-primario);
    color: #ffffff;
    font: inherit;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
  }

  .boton.secundario {
    background: #ffffff;
    color: var(--color-texto-dark);
  }

  .descargar {
    flex-shrink: 0;
    border: 0;
    cursor: pointer;
    display: grid;
    place-items: center;
    width: 36px;
    border-radius: 14px;
    background: #ffffff;
    color: var(--color-texto-dark);
    transition: var(--transicion-vuesax);
  }

  .descargar:hover {
    background: var(--color-primario);
    color: #ffffff;
  }

  .vacio {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 60px 20px;
    border-radius: 26px;
    background: #ffffff;
    color: #8a97a4;
    font-size: 14px;
  }

  .enlace {
    border: 0;
    background: none;
    color: var(--color-primario);
    font: inherit;
    font-weight: 700;
    cursor: pointer;
  }
</style>
