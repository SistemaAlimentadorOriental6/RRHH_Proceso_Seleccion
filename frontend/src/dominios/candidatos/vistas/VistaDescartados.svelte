<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { flip } from "svelte/animate";
  import { fade, fly, slide } from "svelte/transition";
  import { cubicInOut, cubicOut } from "svelte/easing";
  import toast from "svelte-french-toast";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import { Cancel01Icon, Leaf01Icon, ReloadIcon, Alert02Icon } from "@hugeicons/core-free-icons";
  import { destinoMenu, llevarTarjeta } from "../servicios/descarteAnimado.svelte";
  import InfoParametros from "../../configuracion/componentes/InfoParametros.svelte";
  import type { CandidatoEnVivo } from "../modelos/analisisEnVivo";
  import { almacenRrhh, veredictoDe } from "../servicios/almacenAnalisis.svelte";
  import { reanalizarDescartados, type ResumenReanalisis } from "../servicios/reanalisis";
  import AvatarAnalisis from "../componentes/AvatarAnalisis.svelte";
  import TarjetaPersona from "../componentes/TarjetaPersona.svelte";

  interface Propiedades {
    /** descartados: los que el análisis descartó; semillero: los que no cumplen alguna regla pero se guardan para después. */
    tipo?: "descartados" | "semillero" | "pendientes";
    alAbrirFlujoCandidato: (idVacante: string, slugCandidato: string) => void;
    alAbrirResultadosCandidato: (idVacante: string, slugCandidato: string) => void;
    alIrAReclutamiento: () => void;
  }
  let { tipo = "descartados", alAbrirFlujoCandidato, alAbrirResultadosCandidato, alIrAReclutamiento }: Propiedades = $props();

  const esSemillero = $derived(tipo === "semillero");
  const esPendientes = $derived(tipo === "pendientes");
  const esDescartados = $derived(tipo === "descartados");
  const textos = $derived(
    esPendientes
      ? {
          titulo: "Pendientes",
          insignia: "Verificación pendiente",
          resumen:
            "No se pudo verificar su cédula o su RUNT. Si faltan datos se le piden al candidato; cuando los completa, Adi lo lleva a donde corresponde.",
          nombre: "pendientes",
        }
      : esSemillero
      ? {
          titulo: "Semillero",
          insignia: "Para futuras vacantes",
          resumen: "No cumplen alguna regla (edad mínima, EPS, RUNT o SIMIT), pero nada los descarta.",
          nombre: "semilleros",
        }
      : {
          titulo: "Descartados",
          insignia: "Rechazados por el análisis",
          resumen: "Hojas de vida que no cumplen los parámetros de aceptación. Las nuevas llegan aquí solas.",
          nombre: "descartados",
        },
  );

  let textoBusqueda = $state("");

  onMount(() => {
    void almacenRrhh.asegurarVacantes();
    void almacenRrhh.cargarReanalisis();
  });

  // Cada vacante (o el grupo Operador / Conductor) lee lo ya guardado de sus candidatos.
  $effect(() => {
    if (almacenRrhh.vacantes.length === 0) return;
    for (const v of almacenRrhh.vacantes) void almacenRrhh.sesion(v.id).cargarGuardados();
  });

  interface Persona {
    vacanteId: string;
    idsVacantes: string[];
    vacanteTitulo: string;
    candidato: CandidatoEnVivo;
  }

  // ---- Segunda oportunidad (solo Descartados) ----
  let resumen = $state<ResumenReanalisis | null>(null);
  let consultando = $state(false);
  /** Postulaciones en la cola de reanálisis: es la del servidor, la misma para todos los usuarios. */
  const enCola = almacenRrhh.reanalisis;
  /** En cola y ya vistas reanalizándose: su tarjeta se queda aquí hasta que Adi la lleva a su nuevo lugar. */
  const rescatando = new SvelteSet<string>();
  /** Su copia va en brazos de Adi: la tarjeta real se oculta para no verse dos veces. */
  const volando = new SvelteSet<string>();
  const programados = new Set<string>();
  const idDe = (c: CandidatoEnVivo) => c.resultado?.postulacion_id ?? "";

  // Quien se está reanalizando sigue en la lista hasta que termina: si sale semillero o reclutado, Adi se lo lleva.
  const personas = $derived.by((): Persona[] => {
    const buscado = esSemillero ? "semillero" : esPendientes ? "pendiente" : "no_apto";
    const lista: Persona[] = [];
    for (const v of almacenRrhh.vacantes) {
      for (const candidato of almacenRrhh.sesion(v.id).candidatos) {
        const veredicto = veredictoDe(candidato);
        const id = idDe(candidato);
        // Pendientes: quien completó sus datos se reanaliza y se queda hasta que Adi se lo lleva.
        const reanalizando =
          !esSemillero && (rescatando.has(id) || (esDescartados && enCola.has(id) && veredicto === "analizando"));
        if (veredicto === buscado || reanalizando) {
          lista.push({ vacanteId: v.id, idsVacantes: v.ids_miembros ?? [v.id], vacanteTitulo: v.titulo, candidato });
        }
      }
    }
    // Los últimos en llegar primero; los que vienen de antes, por su orden.
    return lista.sort(
      (a, b) => (b.candidato.terminadoEn ?? 0) - (a.candidato.terminadoEn ?? 0) || a.candidato.indice - b.candidato.indice,
    );
  });

  const visibles = $derived.by(() => {
    const termino = textoBusqueda.trim().toLowerCase();
    if (!termino) return personas;
    return personas.filter(({ candidato: c }) => c.nombre.toLowerCase().includes(termino) || c.cedula.includes(termino));
  });

  const cargando = $derived(almacenRrhh.cargando || almacenRrhh.vacantes.some((v) => almacenRrhh.sesion(v.id).cargandoGuardados));
  const vacantesConPersonas = $derived(new Set(personas.map((p) => p.vacanteId)).size);

  // El avatar trabaja mientras alguien de la lista se reanaliza (y un momento después, para reaccionar al
  // resultado) y duerme el resto del tiempo.
  const reanalizandoAhora = $derived(personas.some((p) => p.candidato.fase === "analizando") || rescatando.size > 0);
  let trabajando = $state(false);
  $effect(() => {
    if (reanalizandoAhora) {
      trabajando = true;
      return;
    }
    const t = setTimeout(() => (trabajando = false), 4000);
    return () => clearTimeout(t);
  });
  const sesionAvatar = $derived({
    activo: trabajando,
    estado: trabajando ? "en_curso" : "inactivo",
    candidatos: personas.map((p) => p.candidato),
  });

  // Un descartado en cola que termina su reanálisis: si ahora cumple (reclutado P1) o queda para semillero,
  // Adi lleva su tarjeta a esa entrada del menú; si sigue descartado, se queda aquí.
  $effect(() => {
    for (const { candidato: c } of personas) {
      const id = idDe(c);
      // Pendientes: todo el que se ve pendiente queda vigilado; si cambia de veredicto, Adi lo lleva.
      if (esPendientes && id && veredictoDe(c) === "pendiente") rescatando.add(id);
      if (!enCola.has(id) && !rescatando.has(id)) continue;
      if (c.fase === "analizando") {
        rescatando.add(id);
      } else if (
        c.fase === "completo" &&
        rescatando.has(id) &&
        !programados.has(id) &&
        (!esPendientes || veredictoDe(c) !== "pendiente")
      ) {
        programados.add(id);
        setTimeout(() => terminarRescate(c), 1500); // que se alcance a ver el resultado en la tarjeta
      }
    }
  });

  function terminarRescate(c: CandidatoEnVivo): void {
    const id = idDe(c);
    const veredicto = veredictoDe(c);
    const salir = () => {
      rescatando.delete(id);
      volando.delete(id);
      programados.delete(id);
    };
    // Un pendiente que ya se verificó puede terminar descartado; un descartado reanalizado, no.
    const aDescartados = esPendientes && veredicto === "no_apto";
    if (veredicto !== "apto" && veredicto !== "semillero" && !aDescartados) {
      salir(); // sigue descartado (o pendiente): su tarjeta se queda donde está
      return;
    }
    const destino = veredicto === "apto" ? "validacion-logs" : veredicto === "semillero" ? "semillero" : "descartados";
    const origen = esPendientes ? "ya se verificó:" : "ya no está descartado:";
    toast.success(
      veredicto === "apto"
        ? esPendientes
          ? `${c.nombre} completó sus datos: ahora es Reclutado P1.`
          : `${c.nombre} salió del lado oscuro: ahora es Reclutado P1.`
        : veredicto === "semillero"
          ? `${c.nombre} ${origen} pasa al Semillero.`
          : `${c.nombre} ${origen} pasa a Descartados.`,
    );
    const tarjeta = document.querySelector<HTMLElement>(`.grilla-candidatos [data-postulacion="${CSS.escape(id)}"]`);
    const resultado = tarjeta
      ? llevarTarjeta(tarjeta, aDescartados ? "descartar" : "rescatar", () => destinoMenu(destino), salir)
      : "sin-animacion";
    if (resultado === "ocupado") {
      setTimeout(() => terminarRescate(c), 600); // Adi lleva otra: espera su turno
      return;
    }
    if (resultado === "iniciado") volando.add(id);
    else salir();
  }

  async function calcular(): Promise<void> {
    consultando = true;
    try {
      resumen = await reanalizarDescartados(true);
      if (resumen.reanalizar.length === 0) {
        toast(`Nadie cambiaría con los parámetros v${resumen.version}: no hay a quién reanalizar.`, { icon: "ℹ️" });
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "No se pudo calcular a quién reanalizar.");
    } finally {
      consultando = false;
    }
  }

  async function confirmar(): Promise<void> {
    consultando = true;
    try {
      const r = await reanalizarDescartados(false);
      await almacenRrhh.cargarReanalisis();
      resumen = null;
      toast.success(
        r.encolados > 0
          ? `${r.encolados} ${r.encolados === 1 ? "persona quedó" : "personas quedaron"} en cola con los parámetros v${r.version}. Los resultados llegan solos.`
          : "Ya estaban en cola: se reanalizan en orden.",
      );
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "No se pudo poner en cola el reanálisis.");
    } finally {
      consultando = false;
    }
  }
