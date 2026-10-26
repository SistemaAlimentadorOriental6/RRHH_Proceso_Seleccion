<script lang="ts">
  import toast from 'svelte-french-toast';
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    ArrowLeft01Icon,
    ArrowRight01Icon,
    Award01Icon,
    Cancel01Icon,
    CheckmarkCircle02Icon,
    File02Icon,
    PlayIcon,
    SparklesIcon,
    UserCheck01Icon,
    UserMultiple02Icon
  } from '@hugeicons/core-free-icons';
  import {
    obtenerCandidatosAceptadosOrdenadosPorScore,
    obtenerCandidatosRechazadosVacante,
    type CandidatoReclutamiento,
    type VacanteReclutamiento
  } from '../modelos/modelosReclutamiento';

  interface Propiedades {
    vacante: VacanteReclutamiento;
    alVolverAVacantes: () => void;
    alVerExpedienteCandidato: (slugCandidato: string) => void;
  }

  let { vacante, alVolverAVacantes, alVerExpedienteCandidato }: Propiedades = $props();

  type PestanaVistaVacante = 'clasificacion_primaria' | 'ranking_ia' | 'aceptados' | 'rechazados';

  let pestanaActiva = $state<PestanaVistaVacante>('clasificacion_primaria');
  let simulandoAnalisisIA = $state(false);
  let pasoSimulacionIA = $state('');
  let indiceCandidatoAnalizando = $state(-1);

  const candidatosAceptadosOrdenados = $derived(
    obtenerCandidatosAceptadosOrdenadosPorScore(vacante)
  );

  const candidatosRechazados = $derived(
    obtenerCandidatosRechazadosVacante(vacante)
  );

  /**
   * En la vista primaria mostramos primero los Aceptados (Aptos) y luego los Rechazados (No Aptos)
   */
  const candidatosClasificacionPrimaria = $derived([
    ...candidatosAceptadosOrdenados,
    ...candidatosRechazados
  ]);

  const listaCandidatosSegunPestana = $derived.by((): CandidatoReclutamiento[] => {
    if (pestanaActiva === 'aceptados') return candidatosAceptadosOrdenados;
    if (pestanaActiva === 'rechazados') return candidatosRechazados;
    return candidatosClasificacionPrimaria;
  });

  function esperar(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  async function ejecutarSimulacionScoringIA(): Promise<void> {
    if (simulandoAnalisisIA) return;
    simulandoAnalisisIA = true;
    pestanaActiva = 'ranking_ia';

    toast.success(
      `Iniciando análisis IA sobre los ${candidatosAceptadosOrdenados.length} candidatos Aceptados (Aptos)...`
    );

    for (let i = 0; i < candidatosAceptadosOrdenados.length; i++) {
      const c = candidatosAceptadosOrdenados[i];
      indiceCandidatoAnalizando = i;
      pasoSimulacionIA = `Analizando perfil, hoja de vida, RUNT y SIMIT de ${c.nombreCompleto}...`;
      await esperar(650);
    }

    pasoSimulacionIA = 'Ordenando candidatos Aceptados de mayor a menor según Score IA (100 pts)...';
    await esperar(500);

    indiceCandidatoAnalizando = -1;
    pasoSimulacionIA = '';
    simulandoAnalisisIA = false;

    const ganador = candidatosAceptadosOrdenados[0];
    if (ganador && ganador.evaluacionIA) {
      toast.success(
        `Ranking IA completado: #1 ${ganador.nombreCompleto} (${ganador.evaluacionIA.scoreTotal}/100 pts)`
      );
    }
  }
</script>

<div class="contenedor-detalle-vacante">
  <!-- Barra superior de retorno y contexto de la vacante -->
  <header class="encabezado-vacante">
    <div class="columna-izquierda">
      <button type="button" class="btn-volver" onclick={alVolverAVacantes}>
        <HugeiconsIcon icon={ArrowLeft01Icon} size={16} color="#2c3e50" />
        <span>Volver a Vacantes de Reclutamiento</span>
      </button>

      <div class="titular-vacante">
        <div class="chips-superiores">
          <span class="chip-req">{vacante.codigoRequisicion}</span>
          <span class="chip-area">{vacante.areaOperativa}</span>
          <span class="chip-licencia">Requisito: {vacante.licenciaExigida}</span>
        </div>
        <h1>{vacante.titulo}</h1>
        <p class="subtitulo-sede">
          {vacante.sede} • {vacante.tipoContrato} • Salario: <strong>{vacante.salarioReferencia}</strong>
        </p>
      </div>
    </div>

    <div class="columna-derecha">
      <div class="resumen-embudo">
        <div class="item-embudo">
          <span class="num">{vacante.candidatos.length}</span>
          <span class="lbl">Evaluados</span>
        </div>
        <div class="separador-embudo">→</div>
        <div class="item-embudo aptos">
          <span class="num">{candidatosAceptadosOrdenados.length}</span>
          <span class="lbl">Aceptados (Aptos)</span>
        </div>
        <div class="separador-embudo">→</div>
        <div class="item-embudo rechazados">
          <span class="num">{candidatosRechazados.length}</span>
          <span class="lbl">Rechazados</span>
        </div>
      </div>

      <button
        type="button"
        class="btn-simular-ia"
        disabled={simulandoAnalisisIA}
        onclick={ejecutarSimulacionScoringIA}
      >
        <HugeiconsIcon
          icon={simulandoAnalisisIA ? SparklesIcon : PlayIcon}
          size={16}
          color="#ffffff"
        />
        <span>
          {simulandoAnalisisIA
            ? 'Analizando perfiles con IA...'
            : `Analizar ${candidatosAceptadosOrdenados.length} Aceptados con IA`}
        </span>
      </button>
    </div>
  </header>

  {#if simulandoAnalisisIA}
    <div class="barra-progreso-ia">
      <div class="indicador-pulso"></div>
      <span>{pasoSimulacionIA}</span>
    </div>
  {/if}

  <!-- Pestañas de Segmentación de la Vacante -->
  <nav class="barra-pestanas">
    <button
      type="button"
      class="pestana-btn"
      class:activa={pestanaActiva === 'clasificacion_primaria'}
      onclick={() => (pestanaActiva = 'clasificacion_primaria')}
    >
      <HugeiconsIcon icon={UserMultiple02Icon} size={16} color="currentColor" />
      <span>Clasificación Primaria</span>
      <span class="contador-pestana">{vacante.candidatos.length}</span>
    </button>

    <button
      type="button"
      class="pestana-btn pestana-ia"
      class:activa={pestanaActiva === 'ranking_ia'}
      onclick={() => (pestanaActiva = 'ranking_ia')}
    >
      <HugeiconsIcon icon={SparklesIcon} size={16} color="currentColor" />
      <span>Ranking IA (Mayor a Menor)</span>
      <span class="contador-pestana ia">{candidatosAceptadosOrdenados.length}</span>
    </button>

    <button
      type="button"
      class="pestana-btn"
      class:activa={pestanaActiva === 'aceptados'}
      onclick={() => (pestanaActiva = 'aceptados')}
    >
      <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} color="currentColor" />
      <span>Aceptados (Aptos)</span>
      <span class="contador-pestana exito">{candidatosAceptadosOrdenados.length}</span>
    </button>

    <button
      type="button"
      class="pestana-btn"
      class:activa={pestanaActiva === 'rechazados'}
      onclick={() => (pestanaActiva = 'rechazados')}
    >
      <HugeiconsIcon icon={Cancel01Icon} size={16} color="currentColor" />
      <span>Rechazados (No Aptos)</span>
      <span class="contador-pestana peligro">{candidatosRechazados.length}</span>
    </button>
  </nav>

  <!-- SECCIÓN: RANKING IA DE CANDIDATOS ACEPTADOS (MAYOR A MENOR SCORE) -->
  {#if pestanaActiva === 'ranking_ia'}
    <section class="seccion-ranking-ia">
      <div class="cabecera-explicacion-ia">
        <div class="texto-regla-ia">
          <div class="titulo-regla">
            <HugeiconsIcon icon={Award01Icon} size={18} color="#15803d" />
            <h2>Ranking IA ({candidatosAceptadosOrdenados.length} Candidatos Aptos)</h2>
          </div>
          <p>
            Ordenados de mayor a menor puntuación según experiencia, historial normativo y afinidad al perfil.
          </p>
        </div>

        <div class="rubrica-pesos">
          <div class="item-peso">
            <strong>30 pts</strong>
            <span>Experiencia Pesados</span>
          </div>
          <div class="item-peso">
            <strong>30 pts</strong>
            <span>RUNT y SIMIT</span>
          </div>
          <div class="item-peso">
            <strong>20 pts</strong>
            <span>Estabilidad Laboral</span>
          </div>
          <div class="item-peso">
            <strong>20 pts</strong>
            <span>Afinidad Perfil CV</span>
          </div>
        </div>
      </div>

      <div class="lista-ranking-ia">
        {#each candidatosAceptadosOrdenados as candidato, indice (candidato.id)}
          {@const ia = candidato.evaluacionIA}
          {#if ia}
            <article
              class="tarjeta-ranking-ia"
              class:es-primer-lugar={indice === 0}
              class:analizando-ahora={indiceCandidatoAnalizando === indice}
            >
              <!-- Columna Izquierda: Posición y Puntaje Total -->
              <div class="columna-medalla">
                <div class="insignia-posicion" class:oro={indice === 0} class:plata={indice === 1} class:bronce={indice === 2}>
                  <span class="hash">#</span>
                  <span class="numero">{indice + 1}</span>
                </div>

                <div class="circulo-score">
                  <span class="valor-score">{ia.scoreTotal}</span>
                  <span class="sobre-cien">/ 100 pts</span>
                </div>

                <span class="badge-dictamen-ia">{ia.dictamenIA}</span>
              </div>

              <!-- Columna Central: Datos del Candidato, Desglose y "¿Por qué es el indicado?" -->
              <div class="columna-contenido-ia">
                <div class="cabecera-candidato-ranking">
                  <img
                    src={candidato.fotoUrl}
                    alt={candidato.nombreCompleto}
                    class="foto-ranking"
                  />
                  <div class="datos-principales-ranking">
                    <div class="fila-nombre-apto">
                      <h3>{candidato.nombreCompleto}</h3>
                      <span class="tag-clasificacion-apto">
                        <HugeiconsIcon icon={CheckmarkCircle02Icon} size={13} color="#15803d" />
                        ACEPTADO (APTO)
                      </span>
                    </div>
                    <p class="submeta-ranking">
                      C.C. {candidato.cedula} • {candidato.edad} años • {candidato.cargoActualOanterior} •
                      <strong>{candidato.anosExperienciaConduccion} años experiencia</strong> • {candidato.categoriaLicenciaPrincipal}
                    </p>
                    <div class="empresas-previas">
                      <span>Trayectoria en CV:</span>
                      {#each ia.empresasAnteriores as emp}
                        <span class="chip-empresa">{emp}</span>
                      {/each}
                    </div>
                  </div>

                  <button
                    type="button"
                    class="btn-ver-expediente-ranking"
                    onclick={() => alVerExpedienteCandidato(candidato.slug)}
                  >
                    <HugeiconsIcon icon={File02Icon} size={15} color="#2c3e50" />
                    <span>Ver RUNT, EPS, SIMIT y Detalles</span>
                    <HugeiconsIcon icon={ArrowRight01Icon} size={15} color="#2c3e50" />
                  </button>
                </div>

                <!-- Bloque destacado: ¿Por qué esta persona es la indicada? -->
                <div class="caja-por-que-indicado">
                  <div class="encabezado-por-que">
                    <HugeiconsIcon icon={SparklesIcon} size={15} color="#15803d" />
                    <span>¿Por qué {candidato.nombreCompleto} es la persona indicada? (Sustentación IA)</span>
                  </div>
                  <p>{ia.porQueEsElIndicado}</p>
                </div>

                <!-- Barras de Desglose de la Rúbrica (100 puntos) -->
                <div class="grilla-barras-score">
                  <div class="item-barra">
                    <div class="etiqueta-barra">
                      <span>Experiencia en Conducción Pesada</span>
                      <strong>{ia.scoreExperiencia} / 30 pts</strong>
                    </div>
                    <div class="pista-barra">
                      <div
                        class="relleno-barra"
                        style="width: {(ia.scoreExperiencia / 30) * 100}%"
                      ></div>
                    </div>
                  </div>

                  <div class="item-barra">
                    <div class="etiqueta-barra">
                      <span>Normativa RUNT y Récord SIMIT</span>
                      <strong>{ia.scoreConduccionNormativo} / 30 pts</strong>
                    </div>
                    <div class="pista-barra">
                      <div
                        class="relleno-barra"
                        style="width: {(ia.scoreConduccionNormativo / 30) * 100}%"
                      ></div>
                    </div>
                  </div>

                  <div class="item-barra">
                    <div class="etiqueta-barra">
                      <span>Estabilidad y Continuidad Laboral</span>
                      <strong>{ia.scoreEstabilidad} / 20 pts</strong>
                    </div>
                    <div class="pista-barra">
                      <div
                        class="relleno-barra"
                        style="width: {(ia.scoreEstabilidad / 20) * 100}%"
                      ></div>
                    </div>
                  </div>

                  <div class="item-barra">
                    <div class="etiqueta-barra">
                      <span>Afinidad Operativa y Hoja de Vida</span>
                      <strong>{ia.scoreAfinidad} / 20 pts</strong>
                    </div>
                    <div class="pista-barra">
                      <div
                        class="relleno-barra"
                        style="width: {(ia.scoreAfinidad / 20) * 100}%"
                      ></div>
                    </div>
                  </div>
                </div>

                <!-- Fortalezas Clave -->
                <div class="fila-fortalezas">
                  {#each ia.fortalezasClave as fortaleza}
                    <span class="chip-fortaleza">✓ {fortaleza}</span>
                  {/each}
                </div>
              </div>
            </article>
          {/if}
        {/each}
      </div>
    </section>
  {:else}
    <!-- SECCIÓN: INFORMACIÓN PRIMARIA DE CANDIDATOS (MOSTRANDO DE PRIMERAS SI ES APTO O NO) -->
    <section class="seccion-clasificacion-primaria">
      <div class="aviso-clasificacion">
        <div>
          <h2>Clasificación Primaria de Elegibilidad</h2>
          <p>
            Verificación automática de RUNT, EPS (ADRES), SIMIT y antecedentes disciplinarios.
          </p>
        </div>

        <button
          type="button"
          class="btn-ir-ranking-rapido"
          onclick={() => (pestanaActiva = 'ranking_ia')}
        >
          <HugeiconsIcon icon={SparklesIcon} size={15} color="#15803d" />
          <span>Ver Ranking IA ({candidatosAceptadosOrdenados.length} Aptos) →</span>
        </button>
      </div>

      <div class="grilla-tarjetas-primarias">
        {#each listaCandidatosSegunPestana as candidato (candidato.id)}
          {@const clasificacion = candidato.clasificacionPrimaria}
          <article
            class="tarjeta-primaria-candidato"
            class:borde-apto={clasificacion.esApto}
            class:borde-no-apto={!clasificacion.esApto}
          >
            <!-- Franja superior que muestra DE PRIMERAS si es APTO o NO APTO -->
            <div
              class="franja-dictamen-primario"
              class:es-apto={clasificacion.esApto}
              class:es-no-apto={!clasificacion.esApto}
            >
              <div class="estado-principal">
                {#if clasificacion.esApto}
                  <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} color="#15803d" />
                  <span>CANDIDATO APTO (ACEPTADO)</span>
                {:else}
                  <HugeiconsIcon icon={Cancel01Icon} size={18} color="#dc2626" />
                  <span>CANDIDATO NO APTO (RECHAZADO)</span>
                {/if}
              </div>

              {#if clasificacion.esApto && candidato.evaluacionIA}
                <span class="pildora-score-primaria">
                  Puesto #{candidato.evaluacionIA.posicionRanking} • {candidato.evaluacionIA.scoreTotal} pts IA
                </span>
              {:else}
                <span class="pildora-excluido-ia">Excluido de Ranking IA</span>
              {/if}
            </div>

            <!-- Información Primaria del Candidato -->
            <div class="cuerpo-primario">
              <div class="perfil-cabecera">
                <img
                  src={candidato.fotoUrl}
                  alt={candidato.nombreCompleto}
                  class="avatar-primario"
                />
                <div class="textos-perfil">
                  <h3>{candidato.nombreCompleto}</h3>
                  <span class="documento-edad">
                    C.C. {candidato.cedula} • {candidato.edad} años • {candidato.genero} • {candidato.municipioResidencia}
                  </span>
                  <span class="cargo-exp">
                    {candidato.cargoActualOanterior} ({candidato.anosExperienciaConduccion} años exp.)
                  </span>
                </div>
              </div>

              <!-- Resumen rápido de las 4 validaciones clave de clasificación -->
              <div class="matriz-validacion-primaria">
                <div class="celda-val" class:ok={clasificacion.cumpleRunt} class:error={!clasificacion.cumpleRunt}>
                  <span class="etiqueta-val">RUNT / Licencia</span>
                  <strong>{clasificacion.detalleRunt}</strong>
                </div>

                <div class="celda-val" class:ok={clasificacion.cumpleEps} class:error={!clasificacion.cumpleEps}>
                  <span class="etiqueta-val">EPS (ADRES)</span>
                  <strong>{clasificacion.detalleEps}</strong>
                </div>

                <div class="celda-val" class:ok={clasificacion.cumpleSimit} class:error={!clasificacion.cumpleSimit}>
                  <span class="etiqueta-val">SIMIT / Multas</span>
                  <strong>{clasificacion.detalleSimit}</strong>
                </div>

                <div class="celda-val" class:ok={clasificacion.cumpleAntecedentes} class:error={!clasificacion.cumpleAntecedentes}>
                  <span class="etiqueta-val">Antecedentes y Listas</span>
                  <strong>{clasificacion.detalleAntecedentes}</strong>
                </div>
              </div>

              {#if !clasificacion.esApto && clasificacion.motivosRechazo.length > 0}
                <div class="caja-motivos-rechazo">
                  <strong>Causal de Rechazo Primario:</strong>
                  <ul>
                    {#each clasificacion.motivosRechazo as motivo}
                      <li>{motivo}</li>
                    {/each}
                  </ul>
                </div>
              {:else if clasificacion.esApto && candidato.evaluacionIA}
                <div class="resumen-ia-miniatura">
                  <strong>Síntesis IA ({candidato.evaluacionIA.scoreTotal}/100 pts):</strong>
                  <p>{candidato.evaluacionIA.porQueEsElIndicado}</p>
                </div>
              {/if}
            </div>

            <!-- Botón para entrar a la persona y ver todos sus detalles -->
            <footer class="pie-primario">
              <code class="slug-candidato">/{vacante.slug}/{candidato.slug}</code>
              <button
                type="button"
                class="btn-entrar-persona"
                onclick={() => alVerExpedienteCandidato(candidato.slug)}
              >
                <HugeiconsIcon icon={UserCheck01Icon} size={15} color="#ffffff" />
                <span>Entrar a Persona (Ver RUNT, EPS, SIMIT y CV)</span>
                <HugeiconsIcon icon={ArrowRight01Icon} size={15} color="#ffffff" />
              </button>
            </footer>
          </article>
        {/each}
      </div>
    </section>
  {/if}
</div>

<style>
  /* Estética Vuesax (DESING.md): sin bordes duros, sombras suaves, radios amplios, transiciones 0.25s */
  .contenedor-detalle-vacante {
    padding: 26px 32px 48px;
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 1600px;
    margin: 0 auto;
    width: 100%;
    box-sizing: border-box;
  }

  .encabezado-vacante {
    background: #ffffff;
    border-radius: 28px;
    padding: 26px 30px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 24px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.08);
  }

  .columna-izquierda {
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
  }

  .btn-volver {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    align-self: flex-start;
    background: var(--color-fondo-soft);
    border-radius: var(--radio-boton);
    padding: 9px 16px;
    font-family: inherit;
    font-size: 0.76rem;
    font-weight: 600;
    color: var(--color-texto-dark);
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .btn-volver:hover {
    background: #ffffff;
    transform: translateX(-3px);
    box-shadow: 0 8px 18px -8px rgba(44, 62, 80, 0.18);
  }

  .btn-volver:active {
    transform: scale(0.95);
  }

  .titular-vacante {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .chips-superiores {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-req,
  .chip-area,
  .chip-licencia {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 5px 12px;
    border-radius: 10px;
  }

  .chip-req {
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-weight: 700;
  }

  .chip-area {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: #15803d;
  }

  .chip-licencia {
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
  }

  .titular-vacante h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--color-texto-dark);
  }

  .subtitulo-sede {
    margin: 0;
    font-size: 0.82rem;
    color: var(--color-texto-grey);
  }

  .subtitulo-sede strong {
    color: var(--color-texto-dark);
    font-weight: 600;
  }

  .columna-derecha {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 14px;
    flex-shrink: 0;
  }

  .resumen-embudo {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--color-fondo-soft);
    padding: 10px 18px;
    border-radius: 20px;
  }

  .item-embudo {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 74px;
  }

  .item-embudo .num {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .item-embudo.aptos .num {
    color: #15803d;
  }

  .item-embudo.rechazados .num {
    color: #dc2626;
  }

  .item-embudo .lbl {
    font-size: 0.68rem;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .separador-embudo {
    color: var(--color-texto-grey);
    font-weight: 700;
    opacity: 0.6;
  }

  .btn-simular-ia {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 12px 20px;
    border-radius: var(--radio-boton);
    background: var(--color-primario);
    color: #ffffff;
    font-family: inherit;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.5);
    transition: var(--transicion-vuesax);
  }

  .btn-simular-ia:hover:not(:disabled) {
    background: var(--color-primario-hover);
    transform: translateY(-3px);
    box-shadow: 0 10px 20px -5px rgba(var(--vs-primary-raw), 0.5);
  }

  .btn-simular-ia:active:not(:disabled) {
    transform: scale(0.95);
  }

  .btn-simular-ia:disabled {
    opacity: 0.75;
    cursor: wait;
  }

  .barra-progreso-ia {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 22px;
    border-radius: 20px;
    background: rgba(var(--vs-primary-raw), 0.1);
    box-shadow: 0 10px 24px -14px rgba(var(--vs-primary-raw), 0.5);
    color: #15803d;
    font-size: 0.83rem;
    font-weight: 600;
  }

  .indicador-pulso {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--color-primario);
    box-shadow: 0 0 10px rgba(var(--vs-primary-raw), 0.7);
    animation: pulsoIA 1s infinite ease-in-out;
  }

  @keyframes pulsoIA {
    0%,
    100% {
      transform: scale(1);
      opacity: 1;
    }
    50% {
      transform: scale(1.4);
      opacity: 0.5;
    }
  }

  /* Pestañas */
  .barra-pestanas {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .pestana-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 12px 20px;
    border-radius: 16px;
    background: #ffffff;
    color: var(--color-texto-grey);
    font-family: inherit;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0 6px 18px -10px rgba(44, 62, 80, 0.12);
    transition: var(--transicion-vuesax);
  }

  .pestana-btn:hover {
    color: var(--color-texto-dark);
    transform: translateY(-2px);
    box-shadow: 0 12px 22px -10px rgba(44, 62, 80, 0.2);
  }

  .pestana-btn.activa {
    background: var(--color-texto-dark);
    color: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(44, 62, 80, 0.55);
  }

  .pestana-btn.pestana-ia.activa {
    background: var(--color-primario);
    color: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.6);
  }

  .contador-pestana {
    padding: 2px 9px;
    border-radius: 999px;
    font-size: 0.7rem;
    font-weight: 700;
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
  }

  .contador-pestana.exito {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .contador-pestana.ia {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .contador-pestana.peligro {
    background: rgba(239, 68, 68, 0.12);
    color: #dc2626;
  }

  .pestana-btn.activa .contador-pestana {
    background: rgba(255, 255, 255, 0.22);
    color: #ffffff;
  }

  /* Ranking IA */
  .seccion-ranking-ia {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .cabecera-explicacion-ia {
    background: #ffffff;
    border-radius: 28px;
    padding: 22px 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
    box-shadow: 0 20px 40px -22px rgba(var(--vs-primary-raw), 0.35);
  }

  .texto-regla-ia {
    max-width: 720px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .titulo-regla {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .titulo-regla h2 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .texto-regla-ia p {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.55;
    color: var(--color-texto-grey);
  }

  .rubrica-pesos {
    display: grid;
    grid-template-columns: repeat(4, minmax(95px, 1fr));
    gap: 10px;
    flex-shrink: 0;
  }

  .item-peso {
    background: var(--color-fondo-soft);
    border-radius: 18px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    transition: var(--transicion-vuesax);
  }

  .item-peso:hover {
    transform: translateY(-2px);
    background: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(44, 62, 80, 0.15);
  }

  .item-peso strong {
    font-size: 0.95rem;
    color: #15803d;
  }

  .item-peso span {
    font-size: 0.68rem;
    color: var(--color-texto-grey);
    font-weight: 500;
  }

  .lista-ranking-ia {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .tarjeta-ranking-ia {
    background: #ffffff;
    border-radius: 28px;
    padding: 24px;
    display: flex;
    gap: 24px;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.07);
    transition: var(--transicion-vuesax);
  }

  .tarjeta-ranking-ia:hover {
    transform: translateY(-3px);
    box-shadow: 0 20px 40px -14px rgba(44, 62, 80, 0.12);
  }

  .tarjeta-ranking-ia.es-primer-lugar {
    box-shadow:
      0 0 0 2px rgba(var(--vs-primary-raw), 0.35),
      0 24px 44px -20px rgba(var(--vs-primary-raw), 0.4);
  }

  .tarjeta-ranking-ia.analizando-ahora {
    transform: scale(1.01);
    box-shadow:
      0 0 0 4px rgba(var(--vs-primary-raw), 0.22),
      0 24px 44px -16px rgba(var(--vs-primary-raw), 0.45);
  }

  .columna-medalla {
    width: 142px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 18px 12px;
    background: var(--color-fondo-soft);
    border-radius: 22px;
  }

  .insignia-posicion {
    display: inline-flex;
    align-items: baseline;
    justify-content: center;
    padding: 6px 16px;
    border-radius: 999px;
    background: #ffffff;
    color: var(--color-texto-dark);
    font-weight: 700;
    box-shadow: 0 6px 14px -8px rgba(44, 62, 80, 0.3);
  }

  .insignia-posicion.oro {
    background: var(--color-primario);
    color: #ffffff;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.7);
  }

  .insignia-posicion.plata {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .insignia-posicion.bronce {
    background: rgba(var(--vs-primary-raw), 0.08);
    color: #15803d;
  }

  .insignia-posicion .hash {
    font-size: 0.78rem;
  }

  .insignia-posicion .numero {
    font-size: 1.2rem;
  }

  .circulo-score {
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .valor-score {
    font-size: 2rem;
    font-weight: 700;
    color: var(--color-texto-dark);
    line-height: 1;
  }

  .sobre-cien {
    font-size: 0.72rem;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .badge-dictamen-ia {
    font-size: 0.68rem;
    font-weight: 700;
    text-align: center;
    padding: 4px 11px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .columna-contenido-ia {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;
  }

  .cabecera-candidato-ranking {
    display: flex;
    align-items: flex-start;
    gap: 14px;
    flex-wrap: wrap;
  }

  .foto-ranking {
    width: 60px;
    height: 60px;
    border-radius: 18px;
    object-fit: cover;
    box-shadow: 0 8px 18px -8px rgba(44, 62, 80, 0.35);
    flex-shrink: 0;
  }

  .datos-principales-ranking {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 240px;
  }

  .fila-nombre-apto {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .fila-nombre-apto h3 {
    margin: 0;
    font-size: 1.08rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .tag-clasificacion-apto {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 4px 10px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.14);
    color: #15803d;
  }

  .submeta-ranking {
    margin: 0;
    font-size: 0.78rem;
    color: var(--color-texto-grey);
  }

  .submeta-ranking strong {
    color: var(--color-texto-dark);
    font-weight: 600;
  }

  .empresas-previas {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 2px;
    font-size: 0.72rem;
    color: var(--color-texto-grey);
  }

  .chip-empresa {
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    padding: 3px 10px;
    border-radius: 10px;
    font-weight: 600;
  }

  .btn-ver-expediente-ranking {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 10px 16px;
    border-radius: var(--radio-boton);
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-family: inherit;
    font-size: 0.76rem;
    font-weight: 600;
    cursor: pointer;
    transition: var(--transicion-vuesax);
    flex-shrink: 0;
  }

  .btn-ver-expediente-ranking:hover {
    background: #ffffff;
    transform: translateY(-2px);
    box-shadow: 0 10px 20px -8px rgba(var(--vs-primary-raw), 0.35);
  }

  .btn-ver-expediente-ranking:active {
    transform: scale(0.95);
  }

  .caja-por-que-indicado {
    background: linear-gradient(135deg, rgba(var(--vs-primary-raw), 0.1) 0%, var(--color-fondo-soft) 100%);
    border-radius: 20px;
    padding: 15px 18px;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }

  .encabezado-por-que {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.78rem;
    font-weight: 700;
    color: #15803d;
  }

  .caja-por-que-indicado p {
    margin: 0;
    font-size: 0.81rem;
    line-height: 1.6;
    color: var(--color-texto-navy);
  }

  .grilla-barras-score {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  .item-barra {
    background: var(--color-fondo-soft);
    padding: 12px 14px;
    border-radius: 18px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .etiqueta-barra {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    font-size: 0.7rem;
    color: var(--color-texto-grey);
  }

  .etiqueta-barra strong {
    color: var(--color-texto-dark);
    white-space: nowrap;
  }

  .pista-barra {
    height: 8px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.14);
    overflow: hidden;
  }

  .relleno-barra {
    height: 100%;
    border-radius: 999px;
    background: var(--color-primario);
    box-shadow: 0 0 10px rgba(var(--vs-primary-raw), 0.5);
    transition: width 0.6s ease;
  }

  .fila-fortalezas {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip-fortaleza {
    font-size: 0.72rem;
    font-weight: 600;
    color: #15803d;
    background: rgba(var(--vs-primary-raw), 0.1);
    padding: 5px 12px;
    border-radius: 12px;
  }

  /* Clasificación primaria */
  .seccion-clasificacion-primaria {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .aviso-clasificacion {
    background: #ffffff;
    border-radius: 24px;
    padding: 20px 26px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 18px;
    flex-wrap: wrap;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.07);
  }

  .aviso-clasificacion h2 {
    margin: 0 0 4px;
    font-size: 1rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .aviso-clasificacion p {
    margin: 0;
    font-size: 0.79rem;
    color: var(--color-texto-grey);
  }

  .btn-ir-ranking-rapido {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 11px 18px;
    border-radius: var(--radio-boton);
    background: rgba(var(--vs-primary-raw), 0.12);
    color: #15803d;
    font-family: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    cursor: pointer;
    flex-shrink: 0;
    transition: var(--transicion-vuesax);
  }

  .btn-ir-ranking-rapido:hover {
    background: rgba(var(--vs-primary-raw), 0.2);
    transform: translateY(-2px);
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.5);
  }

  .btn-ir-ranking-rapido:active {
    transform: scale(0.95);
  }

  .grilla-tarjetas-primarias {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(520px, 1fr));
    gap: 24px;
  }

  .tarjeta-primaria-candidato {
    background: #ffffff;
    border-radius: 28px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    box-shadow: 0 10px 30px -10px rgba(44, 62, 80, 0.07);
    transition: var(--transicion-vuesax);
  }

  .tarjeta-primaria-candidato:hover {
    transform: translateY(-3px);
    box-shadow: 0 22px 44px -16px rgba(44, 62, 80, 0.14);
  }

  .tarjeta-primaria-candidato.borde-apto:hover {
    box-shadow: 0 22px 44px -16px rgba(var(--vs-primary-raw), 0.4);
  }

  .tarjeta-primaria-candidato.borde-no-apto:hover {
    box-shadow: 0 22px 44px -16px rgba(239, 68, 68, 0.3);
  }

  .franja-dictamen-primario {
    padding: 14px 24px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    font-size: 0.76rem;
    font-weight: 700;
  }

  .franja-dictamen-primario.es-apto {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: #15803d;
  }

  .franja-dictamen-primario.es-no-apto {
    background: rgba(239, 68, 68, 0.09);
    color: #dc2626;
  }

  .estado-principal {
    display: flex;
    align-items: center;
    gap: 8px;
    letter-spacing: 0.01em;
  }

  .pildora-score-primaria {
    background: var(--color-primario);
    color: #ffffff;
    padding: 5px 14px;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 700;
    box-shadow: 0 8px 16px -8px rgba(var(--vs-primary-raw), 0.7);
  }

  .pildora-excluido-ia {
    background: rgba(239, 68, 68, 0.14);
    color: #b91c1c;
    padding: 5px 14px;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 700;
  }

  .cuerpo-primario {
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    flex: 1;
  }

  .perfil-cabecera {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .avatar-primario {
    width: 62px;
    height: 62px;
    border-radius: 20px;
    object-fit: cover;
    box-shadow: 0 10px 20px -8px rgba(44, 62, 80, 0.35);
    flex-shrink: 0;
  }

  .textos-perfil {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }

  .textos-perfil h3 {
    margin: 0;
    font-size: 1.12rem;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .documento-edad {
    font-size: 0.77rem;
    color: var(--color-texto-grey);
  }

  .cargo-exp {
    font-size: 0.78rem;
    font-weight: 600;
    color: #15803d;
  }

  .matriz-validacion-primaria {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    margin: 2px 0;
  }

  .celda-val {
    padding: 13px 16px;
    border-radius: 18px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: var(--color-fondo-soft);
    transition: var(--transicion-vuesax);
  }

  .celda-val:hover {
    transform: translateY(-2px);
  }

  .celda-val.ok {
    background: rgba(var(--vs-primary-raw), 0.08);
  }

  .celda-val.error {
    background: rgba(239, 68, 68, 0.07);
  }

  .etiqueta-val {
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-texto-grey);
  }

  .celda-val.ok .etiqueta-val {
    color: #15803d;
  }

  .celda-val.error .etiqueta-val {
    color: #dc2626;
  }

  .celda-val strong {
    font-size: 0.78rem;
    color: var(--color-texto-dark);
    line-height: 1.45;
    font-weight: 600;
  }

  .celda-val.error strong {
    color: #b91c1c;
  }

  .caja-motivos-rechazo {
    background: rgba(239, 68, 68, 0.07);
    border-radius: 18px;
    padding: 14px 18px;
    font-size: 0.78rem;
    color: #991b1b;
  }

  .caja-motivos-rechazo ul {
    margin: 6px 0 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .resumen-ia-miniatura {
    background: linear-gradient(135deg, rgba(var(--vs-primary-raw), 0.1) 0%, var(--color-fondo-soft) 100%);
    border-radius: 18px;
    padding: 14px 18px;
    font-size: 0.79rem;
    color: var(--color-texto-dark);
  }

  .resumen-ia-miniatura strong {
    color: #15803d;
    display: block;
    margin-bottom: 3px;
  }

  .resumen-ia-miniatura p {
    margin: 0;
    line-height: 1.55;
    display: -webkit-box;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .pie-primario {
    padding: 16px 24px;
    background: var(--color-fondo-soft);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    flex-wrap: wrap;
  }

  .slug-candidato {
    font-size: 0.71rem;
    color: var(--color-texto-grey);
    overflow-wrap: anywhere;
  }

  .btn-entrar-persona {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 11px 20px;
    border-radius: var(--radio-boton);
    background: var(--color-primario);
    color: #ffffff;
    font-family: inherit;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    box-shadow: 0 10px 20px -10px rgba(var(--vs-primary-raw), 0.5);
    transition: var(--transicion-vuesax);
  }

  .btn-entrar-persona:hover {
    background: var(--color-primario-hover);
    transform: translateY(-3px);
    box-shadow: 0 10px 20px -5px rgba(var(--vs-primary-raw), 0.5);
  }

  .btn-entrar-persona:active {
    transform: scale(0.95);
  }

  @media (max-width: 1240px) {
    .grilla-tarjetas-primarias {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 1100px) {
    .encabezado-vacante,
    .cabecera-explicacion-ia {
      flex-direction: column;
      align-items: flex-start;
    }

    .columna-derecha {
      align-items: flex-start;
      width: 100%;
    }

    .tarjeta-ranking-ia {
      flex-direction: column;
    }

    .columna-medalla {
      width: 100%;
      flex-direction: row;
      justify-content: space-between;
    }

    .grilla-barras-score {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 600px) {
    .contenedor-detalle-vacante {
      padding: 16px 16px 32px;
    }

    .matriz-validacion-primaria,
    .grilla-barras-score,
    .rubrica-pesos {
      grid-template-columns: 1fr;
    }

    .grilla-tarjetas-primarias {
      grid-template-columns: 1fr;
    }
  }
</style>
