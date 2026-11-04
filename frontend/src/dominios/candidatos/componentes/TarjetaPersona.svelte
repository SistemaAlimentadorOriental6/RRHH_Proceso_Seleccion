<script lang="ts">
  import { fade } from "svelte/transition";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import {
    CheckmarkCircle02Icon,
    Cancel01Icon,
    Leaf01Icon,
    Alert02Icon,
    WorkflowSquare10Icon,
    File02Icon,
    Download01Icon,
  } from "@hugeicons/core-free-icons";
  import { ETIQUETA_ETAPA, ORDEN_ETAPAS, type CandidatoEnVivo } from "../modelos/analisisEnVivo";
  import { ETIQUETA_VEREDICTO, veredictoDe } from "../servicios/almacenAnalisis.svelte";
  import { descargarZip, urlExportarCandidato } from "../servicios/clienteRrhh";

  // shortcut: copia el diseño de la tarjeta de VistaVacantesEnVivo (validacion-logs) para Descartados y
  // Semillero; si ese diseño cambia, unificar las dos en este componente.
  interface Propiedades {
    candidato: CandidatoEnVivo;
    /** Título de la vacante a la que se postuló (se muestra como chip). */
    vacanteTitulo: string;
    idsVacantes: string[];
    /** Está en la cola para reanalizarse con los parámetros vigentes. */
    enCola?: boolean;
    alAbrirFlujo: () => void;
    alAbrirResultados: () => void;
  }
  let { candidato, vacanteTitulo, idsVacantes, enCola = false, alAbrirFlujo, alAbrirResultados }: Propiedades = $props();

  const ETIQUETA_CORTA: Record<(typeof ORDEN_ETAPAS)[number], string> = {
    perfil: "Perfil",
    genero: "Género",
    reintegro: "Reint.",
    adres: "EPS",
    runt: "RUNT",
    simit: "SIMIT",
    procuraduria: "Proc.",
    sarlaft: "ONU",
    ofac: "OFAC",
    ia: "IA",
  };

  const veredicto = $derived(veredictoDe(candidato));
  const ev = $derived(candidato.resultado?.evaluacion_idoneidad);
  const ia = $derived(candidato.resultado?.evaluacion_ia ?? null);
  const motivo = $derived(
    candidato.fase === "analizando"
      ? "Reanalizando con los parámetros vigentes…"
      : (ev?.motivos_descarte ?? []).join(" · ") || "No cumple los requisitos de la vacante.",
  );
  const porCorreo = $derived(candidato.urlDetalle.startsWith("correo:"));
</script>

