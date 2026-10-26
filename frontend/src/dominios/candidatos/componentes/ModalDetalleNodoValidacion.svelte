<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    CheckmarkCircle02Icon,
    Cancel01Icon,
    Clock01Icon,
    ReloadIcon,
    File02Icon,
    Shield01Icon,
    UserIcon,
    SparklesIcon,
    EyeIcon
  } from '@hugeicons/core-free-icons';
  import type {
    DatosCandidatoEscenario,
    DefinicionNodoValidacion
  } from '../modelos/flujoValidacion';

  interface Propiedades {
    abierto: boolean;
    nodo: DefinicionNodoValidacion | null;
    candidato: DatosCandidatoEscenario;
    vacante: string;
    pestanaInicial?: 'datos' | 'evidencia' | 'json';
    alCerrar: () => void;
    alAbrirFotoCandidato: () => void;
  }

  let {
    abierto,
    nodo,
    candidato,
    vacante,
    pestanaInicial = 'datos',
    alCerrar,
    alAbrirFotoCandidato
  }: Propiedades = $props();

  let pestanaActiva = $state<'datos' | 'evidencia' | 'json'>('datos');
  let capturaAmpliada = $state(false);
  /** URL de captura que el navegador no pudo cargar (archivo borrado o servidor caído). */
  let capturaFallida = $state('');

  $effect(() => {
    if (abierto) {
      pestanaActiva = pestanaInicial;
      capturaAmpliada = false;
    }
  });

  const esFallo = $derived(nodo?.estado === 'detenido_error');

  const payloadActivo = $derived.by(() => {
    if (!nodo) return {};
    return esFallo ? nodo.payloadSalidaError : nodo.payloadSalidaExito;
  });

  function formatearClave(clave: string): string {
    return clave
      .replace(/([A-Z])/g, ' $1')
      .replace(/_/g, ' ')
      .replace(/^./, (c) => c.toUpperCase());
  }

  function esArregloObjetos(valor: unknown): valor is Record<string, unknown>[] {
    return (
      Array.isArray(valor) &&
      valor.length > 0 &&
      typeof valor[0] === 'object' &&
      valor[0] !== null
    );
  }

  function esArregloTextos(valor: unknown): valor is (string | number)[] {
    return (
      Array.isArray(valor) &&
      (valor.length === 0 ||
        typeof valor[0] === 'string' ||
        typeof valor[0] === 'number')
    );
  }

  function esObjetoSimple(valor: unknown): valor is Record<string, unknown> {
    return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
  }
</script>