</script>

<div class="contenedor-vacantes" class:semillero={esSemillero} class:pendientes={esPendientes}>
  <section class="panel-superior">
    <div class="textos">
      <div class="fila-insignia">
        <span class="insignia-modulo">
          <HugeiconsIcon icon={esSemillero ? Leaf01Icon : esPendientes ? Alert02Icon : Cancel01Icon} size={13} strokeWidth={2.2} />
          <span>{textos.insignia}</span>
        </span>
      </div>
      <h2 class="titulo-seccion">{textos.titulo}</h2>
    </div>

    <div class="bloque-busqueda">
      <div class="kpis">
        <div class="kpi kpi-color">
          <span>{textos.titulo}</span>
          <strong>{personas.length}</strong>
        </div>
        <div class="kpi">
          <span>Vacantes</span>
          <strong>{vacantesConPersonas}</strong>
        </div>
      </div>
      <input type="search" class="buscador input-premium" placeholder="Buscar por nombre o cédula" bind:value={textoBusqueda} />
      {#if esDescartados}
        <InfoParametros />
        <button
          type="button"
          class="boton-premium boton-reanalizar"
          disabled={consultando || personas.length === 0}
          title="Calcula a quién vale la pena reanalizar con los parámetros vigentes; nada se consulta hasta que confirmes"
          onclick={calcular}
        >
          <HugeiconsIcon icon={ReloadIcon} size={14} strokeWidth={2.2} />
          <span>{consultando && !resumen ? "Calculando…" : "Reanalizar con parámetros nuevos"}</span>
        </button>
      {/if}
    </div>
  </section>

  {#if resumen && resumen.reanalizar.length > 0}
    <section class="caja-reanalisis" transition:slide={{ duration: 300 }} aria-label="Reanalizar descartados">
      <div class="encabezado-reanalisis">
        <HugeiconsIcon icon={Alert02Icon} size={18} strokeWidth={2} />
        <div>
          <strong>
            {resumen.reanalizar.length} de {resumen.total} descartados podrían cambiar con los parámetros v{resumen.version}
          </strong>
          <p>
            Solo se reanaliza a quien podría salir distinto. A los demás no se les consulta nada: con lo que ya se sabe de ellos
            seguirían descartados.
          </p>
        </div>
      </div>
      {#if resumen.omitidos.length > 0}
        <div class="omitidos">
          {#each resumen.omitidos as o (o.motivo)}
            <span class="chip-omitido"><strong>{o.cantidad}</strong> {o.motivo}</span>
          {/each}
        </div>
      {/if}
      <ul class="lista-reanalizar">
        {#each resumen.reanalizar.slice(0, 8) as p (p.postulacion_id)}
          <li>
            <strong>{p.nombre}</strong>
            <span>{p.regla_version ? `descartado con v${p.regla_version}` : "descartado antes de las versiones"} · {p.motivo}</span>
          </li>
        {/each}
        {#if resumen.reanalizar.length > 8}
          <li class="mas">y {resumen.reanalizar.length - 8} más…</li>
        {/if}
      </ul>
      <div class="acciones-reanalisis">
        <button type="button" class="boton-secundario" disabled={consultando} onclick={() => (resumen = null)}>Cancelar</button>
        <button type="button" class="boton-premium boton-reanalizar" disabled={consultando} onclick={confirmar}>
          <HugeiconsIcon icon={ReloadIcon} size={14} strokeWidth={2.2} />
          <span>{consultando ? "Poniendo en cola…" : `Reanalizar a ${resumen.reanalizar.length}`}</span>
        </button>
      </div>
    </section>
  {/if}

  <article class="tarjeta-vacante">
    <div class="cabecera-vacante">
      {#key tipo}
        <AvatarAnalisis sesion={sesionAvatar} color={esSemillero ? "#3b82f6" : esPendientes ? "#f59e0b" : "#ef4444"} />
      {/key}
      <div class="info-vacante">
        <div class="fila-chips">
          <span class="chip chip-color">{personas.length} {textos.nombre}</span>
          {#if enCola.size > 0 && esDescartados}
            <span class="chip chip-cola">{enCola.size} en cola para reanalizar</span>
          {/if}
        </div>
        <p class="texto-suave">{textos.resumen}</p>
      </div>
    </div>

    {#if visibles.length === 0}
      <div class="estado-vacio" in:fade={{ duration: 250 }}>
        {#if cargando}
          <span class="spinner"></span>
          <p>Cargando {textos.nombre}…</p>
        {:else if personas.length === 0}
          <p>Todavía no hay {textos.nombre}.</p>
          <button type="button" class="boton-secundario" onclick={alIrAReclutamiento}>Ir a la validación de candidatos</button>
        {:else}
          <p>Ninguno de los {textos.nombre} coincide con la búsqueda.</p>
        {/if}
      </div>
    {:else}
      <div class="grilla-candidatos">
        {#each visibles as { vacanteId, idsVacantes, vacanteTitulo, candidato } (`${vacanteId}|${candidato.slug}`)}
          <div
            class="tarjeta-persona"
            class:persona-activa={candidato.fase === "analizando"}
            class:oculta={volando.has(idDe(candidato))}
            data-postulacion={idDe(candidato)}
            data-slug={candidato.slug}
            data-fase={candidato.fase}
            animate:flip={{ duration: 600, easing: cubicInOut }}
            in:fly={{ y: 24, duration: 350, easing: cubicOut }}
            out:fade={{ duration: 120 }}
          >
            <TarjetaPersona
              {candidato}
              {vacanteTitulo}
              {idsVacantes}
              enCola={enCola.has(idDe(candidato))}
              alAbrirFlujo={() => alAbrirFlujoCandidato(vacanteId, candidato.slug)}
              alAbrirResultados={() => alAbrirResultadosCandidato(vacanteId, candidato.slug)}
            />
          </div>
        {/each}
      </div>
    {/if}
  </article>
</div>

<style>
  /* Mismo diseño que la validación de candidatos (VistaVacantesEnVivo); el acento es rojo o azul. */
  .contenedor-vacantes {
    --acento: 239, 68, 68;
    --acento-texto: #d14343;
    padding: 26px 32px 42px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .contenedor-vacantes.pendientes {
    --acento: 245, 158, 11;
    --acento-texto: #b45309;
  }

  .contenedor-vacantes.semillero {
    --acento: 59, 130, 246;
    --acento-texto: #2563eb;
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
    background: rgba(var(--acento), 0.12);
    color: var(--acento-texto);
    font-size: 11px;
    font-weight: 700;
  }

  .titulo-seccion {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .bloque-busqueda {
    display: flex;
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

  .kpi-color {
    background: rgba(var(--acento), 0.1);
  }

  .kpi-color strong {
    color: var(--acento-texto);
  }

  .buscador {
    width: 240px;
    padding: 9px 14px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
  }

  .boton-reanalizar {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px 18px;
    font-size: 12.5px;
    font-weight: 700;
  }

  .boton-reanalizar:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .caja-reanalisis {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 22px;
    border-radius: 22px;
    background: #ffffff;
    border: 2px solid rgba(var(--vs-primary-raw), 0.35);
    box-shadow: 0 12px 32px -12px rgba(var(--vs-primary-raw), 0.25);
  }

  .encabezado-reanalisis {
    display: flex;
    gap: 12px;
    color: var(--color-primario);
  }

  .encabezado-reanalisis strong {
    display: block;
    font-size: 14px;
    color: var(--color-texto-dark);
  }

  .encabezado-reanalisis p {
    margin-top: 3px;
    font-size: 12px;
    line-height: 1.45;
    color: var(--color-texto-grey);
  }

  .omitidos {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .chip-omitido {
    padding: 4px 10px;
    border-radius: 999px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    font-size: 11px;
    font-weight: 600;
  }

  .chip-omitido strong {
    color: var(--color-texto-dark);
  }

  .lista-reanalizar {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 6px 16px;
  }

  .lista-reanalizar li {
    display: flex;
    flex-direction: column;
    gap: 1px;
    font-size: 12px;
    min-width: 0;
  }

  .lista-reanalizar span {
    font-size: 11px;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .lista-reanalizar .mas {
    color: var(--color-texto-grey);
  }

  .acciones-reanalisis {
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 10px;
  }

  .boton-secundario {
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

  .chip-color {
    background: rgba(var(--acento), 0.1);
    color: var(--acento-texto);
  }

  .chip-cola {
    background: rgba(245, 158, 11, 0.14);
    color: #b9700a;
  }

  .texto-suave {
    font-size: 12.5px;
    color: var(--color-texto-grey);
    line-height: 1.45;
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

  .spinner {
    display: inline-block;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: 3px solid rgba(var(--vs-primary-raw), 0.2);
    border-top-color: var(--color-primario);
    animation: girar 0.8s linear infinite;
  }

  @keyframes girar {
    to {
      transform: rotate(360deg);
    }
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

  .tarjeta-persona:hover {
    background: #ffffff;
    border-color: rgba(var(--acento), 0.3);
    box-shadow: 0 10px 28px -12px rgba(var(--acento), 0.3);
  }

  /* La tarjeta real espera oculta mientras Adi lleva su copia. */
  .oculta {
    visibility: hidden;
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
    .persona-activa,
    .spinner {
      animation: none;
    }
  }

  @media (max-width: 640px) {
    .contenedor-vacantes {
      padding: 16px;
    }

    .buscador {
      width: 100%;
    }

    .grilla-candidatos {
      grid-template-columns: 1fr;
    }
  }
</style>
