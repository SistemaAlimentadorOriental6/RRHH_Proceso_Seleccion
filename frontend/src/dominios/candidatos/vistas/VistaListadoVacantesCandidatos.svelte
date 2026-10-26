<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    Briefcase01Icon,
    UserGroupIcon,
    CheckmarkCircle02Icon,
    Cancel01Icon,
    ReloadIcon,
    ChevronRightIcon,
    Calendar03Icon,
    LicenseIcon,
    WorkflowSquare10Icon,
    EyeIcon
  } from '@hugeicons/core-free-icons';
  import {
    CATALOGO_VACANTES_EVALUACION,
    type CandidatoEvaluadoEnVacante,
    type VacanteConCandidatos
  } from '../modelos/catalogoVacantesCandidatos';
  import ModalFotoCandidato from '../componentes/ModalFotoCandidato.svelte';

  interface Propiedades {
    slugVacanteFiltrada?: string | null;
    alSeleccionarVacante: (slugVacante: string) => void;
    alVerTodasLasVacantes: () => void;
    alIngresarFlujoCandidato: (slugVacante: string, slugCandidato: string) => void;
  }

  let {
    slugVacanteFiltrada = null,
    alSeleccionarVacante,
    alVerTodasLasVacantes,
    alIngresarFlujoCandidato
  }: Propiedades = $props();

  let textoBusqueda = $state('');
  let candidatoFotoModal = $state<{
    candidato: CandidatoEvaluadoEnVacante;
    vacante: VacanteConCandidatos;
  } | null>(null);

  const vacantesVisibles = $derived.by(() => {
    const base = slugVacanteFiltrada
      ? CATALOGO_VACANTES_EVALUACION.filter((v) => v.slug === slugVacanteFiltrada)
      : CATALOGO_VACANTES_EVALUACION;

    const termino = textoBusqueda.trim().toLowerCase();
    if (!termino) return base;

    return base
      .map((vac) => {
        const coincideVacante =
          vac.titulo.toLowerCase().includes(termino) ||
          vac.codigoRequisicion.toLowerCase().includes(termino) ||
          vac.slug.toLowerCase().includes(termino);

        const candidatosFiltrados = vac.candidatos.filter(
          (c) =>
            c.datos.nombre.toLowerCase().includes(termino) ||
            c.datos.cedula.toLowerCase().includes(termino) ||
            c.slug.toLowerCase().includes(termino)
        );

        if (coincideVacante) return vac;
        if (candidatosFiltrados.length > 0) {
          return {
            ...vac,
            candidatos: candidatosFiltrados
          };
        }
        return null;
      })
      .filter((v): v is VacanteConCandidatos => v !== null);
  });

  const totalVacantesActivas = $derived(CATALOGO_VACANTES_EVALUACION.length);
  const totalPersonasEvaluadas = $derived(
    CATALOGO_VACANTES_EVALUACION.reduce((acc, v) => acc + v.candidatos.length, 0)
  );
  const totalCandidatosAptos = $derived(
    CATALOGO_VACANTES_EVALUACION.reduce(
      (acc, v) =>
        acc +
        v.candidatos.filter((c) => c.estadoDictamen !== 'rechazado_regla').length,
      0
    )
  );
</script>