{#if abierto && nodo}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-overlay" onclick={alCerrar}>
    <div
      class="modal-content"
      class:modal-ampliado={capturaAmpliada}
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Cabecera del Modal estilo Vuesax -->
      <header class="cabecera-modal">
        <div class="bloque-titulo-modal">
          <div class="insignia-orden" class:orden-error={esFallo}>
            #{String(nodo.orden).padStart(2, '0')}
          </div>
          <div class="textos-cabecera">
            <div class="fila-categoria">
              <span class="etiqueta-cat">{nodo.categoria}</span>
              <span class="separador">•</span>
              <span class="codigo-nodo">{nodo.codigo}</span>
              <span class="pildora-dictamen" class:dictamen-err={esFallo}>
                {#if esFallo}
                  <HugeiconsIcon icon={Cancel01Icon} size={13} strokeWidth={2.3} />
                  <span>Validación Rechazada / Detenida</span>
                {:else if nodo.estado === 'completado'}
                  <HugeiconsIcon icon={CheckmarkCircle02Icon} size={13} strokeWidth={2.3} />
                  <span>Validación Aprobada</span>
                {:else}
                  <HugeiconsIcon icon={Clock01Icon} size={13} strokeWidth={2} />
                  <span>Vista Previa de Validación</span>
                {/if}
              </span>
            </div>
            <h2 class="titulo-modal">{nodo.titulo}</h2>
            <p class="subtitulo-modal">{nodo.subtitulo}</p>
          </div>
        </div>

        <div class="acciones-cabecera">
          <!-- Tarjeta compacta del candidato con su foto -->
          <button
            type="button"
            class="ficha-candidato-mini"
            onclick={alAbrirFotoCandidato}
            title="Ver foto en grande y expediente del candidato"
          >
            <img src={candidato.fotoUrl} alt={candidato.nombre} class="foto-mini" />
            <div class="datos-mini">
              <strong>{candidato.nombre}</strong>
              <span>CC {candidato.cedula} • Ver foto</span>
            </div>
          </button>

          <button
            type="button"
            class="boton-cerrar-modal"
            onclick={alCerrar}
            title="Cerrar ventana"
          >
            ✕
          </button>
        </div>
      </header>

      <!-- Navegación por Pestañas Suaves (--color-fondo-soft) -->
      <div class="barra-pestanas-vuesax">
        <div class="grupo-pestanas">
          <button
            type="button"
            class="btn-pestana"
            class:activa={pestanaActiva === 'datos'}
            onclick={() => (pestanaActiva = 'datos')}
          >
            <HugeiconsIcon icon={File02Icon} size={15} strokeWidth={2} />
            <span>Toda la Información Devuelta</span>
          </button>

          <button
            type="button"
            class="btn-pestana"
            class:activa={pestanaActiva === 'evidencia'}
            onclick={() => (pestanaActiva = 'evidencia')}
          >
            <HugeiconsIcon icon={EyeIcon} size={15} strokeWidth={2} />
            <span>Captura / Certificado Visual y Foto</span>
          </button>

          <button
            type="button"
            class="btn-pestana"
            class:activa={pestanaActiva === 'json'}
            onclick={() => (pestanaActiva = 'json')}
          >
            <HugeiconsIcon icon={SparklesIcon} size={15} strokeWidth={2} />
            <span>Payload JSON Crudo</span>
          </button>
        </div>

        <div class="resumen-intentos-modal">
          <HugeiconsIcon icon={ReloadIcon} size={13} strokeWidth={2} />
          <span>
            Intentos ejecutados: <strong>{nodo.intentoActual || 1} de {nodo.maxIntentos}</strong>
          </span>
          {#if nodo.duracionRealMs > 0}
            <span class="chip-tiempo">{nodo.duracionRealMs} ms</span>
          {/if}
        </div>
      </div>

      <!-- Cuerpo Amplio con Scroll Suave -->
      <div class="cuerpo-modal-scroll">
        {#if pestanaActiva === 'datos'}
          <!-- Banner de Dictamen Principal -->
          <div class="banner-dictamen" class:banner-error={esFallo}>
            <div class="icono-banner">
              {#if esFallo}
                <HugeiconsIcon icon={Cancel01Icon} size={22} strokeWidth={2.2} />
              {:else}
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={22} strokeWidth={2.2} />
              {/if}
            </div>
            <div class="texto-banner">
              <strong>
                {esFallo ? nodo.evidenciaVisual.selloError : nodo.evidenciaVisual.selloExito}
              </strong>
              <p>
                {esFallo ? nodo.mensajeFalloDefinitivo : nodo.evidenciaVisual.observacionLegal}
              </p>
            </div>
          </div>

          <!-- Grilla Amplia con Todos los Campos Devueltos por el Agente -->
          <div class="grilla-datos-estructurados">
            {#each Object.entries(payloadActivo) as [clave, valor] (clave)}
              {#if esArregloObjetos(valor)}
                <div class="tarjeta-dato-ancho">
                  <span class="etiqueta-dato">{formatearClave(clave)}</span>
                  <div class="tabla-contenedor">
                    <table class="tabla-vuesax">
                      <thead>
                        <tr>
                          {#each Object.keys(valor[0]) as col (col)}
                            <th>{formatearClave(col)}</th>
                          {/each}
                        </tr>
                      </thead>
                      <tbody>
                        {#each valor as fila, i (i)}
                          <tr>
                            {#each Object.values(fila) as celda, j (j)}
                              <td>{String(celda)}</td>
                            {/each}
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                </div>
              {:else if esArregloTextos(valor)}
                <div class="tarjeta-dato-ancho">
                  <span class="etiqueta-dato">{formatearClave(clave)}</span>
                  {#if valor.length === 0}
                    <p class="valor-dato-texto">Sin registros reportados (0)</p>
                  {:else}
                    <div class="lista-chips-valor">
                      {#each valor as item (item)}
                        <span class="chip-valor-item">{item}</span>
                      {/each}
                    </div>
                  {/if}
                </div>
              {:else if esObjetoSimple(valor)}
                <div class="tarjeta-dato-ancho">
                  <span class="etiqueta-dato">{formatearClave(clave)}</span>
                  <div class="subgrilla-objeto">
                    {#each Object.entries(valor) as [subClave, subValor] (subClave)}
                      <div class="subtarjeta-dato">
                        <span class="etiqueta-dato">{formatearClave(subClave)}</span>
                        <strong class="valor-dato">{String(subValor)}</strong>
                      </div>
                    {/each}
                  </div>
                </div>
              {:else}
                <div class="tarjeta-dato">
                  <span class="etiqueta-dato">{formatearClave(clave)}</span>
                  <strong
                    class="valor-dato"
                    class:texto-verde={valor === true || String(valor).includes('SIN') || String(valor).includes('ACTIVO') || String(valor).includes('PAZ')}
                    class:texto-rojo={esFallo && (valor === false || String(valor).includes('RECHAZADO'))}
                  >
                    {#if typeof valor === 'boolean'}
                      {valor ? 'Sí (Verificado)' : 'No'}
                    {:else}
                      {String(valor)}
                    {/if}
                  </strong>
                </div>
              {/if}
            {/each}
          </div>
        {:else if pestanaActiva === 'evidencia'}
          <!-- Vista de Captura Oficial / Evidencia Visual con Foto del Candidato y Zoom -->
          <div class="contenedor-evidencia-visual">
            <div class="barra-herramientas-evidencia">
              <div class="meta-portal">
                <HugeiconsIcon icon={Shield01Icon} size={16} strokeWidth={2} />
                <span>Fuente oficial: <strong>{nodo.evidenciaVisual.entidadOficial}</strong></span>
              </div>

              <div class="botones-zoom-evidencia">
                <button
                  type="button"
                  class="btn-accion-evidencia"
                  onclick={alAbrirFotoCandidato}
                >
                  <HugeiconsIcon icon={UserIcon} size={14} strokeWidth={2} />
                  <span>Ver Foto Candidato en Grande</span>
                </button>
                <button
                  type="button"
                  class="btn-accion-evidencia primario"
                  onclick={() => (capturaAmpliada = !capturaAmpliada)}
                >
                  <span>{capturaAmpliada ? 'Reducir Vista' : 'Ampliar Captura'}</span>
                </button>
              </div>
            </div>

            {#if nodo.capturaUrl}
              <!-- Captura real tomada por el sistema al consultar la fuente -->
              <figure class="captura-real">
                <figcaption>
                  Captura tomada por el sistema al consultar <strong>{nodo.evidenciaVisual.entidadOficial}</strong>
                </figcaption>
                {#if capturaFallida === nodo.capturaUrl}
                  <p class="aviso-sin-captura">
                    No se pudo cargar la imagen: el archivo ya no está en el servidor o el servidor no responde.
                  </p>
                {:else}
                  <a href={nodo.capturaUrl} target="_blank" rel="noopener noreferrer" title="Abrir la captura en una pestaña nueva">
                    <img
                      src={nodo.capturaUrl}
                      alt={`Captura de ${nodo.titulo} para ${candidato.nombre}`}
                      onerror={() => (capturaFallida = nodo?.capturaUrl ?? '')}
                    />
                  </a>
                {/if}
              </figure>
            {:else}
            {#if nodo.etiquetaEstadoVivo !== undefined}
              <p class="aviso-sin-captura">
                Esta consulta no dejó captura (aún no se ejecuta, se omitió por descarte temprano o falló antes de obtener respuesta).
              </p>
            {/if}
            <!-- Certificado / Captura Visual de la Entidad -->
            <div class="lienzo-certificado-oficial">
              <!-- Barra estilo navegador del portal consultado -->
              <div class="barra-navegador-simulado">
                <div class="puntos-ventana">
                  <span></span><span></span><span></span>
                </div>
                <div class="url-simulada">{nodo.evidenciaVisual.portalUrl}</div>
                <span class="radicado-badge">Radicado: {nodo.evidenciaVisual.codigoRadicado}</span>
              </div>

              <!-- Contenido del Certificado con Foto del Candidato y Sello -->
              <div class="hoja-certificado">
                <div class="encabezado-certificado">
                  <div>
                    <span class="entidad-sup">{nodo.evidenciaVisual.entidadOficial}</span>
                    <h3 class="titulo-cert">{nodo.evidenciaVisual.tituloCertificado}</h3>
                  </div>
                  <div class="sello-verificacion" class:sello-rojo={esFallo}>
                    {esFallo ? 'ALERTA / RECHAZADO' : 'VERIFICADO OFICIAL'}
                  </div>
                </div>

                <!-- Ficha con Foto del Candidato dentro del Certificado -->
                <div class="ficha-ciudadano-certificado">
                  <button
                    type="button"
                    class="contenedor-foto-cert"
                    onclick={alAbrirFotoCandidato}
                    title="Clic para ampliar foto del candidato"
                  >
                    <img src={candidato.fotoUrl} alt={candidato.nombre} />
                    <span class="etiqueta-ver-foto">Ampliar Foto</span>
                  </button>

                  <div class="datos-ciudadano-cert">
                    <div class="par-cert">
                      <span>Ciudadano Consultado</span>
                      <strong>{candidato.nombre}</strong>
                    </div>
                    <div class="par-cert">
                      <span>Documento de Identidad</span>
                      <strong>CC {candidato.cedula} ({candidato.municipio})</strong>
                    </div>
                    <div class="par-cert">
                      <span>Vacante Asociada</span>
                      <strong>{vacante}</strong>
                    </div>
                    <div class="par-cert">
                      <span>Categoría Licencia / EPS</span>
                      <strong>{candidato.licencia} • {candidato.eps}</strong>
                    </div>
                  </div>
                </div>

                <!-- Dictamen Central del Certificado -->
                <div class="caja-dictamen-certificado" class:dictamen-cert-error={esFallo}>
                  <span class="etiqueta-dictamen-cert">RESULTADO OFICIAL DE LA CONSULTA EN LÍNEA</span>
                  <strong class="texto-dictamen-cert">
                    {esFallo ? nodo.evidenciaVisual.selloError : nodo.evidenciaVisual.selloExito}
                  </strong>
                  <p class="observacion-cert">{nodo.evidenciaVisual.observacionLegal}</p>
                </div>

                <!-- Resumen de Campos Clave en el Certificado -->
                <div class="grilla-resumen-cert">
                  {#each Object.entries(payloadActivo).slice(0, 6) as [k, v] (k)}
                    {#if typeof v !== 'object'}
                      <div class="celda-cert">
                        <span>{formatearClave(k)}</span>
                        <strong>{String(v)}</strong>
                      </div>
                    {/if}
                  {/each}
                </div>

                <div class="pie-certificado">
                  <span>Firma Digital de Orquestación: <strong>SAO6-RRHH-N8N-{nodo.codigo}</strong></span>
                  <span>Fecha y Hora de Captura: <strong>2026-10-16 10:25:00 -05:00</strong></span>
                </div>
              </div>
            </div>
            {/if}
          </div>
        {:else}
          <!-- Vista de Payload JSON Crudo -->
          <div class="contenedor-json-crudo">
            <pre>{JSON.stringify(payloadActivo, null, 2)}</pre>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  /* Reglas exactas de Modales y Blur (Glassmorphism) de DESING.md */
  .modal-overlay {
    position: fixed;
    inset: 0;
    z-index: 200;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }

  .modal-content {
    width: min(960px, 100%);
    height: min(640px, 90vh);
    background-color: #ffffff;
    border-radius: 28px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow:
      0 25px 50px -12px rgba(0, 0, 0, 0.25),
      0 15px 30px -5px rgba(0, 0, 0, 0.2);
    animation: modalIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  /* El modo ampliado ya no cambia el tamaño del modal */
  .modal-content.modal-ampliado {
    width: min(960px, 100%);
    height: min(640px, 90vh);
  }

  @keyframes modalIn {
    0% {
      opacity: 0;
      transform: scale(0.92) translateY(14px);
    }
    100% {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  .cabecera-modal {
    padding: 20px 28px 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-shrink: 0;
  }

  .bloque-titulo-modal {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .insignia-orden {
    width: 52px;
    height: 52px;
    border-radius: 18px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 17px;
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .insignia-orden.orden-error {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .textos-cabecera {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .fila-categoria {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .pildora-dictamen {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 10px;
    border-radius: 999px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 600;
  }

  .pildora-dictamen.dictamen-err {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .titulo-modal {
    font-size: 20px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .subtitulo-modal {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .acciones-cabecera {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .ficha-candidato-mini {
    display: flex;
    align-items: center;
    gap: 10px;
    background-color: var(--color-fondo-soft);
    padding: 6px 14px 6px 6px;
    border-radius: 999px;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .ficha-candidato-mini:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 20px -6px rgba(44, 62, 80, 0.12);
  }

  .foto-mini {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    object-fit: cover;
  }

  .datos-mini {
    display: flex;
    flex-direction: column;
    text-align: left;
  }

  .datos-mini strong {
    font-size: 12px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .datos-mini span {
    font-size: 10.5px;
    font-weight: 500;
    color: var(--color-primario);
  }

  .boton-cerrar-modal {
    width: 40px;
    height: 40px;
    border-radius: 14px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 15px;
    font-weight: 700;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.25s ease;
  }

  .boton-cerrar-modal:hover {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
    transform: scale(1.05);
  }

  /* Barra de Pestañas */
  .barra-pestanas-vuesax {
    margin: 0 28px;
    padding: 6px 10px;
    background-color: var(--color-fondo-soft);
    border-radius: 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    flex-shrink: 0;
  }

  .grupo-pestanas {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .btn-pestana {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 8px 15px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 600;
    color: var(--color-texto-grey);
    background-color: transparent;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-pestana:hover {
    color: var(--color-texto-dark);
  }

  .btn-pestana.activa {
    background-color: #ffffff;
    color: var(--color-primario);
    box-shadow: 0 4px 14px rgba(44, 62, 80, 0.07);
  }

  .resumen-intentos-modal {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 11.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
    padding-right: 6px;
  }

  .resumen-intentos-modal strong {
    color: var(--color-texto-dark);
  }

  .chip-tiempo {
    background-color: #ffffff;
    padding: 2px 8px;
    border-radius: 8px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  /* Área de Contenido con Gran Espacio */
  .cuerpo-modal-scroll {
    flex: 1;
    min-height: 0;
    padding: 18px 28px 24px;
    overflow-x: hidden;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
    scrollbar-width: thin;
  }

  .banner-dictamen {
    background-color: rgba(var(--vs-primary-raw), 0.1);
    border-radius: 20px;
    padding: 16px 20px;
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .banner-dictamen.banner-error {
    background-color: rgba(239, 68, 68, 0.1);
  }

  .icono-banner {
    width: 44px;
    height: 44px;
    border-radius: 14px;
    background-color: #ffffff;
    color: var(--color-primario);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .banner-error .icono-banner {
    color: #ef4444;
  }

  .texto-banner strong {
    font-size: 13.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
    display: block;
  }

  .texto-banner p {
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-grey);
    margin-top: 2px;
  }

  /* Grilla Estructurada de Datos */
  .grilla-datos-estructurados {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 14px;
  }

  .tarjeta-dato {
    background-color: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .tarjeta-dato-ancho {
    grid-column: 1 / -1;
    background-color: var(--color-fondo-soft);
    border-radius: 20px;
    padding: 16px 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .etiqueta-dato {
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .valor-dato {
    font-size: 14px;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .valor-dato.texto-verde {
    color: var(--color-primario);
  }

  .valor-dato.texto-rojo {
    color: #ef4444;
  }

  .valor-dato-texto {
    font-size: 13px;
    font-weight: 600;
    color: var(--color-texto-dark);
  }

  .lista-chips-valor {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-valor-item {
    background-color: #ffffff;
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    padding: 6px 12px;
    border-radius: 12px;
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.05);
  }

  .subgrilla-objeto {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 10px;
  }

  .subtarjeta-dato {
    background-color: #ffffff;
    border-radius: 14px;
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  /* Tabla Limpia para Historial de Licencias RUNT u otros arreglos */
  .tabla-contenedor {
    overflow-x: auto;
    background-color: #ffffff;
    border-radius: 14px;
    padding: 6px;
  }

  .tabla-vuesax {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  .tabla-vuesax th {
    text-align: left;
    padding: 10px 12px;
    font-size: 11px;
    font-weight: 600;
    color: var(--color-texto-grey);
    background-color: var(--color-fondo-soft);
    border-radius: 8px;
  }

  .tabla-vuesax td {
    padding: 10px 12px;
    font-weight: 600;
    color: var(--color-texto-dark);
    border-bottom: 1px solid var(--color-fondo-soft);
  }

  /* Captura real tomada por el sistema */
  .captura-real {
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .captura-real figcaption {
    font-size: 12px;
    color: var(--color-texto-secundario, #5d6d6b);
  }

  .captura-real img {
    display: block;
    width: 100%;
    height: auto;
    border-radius: 14px;
    border: 1px solid rgba(0, 0, 0, 0.08);
    background: #f4f7f6;
  }

  .aviso-sin-captura {
    margin: 0 0 12px;
    padding: 10px 14px;
    border-radius: 12px;
    background: #f4f7f6;
    font-size: 12px;
    color: #5d6d6b;
  }

  /* Pestaña de Evidencia / Captura Oficial */
  .contenedor-evidencia-visual {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .barra-herramientas-evidencia {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
  }

  .meta-portal {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .meta-portal strong {
    color: var(--color-texto-dark);
  }

  .botones-zoom-evidencia {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .btn-accion-evidencia {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border-radius: 12px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 11.5px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-accion-evidencia.primario {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .btn-accion-evidencia:hover {
    transform: translateY(-2px);
  }

  .lienzo-certificado-oficial {
    background-color: var(--color-fondo-soft);
    border-radius: 22px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .barra-navegador-simulado {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 12px;
  }

  .puntos-ventana {
    display: flex;
    gap: 6px;
  }

  .puntos-ventana span {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: #d5dde2;
  }

  .url-simulada {
    flex: 1;
    background-color: #ffffff;
    padding: 5px 14px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .radicado-badge {
    font-size: 11px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .hoja-certificado {
    background-color: #ffffff;
    border-radius: 18px;
    padding: 24px 28px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    box-shadow: 0 10px 26px -10px rgba(44, 62, 80, 0.08);
  }

  .encabezado-certificado {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 14px;
    flex-wrap: wrap;
  }

  .entidad-sup {
    font-size: 11px;
    font-weight: 600;
    color: var(--color-texto-grey);
    text-transform: uppercase;
  }

  .titulo-cert {
    font-size: 16px;
    font-weight: 700;
    color: var(--color-texto-dark);
    margin-top: 2px;
  }

  .sello-verificacion {
    padding: 6px 14px;
    border-radius: 999px;
    background-color: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
  }

  .sello-verificacion.sello-rojo {
    background-color: rgba(239, 68, 68, 0.14);
    color: #ef4444;
  }

  .ficha-ciudadano-certificado {
    background-color: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 14px 18px;
    display: flex;
    align-items: center;
    gap: 18px;
    flex-wrap: wrap;
  }

  .contenedor-foto-cert {
    position: relative;
    width: 82px;
    height: 82px;
    border-radius: 18px;
    overflow: hidden;
    padding: 0;
    cursor: pointer;
    flex-shrink: 0;
    box-shadow: 0 8px 20px -6px rgba(44, 62, 80, 0.18);
    transition: transform 0.25s ease;
  }

  .contenedor-foto-cert:hover {
    transform: scale(1.05);
  }

  .contenedor-foto-cert img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .etiqueta-ver-foto {
    position: absolute;
    bottom: 0;
    inset-inline: 0;
    background: rgba(44, 62, 80, 0.78);
    color: #ffffff;
    font-size: 9px;
    font-weight: 600;
    padding: 2px 0;
    text-align: center;
  }

  .datos-ciudadano-cert {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
    gap: 10px 16px;
  }

  .par-cert {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .par-cert span {
    font-size: 10.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .par-cert strong {
    font-size: 13px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .caja-dictamen-certificado {
    background-color: rgba(var(--vs-primary-raw), 0.08);
    border-radius: 16px;
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .caja-dictamen-certificado.dictamen-cert-error {
    background-color: rgba(239, 68, 68, 0.08);
  }

  .etiqueta-dictamen-cert {
    font-size: 10px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .texto-dictamen-cert {
    font-size: 15px;
    font-weight: 700;
    color: var(--color-primario);
  }

  .dictamen-cert-error .texto-dictamen-cert {
    color: #ef4444;
  }

  .observacion-cert {
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-dark);
    margin-top: 2px;
  }

  .grilla-resumen-cert {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 10px;
  }

  .celda-cert {
    background-color: var(--color-fondo-soft);
    border-radius: 12px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .celda-cert span {
    font-size: 10px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .celda-cert strong {
    font-size: 12px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .pie-certificado {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 10px;
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
    padding-top: 6px;
  }

  .pie-certificado strong {
    color: var(--color-texto-dark);
  }

  /* JSON Crudo */
  .contenedor-json-crudo {
    background-color: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 18px;
    overflow-x: auto;
  }

  .contenedor-json-crudo pre {
    margin: 0;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    line-height: 1.55;
    color: var(--color-texto-dark);
  }
</style>
