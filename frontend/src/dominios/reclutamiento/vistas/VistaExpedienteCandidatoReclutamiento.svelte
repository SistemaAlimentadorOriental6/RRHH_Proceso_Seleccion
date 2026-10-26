<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    Alert02Icon,
    ArrowLeft01Icon,
    Award01Icon,
    Cancel01Icon,
    Car01Icon,
    CheckmarkCircle02Icon,
    File02Icon,
    Legal01Icon,
    SecurityCheckIcon,
    SparklesIcon,
    UserCheck01Icon,
    ViewIcon
  } from '@hugeicons/core-free-icons';
  import type {
    CandidatoReclutamiento,
    VacanteReclutamiento
  } from '../modelos/modelosReclutamiento';

  interface Propiedades {
    vacante: VacanteReclutamiento;
    candidato: CandidatoReclutamiento;
    alVolverAVacante: () => void;
    alCambiarCandidatoEnVacante: (slugCandidato: string) => void;
  }

  let {
    vacante,
    candidato,
    alVolverAVacante,
    alCambiarCandidatoEnVacante
  }: Propiedades = $props();

  let mostrarModalFoto = $state(false);

  const clasificacion = $derived(candidato.clasificacionPrimaria);
  const runt = $derived(candidato.expediente.runt);
  const adres = $derived(candidato.expediente.adres);
  const simit = $derived(candidato.expediente.simit);
  const antecedentes = $derived(candidato.expediente.antecedentes);
  const ia = $derived(candidato.evaluacionIA);
</script>