<div class="contenedor-listado-vacantes">
  <!-- Cabecera de Resumen y Buscador Estilo Vuesax (DESING.md) -->
  <section class="panel-resumen-superior">
    <div class="textos-encabezado">
      <div class="fila-insignia">
        <span class="insignia-modulo">
          <HugeiconsIcon icon={Briefcase01Icon} size={13} strokeWidth={2.2} />
          <span>Selección por Vacantes y Candidatos</span>
        </span>

        {#if slugVacanteFiltrada}
          <button
            type="button"
            class="boton-limpiar-filtro"
            onclick={alVerTodasLasVacantes}
          >
            ← Ver todas las vacantes ({totalVacantesActivas})
          </button>
        {/if}
      </div>

      <h2 class="titulo-seccion">
        {slugVacanteFiltrada && vacantesVisibles[0]
          ? vacantesVisibles[0].titulo
          : 'Vacantes en Evaluación y Candidatos Postulados'}
      </h2>
      <p class="descripcion-seccion">
        Selecciona cualquier persona evaluada dentro de su vacante para abrir su flujo interactivo n8n de 14 validaciones.
      </p>
    </div>

    <div class="bloque-busqueda-metricas">
      <div class="tarjetas-kpi-mini">
        <div class="kpi-pildora">
          <span>Vacantes</span>
          <strong>{totalVacantesActivas}</strong>
        </div>
        <div class="kpi-pildora">
          <span>Evaluados</span>
          <strong>{totalPersonasEvaluadas}</strong>
        </div>
        <div class="kpi-pildora kpi-verde">
          <span>Aptos 14/14</span>
          <strong>{totalCandidatosAptos}</strong>
        </div>
      </div>

      <input
        type="search"
        class="buscador-vuesax input-premium"
        placeholder="Buscar por vacante, nombre de candidato o cédula..."
        bind:value={textoBusqueda}
      />
    </div>
  </section>

  <!-- Listado de Vacantes con sus Candidatos Evaluados -->
  <div class="lista-vacantes">
    {#each vacantesVisibles as vacante (vacante.id)}
      <article class="tarjeta-vacante-vuesax">
        <!-- Cabecera de la Vacante con su Slug -->
        <div class="cabecera-vacante">
          <div class="info-principal-vacante">
            <div class="fila-codigos">
              <span class="chip-requisicion">{vacante.codigoRequisicion}</span>
              <span class="chip-slug">/{vacante.slug}</span>
              <span class="chip-estado-vacante">{vacante.estadoVacante}</span>
            </div>

            <h3 class="titulo-vacante">
              <button
                type="button"
                class="boton-titulo-vacante"
                onclick={() => alSeleccionarVacante(vacante.slug)}
              >
                {vacante.titulo}
              </button>
            </h3>

            <div class="detalles-operativos-vacante">
              <span class="meta-item">
                <HugeiconsIcon icon={Briefcase01Icon} size={13} strokeWidth={2} />
                <span>{vacante.areaOperativa} • {vacante.sede}</span>
              </span>
              <span class="meta-item">
                <HugeiconsIcon icon={LicenseIcon} size={13} strokeWidth={2} />
                <span>Exige: <strong>{vacante.licenciaExigida}</strong></span>
              </span>
              <span class="meta-item">
                <HugeiconsIcon icon={Calendar03Icon} size={13} strokeWidth={2} />
                <span>Apertura: {vacante.fechaApertura} • {vacante.cuposDisponibles} cupos</span>
              </span>
            </div>
          </div>

          <div class="acciones-vacante">
            <div class="contador-candidatos-vacante">
              <HugeiconsIcon icon={UserGroupIcon} size={16} strokeWidth={2} />
              <span>
                <strong>{vacante.candidatos.length}</strong> personas evaluadas
              </span>
            </div>

            {#if !slugVacanteFiltrada}
              <button
                type="button"
                class="btn-filtrar-vacante"
                onclick={() => alSeleccionarVacante(vacante.slug)}
              >
                <span>Filtrar vacante</span>
                <HugeiconsIcon icon={ChevronRightIcon} size={14} strokeWidth={2.2} />
              </button>
            {/if}
          </div>
        </div>

        <!-- Grilla de Personas Evaluadas en esta Vacante -->
        <div class="grilla-candidatos-vacante">
          {#each vacante.candidatos as itemCandidato (itemCandidato.slug)}
            <div
              class="tarjeta-persona-evaluada"
              class:borde-rechazado={itemCandidato.estadoDictamen === 'rechazado_regla'}
            >
              <!-- Fila Superior: Foto + Nombre + Cédula + Estado -->
              <div class="cabecera-persona">
                <button
                  type="button"
                  class="avatar-persona"
                  title="Ver foto en grande de {itemCandidato.datos.nombre}"
                  onclick={() =>
                    (candidatoFotoModal = {
                      candidato: itemCandidato,
                      vacante
                    })}
                >
                  <img
                    src={itemCandidato.datos.fotoUrl}
                    alt={itemCandidato.datos.nombre}
                  />
                  <span class="etiqueta-foto">Foto</span>
                </button>

                <div class="identidad-persona">
                  <div class="fila-estado-persona">
                    <span
                      class="pildora-dictamen-persona dictamen-{itemCandidato.estadoDictamen}"
                    >
                      {#if itemCandidato.estadoDictamen === 'apto_prioritario'}
                        <HugeiconsIcon
                          icon={CheckmarkCircle02Icon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {:else if itemCandidato.estadoDictamen === 'con_reintento_recuperado'}
                        <HugeiconsIcon
                          icon={ReloadIcon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {:else}
                        <HugeiconsIcon
                          icon={Cancel01Icon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {/if}
                      <span>{itemCandidato.etiquetaDictamen}</span>
                    </span>
                  </div>

                  <h4 class="nombre-persona">{itemCandidato.datos.nombre}</h4>
                  <span class="subtitulo-persona">
                    CC {itemCandidato.datos.cedula} • {itemCandidato.datos.edad} años • {itemCandidato.datos.municipio}
                  </span>
                </div>
              </div>

              <!-- Chips de Resumen Rápido del Candidato -->
              <div class="chips-resumen-persona">
                <div class="chip-dato-persona">
                  <span>Licencia</span>
                  <strong>{itemCandidato.datos.licencia}</strong>
                </div>
                <div class="chip-dato-persona">
                  <span>Avance n8n</span>
                  <strong>
                    {itemCandidato.pasosSuperados} / {itemCandidato.totalPasos} nodos
                  </strong>
                </div>
              </div>

              <p class="observacion-persona">{itemCandidato.nodoObservacion}</p>

              <!-- Pie con Slug y Botones de Acción (Ver Foto / Entrar al Flujo n8n) -->
              <div class="pie-persona">
                <span class="ruta-slug-persona" title="/{vacante.slug}/{itemCandidato.slug}">
                  /{itemCandidato.slug}
                </span>

                <div class="botones-persona">
                  <button
                    type="button"
                    class="btn-ver-foto-ficha"
                    onclick={() =>
                      (candidatoFotoModal = {
                        candidato: itemCandidato,
                        vacante
                      })}
                    title="Ver fotografía y expediente"
                  >
                    <HugeiconsIcon icon={EyeIcon} size={14} strokeWidth={2} />
                    <span>Expediente</span>
                  </button>

                  <button
                    type="button"
                    class="btn-abrir-flujo boton-premium"
                    onclick={() =>
                      alIngresarFlujoCandidato(vacante.slug, itemCandidato.slug)}
                  >
                    <HugeiconsIcon
                      icon={WorkflowSquare10Icon}
                      size={14}
                      strokeWidth={2.2}
                    />
                    <span>Ver Flujo n8n</span>
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      </article>
    {/each}
  </div>

  <!-- Modal Glassmorphism para Ver Foto y Expediente desde el Listado -->
  {#if candidatoFotoModal}
    <ModalFotoCandidato
      abierto={true}
      candidato={candidatoFotoModal.candidato.datos}
      vacante={candidatoFotoModal.vacante.titulo}
      totalCompletados={candidatoFotoModal.candidato.pasosSuperados}
      totalNodos={candidatoFotoModal.candidato.totalPasos}
      alCerrar={() => (candidatoFotoModal = null)}
    />
  {/if}
</div>

<style>
  .contenedor-listado-vacantes {
    padding: 26px 32px 42px;
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  /* Panel Superior Minimalista Vuesax (DESING.md) */
  .panel-resumen-superior {
    background-color: #ffffff;
    border-radius: 26px;
    padding: 24px 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 20px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .textos-encabezado {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-width: 620px;
  }

  .fila-insignia {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
  }

  .insignia-modulo {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 12px;
    border-radius: 999px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
  }

  .boton-limpiar-filtro {
    padding: 4px 12px;
    border-radius: 999px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .boton-limpiar-filtro:hover {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .titulo-seccion {
    font-size: 21px;
    font-weight: 700;
    color: var(--color-texto-dark);
    letter-spacing: -0.01em;
  }

  .descripcion-seccion {
    font-size: 13px;
    font-weight: 500;
    color: var(--color-texto-grey);
    line-height: 1.45;
  }

  .bloque-busqueda-metricas {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 12px;
    flex: 1;
    min-width: 300px;
  }

  .tarjetas-kpi-mini {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
  }

  .kpi-pildora {
    background-color: var(--color-fondo-soft);
    padding: 7px 14px;
    border-radius: 14px;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 11.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .kpi-pildora strong {
    font-size: 14px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .kpi-pildora.kpi-verde {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .kpi-pildora.kpi-verde strong {
    color: var(--color-primario);
  }

  .buscador-vuesax {
    width: 100%;
    max-width: 380px;
    padding: 11px 16px;
    border-radius: 14px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
  }

  /* Tarjeta de cada Vacante */
  .lista-vacantes {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  .tarjeta-vacante-vuesax {
    background-color: #ffffff;
    border-radius: 28px;
    padding: 24px 28px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    box-shadow: 0 14px 36px -14px rgba(44, 62, 80, 0.07);
  }

  .cabecera-vacante {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 16px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--color-fondo-soft);
  }

  .info-principal-vacante {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .fila-codigos {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-requisicion {
    font-size: 10.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
    background-color: var(--color-fondo-soft);
    padding: 3px 10px;
    border-radius: 8px;
  }

  .chip-slug {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--color-texto-grey);
    background-color: var(--color-fondo-soft);
    padding: 3px 10px;
    border-radius: 8px;
  }

  .chip-estado-vacante {
    font-size: 10.5px;
    font-weight: 600;
    color: var(--color-primario);
    background-color: rgba(var(--vs-primary-raw), 0.12);
    padding: 3px 10px;
    border-radius: 999px;
  }

  .titulo-vacante {
    margin: 0;
  }

  .boton-titulo-vacante {
    padding: 0;
    background: transparent;
    text-align: left;
    font-size: 18px;
    font-weight: 700;
    color: var(--color-texto-dark);
    cursor: pointer;
    transition: color 0.25s ease;
  }

  .boton-titulo-vacante:hover {
    color: var(--color-primario);
  }

  .detalles-operativos-vacante {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 16px;
    margin-top: 2px;
  }

  .meta-item {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .meta-item strong {
    color: var(--color-texto-dark);
    font-weight: 600;
  }

  .acciones-vacante {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .contador-candidatos-vacante {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 8px 14px;
    border-radius: 14px;
    background-color: var(--color-fondo-soft);
    font-size: 12px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .contador-candidatos-vacante strong {
    color: var(--color-texto-dark);
    font-weight: 700;
  }

  .btn-filtrar-vacante {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 8px 14px;
    border-radius: 14px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-filtrar-vacante:hover {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    transform: translateY(-2px);
  }

  /* Grilla de Tarjetas de Personas Evaluadas */
  .grilla-candidatos-vacante {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 16px;
  }

  .tarjeta-persona-evaluada {
    background-color: var(--color-fondo-soft);
    border: 1.5px solid transparent;
    border-radius: 22px;
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 13px;
    transition: all 0.25s ease;
  }

  .tarjeta-persona-evaluada:hover {
    background-color: #ffffff;
    border-color: rgba(var(--vs-primary-raw), 0.35);
    transform: translateY(-3px);
    box-shadow: 0 16px 34px -10px rgba(44, 62, 80, 0.1);
  }

  .tarjeta-persona-evaluada.borde-rechazado:hover {
    border-color: rgba(239, 68, 68, 0.35);
  }

  .cabecera-persona {
    display: flex;
    align-items: center;
    gap: 13px;
  }

  .avatar-persona {
    position: relative;
    width: 56px;
    height: 56px;
    border-radius: 18px;
    overflow: hidden;
    padding: 0;
    cursor: pointer;
    flex-shrink: 0;
    box-shadow: 0 8px 18px -6px rgba(44, 62, 80, 0.18);
    transition: transform 0.25s ease;
  }

  .avatar-persona:hover {
    transform: scale(1.06);
  }

  .avatar-persona img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .etiqueta-foto {
    position: absolute;
    bottom: 0;
    inset-inline: 0;
    background: rgba(44, 62, 80, 0.75);
    color: #ffffff;
    font-size: 8.5px;
    font-weight: 700;
    text-align: center;
    padding: 1.5px 0;
  }

  .identidad-persona {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }

  .fila-estado-persona {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .pildora-dictamen-persona {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2.5px 9px;
    border-radius: 999px;
    font-size: 10px;
    font-weight: 600;
  }

  .dictamen-apto_prioritario,
  .dictamen-con_reintento_recuperado {
    background-color: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .dictamen-rechazado_regla {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
  }

  .nombre-persona {
    font-size: 14.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .subtitulo-persona {
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .chips-resumen-persona {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .chip-dato-persona {
    background-color: #ffffff;
    border-radius: 12px;
    padding: 8px 11px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .chip-dato-persona span {
    font-size: 9.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .chip-dato-persona strong {
    font-size: 11px;
    font-weight: 700;
    color: var(--color-texto-dark);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .observacion-persona {
    font-size: 11.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
    line-height: 1.4;
  }

  .pie-persona {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding-top: 4px;
  }

  .ruta-slug-persona {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    font-weight: 500;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 120px;
  }

  .botones-persona {
    display: flex;
    align-items: center;
    gap: 7px;
    flex-shrink: 0;
  }

  .btn-ver-foto-ficha {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 8px 11px;
    border-radius: 12px;
    background-color: #ffffff;
    color: var(--color-texto-dark);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-ver-foto-ficha:hover {
    color: var(--color-primario);
    transform: translateY(-2px);
  }

  .btn-abrir-flujo {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border-radius: 12px;
    font-size: 11.5px;
    font-weight: 700;
  }
</style>
