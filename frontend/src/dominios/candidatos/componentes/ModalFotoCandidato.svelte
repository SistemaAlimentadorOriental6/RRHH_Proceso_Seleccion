<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    CheckmarkCircle02Icon,
    UserIcon,
    Briefcase01Icon,
    File02Icon,
    Shield01Icon
  } from '@hugeicons/core-free-icons';
  import type { DatosCandidatoEscenario } from '../modelos/flujoValidacion';

  interface Propiedades {
    abierto: boolean;
    candidato: DatosCandidatoEscenario;
    vacante: string;
    totalCompletados: number;
    totalNodos: number;
    alCerrar: () => void;
  }

  let {
    abierto,
    candidato,
    vacante,
    totalCompletados,
    totalNodos,
    alCerrar
  }: Propiedades = $props();

  let fotoEnPantallaCompleta = $state(false);

  $effect(() => {
    if (abierto) {
      fotoEnPantallaCompleta = false;
    }
  });
</script>

{#if abierto}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-overlay" onclick={alCerrar}>
    <div
      class="modal-content"
      class:modo-lightbox={fotoEnPantallaCompleta}
      onclick={(e) => e.stopPropagation()}
    >
      {#if fotoEnPantallaCompleta}
        <!-- Vista Lightbox de la Foto en Gran Formato -->
        <div class="contenedor-lightbox">
          <div class="cabecera-lightbox">
            <div class="meta-lightbox">
              <strong>{candidato.nombre}</strong>
              <span>CC {candidato.cedula} • {vacante}</span>
            </div>
            <div class="acciones-lightbox">
              <button
                type="button"
                class="btn-volver-ficha"
                onclick={() => (fotoEnPantallaCompleta = false)}
              >
                Volver al Expediente
              </button>
              <button
                type="button"
                class="btn-cerrar"
                onclick={alCerrar}
              >
                ✕
              </button>
            </div>
          </div>

          <div class="marco-foto-grande">
            <img src={candidato.fotoUrl} alt={candidato.nombre} />
          </div>
        </div>
      {:else}
        <!-- Vista Completa de Foto + Expediente + Hoja de Vida -->
        <header class="cabecera-expediente">
          <div class="titulo-bloque">
            <span class="insignia-expediente">
              <HugeiconsIcon icon={UserIcon} size={13} strokeWidth={2.2} />
              <span>Expediente Digital y Fotografía del Candidato</span>
            </span>
            <h2>{candidato.nombre}</h2>
            <p>{vacante}</p>
          </div>

          <button
            type="button"
            class="btn-cerrar"
            onclick={alCerrar}
            title="Cerrar ventana"
          >
            ✕
          </button>
        </header>

        <div class="cuerpo-expediente">
          <!-- Columna Izquierda: Tarjeta de Fotografía y Estado -->
          <div class="columna-foto">
            <div class="marco-retrato">
              <img src={candidato.fotoUrl} alt={candidato.nombre} />
              <button
                type="button"
                class="boton-ampliar-foto boton-premium"
                onclick={() => (fotoEnPantallaCompleta = true)}
              >
                Ver Foto en Tamaño Completo
              </button>
            </div>

            <div class="tarjeta-avance-candidato">
              <div class="fila-avance">
                <span>Validaciones superadas</span>
                <strong>{totalCompletados} / {totalNodos}</strong>
              </div>
              <div class="barra-avance">
                <div
                  class="relleno-avance"
                  style="width: {Math.round((totalCompletados / totalNodos) * 100)}%;"
                ></div>
              </div>
              <span class="subtexto-verificado">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={13} strokeWidth={2.2} />
                <span>Identidad cruzada con Registraduría / RUNT / ADRES</span>
              </span>
            </div>
          </div>

          <!-- Columna Derecha: Toda la Información del Candidato y Hoja de Vida -->
          <div class="columna-informacion">
            <!-- Grilla de Datos Personales y Operativos -->
            <div class="grilla-ficha">
              <div class="item-ficha">
                <span class="etiqueta-ficha">Documento de Identidad</span>
                <strong class="valor-ficha">CC {candidato.cedula}</strong>
              </div>

              <div class="item-ficha">
                <span class="etiqueta-ficha">Edad y Género</span>
                <strong class="valor-ficha">{candidato.edad > 0 ? `${candidato.edad} años • ` : ''}{candidato.genero}</strong>
              </div>

              <div class="item-ficha">
                <span class="etiqueta-ficha">Licencia de Conducción</span>
                <strong class="valor-ficha texto-primario">{candidato.licencia}</strong>
              </div>

              <div class="item-ficha">
                <span class="etiqueta-ficha">Experiencia en Transporte</span>
                <strong class="valor-ficha">{candidato.experienciaAnios > 0 ? `${candidato.experienciaAnios} años certificados` : 'Sin dato'}</strong>
              </div>

              <div class="item-ficha">
                <span class="etiqueta-ficha">Municipio de Residencia</span>
                <strong class="valor-ficha">{candidato.municipio}</strong>
              </div>

              <div class="item-ficha">
                <span class="etiqueta-ficha">Contacto Directo</span>
                <strong class="valor-ficha">{candidato.telefono}</strong>
              </div>

              <div class="item-ficha ancho-completo">
                <span class="etiqueta-ficha">Afiliación EPS (ADRES BDUA)</span>
                <strong class="valor-ficha">{candidato.eps} — {candidato.regimenEps}</strong>
              </div>
            </div>

            <!-- Resumen Profesional Extraído de la Hoja de Vida -->
            <div class="bloque-resumen-cv">
              <div class="encabezado-bloque-cv">
                <HugeiconsIcon icon={Briefcase01Icon} size={16} strokeWidth={2} />
                <strong>Perfil Profesional y Experiencia Verificada</strong>
              </div>
              <p class="parrafo-resumen">{candidato.resumenPerfil}</p>

              <div class="lista-empresas">
                {#each candidato.empresasPrevias as emp (emp)}
                  <span class="chip-empresa">
                    <HugeiconsIcon icon={CheckmarkCircle02Icon} size={13} strokeWidth={2.2} />
                    <span>{emp}</span>
                  </span>
                {/each}
              </div>
            </div>

            <!-- Archivos PDF Adjuntos del Expediente -->
            <div class="bloque-archivos-pdf">
              <div class="tarjeta-pdf">
                <div class="icono-pdf">
                  <HugeiconsIcon icon={File02Icon} size={20} strokeWidth={2} />
                </div>
                <div class="textos-pdf">
                  <strong>Hoja de Vida Original (PDF)</strong>
                  <span>{candidato.archivoHojaVida}</span>
                </div>
                <span class="badge-pdf-ok">Extraído OCR</span>
              </div>

              <div class="tarjeta-pdf">
                <div class="icono-pdf">
                  <HugeiconsIcon icon={Shield01Icon} size={20} strokeWidth={2} />
                </div>
                <div class="textos-pdf">
                  <strong>Ficha Computrabajo (PDF)</strong>
                  <span>{candidato.archivoPerfil}</span>
                </div>
                <span class="badge-pdf-ok">Verificado</span>
              </div>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    z-index: 210;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }

  .modal-content {
    width: min(920px, 100%);
    height: min(620px, 90vh);
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

  /* El lightbox mantiene el mismo tamaño del modal */
  .modal-content.modo-lightbox {
    width: min(920px, 100%);
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

  .cabecera-expediente {
    padding: 20px 28px 14px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    flex-shrink: 0;
  }

  .insignia-expediente {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 11px;
    border-radius: 999px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 600;
  }

  .titulo-bloque h2 {
    font-size: 21px;
    font-weight: 700;
    color: var(--color-texto-dark);
    margin-top: 6px;
  }

  .titulo-bloque p {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
    margin-top: 2px;
  }

  .btn-cerrar {
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

  .btn-cerrar:hover {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .cuerpo-expediente {
    flex: 1;
    min-height: 0;
    padding: 8px 28px 24px;
    overflow-x: hidden;
    overflow-y: auto;
    display: grid;
    grid-template-columns: 260px 1fr;
    gap: 24px;
    scrollbar-width: thin;
  }

  @media (max-width: 760px) {
    .cuerpo-expediente {
      grid-template-columns: 1fr;
    }
  }

  .columna-foto {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .marco-retrato {
    background-color: var(--color-fondo-soft);
    border-radius: 24px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    align-items: center;
  }

  .marco-retrato img {
    width: 100%;
    aspect-ratio: 1 / 1;
    border-radius: 20px;
    object-fit: cover;
    box-shadow: 0 12px 28px -8px rgba(44, 62, 80, 0.15);
  }

  .boton-ampliar-foto {
    width: 100%;
    padding: 10px 14px;
    border-radius: 14px;
    font-size: 12px;
    font-weight: 700;
    text-align: center;
  }

  .tarjeta-avance-candidato {
    background-color: var(--color-fondo-soft);
    border-radius: 20px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .fila-avance {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 11.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .fila-avance strong {
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .barra-avance {
    width: 100%;
    height: 6px;
    background-color: #ffffff;
    border-radius: 999px;
    overflow: hidden;
  }

  .relleno-avance {
    height: 100%;
    background-color: var(--color-primario);
    border-radius: 999px;
    transition: width 0.3s ease;
  }

  .subtexto-verificado {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--color-primario);
  }

  .columna-informacion {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .grilla-ficha {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(185px, 1fr));
    gap: 10px;
  }

  .item-ficha {
    background-color: var(--color-fondo-soft);
    border-radius: 16px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .item-ficha.ancho-completo {
    grid-column: 1 / -1;
  }

  .etiqueta-ficha {
    font-size: 10.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .valor-ficha {
    font-size: 13px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .valor-ficha.texto-primario {
    color: var(--color-primario);
  }

  .bloque-resumen-cv {
    background-color: var(--color-fondo-soft);
    border-radius: 20px;
    padding: 16px 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .encabezado-bloque-cv {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--color-texto-dark);
  }

  .parrafo-resumen {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
    line-height: 1.5;
  }

  .lista-empresas {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-empresa {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background-color: #ffffff;
    color: var(--color-texto-dark);
    font-size: 11.5px;
    font-weight: 600;
    padding: 6px 12px;
    border-radius: 12px;
  }

  .bloque-archivos-pdf {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 10px;
  }

  .tarjeta-pdf {
    background-color: var(--color-fondo-soft);
    border-radius: 16px;
    padding: 12px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .icono-pdf {
    width: 38px;
    height: 38px;
    border-radius: 12px;
    background-color: #ffffff;
    color: var(--color-primario);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .textos-pdf {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .textos-pdf strong {
    font-size: 12px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .textos-pdf span {
    font-size: 10.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .badge-pdf-ok {
    font-size: 10px;
    font-weight: 700;
    color: var(--color-primario);
    background-color: rgba(var(--vs-primary-raw), 0.12);
    padding: 3px 8px;
    border-radius: 999px;
    flex-shrink: 0;
  }

  /* Lightbox Foto Completa */
  .contenedor-lightbox {
    padding: 22px 26px 26px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .cabecera-lightbox {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .meta-lightbox {
    display: flex;
    flex-direction: column;
  }

  .meta-lightbox strong {
    font-size: 17px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .meta-lightbox span {
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .acciones-lightbox {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .btn-volver-ficha {
    padding: 8px 14px;
    border-radius: 12px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .marco-foto-grande {
    background-color: var(--color-fondo-soft);
    border-radius: 24px;
    padding: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .marco-foto-grande img {
    width: 100%;
    max-width: 440px;
    border-radius: 20px;
    box-shadow: 0 18px 38px -10px rgba(44, 62, 80, 0.2);
  }
</style>
