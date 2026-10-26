<script lang="ts">
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import {
    CheckmarkCircle02Icon,
    Cancel01Icon,
    Alert02Icon,
    WorkflowSquare10Icon,
    File02Icon,
    Download01Icon,
  } from "@hugeicons/core-free-icons";
  import {
    ETIQUETA_ETAPA,
    ORDEN_ETAPAS,
    type CandidatoEnVivo,
  } from "../modelos/analisisEnVivo";
  import { ETIQUETA_VEREDICTO, veredictoDe } from "../servicios/almacenAnalisis.svelte";
  import { urlCaptura } from "../servicios/clienteRrhh";

  interface Propiedades {
    candidato: CandidatoEnVivo | undefined;
    cargando: boolean;
    alVolver: () => void;
    alVerFlujo: () => void;
  }

  let { candidato, cargando, alVolver, alVerFlujo }: Propiedades = $props();

  const r = $derived(candidato?.resultado ?? null);
  const ia = $derived(r?.evaluacion_ia ?? null);
  const veredicto = $derived(candidato ? veredictoDe(candidato) : "en_cola");
  const etapaIa = $derived(candidato?.etapas.ia);

  // Los resultados de cada fuente llegan como objetos abiertos: se leen por nombre con valores por defecto.
  type Mapa = Record<string, unknown>;
  const adres = $derived((r?.adres ?? null) as Mapa | null);
  const runt = $derived((r?.runt ?? null) as Mapa | null);
  const simit = $derived((r?.simit ?? null) as Mapa | null);
  const licencias = $derived((runt?.licencias ?? []) as Mapa[]);

  const texto = (v: unknown): string =>
    v === undefined || v === null || v === "" ? "—" : String(v);
  const categoriasDe = (lic: Mapa): Mapa[] => (lic.detalles_categorias as Mapa[] | undefined) ?? [];
  const siNo = (v: unknown): string => (v === true ? "Sí" : v === false ? "No" : "—");

  const ASPECTOS = $derived([
    { nombre: "Experiencia en transporte", puntos: ia?.score_experiencia ?? 0, max: 30, just: ia?.experiencia?.length ? undefined : ia?.just_experiencia },
    { nombre: "Conducción y normatividad", puntos: ia?.score_conduccion_normativo ?? 0, max: 30, just: ia?.just_normativo },
    { nombre: "Estabilidad laboral", puntos: ia?.score_estabilidad ?? 0, max: 20, just: ia?.just_estabilidad },
    { nombre: "Afinidad al cargo", puntos: ia?.score_afinidad ?? 0, max: 20, just: ia?.just_afinidad },
  ]);

  const documentos = $derived(
    [
      { etiqueta: "Hoja de vida original", ruta: r?.ruta_cv_original },
      { etiqueta: "Hoja de vida (PDF)", ruta: r?.ruta_pdf },
      { etiqueta: "Perfil de Computrabajo (PDF)", ruta: r?.ruta_perfil_pdf },
    ].filter(
        (d, i, todos): d is { etiqueta: string; ruta: string } =>
          Boolean(d.ruta) && todos.findIndex((o) => o.ruta === d.ruta) === i, // el mismo archivo puede ser dos documentos
      ),
  );

  const nivelScore = (total: number) => (total >= 70 ? "alto" : total >= 55 ? "medio" : "bajo");

  function nombreArchivo(ruta: string): string {
    return ruta.split(/[\\/]/).pop() ?? "hoja-de-vida";
  }
</script>

