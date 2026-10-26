<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    ArrowRight01Icon,
    Award01Icon,
    Briefcase01Icon,
    CheckmarkCircle02Icon,
    Cancel01Icon,
    Location01Icon,
    SparklesIcon,
    UserMultiple02Icon
  } from '@hugeicons/core-free-icons';
  import {
    CATALOGO_VACANTES_RECLUTAMIENTO,
    obtenerCandidatosAceptadosOrdenadosPorScore,
    obtenerCandidatosRechazadosVacante,
    type VacanteReclutamiento
  } from '../modelos/modelosReclutamiento';

  interface Propiedades {
    alEntrarAVacante: (slugVacante: string) => void;
    alEntrarACandidatoDirecto: (slugVacante: string, slugCandidato: string) => void;
  }

  let { alEntrarAVacante, alEntrarACandidatoDirecto }: Propiedades = $props();

  let busquedaTexto = $state('');

  const vacantesFiltradas = $derived.by(() => {
    const q = busquedaTexto.trim().toLowerCase();
    if (!q) return CATALOGO_VACANTES_RECLUTAMIENTO;
    return CATALOGO_VACANTES_RECLUTAMIENTO.filter(
      (vacante) =>
        vacante.titulo.toLowerCase().includes(q) ||
        vacante.codigoRequisicion.toLowerCase().includes(q) ||
        vacante.sede.toLowerCase().includes(q) ||
        vacante.candidatos.some(
          (c) =>
            c.nombreCompleto.toLowerCase().includes(q) ||
            c.cedula.includes(q)
        )
    );
  });

  const metricasGlobales = $derived.by(() => {
    let totalPostulados = 0;
    let totalAceptados = 0;
    let totalRechazados = 0;
    let mejorScoreGlobal = 0;

    for (const v of CATALOGO_VACANTES_RECLUTAMIENTO) {
      totalPostulados += v.candidatos.length;
      for (const c of v.candidatos) {
        if (c.clasificacionPrimaria.esApto) {
          totalAceptados += 1;
          if ((c.evaluacionIA?.scoreTotal ?? 0) > mejorScoreGlobal) {
            mejorScoreGlobal = c.evaluacionIA?.scoreTotal ?? 0;
          }
        } else {
          totalRechazados += 1;
        }
      }
    }

    return {
      totalVacantes: CATALOGO_VACANTES_RECLUTAMIENTO.length,
      totalPostulados,
      totalAceptados,
      totalRechazados,
      mejorScoreGlobal
    };
  });

  function obtenerMejorCandidatoVacante(vacante: VacanteReclutamiento) {
    const aceptados = obtenerCandidatosAceptadosOrdenadosPorScore(vacante);
    return aceptados[0] ?? null;
  }
</script>