<div class="fila-superior-persona">
  <span class="pildora veredicto-{veredicto}">
    {#if veredicto === "apto"}
      <HugeiconsIcon icon={CheckmarkCircle02Icon} size={12} strokeWidth={2.3} />
    {:else if veredicto === "no_apto"}
      <HugeiconsIcon icon={Cancel01Icon} size={12} strokeWidth={2.3} />
    {:else if veredicto === "semillero"}
      <HugeiconsIcon icon={Leaf01Icon} size={12} strokeWidth={2.3} />
    {:else if veredicto === "pendiente"}
      <HugeiconsIcon icon={Alert02Icon} size={12} strokeWidth={2.3} />
    {:else if veredicto === "analizando"}
      <span class="mini-spinner"></span>
    {/if}
    {ETIQUETA_VEREDICTO[veredicto]}
  </span>
  <div class="chips-meta-persona">
    {#if enCola && candidato.fase !== "analizando"}
      <span class="etiqueta-meta etiqueta-cola" title="Se reanalizará con los parámetros vigentes">En cola</span>
    {/if}
    {#if ev?.regla_version}
      <span class="etiqueta-meta" title="Evaluado con la versión {ev.regla_version} de los parámetros">Reglas v{ev.regla_version}</span>
    {/if}
    <span class="etiqueta-meta {porCorreo ? 'etiqueta-correo' : 'etiqueta-computrabajo'}">
      {porCorreo ? "Correo" : "Computrabajo"}
    </span>
  </div>
</div>

<div class="cabecera-persona">
  <img class="avatar" src={candidato.fotoUrl} alt={candidato.nombre} width="50" height="50" loading="lazy" decoding="async" />
  <div class="identidad">
    <h4 title={candidato.nombre.toUpperCase()}>{candidato.nombre.toUpperCase()}</h4>
    <span class="subtitulo">{candidato.cedula ? `C.C. ${candidato.cedula}` : "Sin cédula en perfil"}</span>
    <span class="vacante" title={vacanteTitulo}>{vacanteTitulo}</span>
  </div>
  {#if ia}
    <div class="anillo-score" style="--p: {ia.score_total}" title="Score IA: {ia.score_total} de 100 · {ia.dictamen}">
      <div class="anillo-interior">
        <strong>{ia.score_total}</strong>
        <span>SCORE</span>
      </div>
    </div>
  {/if}
</div>

<div class="bloque-etapas" aria-label="Estado de cada validación">
  {#each ORDEN_ETAPAS as etapa (etapa)}
    {@const e = candidato.etapas[etapa]}
    {@const estadoEtapa = e?.estado ?? "pendiente"}
    <div class="item-etapa etapa-{estadoEtapa}" title="{ETIQUETA_ETAPA[etapa]}: {e ? (e.detalle ?? e.estado) : 'pendiente'}">
      <span class="punto punto-{estadoEtapa}"></span>
      <span class="nombre-etapa">{ETIQUETA_CORTA[etapa]}</span>
    </div>
  {/each}
</div>

{#key motivo}
  <p class="motivo motivo-{veredicto}" in:fade={{ duration: 300 }}>{motivo}</p>
{/key}

<div class="pie-persona">
  <button type="button" class="boton-flujo boton-premium" onclick={alAbrirFlujo}>
    <HugeiconsIcon icon={WorkflowSquare10Icon} size={14} strokeWidth={2.2} />
    <span>Ver flujo</span>
  </button>
  <button type="button" class="boton-resultados" onclick={alAbrirResultados}>
    <HugeiconsIcon icon={File02Icon} size={14} strokeWidth={2.2} />
    <span>Ver resultados</span>
  </button>
  {#if candidato.urlDetalle}
    <button
      type="button"
      class="boton-descargar"
      onclick={() => descargarZip(urlExportarCandidato(idsVacantes, candidato.urlDetalle), `${candidato.nombre}.zip`)}
      title="Descargar un .zip con su información (datos.txt y sus hojas de vida)"
      aria-label="Descargar la información de {candidato.nombre}"
    >
      <HugeiconsIcon icon={Download01Icon} size={15} strokeWidth={2.2} />
    </button>
  {/if}
</div>

<style>
  .fila-superior-persona {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 6px 8px;
    min-width: 0;
  }

  .chips-meta-persona {
    display: inline-flex;
    align-items: center;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 6px;
    min-width: 0;
  }

  .etiqueta-meta {
    white-space: nowrap;
    padding: 3px 8px;
    border-radius: 999px;
    background: #ffffff;
    color: var(--color-texto-grey);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.02em;
  }

  .etiqueta-meta.etiqueta-computrabajo {
    background: #e6f6e9;
    color: #1f7a33;
  }

  .etiqueta-meta.etiqueta-correo {
    background: #e8f0fe;
    color: #1a56c4;
  }

  .etiqueta-meta.etiqueta-cola {
    background: rgba(245, 158, 11, 0.14);
    color: #b9700a;
  }

  .cabecera-persona {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .avatar {
    width: 50px;
    height: 50px;
    border-radius: 15px;
    object-fit: cover;
    flex: none;
    background: #ffffff;
  }

  .identidad {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
    flex: 1;
  }

  .identidad h4 {
    font-size: 13px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.01em;
    line-height: 1.3;
    color: var(--color-texto-dark);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .subtitulo {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .vacante {
    font-size: 10.5px;
    color: #8a97a4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .pildora {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 10px;
    border-radius: 999px;
    font-size: 10.5px;
    font-weight: 700;
    background: #ffffff;
    color: var(--color-texto-grey);
    flex-shrink: 0;
  }

  .veredicto-apto {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .veredicto-no_apto {
    background: rgba(239, 68, 68, 0.1);
    color: #d14343;
  }

  .veredicto-semillero {
    background: rgba(59, 130, 246, 0.12);
    color: #2563eb;
  }

  .veredicto-pendiente {
    background: rgba(245, 158, 11, 0.14);
    color: #b9700a;
  }

  .veredicto-analizando {
    background: rgba(59, 130, 246, 0.12);
    color: #2f6fd6;
  }

  .mini-spinner {
    display: inline-block;
    width: 11px;
    height: 11px;
    border-radius: 50%;
    border: 2px solid rgba(var(--vs-primary-raw), 0.2);
    border-top-color: var(--color-primario);
    animation: girar 0.8s linear infinite;
  }

  @keyframes girar {
    to {
      transform: rotate(360deg);
    }
  }

  .anillo-score {
    margin-left: auto;
    flex-shrink: 0;
    width: 54px;
    height: 54px;
    border-radius: 50%;
    padding: 5px;
    background: conic-gradient(var(--color-primario) calc(var(--p) * 1%), rgba(var(--vs-primary-raw), 0.15) 0);
  }

  .anillo-interior {
    width: 100%;
    height: 100%;
    border-radius: 50%;
    background: #ffffff;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  .anillo-interior strong {
    font-size: 18px;
    font-weight: 800;
    color: var(--color-primario);
  }

  .anillo-interior span {
    font-size: 8.5px;
    font-weight: 700;
    color: var(--color-texto-grey);
    margin-top: 2px;
  }

  .bloque-etapas {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 6px;
    background: #ffffff;
    padding: 10px;
    border-radius: 14px;
  }

  .item-etapa {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .nombre-etapa {
    font-size: 9.5px;
    font-weight: 700;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    letter-spacing: 0.02em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .etapa-ok .nombre-etapa {
    color: var(--color-texto-dark);
  }

  .etapa-rechazado .nombre-etapa,
  .etapa-error .nombre-etapa {
    color: #d14343;
  }

  .etapa-semillero .nombre-etapa {
    color: #2563eb;
  }

  .punto {
    width: 100%;
    height: 6px;
    border-radius: 999px;
    background: rgba(44, 62, 80, 0.12);
  }

  .punto-ok {
    background: var(--color-primario);
  }

  .punto-alerta {
    background: #f59e0b;
  }

  .punto-rechazado {
    background: #ef4444;
  }

  .punto-semillero {
    background: #3b82f6;
  }

  .punto-error {
    background: #f97316;
  }

  .punto-omitida {
    background: rgba(44, 62, 80, 0.08);
  }

  .punto-ejecutando,
  .punto-reintentando {
    background: #3b82f6;
    animation: parpadeo 0.9s ease-in-out infinite;
  }

  .punto-reintentando {
    background: #f59e0b;
  }

  @keyframes parpadeo {
    50% {
      opacity: 0.35;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .punto-ejecutando,
    .punto-reintentando,
    .mini-spinner {
      animation: none;
    }
  }

  .motivo {
    font-size: 11.5px;
    font-weight: 500;
    line-height: 1.4;
    padding: 9px 12px;
    border-radius: 12px;
    background: rgba(44, 62, 80, 0.04);
    color: var(--color-texto-grey);
    overflow-wrap: anywhere;
  }

  .motivo-apto {
    background: rgba(var(--vs-primary-raw), 0.08);
    color: #25792b;
  }

  .motivo-no_apto {
    background: rgba(239, 68, 68, 0.08);
    color: #b42323;
  }

  .motivo-semillero {
    background: rgba(59, 130, 246, 0.08);
    color: #1d4ed8;
  }

  .pie-persona {
    margin-top: auto;
    display: flex;
    gap: 8px;
  }

  .boton-flujo {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 9px 14px;
    font-size: 12px;
    font-weight: 700;
  }

  .boton-resultados {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 9px 14px;
    border-radius: 14px;
    background: #ffffff;
    color: var(--color-primario);
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-resultados:hover {
    background: rgba(var(--vs-primary-raw), 0.14);
  }

  .boton-descargar {
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

  .boton-descargar:hover {
    background: var(--color-primario);
    color: #ffffff;
  }
</style>