<div class="pagina">
  <div class="barra-acciones">
    <button type="button" class="boton-secundario" onclick={alVolver}>← Volver a la vacante</button>
    {#if candidato}
      <button type="button" class="boton-secundario" onclick={alVerFlujo}>
        <HugeiconsIcon icon={WorkflowSquare10Icon} size={14} strokeWidth={2.2} />
        <span>Ver flujo</span>
      </button>
    {/if}
  </div>

  {#if !candidato}
    <div class="estado-vacio" aria-busy={cargando}>
      {#if cargando}
        <span class="spinner"></span>
        <p>Leyendo los datos del candidato…</p>
      {:else}
        <p>No se encontró a este candidato en la vacante.</p>
      {/if}
    </div>
  {:else}
    <section class="tarjeta cabecera">
      <img class="avatar" src={candidato.fotoUrl} alt={candidato.nombre} />
      <div class="identidad">
        <h2>{candidato.nombre.toUpperCase()}</h2>
        <p>{candidato.cedula ? `C.C. ${candidato.cedula}` : "Sin cédula en perfil"} · Postulado #{candidato.indice}</p>
        <span class="pildora veredicto-{veredicto}">
          {#if veredicto === "apto"}
            <HugeiconsIcon icon={CheckmarkCircle02Icon} size={13} strokeWidth={2.3} />
          {:else if veredicto === "no_apto"}
            <HugeiconsIcon icon={Cancel01Icon} size={13} strokeWidth={2.3} />
          {:else if veredicto === "pendiente"}
            <HugeiconsIcon icon={Alert02Icon} size={13} strokeWidth={2.3} />
          {:else if veredicto === "analizando"}
            <span class="mini-spinner"></span>
          {/if}
          {ETIQUETA_VEREDICTO[veredicto]}
        </span>
      </div>
      {#if ia}
        <div class="anillo nivel-{nivelScore(ia.score_total)}" style="--p: {ia.score_total}" title="Score IA: {ia.score_total} de 100">
          <div class="anillo-interior">
            <strong>{ia.score_total}</strong>
            <span>de 100</span>
          </div>
        </div>
      {/if}
    </section>

    <section class="tarjeta">
      <h3>Evaluación de la IA</h3>
      {#if ia}
        <p class="dictamen nivel-{nivelScore(ia.score_total)}">{ia.dictamen}</p>
        {#if ia.por_que_es_el_hombre}<p class="resumen">{ia.por_que_es_el_hombre}</p>{/if}

        <div class="aspectos">
          {#each ASPECTOS as a (a.nombre)}
            <div class="aspecto">
              <div class="fila-aspecto">
                <span>{a.nombre}</span>
                <strong>{a.puntos} / {a.max}</strong>
              </div>
              <div class="barra"><div class="relleno" style="width: {(a.puntos / a.max) * 100}%"></div></div>
              {#if a.just}<p class="just">{a.just}</p>{/if}
            </div>
          {/each}
        </div>

        <div class="rejilla-dos">
          {#if ia.fortalezas?.length}
            <div>
              <h4>Fortalezas</h4>
              <ul class="lista ok">{#each ia.fortalezas as f}<li>{f}</li>{/each}</ul>
            </div>
          {/if}
          {#if ia.inconsistencias?.length || (ia.alerta && ia.alerta.toLowerCase() !== "ninguna")}
            <div>
              <h4>Alertas e inconsistencias</h4>
              <ul class="lista mal">
                {#if ia.alerta && ia.alerta.toLowerCase() !== "ninguna"}<li>{ia.alerta}</li>{/if}
                {#each ia.inconsistencias ?? [] as i}<li>{i}</li>{/each}
              </ul>
            </div>
          {/if}
        </div>

        {#if ia.experiencia?.length}
          <h4>Experiencia laboral</h4>
          <ol class="experiencia">
            {#each ia.experiencia as e}
              <li>
                <span class="marca"></span>
                <div class="exp-cuerpo">
                  <div class="exp-titulo">
                    <strong>{e.cargo || "Cargo no indicado"}</strong>
                    {#if e.duracion}<span class="chip">{e.duracion}</span>{/if}
                  </div>
                  <p class="exp-empresa">{e.empresa || "Empresa no indicada"}{e.periodo ? ` · ${e.periodo}` : ""}</p>
                  {#if e.descripcion}<p class="exp-desc">{e.descripcion}</p>{/if}
                </div>
              </li>
            {/each}
          </ol>
        {/if}

        <dl class="datos">
          <div><dt>Nivel educativo</dt><dd>{texto(ia.nivel_educativo)}</dd></div>
          <div><dt>Años de experiencia</dt><dd>{texto(ia.anos_experiencia)}</dd></div>
          <div><dt>Habilidades</dt><dd>{ia.habilidades?.join(" · ") || "—"}</dd></div>
          <div><dt>Observaciones</dt><dd>{texto(ia.observaciones)}</dd></div>
        </dl>
      {:else if etapaIa?.estado === "ejecutando" || etapaIa?.estado === "reintentando"}
        <div class="estado-vacio"><span class="spinner"></span><p>La IA está calificando la hoja de vida…</p></div>
      {:else if !r?.es_apto}
        <p class="aviso">
          La IA solo califica a quien cumple todas las validaciones.
          {#if r?.evaluacion_idoneidad?.motivos_descarte?.length}
            Motivo: {r.evaluacion_idoneidad.motivos_descarte.join(" · ")}
          {/if}
        </p>
      {:else}
        <p class="aviso">
          {etapaIa?.estado === "error" ? etapaIa.detalle : "Este candidato es apto pero la IA todavía no lo calificó."}
          Se reintenta al pulsar «Analizar pendientes» en la vacante.
        </p>
      {/if}
    </section>

    <div class="rejilla-dos columnas">
      <section class="tarjeta">
        <h3>Datos del candidato</h3>
        <dl class="datos">
          <div><dt>Género</dt><dd>{texto(r?.evaluacion_idoneidad?.genero ?? r?.genero)}</dd></div>
          <div><dt>Edad</dt><dd>{r?.edad ? `${r.edad} años` : "—"}</dd></div>
          <div><dt>EPS</dt><dd>{texto(adres?.eps)}</dd></div>
          <div><dt>Régimen</dt><dd>{texto(adres?.regimen)}</dd></div>
          <div><dt>Estado de afiliación</dt><dd>{texto(adres?.estado_afiliacion)}</dd></div>
          <div><dt>Tipo de afiliado</dt><dd>{texto(adres?.tipo_afiliado)}</dd></div>
          <div><dt>Ubicación</dt><dd>{adres?.municipio ? `${adres.municipio}, ${texto(adres.departamento)}` : "—"}</dd></div>
          <div><dt>Afiliado desde</dt><dd>{texto(adres?.fecha_afiliacion)}</dd></div>
        </dl>
      </section>

      <section class="tarjeta">
        <h3>RUNT</h3>
        <dl class="datos">
          <div><dt>Inscrito</dt><dd>{siNo(runt?.inscrito)}</dd></div>
          <div><dt>Fecha de inscripción</dt><dd>{texto(runt?.fecha_inscripcion)}</dd></div>
          <div><dt>Estado de la persona</dt><dd>{texto(runt?.estado_persona)}</dd></div>
          <div><dt>Conductor activo</dt><dd>{siNo(runt?.conductor_activo)}</dd></div>
          <div><dt>Categorías activas</dt><dd>{(runt?.categorias_activas as string[] | undefined)?.join(" · ") || "—"}</dd></div>
          <div><dt>Multas en RUNT</dt><dd>{siNo(runt?.tiene_multas)}</dd></div>
          <div><dt>Paz y salvo RUNT</dt><dd>{siNo(runt?.paz_y_salvo)}</dd></div>
        </dl>
        {#each licencias as lic}
          <div class="licencia">
            <strong>Licencia {texto(lic.numero_licencia)}</strong>
            <span class="chip">{texto(lic.estado)}</span>
            <p>
              Expedida {texto(lic.fecha_expedicion)} · {texto(lic.ot_expide)}
              {#each categoriasDe(lic) as d}
                <br />{texto(d.categoria)}: vence {texto(d.fecha_vencimiento)}
              {/each}
            </p>
          </div>
        {/each}
      </section>

      <section class="tarjeta">
        <h3>SIMIT</h3>
        <dl class="datos">
          <div><dt>Estado</dt><dd>{texto(simit?.estado)}</dd></div>
          <div><dt>Paz y salvo</dt><dd>{siNo(simit?.paz_y_salvo)}</dd></div>
          <div><dt>Comparendos</dt><dd>{texto(simit?.cantidad_comparendos ?? 0)}</dd></div>
          <div><dt>Multas</dt><dd>{texto(simit?.cantidad_multas ?? 0)}</dd></div>
          <div><dt>Acuerdos de pago</dt><dd>{texto(simit?.cantidad_acuerdos ?? 0)}</dd></div>
          <div><dt>Deuda total</dt><dd>{texto(simit?.total_deuda ?? "$0")}</dd></div>
        </dl>
      </section>

      <section class="tarjeta">
        <h3>Validaciones del flujo</h3>
        <ul class="etapas">
          {#each ORDEN_ETAPAS as id (id)}
            {@const e = candidato.etapas[id]}
            <li>
              <span class="punto punto-{e?.estado ?? 'pendiente'}"></span>
              <div>
                <strong>{ETIQUETA_ETAPA[id]}</strong>
                <p>{e?.detalle ?? e?.estado ?? "pendiente"}</p>
              </div>
            </li>
          {/each}
        </ul>
      </section>
    </div>

    <section class="tarjeta">
      <h3>Hojas de vida</h3>
      {#if documentos.length === 0}
        <p class="aviso">No hay documentos descargados de este candidato.</p>
      {:else}
        <div class="documentos">
          {#each documentos as d (d.ruta)}
            <div class="documento">
              <HugeiconsIcon icon={File02Icon} size={20} strokeWidth={2} />
              <div>
                <strong>{d.etiqueta}</strong>
                <span>{nombreArchivo(d.ruta)}</span>
              </div>
              <a class="boton-secundario" href={urlCaptura(d.ruta)} target="_blank" rel="noopener">Ver</a>
              <a class="boton-descarga" href={urlCaptura(d.ruta)} download={nombreArchivo(d.ruta)}>
                <HugeiconsIcon icon={Download01Icon} size={14} strokeWidth={2.2} />
                <span>Descargar</span>
              </a>
            </div>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .pagina {
    padding: 26px 32px 42px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .barra-acciones {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
  }

  .tarjeta {
    background: #ffffff;
    border-radius: 22px;
    padding: 20px 24px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  h3 {
    font-size: 14px;
    font-weight: 700;
  }

  h4 {
    font-size: 12px;
    font-weight: 700;
    margin-bottom: 6px;
  }

  .cabecera {
    flex-direction: row;
    align-items: center;
    gap: 18px;
    flex-wrap: wrap;
  }

  .avatar {
    width: 72px;
    height: 72px;
    border-radius: 20px;
    object-fit: cover;
    background: var(--color-fondo-soft);
  }

  .identidad {
    flex: 1;
    min-width: 200px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .identidad h2 {
    font-size: 18px;
    font-weight: 800;
  }

  .identidad p {
    font-size: 12.5px;
    color: var(--color-texto-grey);
    font-weight: 600;
  }

  .pildora {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 4px 12px;
    border-radius: 999px;
    font-size: 11.5px;
    font-weight: 700;
    background: var(--color-fondo-soft);
  }

  .veredicto-apto {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .veredicto-no_apto {
    background: rgba(239, 68, 68, 0.1);
    color: #d14343;
  }

  .veredicto-pendiente {
    background: rgba(245, 158, 11, 0.14);
    color: #b9700a;
  }

  .veredicto-analizando {
    background: rgba(59, 130, 246, 0.12);
    color: #2f6fd6;
  }

  .anillo {
    --color: var(--color-primario);
    width: 96px;
    height: 96px;
    border-radius: 50%;
    padding: 8px;
    background: conic-gradient(var(--color) calc(var(--p) * 1%), rgba(44, 62, 80, 0.1) 0);
  }

  .anillo.nivel-medio {
    --color: #f59e0b;
  }

  .anillo.nivel-bajo {
    --color: #ef4444;
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
    font-size: 28px;
    font-weight: 800;
  }

  .anillo-interior span {
    font-size: 10px;
    font-weight: 700;
    color: var(--color-texto-grey);
    margin-top: 3px;
  }

  .dictamen {
    font-size: 16px;
    font-weight: 800;
    color: var(--color-primario);
  }

  .dictamen.nivel-medio {
    color: #b9700a;
  }

  .dictamen.nivel-bajo {
    color: #d14343;
  }

  .resumen,
  .just,
  .aviso {
    font-size: 12.5px;
    line-height: 1.5;
    color: var(--color-texto-dark);
  }

  .aviso {
    padding: 12px 14px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    font-weight: 500;
  }

  .just {
    color: var(--color-texto-grey);
    font-size: 11.5px;
  }

  .aspectos {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 14px;
  }

  .aspecto {
    padding: 12px 14px;
    border-radius: 16px;
    background: var(--color-fondo-soft);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .fila-aspecto {
    display: flex;
    justify-content: space-between;
    font-size: 12.5px;
    font-weight: 600;
  }

  .barra {
    height: 7px;
    border-radius: 999px;
    background: rgba(44, 62, 80, 0.1);
    overflow: hidden;
  }

  .relleno {
    height: 100%;
    border-radius: inherit;
    background: var(--color-primario);
  }

  .rejilla-dos {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 16px;
  }

  .rejilla-dos.columnas {
    align-items: start;
  }

  .lista {
    margin: 0;
    padding-left: 18px;
    font-size: 12px;
    line-height: 1.5;
  }

  .lista.ok li::marker {
    color: var(--color-primario);
  }

  .lista.mal {
    color: #b42323;
  }

  .datos {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 10px 16px;
    margin: 0;
  }

  .datos div {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  dt {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-texto-grey);
  }

  dd {
    margin: 0;
    font-size: 12.5px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .pie-ia {
    font-size: 10.5px;
    color: var(--color-texto-grey);
  }

  .experiencia {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .experiencia li {
    display: flex;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 16px;
    background: var(--color-fondo-soft);
  }

  .marca {
    flex: none;
    width: 4px;
    border-radius: 999px;
    background: var(--color-primario);
  }

  .exp-cuerpo {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .exp-titulo {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    font-size: 13px;
  }

  .exp-empresa {
    font-size: 12px;
    font-weight: 600;
    color: var(--color-texto-dark);
  }

  .exp-desc {
    font-size: 11.5px;
    line-height: 1.45;
    color: var(--color-texto-grey);
  }

  .licencia {
    padding: 10px 12px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    font-size: 12px;
  }

  .licencia p {
    margin-top: 4px;
    color: var(--color-texto-grey);
    line-height: 1.5;
  }

  .chip {
    margin-left: 6px;
    padding: 2px 8px;
    border-radius: 999px;
    background: #ffffff;
    font-size: 10.5px;
    font-weight: 700;
  }

  .etapas {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .etapas li {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    font-size: 12px;
  }

  .etapas p {
    color: var(--color-texto-grey);
    line-height: 1.4;
    overflow-wrap: anywhere;
  }

  .punto {
    flex: none;
    width: 10px;
    height: 10px;
    margin-top: 4px;
    border-radius: 50%;
    background: rgba(44, 62, 80, 0.15);
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

  .punto-error {
    background: #f97316;
  }

  .documentos {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .documento {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 16px;
    background: var(--color-fondo-soft);
    flex-wrap: wrap;
  }

  .documento div {
    flex: 1;
    min-width: 180px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 12.5px;
  }

  .documento span {
    font-size: 11px;
    color: var(--color-texto-grey);
    overflow-wrap: anywhere;
  }

  .boton-secundario,
  .boton-descarga {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 9px 15px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    text-decoration: none;
    transition: var(--transicion-vuesax);
  }

  .documento .boton-secundario {
    background: #ffffff;
  }

  .boton-descarga {
    background: var(--color-primario);
    color: #ffffff;
  }

  .boton-secundario:hover {
    background: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .estado-vacio {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 38px 20px;
    text-align: center;
    color: var(--color-texto-grey);
    font-size: 13px;
    font-weight: 500;
  }

  .spinner,
  .mini-spinner {
    display: inline-block;
    border-radius: 50%;
    border: 3px solid rgba(var(--vs-primary-raw), 0.2);
    border-top-color: var(--color-primario);
    animation: girar 0.8s linear infinite;
  }

  .spinner {
    width: 26px;
    height: 26px;
  }

  .mini-spinner {
    width: 11px;
    height: 11px;
    border-width: 2px;
  }

  @keyframes girar {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 640px) {
    .pagina {
      padding: 16px;
    }
  }
</style>