<div class="contenedor-reclutamiento">
  <!-- Banner Minimalista Vuesax (DESING.md) -->
  <section class="banner-reclutamiento">
    <div class="banner-info">
      <div class="insignia-modulo">
        <HugeiconsIcon icon={SparklesIcon} size={14} color="#4cc253" />
        <span>Reclutamiento • Selección Inteligente</span>
      </div>
      <h1>Vacantes y Ranking IA</h1>
      <p>
        Convocatorias operativas activas, clasificación primaria de conductores y ordenamiento por idoneidad.
      </p>
    </div>

    <div class="kpis-globales">
      <div class="tarjeta-kpi">
        <span class="kpi-valor">{metricasGlobales.totalVacantes}</span>
        <span class="kpi-etiqueta">Vacantes</span>
      </div>
      <div class="tarjeta-kpi">
        <span class="kpi-valor">{metricasGlobales.totalPostulados}</span>
        <span class="kpi-etiqueta">Evaluados</span>
      </div>
      <div class="tarjeta-kpi kpi-exito">
        <span class="kpi-valor">{metricasGlobales.totalAceptados}</span>
        <span class="kpi-etiqueta">Aceptados</span>
      </div>
      <div class="tarjeta-kpi kpi-peligro">
        <span class="kpi-valor">{metricasGlobales.totalRechazados}</span>
        <span class="kpi-etiqueta">Rechazados</span>
      </div>
    </div>
  </section>

  <!-- Barra de búsqueda limpia (sin notas técnicas) -->
  <div class="barra-herramientas">
    <div class="caja-busqueda">
      <input
        type="text"
        placeholder="Buscar vacante, sede, cargo o nombre de candidato..."
        bind:value={busquedaTexto}
      />
    </div>
  </div>

  <!-- Grilla de Vacantes con Estilo Vuesax -->
  <div class="grilla-vacantes">
    {#each vacantesFiltradas as vacante (vacante.id)}
      {@const aceptadosOrdenados = obtenerCandidatosAceptadosOrdenadosPorScore(vacante)}
      {@const rechazados = obtenerCandidatosRechazadosVacante(vacante)}
      {@const topCandidato = obtenerMejorCandidatoVacante(vacante)}

      <article class="tarjeta-vacante">
        <div class="cabecera-vacante">
          <div class="fila-meta">
            <span class="badge-codigo">{vacante.codigoRequisicion}</span>
            <span class="badge-estado-ia">
              <HugeiconsIcon icon={SparklesIcon} size={13} color="#15803d" />
              {vacante.estadoVacante}
            </span>
            <span class="badge-cupos">{vacante.cuposDisponibles} cupos</span>
          </div>

          <button
            type="button"
            class="titulo-vacante-btn"
            onclick={() => alEntrarAVacante(vacante.slug)}
          >
            <h2>{vacante.titulo}</h2>
          </button>

          <div class="detalles-operativos">
            <span>
              <HugeiconsIcon icon={Location01Icon} size={14} color="#64748b" />
              {vacante.sede}
            </span>
            <span>
              <HugeiconsIcon icon={Briefcase01Icon} size={14} color="#64748b" />
              {vacante.licenciaExigida}
            </span>
            <span>{vacante.salarioReferencia}</span>
          </div>
        </div>

        <!-- Resumen de Clasificación Primaria -->
        <div class="franja-clasificacion">
          <div class="bloque-conteo total">
            <HugeiconsIcon icon={UserMultiple02Icon} size={18} color="#2c3e50" />
            <div>
              <strong>{vacante.candidatos.length}</strong>
              <span>Postulados</span>
            </div>
          </div>

          <div class="bloque-conteo aceptados">
            <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} color="#16a34a" />
            <div>
              <strong>{aceptadosOrdenados.length} Aceptados</strong>
              <span>Aptos</span>
            </div>
          </div>

          <div class="bloque-conteo rechazados">
            <HugeiconsIcon icon={Cancel01Icon} size={18} color="#dc2626" />
            <div>
              <strong>{rechazados.length} Rechazados</strong>
              <span>No Aptos</span>
            </div>
          </div>
        </div>

        <!-- Vista previa del Top #1 según IA -->
        {#if topCandidato && topCandidato.evaluacionIA}
          <div class="destacado-top-ia">
            <div class="etiqueta-top-ia">
              <div class="medalla-top">
                <HugeiconsIcon icon={Award01Icon} size={15} color="#15803d" />
                <span>#1 en Ranking IA ({topCandidato.evaluacionIA.scoreTotal}/100 pts)</span>
              </div>
              <button
                type="button"
                class="link-ver-expediente"
                onclick={() => alEntrarACandidatoDirecto(vacante.slug, topCandidato.slug)}
              >
                Ver expediente →
              </button>
            </div>

            <div class="cuerpo-top-ia">
              <img
                src={topCandidato.fotoUrl}
                alt={topCandidato.nombreCompleto}
                class="avatar-top"
              />
              <div class="info-top">
                <div class="nombre-linea">
                  <strong>{topCandidato.nombreCompleto}</strong>
                  <span class="chip-apto">{topCandidato.evaluacionIA.dictamenIA}</span>
                </div>
                <span class="meta-top">
                  C.C. {topCandidato.cedula} • {topCandidato.categoriaLicenciaPrincipal} • {topCandidato.anosExperienciaConduccion} años exp.
                </span>
                <p class="razon-corta">
                  {topCandidato.evaluacionIA.porQueEsElIndicado}
                </p>
              </div>
            </div>
          </div>
        {/if}

        <!-- Píldoras rápidas de candidatos evaluados en la vacante -->
        <div class="previa-candidatos">
          <span class="subtitulo-previa">Candidatos evaluados:</span>
          <div class="lista-chips-candidatos">
            {#each vacante.candidatos as cand (cand.id)}
              <button
                type="button"
                class="chip-persona"
                class:es-apto={cand.clasificacionPrimaria.esApto}
                class:es-no-apto={!cand.clasificacionPrimaria.esApto}
                onclick={() => alEntrarACandidatoDirecto(vacante.slug, cand.slug)}
              >
                <span class="punto-estado"></span>
                <span class="nombre-chip">{cand.nombreCompleto.split(' ').slice(0, 2).join(' ')}</span>
                {#if cand.clasificacionPrimaria.esApto && cand.evaluacionIA}
                  <span class="puntaje-chip">{cand.evaluacionIA.scoreTotal} pts</span>
                {:else}
                  <span class="no-apto-chip">No Apto</span>
                {/if}
              </button>
            {/each}
          </div>
        </div>

        <!-- Pie de tarjeta con CTA -->
        <div class="pie-tarjeta-vacante">
          <button
            type="button"
            class="btn-entrar-vacante"
            onclick={() => alEntrarAVacante(vacante.slug)}
          >
            <span>Ver Vacante y Ranking IA</span>
            <HugeiconsIcon icon={ArrowRight01Icon} size={16} color="#ffffff" />
          </button>
        </div>
      </article>
    {/each}
  </div>
</div>

<style>
  .contenedor-reclutamiento {
    padding: 26px 32px 48px;
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 1600px;
    margin: 0 auto;
    width: 100%;
    box-sizing: border-box;
  }

  .banner-reclutamiento {
    background: #ffffff;
    border-radius: 26px;
    padding: 24px 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.05);
    border: 1px solid rgba(226, 232, 240, 0.75);
  }

  .banner-info {
    max-width: 680px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .insignia-modulo {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    align-self: flex-start;
    background: rgba(var(--vs-primary-raw), 0.1);
    color: #15803d;
    padding: 5px 12px;
    border-radius: 999px;
    font-size: 0.74rem;
    font-weight: 600;
  }

  .banner-info h1 {
    margin: 0;
    font-size: 1.38rem;
    font-weight: 700;
    color: var(--color-texto-dark);
    letter-spacing: -0.02em;
  }

  .banner-info p {
    margin: 0;
    font-size: 0.84rem;
    line-height: 1.5;
    color: var(--color-texto-grey);
  }

  .kpis-globales {
    display: grid;
    grid-template-columns: repeat(4, minmax(105px, 1fr));
    gap: 12px;
    flex-shrink: 0;
  }

  .tarjeta-kpi {
    background: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    border: 1px solid transparent;
    transition: all 0.25s ease;
  }

  .tarjeta-kpi:hover {
    transform: translateY(-2px);
  }

  .tarjeta-kpi.kpi-exito {
    background: rgba(var(--vs-primary-raw), 0.08);
  }

  .tarjeta-kpi.kpi-peligro {
    background: rgba(239, 68, 68, 0.06);
  }

  .kpi-valor {
    font-size: 1.45rem;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1.1;
  }

  .tarjeta-kpi.kpi-exito .kpi-valor {
    color: #15803d;
  }

  .tarjeta-kpi.kpi-peligro .kpi-valor {
    color: #dc2626;
  }

  .kpi-etiqueta {
    margin-top: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .barra-herramientas {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .caja-busqueda {
    flex: 1;
  }

  .caja-busqueda input {
    width: 100%;
    padding: 14px 22px;
    border-radius: 16px;
    border: 2px solid transparent;
    background: #ffffff;
    font-family: inherit;
    font-size: 0.85rem;
    color: var(--color-texto-dark);
    box-shadow: 0 4px 18px rgba(44, 62, 80, 0.04);
    transition: all 0.25s ease;
  }

  .caja-busqueda input:focus {
    outline: none;
    background: #ffffff;
    border-color: var(--color-primario);
    box-shadow: 0px 5px 20px 0px rgba(var(--vs-primary-raw), 0.15);
    transform: translateY(-2px);
  }

  .grilla-vacantes {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
    gap: 20px;
  }

  .tarjeta-vacante {
    background: #ffffff;
    border-radius: 26px;
    padding: 26px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.05);
    border: 1px solid rgba(226, 232, 240, 0.75);
    transition: transform 0.25s ease, box-shadow 0.25s ease;
  }

  .tarjeta-vacante:hover {
    transform: translateY(-3px);
    box-shadow: 0 20px 40px -12px rgba(44, 62, 80, 0.1);
  }

  .cabecera-vacante {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .fila-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .badge-codigo {
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--color-texto-dark);
    background: var(--color-fondo-soft);
    padding: 4px 10px;
    border-radius: 8px;
  }

  .badge-estado-ia {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--color-primario);
    background: rgba(var(--vs-primary-raw), 0.12);
    padding: 4px 11px;
    border-radius: 8px;
  }

  .badge-cupos {
    margin-left: auto;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--color-texto-grey);
    background: var(--color-fondo-soft);
    padding: 4px 10px;
    border-radius: 8px;
  }

  .titulo-vacante-btn {
    background: none;
    border: none;
    padding: 0;
    text-align: left;
    cursor: pointer;
    font-family: inherit;
  }

  .titulo-vacante-btn h2 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 700;
    color: var(--color-texto-dark);
    transition: color 0.25s ease;
  }

  .titulo-vacante-btn:hover h2 {
    color: var(--color-primario);
  }

  .detalles-operativos {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
    font-size: 0.78rem;
    color: var(--color-texto-grey);
  }

  .detalles-operativos span {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }

  .franja-clasificacion {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .bloque-conteo {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 11px 14px;
    border-radius: 16px;
    background: var(--color-fondo-soft);
    border: 1px solid transparent;
  }

  .bloque-conteo.aceptados {
    background: rgba(var(--vs-primary-raw), 0.08);
  }

  .bloque-conteo.rechazados {
    background: rgba(239, 68, 68, 0.06);
  }

  .bloque-conteo div {
    display: flex;
    flex-direction: column;
  }

  .bloque-conteo strong {
    font-size: 0.86rem;
    color: var(--color-texto-dark);
  }

  .bloque-conteo.aceptados strong {
    color: #15803d;
  }

  .bloque-conteo.rechazados strong {
    color: #dc2626;
  }

  .bloque-conteo span {
    font-size: 0.7rem;
    color: var(--color-texto-grey);
    font-weight: 500;
  }

  .destacado-top-ia {
    background: linear-gradient(135deg, rgba(var(--vs-primary-raw), 0.08) 0%, rgba(248, 250, 252, 0.95) 100%);
    border: 1px solid rgba(var(--vs-primary-raw), 0.22);
    border-left: 3px solid var(--color-primario);
    border-radius: 18px;
    padding: 14px 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .etiqueta-top-ia {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .medalla-top {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.74rem;
    font-weight: 700;
    color: #15803d;
  }

  .link-ver-expediente {
    background: none;
    border: none;
    font-family: inherit;
    font-size: 0.73rem;
    font-weight: 600;
    color: #15803d;
    cursor: pointer;
    padding: 2px 6px;
  }

  .link-ver-expediente:hover {
    text-decoration: underline;
  }

  .cuerpo-top-ia {
    display: flex;
    align-items: flex-start;
    gap: 12px;
  }

  .avatar-top {
    width: 48px;
    height: 48px;
    border-radius: 14px;
    object-fit: cover;
    border: 2px solid #ffffff;
    box-shadow: 0 4px 10px rgba(0, 0, 0, 0.08);
    flex-shrink: 0;
  }

  .info-top {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .nombre-linea {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .nombre-linea strong {
    font-size: 0.88rem;
    color: #2c3e50;
  }

  .chip-apto {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 999px;
    background: #16a34a;
    color: #ffffff;
  }

  .meta-top {
    font-size: 0.73rem;
    color: #64748b;
    font-weight: 500;
  }

  .razon-corta {
    margin: 4px 0 0;
    font-size: 0.75rem;
    line-height: 1.45;
    color: #334155;
    display: -webkit-box;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .previa-candidatos {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .subtitulo-previa {
    font-size: 0.73rem;
    font-weight: 600;
    color: #64748b;
  }

  .lista-chips-candidatos {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-persona {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 6px 11px;
    border-radius: 12px;
    border: 1px solid #e2e8f0;
    background: #f8fafc;
    font-family: inherit;
    font-size: 0.74rem;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .chip-persona:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.06);
  }

  .chip-persona.es-apto {
    border-color: rgba(76, 194, 83, 0.35);
    background: rgba(76, 194, 83, 0.06);
  }

  .chip-persona.es-no-apto {
    border-color: rgba(239, 68, 68, 0.28);
    background: rgba(239, 68, 68, 0.05);
  }

  .punto-estado {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #16a34a;
  }

  .chip-persona.es-no-apto .punto-estado {
    background: #dc2626;
  }

  .nombre-chip {
    font-weight: 600;
    color: #2c3e50;
  }

  .puntaje-chip {
    font-size: 0.68rem;
    font-weight: 700;
    color: #15803d;
    background: rgba(76, 194, 83, 0.16);
    padding: 1px 6px;
    border-radius: 6px;
  }

  .no-apto-chip {
    font-size: 0.68rem;
    font-weight: 700;
    color: #dc2626;
    background: rgba(239, 68, 68, 0.12);
    padding: 1px 6px;
    border-radius: 6px;
  }

  .pie-tarjeta-vacante {
    margin-top: auto;
    padding-top: 14px;
    border-top: 1px solid #f1f5f9;
    display: flex;
    align-items: center;
    justify-content: flex-end;
  }

  .btn-entrar-vacante {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 11px 22px;
    border-radius: 14px;
    border: none;
    background: var(--color-primario);
    color: #ffffff;
    font-family: inherit;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0px 10px 20px -8px rgba(var(--vs-primary-raw), 0.5);
    transition: all 0.25s ease;
  }

  .btn-entrar-vacante:hover {
    transform: translateY(-2px);
    box-shadow: 0px 14px 24px -6px rgba(var(--vs-primary-raw), 0.6);
  }

  @media (max-width: 1100px) {
    .banner-reclutamiento {
      flex-direction: column;
      align-items: flex-start;
    }

    .kpis-globales {
      width: 100%;
      grid-template-columns: repeat(2, 1fr);
    }

    .grilla-vacantes {
      grid-template-columns: 1fr;
    }
  }
</style>