<div class="contenedor-expediente">
  <!-- Barra superior de navegación entre personas de la misma vacante -->
  <div class="barra-superior-expediente">
    <button type="button" class="btn-volver-vacante" onclick={alVolverAVacante}>
      <HugeiconsIcon icon={ArrowLeft01Icon} size={16} color="#2c3e50" />
      <span>Volver a {vacante.titulo}</span>
    </button>

    <div class="selector-rapido-personas">
      <span class="lbl-selector">Cambiar persona en esta vacante:</span>
      <div class="pildoras-personas">
        {#each vacante.candidatos as item (item.id)}
          <button
            type="button"
            class="pildora-persona"
            class:activa={item.slug === candidato.slug}
            class:apto={item.clasificacionPrimaria.esApto}
            class:no-apto={!item.clasificacionPrimaria.esApto}
            onclick={() => alCambiarCandidatoEnVacante(item.slug)}
          >
            <span class="punto"></span>
            <span>{item.nombreCompleto.split(' ').slice(0, 2).join(' ')}</span>
          </button>
        {/each}
      </div>
    </div>
  </div>

  <!-- Tarjeta Principal de Identidad y Clasificación Primaria -->
  <section
    class="cabecera-identidad"
    class:es-apto={clasificacion.esApto}
    class:es-no-apto={!clasificacion.esApto}
  >
    <div class="bloque-avatar">
      <button
        type="button"
        class="contenedor-foto-click"
        onclick={() => (mostrarModalFoto = true)}
        title="Ampliar fotografía del candidato"
      >
        <img src={candidato.fotoUrl} alt={candidato.nombreCompleto} />
        <span class="badge-zoom">
          <HugeiconsIcon icon={ViewIcon} size={13} color="#ffffff" />
          Ampliar Foto
        </span>
      </button>
    </div>

    <div class="bloque-datos-personales">
      <div class="fila-badges-estado">
        {#if clasificacion.esApto}
          <span class="insignia-estado-grande apto">
            <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} color="#15803d" />
            CLASIFICACIÓN PRIMARIA: ACEPTADO (APTO)
          </span>
        {:else}
          <span class="insignia-estado-grande no-apto">
            <HugeiconsIcon icon={Cancel01Icon} size={16} color="#dc2626" />
            CLASIFICACIÓN PRIMARIA: RECHAZADO (NO APTO)
          </span>
        {/if}

        {#if ia}
          <span class="insignia-ranking-ia">
            <HugeiconsIcon icon={Award01Icon} size={15} color="#15803d" />
            Puesto #{ia.posicionRanking} en Ranking IA • {ia.scoreTotal}/100 pts ({ia.dictamenIA})
          </span>
        {/if}
      </div>

      <h1>{candidato.nombreCompleto}</h1>
      <p class="subtitulo-cargo">
        {candidato.cargoActualOanterior} • <strong>{candidato.anosExperienciaConduccion} años de experiencia</strong> •
        Postulado a: <strong>{vacante.titulo}</strong>
      </p>

      <div class="grilla-ficha-primaria">
        <div class="dato-ficha">
          <span>Documento (C.C.)</span>
          <strong>{candidato.cedula}</strong>
        </div>
        <div class="dato-ficha">
          <span>Edad y Género</span>
          <strong>{candidato.edad} años • {candidato.genero}</strong>
        </div>
        <div class="dato-ficha">
          <span>Licencia Principal</span>
          <strong>{candidato.categoriaLicenciaPrincipal}</strong>
        </div>
        <div class="dato-ficha">
          <span>Residencia y Contacto</span>
          <strong>{candidato.municipioResidencia} • {candidato.telefono}</strong>
        </div>
      </div>
    </div>
  </section>

  <!-- Si es Aceptado (Apto): Tarjeta de Evaluación IA y "¿Por qué es el indicado?" -->
  {#if clasificacion.esApto && ia}
    <section class="panel-evaluacion-ia-detalle">
      <div class="cabecera-panel-ia">
        <div class="titulo-ia">
          <HugeiconsIcon icon={SparklesIcon} size={20} color="#15803d" />
          <div>
            <h2>Dictamen de Inteligencia Artificial y Score de Selección</h2>
            <span>Modelo: {ia.modeloUtilizado} • Tiempo de análisis: {ia.tiempoInferenciaSegundos}s</span>
          </div>
        </div>

        <div class="marcador-score-total">
          <span class="puesto">Ranking #{ia.posicionRanking}</span>
          <div class="puntaje">
            <strong>{ia.scoreTotal}</strong>
            <span>/ 100 pts</span>
          </div>
        </div>
      </div>

      <div class="bloque-justificacion-ia">
        <h3>¿Por qué {candidato.nombreCompleto} es la persona indicada para esta vacante?</h3>
        <p>{ia.porQueEsElIndicado}</p>
      </div>

      <div class="grilla-desglose-ia">
        <div class="tarjeta-rubrica">
          <div class="top-rubrica">
            <span>Experiencia Pesados</span>
            <strong>{ia.scoreExperiencia} / 30 pts</strong>
          </div>
          <div class="barra"><div style="width: {(ia.scoreExperiencia / 30) * 100}%"></div></div>
        </div>

        <div class="tarjeta-rubrica">
          <div class="top-rubrica">
            <span>Normativa RUNT / SIMIT</span>
            <strong>{ia.scoreConduccionNormativo} / 30 pts</strong>
          </div>
          <div class="barra"><div style="width: {(ia.scoreConduccionNormativo / 30) * 100}%"></div></div>
        </div>

        <div class="tarjeta-rubrica">
          <div class="top-rubrica">
            <span>Estabilidad Laboral</span>
            <strong>{ia.scoreEstabilidad} / 20 pts</strong>
          </div>
          <div class="barra"><div style="width: {(ia.scoreEstabilidad / 20) * 100}%"></div></div>
        </div>

        <div class="tarjeta-rubrica">
          <div class="top-rubrica">
            <span>Afinidad Perfil y CV</span>
            <strong>{ia.scoreAfinidad} / 20 pts</strong>
          </div>
          <div class="barra"><div style="width: {(ia.scoreAfinidad / 20) * 100}%"></div></div>
        </div>
      </div>

      <div class="columnas-cv-ia">
        <div class="caja-info-ia">
          <h4>Fortalezas Identificadas por la IA</h4>
          <ul>
            {#each ia.fortalezasClave as f}
              <li>{f}</li>
            {/each}
          </ul>
        </div>

        <div class="caja-info-ia">
          <h4>Trayectoria, Nivel Educativo y Observaciones</h4>
          <p><strong>Nivel educativo:</strong> {ia.nivelEducativo}</p>
          <p><strong>Empresas anteriores:</strong> {ia.empresasAnteriores.join(' • ')}</p>
          <p><strong>Observación operativa:</strong> {ia.observacionesRiesgo}</p>
        </div>
      </div>
    </section>
  {:else if !clasificacion.esApto}
    <!-- Si es Rechazado (No Apto): Alerta detallada del bloqueo primario -->
    <section class="panel-rechazo-primario">
      <div class="icono-rechazo">
        <HugeiconsIcon icon={Alert02Icon} size={24} color="#dc2626" />
      </div>
      <div class="texto-rechazo">
        <h2>Candidato Rechazado en Clasificación Primaria (Excluido del Ranking IA)</h2>
        <p>
          Esta persona no pasó a la fase de evaluación por Score IA debido a los siguientes incumplimientos obligatorios:
        </p>
        <ul>
          {#each clasificacion.motivosRechazo as motivo}
            <li>{motivo}</li>
          {/each}
        </ul>
      </div>
    </section>
  {/if}

  <!-- GRILLA DE DETALLES TÉCNICOS: RUNT, EPS (ADRES), SIMIT Y ANTECEDENTES -->
  <div class="grilla-expediente-oficial">
    <!-- 1. DETALLE RUNT Y LICENCIAS DE CONDUCCIÓN -->
    <article class="tarjeta-entidad">
      <header class="cabecera-entidad">
        <div class="icono-titulo">
          <div class="caja-icono" class:ok={clasificacion.cumpleRunt} class:err={!clasificacion.cumpleRunt}>
            <HugeiconsIcon icon={Car01Icon} size={18} color="currentColor" />
          </div>
          <div>
            <h3>RUNT — Registro Único Nacional de Tránsito</h3>
            <span>Validación de licencias C2/C3, antigüedad y estado del conductor</span>
          </div>
        </div>
        <span class="chip-veredicto" class:ok={clasificacion.cumpleRunt} class:err={!clasificacion.cumpleRunt}>
          {clasificacion.cumpleRunt ? 'CUMPLE RUNT' : 'NO CUMPLE RUNT'}
        </span>
      </header>

      <div class="cuerpo-entidad">
        <div class="pares-datos">
          <div>
            <span>Nro. Inscripción RUNT</span>
            <strong>{runt.nroInscripcionRunt} ({runt.fechaInscripcion})</strong>
          </div>
          <div>
            <span>Estado Conductor</span>
            <strong>{runt.estadoConductor}</strong>
          </div>
          <div>
            <span>Estado Paz y Salvo</span>
            <strong>{runt.estadoPazYSalvo} ({runt.numeroPazYSalvo})</strong>
          </div>
          <div>
            <span>Restricciones</span>
            <strong>{runt.restriccionesConductor}</strong>
          </div>
        </div>

        <div class="tabla-contenedor">
          <h4>Licencias de Conducción Registradas</h4>
          <table class="tabla-limpia">
            <thead>
              <tr>
                <th>Nro. Licencia</th>
                <th>Categoría</th>
                <th>Expedición</th>
                <th>Vencimiento</th>
                <th>Antigüedad</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {#each runt.licencias as lic}
                <tr>
                  <td><code>{lic.numeroLicencia}</code></td>
                  <td><strong class="badge-cat">{lic.categoria}</strong></td>
                  <td>{lic.fechaExpedicion}</td>
                  <td>{lic.fechaVencimiento}</td>
                  <td>{lic.antiguedadAnios} años</td>
                  <td>
                    <span class="estado-lic" class:activa={lic.estado === 'ACTIVA'}>
                      {lic.estado}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </article>

    <!-- 2. DETALLE EPS / ADRES (BDUA) -->
    <article class="tarjeta-entidad">
      <header class="cabecera-entidad">
        <div class="icono-titulo">
          <div class="caja-icono" class:ok={clasificacion.cumpleEps} class:err={!clasificacion.cumpleEps}>
            <HugeiconsIcon icon={UserCheck01Icon} size={18} color="currentColor" />
          </div>
          <div>
            <h3>ADRES / EPS — Afiliación en Salud (BDUA)</h3>
            <span>Verificación de entidad promotora de salud autorizada y estado activo</span>
          </div>
        </div>
        <span class="chip-veredicto" class:ok={clasificacion.cumpleEps} class:err={!clasificacion.cumpleEps}>
          {clasificacion.cumpleEps ? 'EPS AUTORIZADA' : 'EPS NO VÁLIDA'}
        </span>
      </header>

      <div class="cuerpo-entidad">
        <div class="pares-datos">
          <div>
            <span>Entidad EPS Registrada</span>
            <strong>{adres.entidadEps}</strong>
          </div>
          <div>
            <span>Estado de Afiliación</span>
            <strong>{adres.estadoAfiliacion}</strong>
          </div>
          <div>
            <span>Régimen y Tipo de Afiliado</span>
            <strong>{adres.regimen} • {adres.tipoAfiliado}</strong>
          </div>
          <div>
            <span>Fecha Afiliación Efectiva</span>
            <strong>{adres.fechaAfiliacionEfectiva} ({adres.departamentoMunicipio})</strong>
          </div>
        </div>

        <div class="nota-entidad" class:alerta={!clasificacion.cumpleEps}>
          <strong>Dictamen de Nómina SAO6:</strong> {clasificacion.detalleEps}
        </div>
      </div>
    </article>

    <!-- 3. DETALLE SIMIT — MULTAS, COMPARENDOS Y ACUERDOS DE PAGO -->
    <article class="tarjeta-entidad">
      <header class="cabecera-entidad">
        <div class="icono-titulo">
          <div class="caja-icono" class:ok={clasificacion.cumpleSimit} class:err={!clasificacion.cumpleSimit}>
            <HugeiconsIcon icon={Legal01Icon} size={18} color="currentColor" />
          </div>
          <div>
            <h3>SIMIT — Federación Colombiana de Municipios</h3>
            <span>Estado de cuenta nacional de multas, comparendos y acuerdos de pago</span>
          </div>
        </div>
        <span class="chip-veredicto" class:ok={clasificacion.cumpleSimit} class:err={!clasificacion.cumpleSimit}>
          {clasificacion.cumpleSimit ? 'SIMIT APROBADO' : 'RECHAZADO POR MULTAS'}
        </span>
      </header>

      <div class="cuerpo-entidad">
        <div class="pares-datos">
          <div>
            <span>Multas Pendientes</span>
            <strong>{simit.cantidadMultas} multas ({simit.totalDeudaFormateado})</strong>
          </div>
          <div>
            <span>Comparendos Activos</span>
            <strong>{simit.cantidadComparendos} comparendos</strong>
          </div>
          <div>
            <span>Acuerdos de Pago</span>
            <strong>{simit.cantidadAcuerdosPago} acuerdos ({simit.totalEnAcuerdosFormateado})</strong>
          </div>
          <div>
            <span>Regla SAO6 Aplicada</span>
            <strong>Máximo $0 en mora o acuerdo &lt;= $1.000.000 COP</strong>
          </div>
        </div>

        {#if simit.detalleComparendos.length > 0}
          <div class="tabla-contenedor">
            <h4>Resoluciones y Acuerdos Registrados en SIMIT</h4>
            <table class="tabla-limpia">
              <thead>
                <tr>
                  <th>Resolución</th>
                  <th>Fecha</th>
                  <th>Código</th>
                  <th>Secretaría</th>
                  <th>Estado</th>
                  <th>Valor</th>
                </tr>
              </thead>
              <tbody>
                {#each simit.detalleComparendos as item}
                  <tr>
                    <td><code>{item.numeroResolucion}</code></td>
                    <td>{item.fechaComparendo}</td>
                    <td><strong>{item.codigoInfraccion}</strong></td>
                    <td>{item.secretariaTransito}</td>
                    <td>{item.estado}</td>
                    <td><strong>{item.valorTotal}</strong></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <div class="nota-entidad">
            <strong>Certificación SIMIT:</strong> {simit.mensajeEstado}
          </div>
        {/if}
      </div>
    </article>

    <!-- 4. ANTECEDENTES DISCIPLINARIOS, FISCALES, JUDICIALES Y SARLAFT + HOJA DE VIDA -->
    <article class="tarjeta-entidad">
      <header class="cabecera-entidad">
        <div class="icono-titulo">
          <div class="caja-icono ok">
            <HugeiconsIcon icon={SecurityCheckIcon} size={18} color="currentColor" />
          </div>
          <div>
            <h3>Antecedentes, Listas Restrictivas y Hoja de Vida</h3>
            <span>Procuraduría, Contraloría, Policía Nacional, SARLAFT ONU/OFAC y Perfil</span>
          </div>
        </div>
        <span class="chip-veredicto ok">100% VERIFICADO</span>
      </header>

      <div class="cuerpo-entidad">
        <div class="pares-datos">
          <div>
            <span>Procuraduría General (PGN)</span>
            <strong>{antecedentes.procuraduriaEstado} ({antecedentes.procuraduriaCertificado})</strong>
          </div>
          <div>
            <span>Contraloría General (SIBOR)</span>
            <strong>{antecedentes.contraloriaEstado}</strong>
          </div>
          <div>
            <span>Policía Nacional / Judicial</span>
            <strong>{antecedentes.policiaEstado}</strong>
          </div>
          <div>
            <span>SARLAFT ONU y OFAC</span>
            <strong>{antecedentes.sarlaftOnuEstado} • {antecedentes.ofacEstado}</strong>
          </div>
        </div>

        <div class="extracto-cv-caja">
          <div class="encabezado-cv">
            <HugeiconsIcon icon={File02Icon} size={15} color="#2c3e50" />
            <span>Extracto Analizado de la Hoja de Vida y Reintegro SAO6</span>
          </div>
          <p>{candidato.extractoHojaDeVida}</p>
          <span class="meta-reintegro">
            <strong>Historial Reintegro SAO6:</strong> {antecedentes.reintegroSao6Estado}
          </span>
        </div>
      </div>
    </article>
  </div>
</div>

{#if mostrarModalFoto}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="velo-modal-foto" onclick={() => (mostrarModalFoto = false)}>
    <div class="ventana-foto" onclick={(e) => e.stopPropagation()}>
      <img src={candidato.fotoUrl} alt={candidato.nombreCompleto} />
      <div class="pie-ventana-foto">
        <div>
          <strong>{candidato.nombreCompleto}</strong>
          <span>C.C. {candidato.cedula} • {candidato.categoriaLicenciaPrincipal}</span>
        </div>
        <button type="button" onclick={() => (mostrarModalFoto = false)}>Cerrar</button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* Estética Vuesax (DESING.md): sin bordes duros, sombras suaves, radios amplios, transiciones 0.25s */
  .contenedor-expediente {
    padding: 26px 32px 48px;
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 1600px;
    margin: 0 auto;
    width: 100%;
    box-sizing: border-box;
  }

  .barra-superior-expediente {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-wrap: wrap;
    background: #ffffff;
    padding: 14px 20px;
    border-radius: 22px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.07);
  }

  .btn-volver-vacante {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 9px 16px;
    border-radius: var(--radio-boton);
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-family: inherit;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .btn-volver-vacante:hover {
    background: #ffffff;
    transform: translateX(-3px);
    box-shadow: 0 8px 18px -8px rgba(44, 62, 80, 0.18);
  }

  .btn-volver-vacante:active {
    transform: scale(0.95);
  }

  .selector-rapido-personas {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .lbl-selector {
    font-size: 0.74rem;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .pildoras-personas {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .pildora-persona {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 13px;
    border-radius: 12px;
    background: var(--color-fondo-soft);
    font-family: inherit;
    font-size: 0.73rem;
    font-weight: 600;
    color: var(--color-texto-dark);
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .pildora-persona:hover {
    background: #ffffff;
    transform: translateY(-2px);
    box-shadow: 0 8px 18px -8px rgba(44, 62, 80, 0.2);
  }

  .pildora-persona .punto {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-primario);
  }

  .pildora-persona.no-apto .punto {
    background: #ef4444;
  }

  .pildora-persona.activa {
    background: var(--color-primario);
    color: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.6);
  }

  .pildora-persona.activa.no-apto {
    background: #ef4444;
    box-shadow: 0 10px 20px -10px rgba(239, 68, 68, 0.6);
  }

  .pildora-persona.activa .punto {
    background: #ffffff;
  }

  /* Identidad */
  .cabecera-identidad {
    background: #ffffff;
    border-radius: 28px;
    padding: 28px;
    display: flex;
    align-items: center;
    gap: 28px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.08);
    position: relative;
    overflow: hidden;
  }

  .cabecera-identidad::before {
    content: '';
    position: absolute;
    inset: 0 auto 0 0;
    width: 5px;
    background: var(--color-primario);
  }

  .cabecera-identidad.es-no-apto::before {
    background: #ef4444;
  }

  .cabecera-identidad.es-apto {
    box-shadow: 0 20px 40px -18px rgba(var(--vs-primary-raw), 0.3);
  }

  .cabecera-identidad.es-no-apto {
    box-shadow: 0 20px 40px -18px rgba(239, 68, 68, 0.25);
  }

  .contenedor-foto-click {
    position: relative;
    width: 116px;
    height: 116px;
    border-radius: 28px;
    overflow: hidden;
    padding: 0;
    cursor: pointer;
    background: var(--color-fondo-soft);
    box-shadow: 0 12px 24px -10px rgba(44, 62, 80, 0.3);
    transition: var(--transicion-vuesax);
    flex-shrink: 0;
  }

  .contenedor-foto-click:hover {
    transform: translateY(-3px);
    box-shadow: 0 18px 30px -10px rgba(44, 62, 80, 0.35);
  }

  .contenedor-foto-click img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .badge-zoom {
    position: absolute;
    inset: auto 0 0 0;
    background: rgba(44, 62, 80, 0.7);
    backdrop-filter: blur(4px);
    color: #ffffff;
    font-size: 0.65rem;
    font-weight: 600;
    padding: 5px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
  }

  .bloque-datos-personales {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
  }

  .fila-badges-estado {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .insignia-estado-grande {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .insignia-estado-grande.apto {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .insignia-estado-grande.no-apto {
    background: rgba(239, 68, 68, 0.12);
    color: #dc2626;
  }

  .insignia-ranking-ia {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
    border-radius: 999px;
    background: var(--color-fondo-soft);
    color: #15803d;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .bloque-datos-personales h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--color-texto-dark);
  }

  .subtitulo-cargo {
    margin: 0;
    font-size: 0.84rem;
    color: var(--color-texto-grey);
  }

  .subtitulo-cargo strong {
    color: var(--color-texto-dark);
    font-weight: 600;
  }

  .grilla-ficha-primaria {
    margin-top: 6px;
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  .dato-ficha {
    background: var(--color-fondo-soft);
    padding: 12px 16px;
    border-radius: 16px;
    display: flex;
    flex-direction: column;
    gap: 3px;
    transition: var(--transicion-vuesax);
  }

  .dato-ficha:hover {
    transform: translateY(-2px);
    background: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(44, 62, 80, 0.15);
  }

  .dato-ficha span {
    font-size: 0.66rem;
    font-weight: 500;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .dato-ficha strong {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--color-texto-dark);
  }

  /* Panel evaluación IA */
  .panel-evaluacion-ia-detalle {
    background: #ffffff;
    border-radius: 28px;
    padding: 26px 28px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    box-shadow: 0 20px 40px -20px rgba(var(--vs-primary-raw), 0.3);
  }

  .cabecera-panel-ia {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-wrap: wrap;
  }

  .titulo-ia {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .titulo-ia h2 {
    margin: 0;
    font-size: 1.08rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .titulo-ia span {
    font-size: 0.74rem;
    color: var(--color-texto-grey);
  }

  .marcador-score-total {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(var(--vs-primary-raw), 0.1);
    padding: 10px 18px;
    border-radius: 18px;
  }

  .marcador-score-total .puesto {
    font-size: 0.76rem;
    font-weight: 700;
    color: #15803d;
    background: #ffffff;
    padding: 4px 12px;
    border-radius: 999px;
    box-shadow: 0 6px 14px -8px rgba(var(--vs-primary-raw), 0.6);
  }

  .marcador-score-total .puntaje strong {
    font-size: 1.5rem;
    font-weight: 700;
    color: #15803d;
  }

  .marcador-score-total .puntaje span {
    font-size: 0.76rem;
    color: #15803d;
    font-weight: 600;
  }

  .bloque-justificacion-ia {
    background: linear-gradient(135deg, rgba(var(--vs-primary-raw), 0.1) 0%, var(--color-fondo-soft) 100%);
    padding: 16px 20px;
    border-radius: 20px;
  }

  .bloque-justificacion-ia h3 {
    margin: 0 0 6px;
    font-size: 0.86rem;
    font-weight: 700;
    color: #15803d;
  }

  .bloque-justificacion-ia p {
    margin: 0;
    font-size: 0.83rem;
    line-height: 1.6;
    color: var(--color-texto-navy);
  }

  .grilla-desglose-ia {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  .tarjeta-rubrica {
    background: var(--color-fondo-soft);
    padding: 14px 16px;
    border-radius: 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    transition: var(--transicion-vuesax);
  }

  .tarjeta-rubrica:hover {
    transform: translateY(-2px);
    background: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(44, 62, 80, 0.15);
  }

  .top-rubrica {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    font-size: 0.74rem;
    color: var(--color-texto-grey);
  }

  .top-rubrica strong {
    color: var(--color-texto-dark);
    white-space: nowrap;
  }

  .tarjeta-rubrica .barra {
    height: 8px;
    background: rgba(var(--vs-primary-raw), 0.14);
    border-radius: 999px;
    overflow: hidden;
  }

  .tarjeta-rubrica .barra div {
    height: 100%;
    background: var(--color-primario);
    border-radius: 999px;
    box-shadow: 0 0 10px rgba(var(--vs-primary-raw), 0.5);
    transition: width 0.6s ease;
  }

  .columnas-cv-ia {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 14px;
  }

  .caja-info-ia {
    background: var(--color-fondo-soft);
    border-radius: 20px;
    padding: 16px 18px;
    font-size: 0.79rem;
    line-height: 1.5;
    color: var(--color-texto-dark);
  }

  .caja-info-ia h4 {
    margin: 0 0 8px;
    font-size: 0.84rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .caja-info-ia ul {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .caja-info-ia p {
    margin: 4px 0;
  }

  .panel-rechazo-primario {
    background: rgba(239, 68, 68, 0.06);
    border-radius: 28px;
    padding: 22px 26px;
    display: flex;
    align-items: flex-start;
    gap: 16px;
    color: #991b1b;
    box-shadow: 0 20px 40px -22px rgba(239, 68, 68, 0.35);
  }

  .texto-rechazo h2 {
    margin: 0 0 6px;
    font-size: 1.02rem;
    font-weight: 700;
  }

  .texto-rechazo p {
    margin: 0 0 8px;
    font-size: 0.82rem;
  }

  .texto-rechazo ul {
    margin: 0;
    padding-left: 18px;
    font-size: 0.82rem;
    font-weight: 600;
  }

  /* Entidades RUNT / ADRES / SIMIT / Antecedentes */
  .grilla-expediente-oficial {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 20px;
  }

  .tarjeta-entidad {
    background: #ffffff;
    border-radius: 26px;
    padding: 24px 26px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.07);
    display: flex;
    flex-direction: column;
    gap: 18px;
    min-width: 0;
    transition: var(--transicion-vuesax);
  }

  .tarjeta-entidad:hover {
    transform: translateY(-3px);
    box-shadow: 0 20px 40px -14px rgba(44, 62, 80, 0.12);
  }

  .cabecera-entidad {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .icono-titulo {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .caja-icono {
    width: 42px;
    height: 42px;
    border-radius: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .caja-icono.ok {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: #15803d;
  }

  .caja-icono.err {
    background: rgba(239, 68, 68, 0.12);
    color: #dc2626;
  }

  .icono-titulo h3 {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .icono-titulo span {
    font-size: 0.72rem;
    color: var(--color-texto-grey);
  }

  .chip-veredicto {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 5px 12px;
    border-radius: 999px;
    flex-shrink: 0;
  }

  .chip-veredicto.ok {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .chip-veredicto.err {
    background: rgba(239, 68, 68, 0.14);
    color: #dc2626;
  }

  .cuerpo-entidad {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .pares-datos {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
  }

  .pares-datos div {
    background: var(--color-fondo-soft);
    border-radius: 16px;
    padding: 11px 14px;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .pares-datos span {
    font-size: 0.65rem;
    font-weight: 500;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .pares-datos strong {
    font-size: 0.77rem;
    font-weight: 600;
    color: var(--color-texto-dark);
  }

  .tabla-contenedor {
    overflow-x: auto;
  }

  .tabla-contenedor h4 {
    margin: 0 0 10px;
    font-size: 0.78rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .tabla-limpia {
    width: 100%;
    border-collapse: separate;
    border-spacing: 0;
    font-size: 0.74rem;
    background: var(--color-fondo-soft);
    border-radius: 16px;
    overflow: hidden;
  }

  .tabla-limpia th {
    text-align: left;
    padding: 10px 12px;
    color: var(--color-texto-grey);
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .tabla-limpia td {
    padding: 10px 12px;
    color: var(--color-texto-dark);
    transition: var(--transicion-vuesax);
  }

  .tabla-limpia tbody tr:hover td {
    background: #ffffff;
  }

  .tabla-limpia code {
    font-size: 0.72rem;
  }

  .badge-cat {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: #15803d;
    padding: 3px 9px;
    border-radius: 8px;
  }

  .estado-lic {
    font-size: 0.68rem;
    font-weight: 700;
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(239, 68, 68, 0.12);
    color: #dc2626;
  }

  .estado-lic.activa {
    background: rgba(var(--vs-primary-raw), 0.15);
    color: #15803d;
  }

  .nota-entidad {
    background: rgba(var(--vs-primary-raw), 0.08);
    border-radius: 16px;
    padding: 12px 16px;
    font-size: 0.76rem;
    line-height: 1.5;
    color: #166534;
  }

  .nota-entidad.alerta {
    background: rgba(239, 68, 68, 0.07);
    color: #991b1b;
  }

  .extracto-cv-caja {
    background: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .encabezado-cv {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.76rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .extracto-cv-caja p {
    margin: 0;
    font-size: 0.77rem;
    line-height: 1.55;
    color: var(--color-texto-dark);
  }

  .meta-reintegro {
    font-size: 0.74rem;
    color: #15803d;
  }

  /* Modal foto (glassmorphism) */
  .velo-modal-foto {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.3);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 120;
    padding: 20px;
  }

  .ventana-foto {
    background: #ffffff;
    border-radius: var(--radio-modal);
    padding: 20px;
    max-width: 380px;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 16px;
    box-shadow:
      0 25px 50px -12px rgba(0, 0, 0, 0.25),
      0 15px 30px -5px rgba(0, 0, 0, 0.2);
    animation: modalIn 0.4s var(--curva-elastica);
  }

  .ventana-foto img {
    width: 100%;
    border-radius: 20px;
  }

  .pie-ventana-foto {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .pie-ventana-foto div {
    display: flex;
    flex-direction: column;
  }

  .pie-ventana-foto strong {
    font-size: 0.9rem;
    color: var(--color-texto-dark);
  }

  .pie-ventana-foto span {
    font-size: 0.74rem;
    color: var(--color-texto-grey);
  }

  .pie-ventana-foto button {
    padding: 9px 18px;
    border-radius: var(--radio-boton);
    background: var(--color-primario);
    color: #ffffff;
    font-family: inherit;
    font-size: 0.76rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.5);
    transition: var(--transicion-vuesax);
  }

  .pie-ventana-foto button:hover {
    transform: translateY(-3px);
    box-shadow: 0 10px 20px -5px rgba(var(--vs-primary-raw), 0.5);
  }

  .pie-ventana-foto button:active {
    transform: scale(0.95);
  }

  @media (max-width: 1100px) {
    .cabecera-identidad {
      flex-direction: column;
      align-items: flex-start;
    }

    .grilla-ficha-primaria,
    .grilla-desglose-ia,
    .columnas-cv-ia,
    .grilla-expediente-oficial {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 600px) {
    .contenedor-expediente {
      padding: 16px 16px 32px;
    }

    .pares-datos {
      grid-template-columns: 1fr;
    }
  }
</style>
