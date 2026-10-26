<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    File02Icon,
    Globe02Icon,
    Shield01Icon,
    PoliceBadgeIcon,
    JusticeScale01Icon,
    FileValidationIcon,
    Calendar03Icon,
    UserCheck01Icon,
    UserIcon,
    LicenseIcon,
    Car01Icon,
    TrafficLightIcon,
    Alert02Icon,
    SparklesIcon,
    CheckmarkCircle02Icon,
    Cancel01Icon,
    ReloadIcon,
    Loading03Icon,
    Clock01Icon,
    EyeIcon,
    GitBranchIcon,
    Call02Icon,
    Notification03Icon,
    WhatsappIcon,
    Download01Icon
  } from '@hugeicons/core-free-icons';
  import type {
    DatosCandidatoEscenario,
    DefinicionNodoValidacion,
    ModoSimulacionNodo
  } from '../modelos/flujoValidacion';

  interface Propiedades {
    nodo: DefinicionNodoValidacion;
    candidato: DatosCandidatoEscenario;
    seleccionado: boolean;
    esUltimo: boolean;
    ejecucionEnCurso: boolean;
    alSeleccionar: (nodo: DefinicionNodoValidacion) => void;
    alCambiarModo: (idNodo: string, nuevoModo: ModoSimulacionNodo) => void;
    alIniciarArrastre: (idNodo: string, evento: PointerEvent) => void;
    alAbrirDetalle: (nodo: DefinicionNodoValidacion, pestanaInicial: 'datos' | 'evidencia') => void;
    alAbrirFotoCandidato: () => void;
    /** Datos reales del backend: se ocultan los controles de simulación por nodo. */
    modoEnVivo?: boolean;
  }

  let {
    nodo,
    candidato,
    seleccionado,
    esUltimo,
    ejecucionEnCurso,
    alSeleccionar,
    alCambiarModo,
    alIniciarArrastre,
    alAbrirDetalle,
    alAbrirFotoCandidato,
    modoEnVivo = false
  }: Propiedades = $props();

  function obtenerIconoPorClave(clave: string) {
    switch (clave) {
      case 'archivo':
        return File02Icon;
      case 'global':
        return Globe02Icon;
      case 'escudo':
        return Shield01Icon;
      case 'policia':
        return PoliceBadgeIcon;
      case 'balanza':
        return JusticeScale01Icon;
      case 'documentoCheck':
        return FileValidationIcon;
      case 'calendario':
        return Calendar03Icon;
      case 'reintegro':
        return UserCheck01Icon;
      case 'usuario':
        return UserIcon;
      case 'licencia':
        return LicenseIcon;
      case 'runt':
        return Car01Icon;
      case 'simit':
        return TrafficLightIcon;
      case 'multas':
        return Alert02Icon;
      case 'experiencia':
        return SparklesIcon;
      case 'condicional':
        return GitBranchIcon;
      case 'contacto':
        return Call02Icon;
      case 'notificacion':
        return Notification03Icon;
      case 'whatsapp':
        return WhatsappIcon;
      default:
        return File02Icon;
    }
  }

  function obtenerEtiquetaEstado(estado: DefinicionNodoValidacion['estado']): string {
    if (modoEnVivo && nodo.etiquetaEstadoVivo) return nodo.etiquetaEstadoVivo;
    switch (estado) {
      case 'ejecutando':
        return `Paso ${nodo.intentoActual}/${nodo.maxIntentos}`;
      case 'reintentando':
        return `Reintento ${nodo.intentoActual}/${nodo.maxIntentos}`;
      case 'completado':
        return 'Aprobado';
      case 'detenido_error':
        return modoEnVivo ? 'No cumple' : 'Detenido';
      case 'omitido':
        return 'Omitido';
      default:
        return 'En cola';
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="tarjeta-nodo-vuesax estado-{nodo.estado}"
  class:nodo-seleccionado={seleccionado}
  style="transform: translate3d({nodo.posicionX}px, {nodo.posicionY}px, 0);"
  onpointerdown={(e) => {
    e.stopPropagation();
    alSeleccionar(nodo);
    alIniciarArrastre(nodo.id, e);
  }}
  ondblclick={(e) => {
    e.stopPropagation();
    alAbrirDetalle(nodo, 'datos');
  }}
>
  <!-- Puerto de entrada izquierdo minimalista -->
  {#if nodo.orden > 1}
    <div
      class="puerto-vuesax puerto-entrada"
      class:puerto-activo={nodo.estado === 'completado' || nodo.estado === 'ejecutando' || nodo.estado === 'reintentando'}
      class:puerto-error={nodo.estado === 'detenido_error'}
      aria-hidden="true"
    ></div>
  {:else}
    <span class="insignia-inicio">Inicio</span>
  {/if}

  <!-- Puerto de salida derecho minimalista -->
  {#if !esUltimo}
    <div
      class="puerto-vuesax puerto-salida"
      class:puerto-activo={nodo.estado === 'completado'}
      class:puerto-reintento={nodo.estado === 'reintentando' || nodo.estado === 'ejecutando'}
      class:puerto-error={nodo.estado === 'detenido_error'}
      aria-hidden="true"
    ></div>
  {/if}

  <!-- Fila Superior: Paso, Categoría y Píldora Suave -->
  <div class="fila-superior">
    <span class="etiqueta-paso">
      <strong>#{String(nodo.orden).padStart(2, '0')}</strong>
      <span>{nodo.categoria}</span>
    </span>

    <span class="pildora-estado pildora-{nodo.estado}">
      {#if nodo.estado === 'ejecutando'}
        <span class="giro-suave">
          <HugeiconsIcon icon={Loading03Icon} size={11} strokeWidth={2.2} />
        </span>
      {:else if nodo.estado === 'reintentando'}
        <span class="giro-suave">
          <HugeiconsIcon icon={ReloadIcon} size={11} strokeWidth={2.2} />
        </span>
      {:else if nodo.estado === 'completado'}
        <HugeiconsIcon icon={CheckmarkCircle02Icon} size={11} strokeWidth={2.3} />
      {:else if nodo.estado === 'detenido_error'}
        <HugeiconsIcon icon={Cancel01Icon} size={11} strokeWidth={2.3} />
      {:else}
        <HugeiconsIcon icon={Clock01Icon} size={11} strokeWidth={2} />
      {/if}
      <span>{obtenerEtiquetaEstado(nodo.estado)}</span>
    </span>
  </div>

  <!-- Cuerpo Limpio: Icono Suave o Foto del Candidato (en Nodo #1) + Título y Subtítulo -->
  <div class="cuerpo-nodo">
    {#if nodo.orden === 1}
      <button
        type="button"
        class="avatar-nodo-candidato"
        title="Ver foto ampliada y expediente de {candidato.nombre}"
        onpointerdown={(e) => e.stopPropagation()}
        onclick={(e) => {
          e.stopPropagation();
          alAbrirFotoCandidato();
        }}
      >
        <img src={candidato.fotoUrl} alt={candidato.nombre} />
        <span class="insignia-lupa">Foto</span>
      </button>
    {:else}
      <div class="icono-vuesax icono-{nodo.estado}">
        <HugeiconsIcon
          icon={obtenerIconoPorClave(nodo.claveIcono)}
          size={20}
          strokeWidth={1.9}
        />
      </div>
    {/if}

    <div class="textos-nodo">
      <h3 class="titulo-nodo">{nodo.titulo}</h3>
      <p class="subtitulo-nodo">{nodo.subtitulo}</p>
    </div>
  </div>

  <!-- Grilla de Datos Destacados Devueltos (Más espacio de lectura directo en la card) -->
  {#if nodo.orden === 1}
    <!-- Ficha del candidato: nombre y cédula completos + descarga de la hoja de vida -->
    <div class="ficha-candidato-nodo">
      <div class="dato-ficha-nodo">
        <span class="etiqueta-campo">Candidato</span>
        <strong class="nombre-ficha">{candidato.nombre}</strong>
      </div>
      <div class="fila-ficha-nodo">
        <div class="dato-ficha-nodo">
          <span class="etiqueta-campo">Cédula</span>
          <strong class="valor-ficha-nodo">{candidato.cedula}</strong>
        </div>
        <div class="dato-ficha-nodo">
          <span class="etiqueta-campo">Estado</span>
          <strong
            class="valor-ficha-nodo"
            class:valor-ok={nodo.estado === 'completado'}
            class:valor-err={nodo.estado === 'detenido_error'}
          >
            {obtenerEtiquetaEstado(nodo.estado)}
          </strong>
        </div>
      </div>
      {#if candidato.urlHojaVida}
        <a
          class="btn-descargar-cv"
          href={candidato.urlHojaVida}
          download
          target="_blank"
          rel="noopener noreferrer"
          title="Descargar la hoja de vida original"
          onpointerdown={(e) => e.stopPropagation()}
          onclick={(e) => e.stopPropagation()}
        >
          <HugeiconsIcon icon={Download01Icon} size={14} strokeWidth={2.2} />
          <span>Descargar hoja de vida</span>
        </a>
      {:else}
        <span class="btn-descargar-cv sin-cv">Hoja de vida no disponible</span>
      {/if}
    </div>
  {:else}
  <div
    class="grilla-campos-nodo"
    style="grid-template-columns: repeat({nodo.camposDestacados.length <= 3 ? nodo.camposDestacados.length : 2}, minmax(0, 1fr));"
  >
    {#each nodo.camposDestacados as campo (campo.etiqueta)}
      <div class="item-campo-nodo">
        <span class="etiqueta-campo">{campo.etiqueta}</span>
        <strong
          class="valor-campo"
          class:valor-ok={nodo.estado === 'completado'}
          class:valor-err={nodo.estado === 'detenido_error'}
        >
          {#if nodo.estado === 'detenido_error'}
            {campo.valorError}
          {:else if nodo.estado === 'completado'}
            {campo.valorExito}
          {:else if nodo.estado === 'ejecutando' || nodo.estado === 'reintentando'}
            Consultando...
          {:else}
            {campo.valorExito}
          {/if}
        </strong>
      </div>
    {/each}
  </div>
  {/if}

  <!-- Bloque de Estado Suave (--color-fondo-soft) con Botón para Abrir Toda la Info y Foto/Captura -->
  <div class="caja-estado-soft">
    <div class="meta-intentos" class:oculto={nodo.esCondicional || nodo.esRama}>
      <div class="puntos-intento">
        {#each Array(nodo.maxIntentos) as _, idx}
          <span
            class="punto"
            class:ok={nodo.estado === 'completado' && idx < nodo.intentoActual}
            class:warn={nodo.estado === 'reintentando' && idx < nodo.intentoActual}
            class:err={nodo.estado === 'detenido_error' && idx < nodo.intentoActual}
            class:run={nodo.estado === 'ejecutando' && idx === 0}
          ></span>
        {/each}
        <span class="texto-intentos">
          Intento {nodo.intentoActual}/{nodo.maxIntentos}
        </span>
      </div>

      {#if nodo.duracionRealMs > 0}
        <span class="tiempo-ms">{nodo.duracionRealMs} ms</span>
      {/if}
    </div>

    <div
      class="acciones-inspeccion-nodo"
      onpointerdown={(e) => e.stopPropagation()}
      onclick={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        class="btn-ver-info"
        onclick={() => alAbrirDetalle(nodo, 'datos')}
        title="Ver toda la información devuelta por esta validación"
      >
        <HugeiconsIcon icon={EyeIcon} size={13} strokeWidth={2.1} />
        <span>Ver toda la info</span>
      </button>

      <button
        type="button"
        class="btn-ver-evidencia"
        class:sin-captura={modoEnVivo && !nodo.capturaUrl}
        onclick={() => alAbrirDetalle(nodo, 'evidencia')}
        title={modoEnVivo && !nodo.capturaUrl
          ? 'Aún no hay captura de esta consulta'
          : 'Ver captura / certificado oficial y foto'}
      >
        <span>Ver captura</span>
      </button>
    </div>
  </div>

  {#if !modoEnVivo}
  <!-- Selector minimalista de modo de prueba -->
    <div
      class="selector-modo-soft"
      onpointerdown={(e) => e.stopPropagation()}
      onclick={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        class="opcion-modo"
        class:activo-ok={nodo.modoSimulacion === 'exito_directo'}
        disabled={ejecucionEnCurso}
        onclick={() => alCambiarModo(nodo.id, 'exito_directo')}
      >
        Directo
      </button>
      <button
        type="button"
        class="opcion-modo"
        class:activo-retry={nodo.modoSimulacion === 'recuperar_en_reintento'}
        disabled={ejecucionEnCurso}
        onclick={() => alCambiarModo(nodo.id, 'recuperar_en_reintento')}
      >
        Reintento
      </button>
      <button
        type="button"
        class="opcion-modo"
        class:activo-stop={nodo.modoSimulacion === 'fallo_definitivo'}
        disabled={ejecucionEnCurso}
        onclick={() => alCambiarModo(nodo.id, 'fallo_definitivo')}
      >
        Fallo
      </button>
    </div>
  {/if}
</div>

<style>
  .meta-intentos.oculto {
    display: none;
  }

  .tarjeta-nodo-vuesax {
    position: absolute;
    top: 0;
    left: 0;
    width: 282px;
    background-color: #ffffff;
    border-radius: 26px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 11px;
    cursor: grab;
    user-select: none;
    touch-action: none;
    /* Anillo de estado con box-shadow (sin bordes duros) + sombra suave de profundidad */
    --anillo: 0 0 0 0 transparent;
    box-shadow:
      var(--anillo),
      0 12px 32px -10px rgba(44, 62, 80, 0.1);
    transition:
      box-shadow 0.25s ease,
      opacity 0.25s ease;
    z-index: 10;
  }

  .tarjeta-nodo-vuesax:hover {
    box-shadow:
      var(--anillo),
      0 20px 40px -12px rgba(44, 62, 80, 0.18);
  }

  .tarjeta-nodo-vuesax:active {
    cursor: grabbing;
    z-index: 30;
  }

  .tarjeta-nodo-vuesax.nodo-seleccionado {
    --anillo: 0 0 0 2px rgba(var(--vs-primary-raw), 0.4);
    box-shadow:
      var(--anillo),
      0 20px 40px -10px rgba(var(--vs-primary-raw), 0.25);
    z-index: 25;
  }

  .tarjeta-nodo-vuesax.estado-ejecutando {
    --anillo: 0 0 0 3px rgba(var(--vs-primary-raw), 0.55);
    box-shadow:
      var(--anillo),
      0 0 0 10px rgba(var(--vs-primary-raw), 0.1),
      0 20px 40px -8px rgba(var(--vs-primary-raw), 0.35);
    z-index: 28;
  }

  .tarjeta-nodo-vuesax.estado-reintentando {
    --anillo: 0 0 0 3px rgba(245, 158, 11, 0.55);
    box-shadow:
      var(--anillo),
      0 0 0 10px rgba(245, 158, 11, 0.1),
      0 20px 40px -8px rgba(245, 158, 11, 0.3);
    z-index: 29;
  }

  .tarjeta-nodo-vuesax.estado-completado {
    --anillo: 0 0 0 2px rgba(var(--vs-primary-raw), 0.28);
    box-shadow:
      var(--anillo),
      0 16px 32px -12px rgba(var(--vs-primary-raw), 0.28);
  }

  .tarjeta-nodo-vuesax.estado-detenido_error {
    --anillo: 0 0 0 3px rgba(239, 68, 68, 0.5);
    box-shadow:
      var(--anillo),
      0 0 0 10px rgba(239, 68, 68, 0.08),
      0 20px 40px -8px rgba(239, 68, 68, 0.3);
    z-index: 29;
  }

  .tarjeta-nodo-vuesax.estado-omitido {
    opacity: 0.45;
  }

  .insignia-inicio {
    position: absolute;
    top: -10px;
    left: 20px;
    background: var(--color-primario);
    color: #ffffff;
    font-size: 9.5px;
    font-weight: 700;
    padding: 3px 10px;
    border-radius: 999px;
    box-shadow: 0 8px 16px -6px rgba(var(--vs-primary-raw), 0.6);
  }

  /* Puertos anclados a la altura del icono/título: los cables llegan exactamente a ellos */
  .puerto-vuesax {
    position: absolute;
    top: 71px;
    transform: translateY(-50%);
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background-color: #ffffff;
    box-shadow:
      0 0 0 3px #d3dce2,
      0 3px 8px rgba(44, 62, 80, 0.12);
    z-index: 15;
    transition: all 0.25s ease;
  }

  .puerto-entrada {
    left: -7px;
  }

  .puerto-salida {
    right: -7px;
  }

  .puerto-vuesax.puerto-activo {
    background-color: var(--color-primario);
    box-shadow:
      0 0 0 3px rgba(var(--vs-primary-raw), 0.3),
      0 0 12px rgba(var(--vs-primary-raw), 0.6);
  }

  .puerto-vuesax.puerto-reintento {
    background-color: #f59e0b;
    box-shadow:
      0 0 0 3px rgba(245, 158, 11, 0.3),
      0 0 12px rgba(245, 158, 11, 0.6);
  }

  .puerto-vuesax.puerto-error {
    background-color: #ef4444;
    box-shadow:
      0 0 0 3px rgba(239, 68, 68, 0.3),
      0 0 12px rgba(239, 68, 68, 0.6);
  }

  .fila-superior {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .etiqueta-paso {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 10px;
    font-weight: 500;
    color: var(--color-texto-grey);
    min-width: 0;
  }

  .etiqueta-paso strong {
    font-weight: 700;
    color: var(--color-texto-dark);
    background-color: var(--color-fondo-soft);
    padding: 2px 7px;
    border-radius: 8px;
  }

  .etiqueta-paso span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .pildora-estado {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 9px;
    border-radius: 999px;
    font-size: 9.5px;
    font-weight: 600;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    flex-shrink: 0;
    transition: all 0.25s ease;
  }

  .pildora-ejecutando,
  .pildora-completado {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .pildora-reintentando {
    background-color: rgba(245, 158, 11, 0.14);
    color: #d97706;
  }

  .pildora-detenido_error {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .giro-suave {
    display: inline-flex;
    animation: girar 0.9s linear infinite;
  }

  @keyframes girar {
    to {
      transform: rotate(360deg);
    }
  }

  .cuerpo-nodo {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .avatar-nodo-candidato {
    position: relative;
    width: 46px;
    height: 46px;
    border-radius: 14px;
    overflow: hidden;
    padding: 0;
    cursor: pointer;
    flex-shrink: 0;
    box-shadow: 0 6px 14px -4px rgba(44, 62, 80, 0.16);
    transition: transform 0.25s ease;
  }

  .avatar-nodo-candidato:hover {
    transform: scale(1.06);
  }

  .avatar-nodo-candidato img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .insignia-lupa {
    position: absolute;
    bottom: 0;
    inset-inline: 0;
    background: rgba(44, 62, 80, 0.72);
    color: #ffffff;
    font-size: 8px;
    font-weight: 700;
    text-align: center;
    padding: 1px 0;
  }

  .icono-vuesax {
    width: 42px;
    height: 42px;
    border-radius: 14px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: all 0.25s ease;
  }

  .icono-ejecutando,
  .icono-completado {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .icono-reintentando {
    background-color: rgba(245, 158, 11, 0.14);
    color: #d97706;
  }

  .icono-detenido_error {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .textos-nodo {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .titulo-nodo {
    font-size: 13px;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1.25;
  }

  .subtitulo-nodo {
    font-size: 10.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
    line-height: 1.3;
    margin-top: 2px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Grilla de chips con datos devueltos en la card */
  .grilla-campos-nodo {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 6px;
  }

  .item-campo-nodo {
    background-color: var(--color-fondo-soft);
    border-radius: 14px;
    padding: 7px 10px;
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }

  .etiqueta-campo {
    font-size: 9px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .valor-campo {
    font-size: 10.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .valor-campo.valor-ok {
    color: var(--color-primario);
  }

  .valor-campo.valor-err {
    color: #ef4444;
  }

  /* Ficha del candidato en el nodo #1 */
  .ficha-candidato-nodo {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .dato-ficha-nodo {
    background-color: var(--color-fondo-soft);
    border-radius: 14px;
    padding: 7px 10px;
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }

  .fila-ficha-nodo {
    display: grid;
    grid-template-columns: 1.3fr 1fr;
    gap: 6px;
  }

  .nombre-ficha {
    font-size: 11.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .valor-ficha-nodo {
    font-size: 11px;
    font-weight: 700;
    color: var(--color-texto-dark);
    overflow-wrap: anywhere;
  }

  .valor-ficha-nodo.valor-ok {
    color: var(--color-primario);
  }

  .valor-ficha-nodo.valor-err {
    color: #ef4444;
  }

  .btn-descargar-cv {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px 10px;
    border-radius: 12px;
    background-color: var(--color-primario);
    color: #ffffff;
    font-size: 10.5px;
    font-weight: 700;
    text-decoration: none;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-descargar-cv:hover {
    background-color: var(--color-primario-hover);
    transform: translateY(-1px);
  }

  .btn-descargar-cv.sin-cv {
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    cursor: default;
  }

  .btn-descargar-cv.sin-cv:hover {
    transform: none;
    background-color: var(--color-fondo-soft);
  }

  /* Caja de estado e inspección */
  .caja-estado-soft {
    background-color: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .meta-intentos {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .puntos-intento {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .punto {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: #d5dde2;
    transition: all 0.25s ease;
  }

  .punto.ok,
  .punto.run {
    background-color: var(--color-primario);
  }

  .punto.warn {
    background-color: #f59e0b;
  }

  .punto.err {
    background-color: #ef4444;
  }

  .texto-intentos {
    font-size: 9.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
    margin-left: 3px;
  }

  .tiempo-ms {
    font-size: 9.5px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .acciones-inspeccion-nodo {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .btn-ver-info {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    padding: 6px 8px;
    border-radius: 10px;
    background-color: #ffffff;
    color: var(--color-texto-dark);
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.05);
    transition: all 0.25s ease;
  }

  .btn-ver-info:hover {
    color: var(--color-primario);
    transform: translateY(-1px);
    box-shadow: 0 5px 14px rgba(var(--vs-primary-raw), 0.16);
  }

  .btn-ver-evidencia {
    padding: 6px 9px;
    border-radius: 10px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-ver-evidencia.sin-captura {
    opacity: 0.55;
  }

  .btn-ver-evidencia:hover {
    background-color: var(--color-primario);
    color: #ffffff;
    transform: translateY(-1px);
  }

  /* Selector de modo en cápsula suave: se despliega al seleccionar o pasar el cursor */
  .selector-modo-soft {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
    background-color: var(--color-fondo-soft);
    padding: 3px;
    border-radius: 12px;
    max-height: 0;
    margin-top: -11px;
    padding-block: 0;
    opacity: 0;
    overflow: hidden;
    pointer-events: none;
    transition: all 0.25s ease;
  }

  .tarjeta-nodo-vuesax:hover .selector-modo-soft,
  .tarjeta-nodo-vuesax.nodo-seleccionado .selector-modo-soft {
    max-height: 40px;
    margin-top: 0;
    padding-block: 3px;
    opacity: 1;
    pointer-events: auto;
  }

  .opcion-modo {
    font-size: 9.5px;
    font-weight: 600;
    padding: 4px 0;
    border-radius: 9px;
    background-color: transparent;
    color: var(--color-texto-grey);
    cursor: pointer;
    text-align: center;
    transition: all 0.25s ease;
  }

  .opcion-modo:hover:not(:disabled) {
    color: var(--color-texto-dark);
  }

  .opcion-modo:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .opcion-modo.activo-ok {
    background-color: #ffffff;
    color: var(--color-primario);
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.06);
  }

  .opcion-modo.activo-retry {
    background-color: #ffffff;
    color: #d97706;
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.06);
  }

  .opcion-modo.activo-stop {
    background-color: #ffffff;
    color: #ef4444;
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.06);
  }
</style>
