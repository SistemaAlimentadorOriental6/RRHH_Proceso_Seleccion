<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import { fade, fly, slide } from "svelte/transition";
  import { cubicInOut, cubicOut } from "svelte/easing";
  import { SvelteSet } from "svelte/reactivity";
  import { despachados, destinoDescartados, llevarTarjeta } from "../servicios/descarteAnimado.svelte";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import {
    Briefcase01Icon,
    UserGroupIcon,
    StopIcon,
    ReloadIcon,
    WorkflowSquare10Icon,
    CheckmarkCircle02Icon,
    Cancel01Icon,
    Alert02Icon,
    ChevronRightIcon,
    File02Icon,
    Download01Icon,
    ChevronLeftIcon,
  } from "@hugeicons/core-free-icons";
  import { descargarZip, urlExportarAptos, urlExportarCandidato } from "../servicios/clienteRrhh";
  import AvatarAnalisis from "../componentes/AvatarAnalisis.svelte";
  import {
    ETIQUETA_ETAPA,
    ORDEN_ETAPAS,
    type CandidatoEnVivo,
    type VacanteApi,
  } from "../modelos/analisisEnVivo";
  import {
    ETIQUETA_VEREDICTO,
    almacenRrhh,
    etiquetaEstadoAnalisis,
    veredictoDe,
  } from "../servicios/almacenAnalisis.svelte";

  interface Propiedades {
    /** Id de la vacante seleccionada en la URL; null muestra el listado de todas. */
    idVacanteFiltrada?: string | null;
    alSeleccionarVacante: (idVacante: string) => void;
    alVerTodasLasVacantes: () => void;
    /** Los rechazados ya no se ven aquí: viven en Descartados. */
    alVerDescartados?: () => void;
    alAbrirFlujoCandidato: (idVacante: string, slugCandidato: string) => void;
    alAbrirResultadosCandidato: (idVacante: string, slugCandidato: string) => void;
  }

  let {
    idVacanteFiltrada = null,
    alSeleccionarVacante,
    alVerTodasLasVacantes,
    alVerDescartados,
    alAbrirFlujoCandidato,
    alAbrirResultadosCandidato,
  }: Propiedades = $props();

  let textoBusqueda = $state("");

  const ETIQUETA_CONEXION = {
    conectando: "Conectando…",
    en_vivo: "En vivo",
    reconectando: "Reconectando…",
    pausada: "En pausa",
  } as const;

  // Reloj para "hace X min": basta con avanzar cada 30 s.
  let ahora = $state(Date.now());
  $effect(() => {
    const t = setInterval(() => (ahora = Date.now()), 30_000);
    return () => clearInterval(t);
  });
  function haceCuanto(momento: number): string {
    const minutos = Math.floor((ahora - momento) / 60_000);
    if (minutos < 1) return "actualizada hace un momento";
    if (minutos < 60) return `actualizada hace ${minutos} min`;
    return `actualizada hace ${Math.floor(minutos / 60)} h`;
  }
  /** 0 = analiza a todos los postulados (nada en la pantalla lo cambia). */
  const limiteAnalisis = 0;

  // Personas en total = inscritos en Computrabajo + hojas de vida recibidas por correo (que van dentro
  // del grupo Operador / Conductor, o sueltas si no hay con qué agruparlas).
  const totalPersonas = $derived(
    almacenRrhh.vacantes.reduce((suma, v) => suma + (v.total_candidatos ?? 0), 0),
  );
  const totalCorreo = $derived(
    almacenRrhh.vacantes.reduce(
      (suma, v) => suma + (v.origen === "correo" ? (v.total_candidatos ?? 0) : (v.total_correo ?? 0)),
      0,
    ),
  );
  const totalComputrabajo = $derived(totalPersonas - totalCorreo);

  const vacanteActiva = $derived(
    idVacanteFiltrada ? almacenRrhh.vacantePorId(idVacanteFiltrada) : undefined,
  );
  const sesionActiva = $derived(
    idVacanteFiltrada ? almacenRrhh.sesion(idVacanteFiltrada) : null,
  );

  const vacantesVisibles = $derived.by(() => {
    const base = idVacanteFiltrada
      ? almacenRrhh.vacantes.filter(
          (v) => v.id.toLowerCase() === idVacanteFiltrada.toLowerCase(),
        )
      : almacenRrhh.vacantes;
    const termino = textoBusqueda.trim().toLowerCase();
    if (!termino) return base;
    return base.filter(
      (v) =>
        v.titulo.toLowerCase().includes(termino) ||
        v.id.toLowerCase().includes(termino) ||
        v.slug.toLowerCase().includes(termino),
    );
  });

  const ETIQUETA_CORTA_ETAPA: Record<(typeof ORDEN_ETAPAS)[number], string> = {
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

  // En cola va justo después de lo que se está analizando: lo nuevo se ve arriba, en vez de aparecer
  // al final y saltar hasta el principio cuando le toca.
  const PESO_ORDEN_VEREDICTO: Record<string, number> = {
    analizando: 0,
    recien: 0.2,
    en_cola: 0.5,
    apto: 1,
    pendiente: 2,
    no_apto: 3,
  };

  /** Cuánto se queda una tarjeta en su sitio, resaltada, después de terminar y antes de acomodarse en su grupo. */
  const REPOSO_TARJETA_MS = 3000;
  let reloj = $state(Date.now());
  function enReposo(c: CandidatoEnVivo): boolean {
    return c.terminadoEn !== null && reloj - c.terminadoEn < REPOSO_TARJETA_MS;
  }
  // Mientras haya tarjetas en reposo, se despierta justo cuando termina el más próximo a acomodarse.
  $effect(() => {
    const restantes = (sesionActiva?.candidatos ?? [])
      .filter((c) => c.terminadoEn !== null && reloj - c.terminadoEn < REPOSO_TARJETA_MS)
      .map((c) => REPOSO_TARJETA_MS - (reloj - (c.terminadoEn as number)));
    if (restantes.length === 0) return;
    const t = setTimeout(() => (reloj = Date.now()), Math.min(...restantes) + 20);
    return () => clearTimeout(t);
  });

  type OrdenCandidatos = "defecto" | "mayor" | "menor";
  let orden = $state<OrdenCandidatos>("defecto");

  /** Puntaje total de la IA, o null si el candidato aún no tiene calificación. */
  function puntaje(c: CandidatoEnVivo): number | null {
    return c.resultado?.evaluacion_ia?.score_total ?? null;
  }

  // Quien termina en vivo se queda un momento en "Por verificar" y Adi se lo lleva, sin prisa y de a
  // uno: a Descartados si fue rechazado, a su lugar en Analizados si no. Los que ya estaban rechazados
  // al abrir la pantalla simplemente no aparecen aquí (están en Descartados).
  // (la lista `despachados` vive en descarteAnimado: sobrevive a salir y volver a esta pantalla)
  /** Más allá de esto una tarjeta terminada ya no se lleva con animación: se acomoda directamente. */
  const MAX_ANTIGUEDAD_LLEVAR_MS = 20_000;
  /** Aptos y pendientes que Adi aún está trayendo: su tarjeta real ya existe en Analizados, pero oculta. */
  const enCamino = new SvelteSet<string>();
  const ESPERA_ANTES_DE_LLEVAR_MS = 1500;
  const programados = new Set<string>();

  /** Terminó en esta sesión y Adi todavía no se lo ha llevado. */
  const esperaTurno = (c: CandidatoEnVivo) =>
    c.terminadoEn !== null && c.fase === "completo" && !despachados.has(c.slug);

  function llevar(slug: string): void {
    const c = sesionActiva?.candidatos.find((x) => x.slug === slug);
    if (!c || despachados.has(slug)) {
      programados.delete(slug);
      return;
    }
    const tarjeta = document.querySelector<HTMLElement>(`.fila-cola [data-slug="${CSS.escape(slug)}"]`);
    if (!tarjeta) {
      despachados.add(slug);
      programados.delete(slug);
      return;
    }
    const descartar = veredictoDe(c) === "no_apto";
    const resultado = llevarTarjeta(
      tarjeta,
      descartar ? "descartar" : "analizar",
      descartar
        ? destinoDescartados
        : () => document.querySelector<HTMLElement>(`.grilla-candidatos [data-slug="${CSS.escape(slug)}"]`),
      () => enCamino.delete(slug),
    );
    if (resultado === "ocupado") {
      setTimeout(() => llevar(slug), 500); // Adi lleva otra: espera su turno
      return;
    }
    if (resultado === "iniciado" && !descartar) enCamino.add(slug);
    despachados.add(slug);
    programados.delete(slug);
  }

  $effect(() => {
    for (const c of sesionActiva?.candidatos ?? []) {
      if (!esperaTurno(c) || programados.has(c.slug)) continue;
      if (Date.now() - (c.terminadoEn as number) > MAX_ANTIGUEDAD_LLEVAR_MS) {
        despachados.add(c.slug); // terminó hace rato (p. ej. mientras estaba en otra pantalla)
        continue;
      }
      programados.add(c.slug);
      const espera = Math.max(0, (c.terminadoEn as number) + ESPERA_ANTES_DE_LLEVAR_MS - Date.now());
      setTimeout(() => llevar(c.slug), espera);
    }
  });
  const sigueEnReclutamiento = (c: CandidatoEnVivo) => veredictoDe(c) !== "no_apto" || esperaTurno(c);

  const candidatosVisibles = $derived.by((): CandidatoEnVivo[] => {
    if (!sesionActiva) return [];
    const termino = textoBusqueda.trim().toLowerCase();
    const enReclutamiento = sesionActiva.candidatos.filter(sigueEnReclutamiento);
    const filtrados = !termino
      ? enReclutamiento
      : enReclutamiento.filter(
          (c) =>
            c.nombre.toLowerCase().includes(termino) ||
            c.cedula.includes(termino),
        );
    return [...filtrados].sort((a, b) => {
      if (orden !== "defecto") {
        // Por puntuación de la IA: quien no tiene puntaje (no apto, sin calificar) va al final.
        const sa = puntaje(a);
        const sb = puntaje(b);
        if (sa !== sb) {
          if (sa === null) return 1;
          if (sb === null) return -1;
          return orden === "mayor" ? sb - sa : sa - sb;
        }
      }
      // Por defecto, los que se van terminando salen primero (el más reciente arriba); los que ya
      // venían del guardado siguen detrás, con su orden de siempre.
      if (orden === "defecto" && (a.terminadoEn !== null || b.terminadoEn !== null)) {
        if (a.terminadoEn === null) return 1;
        if (b.terminadoEn === null) return -1;
        if (a.terminadoEn !== b.terminadoEn) return b.terminadoEn - a.terminadoEn;
      }
      const pesoA = PESO_ORDEN_VEREDICTO[veredictoDe(a)] ?? 99;
      const pesoB = PESO_ORDEN_VEREDICTO[veredictoDe(b)] ?? 99;
      if (pesoA !== pesoB) return pesoA - pesoB;
      return a.indice - b.indice;
    });
  });

  function resumenEstadoCandidato(candidato: CandidatoEnVivo): {
    texto: string;
    tipo: "apto" | "descarte" | "proceso";
  } {
    const veredicto = veredictoDe(candidato);
    const motivos = candidato.resultado?.evaluacion_idoneidad?.motivos_descarte;
    if (motivos && motivos.length > 0) {
      return { texto: motivos.join(" · "), tipo: "descarte" };
    }
    if (veredicto === "apto") {
      return {
        texto: "Cumple perfil, género, EPS activa, RUNT y SIMIT al día.",
        tipo: "apto",
      };
    }
    if (veredicto === "analizando") {
      return {
        texto: "Validando fuentes oficiales en tiempo real…",
        tipo: "proceso",
      };
    }
    if (veredicto === "pendiente") {
      return {
        texto: "Requiere revisión manual de alguna fuente externa.",
        tipo: "proceso",
      };
    }
    return { texto: "Pendiente de iniciar validación.", tipo: "proceso" };
  }

  onMount(() => {
    void almacenRrhh.asegurarVacantes();
  });

  // Al entrar a una vacante se cargan sus candidatos ya analizados (no abre Chrome).
  $effect(() => {
    if (sesionActiva && almacenRrhh.vacantes.length > 0)
      void sesionActiva.cargarGuardados();
  });

  // Las tarjetas del listado toman sus totales de la lista de vacantes (los calcula la base): ya no se
  // cargan todos los candidatos de cada vacante solo para contarlos. Si la sesión ya tiene a sus
  // candidatos (vacante abierta o análisis en vivo), se cuentan ahí, que está más al día.
  function resumenVacante(vacante: VacanteApi, sesion: ReturnType<typeof almacenRrhh.sesion>) {
    const completos = sesion.candidatos.filter((c) => c.fase === "completo");
    if (completos.length === 0 && vacante.resumen) {
      const r = vacante.resumen;
      return { postulados: r.postulados, aceptados: r.aptos, rechazados: r.no_aptos, pendientes: r.pendientes };
    }
    const veredictos = completos.map(veredictoDe);
    return {
      postulados: veredictos.length,
      aceptados: veredictos.filter((v) => v === "apto").length,
      rechazados: veredictos.filter((v) => v === "no_apto").length,
      pendientes: veredictos.filter((v) => v === "pendiente").length,
    };
  }

  // "Por verificar": lo que aún se analiza (primero el que está en curso, después la fila de espera) y,
  // un momento, el rechazado que Adi está por llevarse. Todo lo demás va a "Analizados", donde cada
  // tarjeta termina directo en su lugar final: al cambiar de sección viaja en vez de aparecer y desaparecer.
  const enCola = (c: CandidatoEnVivo) => c.fase !== "completo" || esperaTurno(c);
  const esCompacta = (c: CandidatoEnVivo) => c.fase === "en_cola" && !c.resultado;
  const claveCola = (c: CandidatoEnVivo) => (esperaTurno(c) ? -1 : c.fase === "analizando" ? 0 : 1);
  const candidatosCola = $derived(
    candidatosVisibles.filter(enCola).sort((a, b) => claveCola(a) - claveCola(b) || a.indice - b.indice),
  );
  const candidatosResultados = $derived(candidatosVisibles.filter((c) => !enCola(c)));

  // "Analizados" se pinta por tandas: cientos de tarjetas a la vez (cada una con sus etapas y animaciones)
  // volvían lenta la pantalla. Los que terminan en vivo salen arriba, así que siempre quedan a la vista.
  const TANDA_RESULTADOS = 60;
  let limiteResultados = $state(TANDA_RESULTADOS);
  $effect(() => {
    void idVacanteFiltrada;
    void textoBusqueda;
    void orden;
    limiteResultados = TANDA_RESULTADOS; // otra vacante, búsqueda u orden: se vuelve a la primera tanda
  });
  const resultadosVisibles = $derived(candidatosResultados.slice(0, limiteResultados));

  // Reacomodar con animación sirve con pocas tarjetas; con muchas, medir y mover todas cada vez que
  // alguien termina daba tirones (y el avatar mide la pantalla en cada cuadro). Ahí se acomodan sin más.
  const MAX_TARJETAS_ANIMADAS = 40;
  const sinMovimiento = typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  function acomodar(nodo: Element, rects: { from: DOMRect; to: DOMRect }, parametros?: Parameters<typeof flip>[2]) {
    if (sinMovimiento || candidatosCola.length + resultadosVisibles.length > MAX_TARJETAS_ANIMADAS) return { duration: 0 };
    return flip(nodo, rects, parametros);
  }

  // Un solo Adi en la lista: en la vacante que se analiza o, si ninguna, en la primera. Antes cada tarjeta
  // tenía el suyo, cada uno con su bucle de animación midiendo la pantalla.
  const idConAvatar = $derived(
    (vacantesVisibles.find((v) => almacenRrhh.sesion(v.id).activo) ?? vacantesVisibles[0])?.id ?? null,
  );

  // Fila de espera con scroll cuidado: bordes que se desvanecen donde hay más contenido, flechas
  // para recorrerla y vuelta al inicio cuando cambia quién va primero (el que se analiza).
  let filaCola = $state<HTMLElement | null>(null);
  function controlScroll(nodo: HTMLElement) {
    const marco = nodo.parentElement as HTMLElement;
    const actualizar = () => {
      const izq = nodo.scrollLeft > 4;
      const der = nodo.scrollLeft + nodo.clientWidth < nodo.scrollWidth - 4;
      marco.dataset.izq = String(izq);
      marco.dataset.der = String(der);
      nodo.style.setProperty("--fi", izq ? "40px" : "0px");
      nodo.style.setProperty("--fd", der ? "40px" : "0px");
    };
    nodo.addEventListener("scroll", actualizar, { passive: true });
    const tamano = new ResizeObserver(actualizar);
    tamano.observe(nodo);
    const hijos = new MutationObserver(actualizar);
    hijos.observe(nodo, { childList: true });
    actualizar();
    return {
      destroy() {
        nodo.removeEventListener("scroll", actualizar);
        tamano.disconnect();
        hijos.disconnect();
      },
    };
  }
  const recorrerCola = (sentido: -1 | 1) =>
    filaCola?.scrollBy({ left: sentido * filaCola.clientWidth * 0.8, behavior: "smooth" });
  $effect(() => {
    void candidatosCola[0]?.slug;
    filaCola?.scrollTo({ left: 0, behavior: "smooth" });
  });

  function iniciar(vacante: VacanteApi, forzar: boolean): void {
    const sesion = almacenRrhh.sesion(vacante.id);
    if (idVacanteFiltrada !== vacante.id) alSeleccionarVacante(vacante.id);
    void sesion.iniciar({ limite: limiteAnalisis, forzar });
  }

  function porcentaje(procesados: number, total: number): number {
    if (total <= 0) return 0;
    return Math.min(100, Math.round((procesados / total) * 100));
  }

  function totalEsperado(vacante: VacanteApi, descubiertos: number): number {
    const inscritos = vacante.total_candidatos ?? 0;
    const tope =
      limiteAnalisis > 0
        ? limiteAnalisis * (vacante.ids_miembros?.length ?? 1)
        : Infinity;
    return Math.max(Math.min(inscritos || descubiertos, tope), descubiertos);
  }
</script>

<div class="contenedor-vacantes">
  <section class="panel-superior">
    <div class="textos">
      <div class="fila-insignia">
        <span class="insignia-modulo">
          <HugeiconsIcon icon={Briefcase01Icon} size={13} strokeWidth={2.2} />
          <span>Computrabajo en vivo</span>
        </span>
        <span
          class="estado-conexion conexion-{almacenRrhh.conexion}"
          role="status"
          title="Conexión con el servidor: los cambios llegan solos mientras diga «En vivo»"
        >
          <span class="punto-conexion" aria-hidden="true"></span>
          {ETIQUETA_CONEXION[almacenRrhh.conexion]}
        </span>
        {#if almacenRrhh.listaLeidaEn !== null}
          <span class="edad-lista">
            {almacenRrhh.contando ? "Actualizando inscritos…" : `Lista ${haceCuanto(almacenRrhh.listaLeidaEn)}`}
          </span>
          <button
            type="button"
            class="boton-actualizar"
            disabled={almacenRrhh.contando || almacenRrhh.cargando}
            title="Volver a leer las vacantes y sus inscritos en Computrabajo"
            onclick={() => almacenRrhh.cargarVacantes(true)}
          >
            <HugeiconsIcon icon={ReloadIcon} size={12} strokeWidth={2.2} />
            <span>Actualizar</span>
          </button>
        {/if}
        {#if idVacanteFiltrada}
          <button
            type="button"
            class="boton-volver"
            onclick={alVerTodasLasVacantes}
          >
            ← Ver todas las vacantes ({almacenRrhh.vacantes.length})
          </button>
        {/if}
      </div>
      <h2 class="titulo-seccion">
        {vacanteActiva ? vacanteActiva.titulo : "Vacantes activas y candidatos"}
      </h2>
    </div>

    <div class="bloque-busqueda">
      <div class="kpis">
        <div class="kpi">
          <span>Vacantes</span>
          <strong>{almacenRrhh.vacantes.length}</strong>
        </div>
        {#if !idVacanteFiltrada && totalPersonas > 0}
          <div
            class="kpi"
            title="{totalComputrabajo} inscritos en Computrabajo + {totalCorreo} hojas de vida por correo"
          >
            <span>Personas en total</span>
            <strong>{totalPersonas}</strong>
          </div>
        {/if}
        {#if sesionActiva}
          <div class="kpi kpi-verde">
            <span>Aptos</span>
            <strong
              >{sesionActiva.candidatos.filter((c) => veredictoDe(c) === "apto")
                .length}</strong
            >
          </div>
          <div class="kpi">
            <span>Personas</span>
            <strong>{sesionActiva.candidatos.length}</strong>
          </div>
        {/if}
      </div>
      {#if idVacanteFiltrada}
        <select
          class="selector-orden input-premium"
          bind:value={orden}
          aria-label="Ordenar candidatos"
          title="Ordenar candidatos"
        >
          <option value="defecto">Orden: aptos primero</option>
          <option value="mayor">Mayor puntuación primero</option>
          <option value="menor">Menor puntuación primero</option>
        </select>
      {/if}
      <input
        type="search"
        class="buscador input-premium"
        placeholder={idVacanteFiltrada
          ? "Buscar candidato…"
          : "Buscar vacante…"}
        bind:value={textoBusqueda}
      />
    </div>
  </section>

  {#if almacenRrhh.error}
    <div class="caja-aviso caja-error" role="alert">
      <HugeiconsIcon icon={Alert02Icon} size={18} strokeWidth={2} />
      <div>
        <strong>No se pudieron cargar las vacantes</strong>
        <p>{almacenRrhh.error}</p>
      </div>
      <button
        type="button"
        class="boton-secundario"
        onclick={() => almacenRrhh.cargarVacantes(true)}
      >
        <HugeiconsIcon icon={ReloadIcon} size={14} strokeWidth={2} />
        <span>Reintentar</span>
      </button>
    </div>
  {:else if almacenRrhh.aviso}
    <div class="caja-aviso" role="status">
      <HugeiconsIcon icon={Alert02Icon} size={18} strokeWidth={2} />
      <p>{almacenRrhh.aviso}</p>
    </div>
  {/if}

  {#if almacenRrhh.cargando && almacenRrhh.vacantes.length === 0}
    <div class="estado-vacio" aria-busy="true">
      <span class="spinner"></span>
      <p>
        Leyendo las vacantes activas en Computrabajo… solo la primera vez
        tarda: después la lista sale al instante y se actualiza sola.
      </p>
    </div>
  {:else if idVacanteFiltrada && !vacanteActiva && almacenRrhh.vacantes.length > 0}
    <div class="estado-vacio">
      <p>
        La vacante <strong>{idVacanteFiltrada}</strong> no está entre las ofertas
        activas de Computrabajo.
      </p>
      <button
        type="button"
        class="boton-secundario"
        onclick={alVerTodasLasVacantes}>Ver todas las vacantes</button
      >
    </div>
  {:else if vacantesVisibles.length === 0 && !almacenRrhh.error}
    <div class="estado-vacio">
      <p>
        {textoBusqueda
          ? "Ninguna vacante coincide con la búsqueda."
          : "No hay vacantes activas."}
      </p>
    </div>
  {/if}

  <div class="lista-vacantes">
    {#each vacantesVisibles as vacante (vacante.id)}
      {@const sesion = almacenRrhh.sesion(vacante.id)}
      <article class="tarjeta-vacante">
        <div class="cabecera-vacante">
          {#if vacante.id === idConAvatar}
            <AvatarAnalisis {sesion} />
          {/if}
          <div class="info-vacante">
            <div class="fila-chips">
              {#if vacante.origen === "correo"}
                <span class="chip chip-id" title="Hojas de vida recibidas por correo"
                  >Correo</span
                >
              {:else}
                <span class="chip chip-id" title="Identificador en Computrabajo"
                  >{vacante.id.slice(0, 8)}…</span
                >
              {/if}
              {#if vacante.ids_miembros}
                <span class="chip chip-procesada"
                  >{vacante.ids_miembros.length} vacantes agrupadas</span
                >
              {/if}
              <span class="chip chip-slug">/{vacante.slug}</span>
              {#if vacante.excluida}
                <span class="chip chip-excluida"
                  >Excluida por configuración</span
                >
              {:else if vacante.procesada}
                <span class="chip chip-procesada">Con análisis guardado</span>
              {:else}
                <span class="chip">Sin analizar</span>
              {/if}
            </div>
            <h3 class="titulo-vacante">
              {#if idVacanteFiltrada}
                {vacante.titulo}
              {:else}
                <button
                  type="button"
                  class="boton-titulo"
                  onclick={() => alSeleccionarVacante(vacante.id)}
                >
                  {vacante.titulo}
                </button>
              {/if}
            </h3>
            <div class="meta">
              <span class="meta-item">
                <HugeiconsIcon icon={UserGroupIcon} size={14} strokeWidth={2} />
                {#if vacante.total_candidatos !== null}
                  {#if vacante.origen === "correo"}
                    <span
                      ><strong>{vacante.total_candidatos}</strong> hojas de vida recibidas por correo</span
                    >
                  {:else if vacante.total_correo}
                    <span
                      ><strong>{vacante.total_candidatos - vacante.total_correo}</strong> inscritos en Computrabajo
                      + <strong>{vacante.total_correo}</strong> por correo</span
                    >
                  {:else}
                    <span
                      ><strong>{vacante.total_candidatos}</strong> inscritos en Computrabajo</span
                    >
                  {/if}
                {:else if almacenRrhh.contando}
                  <span class="texto-suave">Contando inscritos…</span>
                {:else}
                  <span class="texto-suave" title={vacante.error_conteo}
                    >Inscritos sin contar</span
                  >
                {/if}
              </span>
              {#if resumenVacante(vacante, sesion).postulados > 0}
                <span class="meta-item">
                  <HugeiconsIcon
                    icon={CheckmarkCircle02Icon}
                    size={14}
                    strokeWidth={2}
                  />
                  <span
                    ><strong>{resumenVacante(vacante, sesion).postulados}</strong> con resultado</span
                  >
                </span>
              {/if}
            </div>
          </div>

          <div class="acciones-vacante">
            {#if !idVacanteFiltrada}
              <button
                type="button"
                class="boton-secundario"
                onclick={() => alSeleccionarVacante(vacante.id)}
              >
                <span>Ver candidatos</span>
                <HugeiconsIcon
                  icon={ChevronRightIcon}
                  size={14}
                  strokeWidth={2.2}
                />
              </button>
            {/if}

            {#if sesion.activo}
              <button
                type="button"
                class="boton-detener"
                onclick={() => sesion.detener()}
              >
                <HugeiconsIcon icon={StopIcon} size={15} strokeWidth={2.3} />
                <span>Detener</span>
              </button>
            {:else}
              {#if sesion.candidatos.length > 0 && idVacanteFiltrada}
                <button
                  type="button"
                  class="boton-secundario"
                  disabled={vacante.excluida}
                  title="Vuelve a consultar a todos, aunque ya tengan resultado guardado"
                  onclick={() => iniciar(vacante, true)}
                >
                  <HugeiconsIcon icon={ReloadIcon} size={14} strokeWidth={2} />
                  <span>Reanalizar todos</span>
                </button>
              {/if}
            {/if}
            {#if resumenVacante(vacante, sesion).aceptados > 0}
              <button
                type="button"
                class="boton-secundario"
                onclick={() => descargarZip(urlExportarAptos(vacante.ids_miembros ?? [vacante.id]), "candidatos-aptos.zip")}
                title="Descarga un .zip con los aptos: hojas de vida, cédulas, teléfonos y correos"
              >
                <HugeiconsIcon icon={Download01Icon} size={14} strokeWidth={2} />
                <span>Exportar aptos</span>
              </button>
            {/if}
          </div>
        </div>

        {#if resumenVacante(vacante, sesion).postulados > 0}
          {@const r = resumenVacante(vacante, sesion)}
          <div class="resumen-vacante">
            <div class="fila-cifras">
              <div class="cifra"><strong>{r.postulados}</strong><span>Postulados</span></div>
              <div class="cifra cifra-ok"><strong>{r.aceptados}</strong><span>Aceptados · Aptos</span></div>
              <button
                type="button"
                class="cifra cifra-mal cifra-enlace"
                disabled={!alVerDescartados}
                title="Los rechazados están en Descartados"
                onclick={() => alVerDescartados?.()}
              ><strong>{r.rechazados}</strong><span>Descartados · No aptos</span></button>
              {#if r.pendientes > 0}
                <div class="cifra cifra-aviso"><strong>{r.pendientes}</strong><span>Verificación pendiente</span></div>
              {/if}
            </div>
          </div>
        {/if}

        {#if sesion.estado !== "inactivo" || sesion.mensaje}
          <div class="franja-estado estado-{sesion.estado}">
            <div class="fila-estado">
              <strong>{etiquetaEstadoAnalisis(sesion.estado)}</strong>
              {#if sesion.mensaje}<span class="mensaje">{sesion.mensaje}</span
                >{/if}
              {#if sesion.bloqueadoPor}
                <button
                  type="button"
                  class="boton-secundario"
                  onclick={() =>
                    alSeleccionarVacante(sesion.bloqueadoPor?.id ?? "")}
                >
                  Ir a ese análisis
                </button>
              {/if}
            </div>
            {#if sesion.estado !== "error" || sesion.resumen.descubiertos > 0}
              <div class="barra-progreso" aria-hidden="true">
                <div
                  class="relleno"
                  style="width: {porcentaje(
                    sesion.resumen.procesados,
                    totalEsperado(vacante, sesion.resumen.descubiertos),
                  )}%"
                ></div>
              </div>
              <div class="contadores">
                <span
                  >{sesion.resumen.procesados} de {totalEsperado(
                    vacante,
                    sesion.resumen.descubiertos,
                  )} revisados</span
                >
                <span class="c-apto">{sesion.resumen.aptos} aptos</span>
                <span class="c-no-apto"
                  >{sesion.resumen.no_aptos - sesion.resumen.pendientes} no aptos</span
                >
                {#if sesion.resumen.pendientes > 0}
                  <span class="c-pendiente"
                    >{sesion.resumen.pendientes} con verificación pendiente</span
                  >
                {/if}
                {#if sesion.resumen.cache > 0}<span
                    >{sesion.resumen.cache} desde guardado</span
                  >{/if}
              </div>
            {/if}
          </div>
        {/if}

        {#if idVacanteFiltrada}
          {#if sesion.cargandoGuardados && sesion.candidatos.length === 0}
            <div class="estado-vacio" aria-busy="true">
              <span class="spinner"></span>
              <p>Leyendo los candidatos guardados…</p>
            </div>
          {:else if sesion.candidatos.length === 0 && sesion.errorCarga}
            <div class="estado-vacio" role="alert">
              <p>No se pudieron leer los candidatos guardados: {sesion.errorCarga}. Reintentando…</p>
            </div>
          {:else if sesion.candidatos.length === 0}
            <div class="estado-vacio">
              <p>
                Esta vacante todavía no tiene personas analizadas. Pulsa <strong
                  >Analizar en vivo</strong
                >
                para leer sus postulados y validarlos uno por uno.
              </p>
            </div>
          {:else}
            {#snippet cuerpoTarjeta(candidato: CandidatoEnVivo, vacante: VacanteApi)}
              {@const veredicto = veredictoDe(candidato)}
              {@const resumen = resumenEstadoCandidato(candidato)}
              {@const ia = candidato.resultado?.evaluacion_ia ?? null}
              {#if esCompacta(candidato)}
                <div class="fila-superior-persona">
                  <span class="pildora veredicto-en_cola">En cola</span>
                  <span class="etiqueta-orden">#{candidato.indice}</span>
                </div>
                <div class="cabecera-persona">
                  <img class="avatar" src={candidato.fotoUrl} alt={candidato.nombre} width="50" height="50" loading="lazy" decoding="async" />
                  <div class="identidad">
                    <h4 title={candidato.nombre.toUpperCase()}>{candidato.nombre.toUpperCase()}</h4>
                    <span class="esqueleto esq-linea esq-corta"></span>
                  </div>
                </div>
              {:else}
                  <div class="fila-superior-persona">
                    <span class="pildora veredicto-{veredicto}">
                      {#if veredicto === "apto"}
                        <HugeiconsIcon
                          icon={CheckmarkCircle02Icon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {:else if veredicto === "no_apto"}
                        <HugeiconsIcon
                          icon={Cancel01Icon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {:else if veredicto === "pendiente"}
                        <HugeiconsIcon
                          icon={Alert02Icon}
                          size={12}
                          strokeWidth={2.3}
                        />
                      {:else if veredicto === "analizando"}
                        <span class="mini-spinner"></span>
                      {/if}
                      {ETIQUETA_VEREDICTO[veredicto]}
                    </span>
                    <div class="chips-meta-persona">
                      {#if candidato.desdeCache}
                        <span class="etiqueta-meta">Guardado</span>
                      {/if}
                      {#if candidato.urlDetalle.startsWith("correo:")}
                        <span
                          class="etiqueta-meta etiqueta-correo"
                          title="Llegó como hoja de vida por correo, no por Computrabajo"
                          >Correo</span
                        >
                      {:else}
                        <span
                          class="etiqueta-meta etiqueta-computrabajo"
                          title="Se postuló por Computrabajo"
                          >Computrabajo</span
                        >
                      {/if}
                      <span
                        class="etiqueta-orden"
                        title={candidato.urlDetalle.startsWith("correo:")
                          ? "Orden de llegada de la hoja de vida por correo"
                          : "Orden de postulación en Computrabajo"}
                      >
                        {candidato.urlDetalle.startsWith("correo:") ? "Hoja de vida" : "Postulado"} #{candidato.indice}
                      </span>
                    </div>
                  </div>

                  <div class="cabecera-persona">
                    <img
                      class="avatar"
                      src={candidato.fotoUrl}
                      alt={candidato.nombre}
                      width="50"
                      height="50"
                      loading="lazy"
                      decoding="async"
                    />
                    <div class="identidad">
                      <h4 title={candidato.nombre.toUpperCase()}>
                        {candidato.nombre.toUpperCase()}
                      </h4>
                      {#if !candidato.resultado && !candidato.cedula}
                        <span class="esqueleto esq-linea esq-corta"></span>
                      {:else}
                        <span class="subtitulo" in:fade={{ duration: 300 }}>
                          {candidato.cedula
                            ? `C.C. ${candidato.cedula}`
                            : "Sin cédula en perfil"}
                        </span>
                      {/if}
                    </div>
                    {#if !candidato.resultado}
                      <span class="esqueleto esq-anillo" aria-hidden="true"></span>
                    {:else if ia}
                      <div
                        class="anillo-score"
                        style="--p: {ia.score_total}"
                        title="Score IA: {ia.score_total} de 100 · {ia.dictamen}"
                      >
                        <div class="anillo-interior">
                          <strong>{ia.score_total}</strong>
                          <span>SCORE</span>
                        </div>
                      </div>
                    {/if}
                  </div>

                  <div
                    class="bloque-etapas"
                    aria-label="Estado de cada validación"
                  >
                    {#each ORDEN_ETAPAS as etapa (etapa)}
                      {@const e = candidato.etapas[etapa]}
                      {@const estadoEtapa = e?.estado ?? "pendiente"}
                      <div
                        class="item-etapa etapa-{estadoEtapa}"
                        title="{ETIQUETA_ETAPA[etapa]}: {e
                          ? (e.detalle ?? e.estado)
                          : 'pendiente'}"
                      >
                        <span class="punto punto-{estadoEtapa}"></span>
                        <span class="nombre-etapa"
                          >{ETIQUETA_CORTA_ETAPA[etapa]}</span
                        >
                      </div>
                    {/each}
                  </div>

                  {#if !candidato.resultado}
                    <div class="esq-bloque" aria-label="Cargando información del candidato">
                      <span class="esqueleto esq-linea"></span>
                      <span class="esqueleto esq-linea esq-media"></span>
                      <span class="esqueleto esq-linea esq-corta"></span>
                    </div>
                  {:else}
                    <p class="motivo motivo-{resumen.tipo}" in:fade={{ duration: 350 }}>
                      {resumen.texto}
                    </p>
                  {/if}

                  {#if ia}
                    <div class="sintesis-ia">
                      <span class="etiqueta-sintesis">Síntesis IA</span>
                      <p>{ia.por_que_es_el_hombre ?? ia.dictamen}</p>
                    </div>
                  {/if}

                  <div class="pie-persona">
                    <button
                      type="button"
                      class="boton-flujo boton-premium"
                      onclick={() =>
                        alAbrirFlujoCandidato(vacante.id, candidato.slug)}
                    >
                      <HugeiconsIcon
                        icon={WorkflowSquare10Icon}
                        size={14}
                        strokeWidth={2.2}
                      />
                      <span>Ver flujo</span>
                    </button>
                    {#if candidato.fase === "completo"}
                      <button
                        type="button"
                        class="boton-resultados"
                        onclick={() =>
                          alAbrirResultadosCandidato(vacante.id, candidato.slug)}
                      >
                        <HugeiconsIcon
                          icon={File02Icon}
                          size={14}
                          strokeWidth={2.2}
                        />
                        <span>Ver resultados</span>
                      </button>
                      {#if candidato.urlDetalle}
                        <button
                          type="button"
                          class="boton-descargar"
                          onclick={() =>
                            descargarZip(
                              urlExportarCandidato(vacante.ids_miembros ?? [vacante.id], candidato.urlDetalle),
                              `${candidato.nombre}.zip`,
                            )}
                          title="Descargar un .zip con su información (datos.txt y sus hojas de vida)"
                          aria-label="Descargar la información de {candidato.nombre}"
                        >
                          <HugeiconsIcon icon={Download01Icon} size={15} strokeWidth={2.2} />
                        </button>
                      {/if}
                    {/if}
                  </div>
              {/if}
            {/snippet}

            {#if candidatosCola.length > 0}
              <section
                class="seccion-cola"
                in:slide={{ duration: 400 }}
                out:slide={{ delay: 900, duration: 350 }}
              >
                <div class="titulo-seccion">
                  <h4>Por verificar <span class="contador-seccion">{candidatosCola.length}</span></h4>
                  <span class="ayuda-seccion">Al terminar pasan a Analizados</span>
                </div>
                <div class="cola-marco">
                  <button type="button" class="flecha-cola flecha-izq" aria-label="Ver anteriores" onclick={() => recorrerCola(-1)}>
                    <HugeiconsIcon icon={ChevronLeftIcon} size={16} strokeWidth={2.4} />
                  </button>
                  <div class="fila-cola" bind:this={filaCola} use:controlScroll>
                  {#each candidatosCola as candidato (candidato.slug)}
                <div
                  data-slug={candidato.slug}
                  data-fase={candidato.fase}
                  class="tarjeta-persona"
                  data-espera={esperaTurno(candidato)}
                  class:compacta={esCompacta(candidato)}
                  class:oculta={enCamino.has(candidato.slug)}
                  class:persona-activa={candidato.fase === "analizando"}
                  class:recien-apto={enReposo(candidato) && veredictoDe(candidato) === "apto"}
                  class:recien-no-apto={enReposo(candidato) && veredictoDe(candidato) === "no_apto"}
                  class:recien-pendiente={enReposo(candidato) && veredictoDe(candidato) === "pendiente"}
                  animate:acomodar={{ duration: 600, easing: cubicInOut }}
                  in:fly={{ y: 24, duration: 350, easing: cubicOut }}
                  out:fade={{ duration: 120 }}
                >
                  {@render cuerpoTarjeta(candidato, vacante)}
                </div>
                  {/each}
                  </div>
                  <button type="button" class="flecha-cola flecha-der" aria-label="Ver siguientes" onclick={() => recorrerCola(1)}>
                    <HugeiconsIcon icon={ChevronRightIcon} size={16} strokeWidth={2.4} />
                  </button>
                </div>
              </section>
            {/if}

            <section class="seccion-resultados">
              <div class="titulo-seccion">
                <h4>Analizados <span class="contador-seccion">{candidatosResultados.length}</span></h4>
              </div>
              <div class="grilla-candidatos">
                {#each resultadosVisibles as candidato (candidato.slug)}
                <div
                  data-slug={candidato.slug}
                  data-fase={candidato.fase}
                  class="tarjeta-persona"
                  data-espera={esperaTurno(candidato)}
                  class:compacta={esCompacta(candidato)}
                  class:oculta={enCamino.has(candidato.slug)}
                  class:persona-activa={candidato.fase === "analizando"}
                  class:recien-apto={enReposo(candidato) && veredictoDe(candidato) === "apto"}
                  class:recien-no-apto={enReposo(candidato) && veredictoDe(candidato) === "no_apto"}
                  class:recien-pendiente={enReposo(candidato) && veredictoDe(candidato) === "pendiente"}
                  animate:acomodar={{ duration: 600, easing: cubicInOut }}
                  in:fly={{ y: 24, duration: 350, easing: cubicOut }}
                  out:fade={{ duration: 120 }}
                >
                  {@render cuerpoTarjeta(candidato, vacante)}
                </div>
                {/each}
              </div>
              {#if candidatosResultados.length > resultadosVisibles.length}
                <button
                  type="button"
                  class="boton-secundario ver-mas"
                  onclick={() => (limiteResultados += TANDA_RESULTADOS)}
                >
                  Ver más ({candidatosResultados.length - resultadosVisibles.length} restantes)
                </button>
              {/if}
            </section>
          {/if}
        {/if}
      </article>
    {/each}
  </div>
</div>

<style>
  .estado-conexion,
  .edad-lista,
  .boton-actualizar {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #5c6b7a;
  }

  .estado-conexion {
    padding: 3px 10px;
    border-radius: 999px;
    background: #f1f4f7;
    font-weight: 600;
  }

  .punto-conexion {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #9aa5b1;
  }

  .conexion-en_vivo {
    background: #e8f8ea;
    color: #1f7a2e;
  }

  .conexion-en_vivo .punto-conexion {
    background: #2fb344;
    box-shadow: 0 0 0 3px rgba(47, 179, 68, 0.2);
  }

  .conexion-reconectando {
    background: #fff4e0;
    color: #8a5a00;
  }

  .conexion-reconectando .punto-conexion {
    background: #f0a020;
  }

  .boton-actualizar {
    border: none;
    background: none;
    padding: 3px 6px;
    border-radius: 8px;
    cursor: pointer;
  }

  .boton-actualizar:hover:not(:disabled) {
    background: #f1f4f7;
    color: #2c3e50;
  }

  .boton-actualizar:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .ver-mas {
    display: flex;
    margin: 14px auto 0;
  }

  .contenedor-vacantes {
    padding: 26px 32px 42px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .panel-superior {
    background: #ffffff;
    border-radius: 20px;
    padding: 14px 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 14px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .textos {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-width: 580px;
  }

  .fila-insignia {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .insignia-modulo {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
  }

  .boton-volver {
    padding: 3px 10px;
    border-radius: 999px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-volver:hover {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .titulo-seccion {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .bloque-busqueda {
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .kpis {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .kpi {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 12px;
    border-radius: 12px;
    background: var(--color-fondo-soft);
  }

  .kpi span {
    font-size: 10px;
    font-weight: 600;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .kpi strong {
    font-size: 14px;
    font-weight: 700;
  }

  .kpi-verde {
    background: rgba(var(--vs-primary-raw), 0.12);
  }

  .kpi-verde strong {
    color: var(--color-primario);
  }

  .buscador {
    width: 240px;
    padding: 9px 14px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
  }

  .caja-aviso {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-radius: 18px;
    background: rgba(245, 158, 11, 0.1);
    color: #92590b;
    font-size: 13px;
    font-weight: 500;
  }

  .caja-aviso p {
    flex: 1;
    line-height: 1.45;
  }

  .caja-error {
    background: rgba(239, 68, 68, 0.08);
    color: #b42323;
  }

  .caja-error div {
    flex: 1;
  }

  .caja-error strong {
    display: block;
    margin-bottom: 2px;
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
    line-height: 1.5;
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

  .lista-vacantes {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .tarjeta-vacante {
    background: #ffffff;
    border-radius: 26px;
    padding: 22px 26px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .cabecera-vacante {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 16px;
  }

  .info-vacante {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 0;
    flex: 1 1 320px;
  }

  .fila-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip {
    padding: 3px 10px;
    border-radius: 999px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    font-size: 11px;
    font-weight: 600;
  }

  .chip-id {
    font-family: ui-monospace, Consolas, monospace;
  }

  .chip-procesada {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .chip-excluida {
    background: rgba(239, 68, 68, 0.1);
    color: #c43030;
  }

  .titulo-vacante {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .boton-titulo {
    text-align: left;
    font: inherit;
    color: inherit;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-titulo:hover {
    color: var(--color-primario);
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
  }

  .meta-item {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .texto-suave {
    color: var(--color-texto-grey);
  }

  .acciones-vacante {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
  }

  .boton-analizar,
  .boton-flujo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px 18px;
    font-size: 12.5px;
    font-weight: 700;
  }

  .boton-flujo {
    flex: 1;
    padding: 9px 14px;
    font-size: 12px;
  }

  .boton-secundario {
    display: inline-flex;
    text-decoration: none;
    align-items: center;
    gap: 6px;
    padding: 9px 15px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-secundario:hover:not(:disabled) {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .boton-secundario:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .boton-detener {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 18px;
    border-radius: 14px;
    background: #ef4444;
    color: #ffffff;
    font-size: 12.5px;
    font-weight: 700;
    cursor: pointer;
    box-shadow: 0 10px 20px -10px rgba(239, 68, 68, 0.55);
    transition: var(--transicion-vuesax);
  }

  .boton-detener:hover {
    background: #dc2626;
  }

  .resumen-vacante {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px 18px;
    border-radius: 18px;
    background: var(--color-fondo-soft);
  }

  .fila-cifras {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .cifra {
    display: flex;
    flex-direction: column;
    min-width: 120px;
    padding: 8px 14px;
    border-radius: 14px;
    background: #ffffff;
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
    text-decoration: none;
    transition: var(--transicion-vuesax);
  }

  .boton-descargar:hover {
    background: var(--color-primario);
    color: #ffffff;
  }

  .cifra-enlace {
    border: 0;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .cifra-enlace:disabled {
    cursor: default;
  }

  .cifra-enlace:not(:disabled):hover {
    transform: translateY(-2px);
  }

  .cifra strong {
    font-size: 20px;
  }

  .cifra span {
    font-size: 10.5px;
    color: var(--color-texto-grey);
  }

  .cifra-ok strong {
    color: var(--color-primario);
  }

  .cifra-mal strong {
    color: #d14343;
  }

  .cifra-aviso strong {
    color: #c27a0a;
  }

  .sintesis-ia {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    border-radius: 14px;
    background: rgba(var(--vs-primary-raw), 0.07);
  }

  .anillo-score {
    margin-left: auto;
    flex-shrink: 0;
    width: 54px;
    height: 54px;
    border-radius: 50%;
    padding: 5px;
    background: conic-gradient(
      var(--color-primario) calc(var(--p) * 1%),
      rgba(var(--vs-primary-raw), 0.15) 0
    );
  }

  .cabecera-persona :global(.identidad),
  .identidad {
    min-width: 0;
  }

  .tarjeta-persona {
    display: flex;
    flex-direction: column;
  }

  .pie-persona {
    margin-top: auto;
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

  .etiqueta-sintesis {
    font-size: 9px;
    font-weight: 800;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-primario);
  }

  .sintesis-ia p {
    margin: 0;
    font-size: 10.5px;
    line-height: 1.4;
    color: var(--color-texto-dark);
    max-height: 4.2em;
    overflow-y: auto;
    padding-right: 4px;
    scrollbar-width: thin;
  }

  .franja-estado {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 18px;
    border-radius: 18px;
    background: var(--color-fondo-soft);
  }

  .franja-estado.estado-en_curso {
    background: rgba(var(--vs-primary-raw), 0.08);
  }

  .franja-estado.estado-error {
    background: rgba(239, 68, 68, 0.07);
  }

  .fila-estado {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
    font-size: 13px;
  }

  .fila-estado .mensaje {
    color: var(--color-texto-grey);
    font-weight: 500;
    flex: 1;
    min-width: 200px;
  }

  .barra-progreso {
    height: 8px;
    border-radius: 999px;
    background: rgba(44, 62, 80, 0.08);
    overflow: hidden;
  }

  .relleno {
    height: 100%;
    border-radius: inherit;
    background: var(--color-primario);
    transition: width 0.4s ease;
  }

  .contadores {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    font-size: 12px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .c-apto {
    color: var(--color-primario);
  }

  .c-no-apto {
    color: #d14343;
  }

  .c-pendiente {
    color: #c27a0a;
  }

  .titulo-seccion {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 12px;
  }

  .titulo-seccion h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  /* La tarjeta real espera oculta (guardando su sitio) mientras Adi trae su copia. */
  .oculta {
    visibility: hidden;
  }

  .contador-seccion {
    margin-left: 6px;
    padding: 2px 9px;
    border-radius: 999px;
    background: #ffffff;
    font-size: 11px;
    color: #6b7a89;
  }

  .ayuda-seccion {
    font-size: 11.5px;
    color: #8a97a4;
  }

  .seccion-resultados {
    margin-top: 8px;
  }

  /* Fila de espera: el que se analiza va primero, completo; los demás, compactos. El margen de arriba
     deja sitio a Adi, que se posa sobre la esquina de la tarjeta y no debe quedar recortado. */
  .fila-cola {
    display: flex;
    align-items: flex-start;
    gap: 16px;
    padding: 56px 12px 18px;
    margin: -44px -12px 0;
    overflow-x: auto;
    scrollbar-width: thin;
    scrollbar-color: rgba(var(--vs-primary-raw), 0.45) transparent;
    scroll-snap-type: x proximity;
    scroll-padding-inline: 12px;
    overscroll-behavior-x: contain;
    --fi: 0px;
    --fd: 0px;
    mask-image: linear-gradient(to right, transparent 0, #000 var(--fi), #000 calc(100% - var(--fd)), transparent 100%);
  }

  .fila-cola::-webkit-scrollbar {
    height: 8px;
  }

  .fila-cola::-webkit-scrollbar-thumb {
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.4);
  }

  .fila-cola::-webkit-scrollbar-track {
    background: transparent;
  }

  .cola-marco {
    position: relative;
  }

  .flecha-cola {
    position: absolute;
    top: 50%;
    z-index: 5;
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: #ffffff;
    color: var(--color-texto-dark);
    box-shadow: 0 8px 20px -6px rgba(44, 62, 80, 0.35);
    cursor: pointer;
    opacity: 0;
    pointer-events: none;
    transform: translateY(-50%) scale(0.8);
    transition: opacity 0.25s ease, transform 0.25s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .flecha-izq {
    left: -6px;
  }

  .flecha-der {
    right: -6px;
  }

  .flecha-cola > :global(svg) {
    display: block;
  }

  .cola-marco[data-izq="true"] .flecha-izq,
  .cola-marco[data-der="true"] .flecha-der {
    opacity: 1;
    pointer-events: auto;
    transform: translateY(-50%) scale(1);
  }

  .flecha-cola:hover {
    background: var(--color-primario);
    color: #ffffff;
  }

  .fila-cola > :global(.tarjeta-persona) {
    scroll-snap-align: start;
    flex: 0 0 340px;
    transition: flex-basis 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .fila-cola > :global(.tarjeta-persona.compacta) {
    flex-basis: 230px;
    gap: 10px;
    padding: 14px;
  }

  .grilla-candidatos {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
    gap: 16px;
    align-items: stretch;
  }

  .tarjeta-persona {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 18px;
    border-radius: 22px;
    background: var(--color-fondo-soft);
    border: 2px solid transparent;
    transition: var(--transicion-vuesax);
  }

  /* Al terminar, la tarjeta se queda un momento en su sitio resaltada con su resultado. */
  .recien-apto,
  .recien-no-apto,
  .recien-pendiente {
    background: #ffffff;
    transition: var(--transicion-vuesax), box-shadow 0.4s ease;
  }

  .recien-apto {
    border-color: var(--color-primario);
    box-shadow: 0 0 0 4px rgba(var(--vs-primary-raw), 0.18);
  }

  .recien-no-apto {
    border-color: #e05555;
    box-shadow: 0 0 0 4px rgba(224, 85, 85, 0.16);
  }

  .recien-pendiente {
    border-color: #e0a030;
    box-shadow: 0 0 0 4px rgba(224, 160, 48, 0.16);
  }

  /* Esqueletos: lo que aún no se conoce de una persona recién llegada. */
  .esqueleto {
    display: block;
    border-radius: 8px;
    background: linear-gradient(90deg, #e6ece8 25%, #f4f8f5 50%, #e6ece8 75%);
    background-size: 200% 100%;
    animation: brillo-esqueleto 1.3s linear infinite;
  }

  .esq-linea {
    height: 11px;
    width: 100%;
  }

  .esq-media {
    width: 75%;
  }

  .esq-corta {
    width: 45%;
  }

  .esq-anillo {
    flex-shrink: 0;
    width: 56px;
    height: 56px;
    border-radius: 50%;
  }

  .esq-bloque {
    display: flex;
    flex-direction: column;
    gap: 9px;
    padding: 12px 4px;
  }

  @keyframes brillo-esqueleto {
    to {
      background-position: -200% 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .esqueleto {
      animation: none;
    }
  }

  .persona-activa {
    background: #ffffff;
    border-color: var(--color-primario);
    box-shadow: 0 10px 28px -10px rgba(var(--vs-primary-raw), 0.35);
    animation: latido-activa 1.8s ease-in-out infinite;
  }

  @keyframes latido-activa {
    50% {
      box-shadow: 0 14px 36px -8px rgba(var(--vs-primary-raw), 0.6);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .persona-activa {
      animation: none;
    }
  }

  .fila-superior-persona {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .selector-orden {
    min-width: 200px;
    cursor: pointer;
  }

  .etiqueta-meta.etiqueta-computrabajo {
    background: #e6f6e9;
    color: #1f7a33;
    font-weight: 700;
  }

  .etiqueta-meta.etiqueta-correo {
    background: #e8f0fe;
    color: #1a56c4;
    font-weight: 700;
  }

  .chips-meta-persona {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .etiqueta-meta,
  .etiqueta-orden {
    padding: 3px 8px;
    border-radius: 999px;
    background: #ffffff;
    color: var(--color-texto-grey);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.02em;
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

  .motivo-descarte {
    background: rgba(239, 68, 68, 0.08);
    color: #b42323;
  }

  .pie-persona {
    margin-top: auto;
    display: flex;
    gap: 8px;
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
</style>
