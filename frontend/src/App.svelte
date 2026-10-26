<script lang="ts">
  import { onMount } from "svelte";
  import { fade, fly } from "svelte/transition";
  import toast, { Toaster } from "svelte-french-toast";
  import {
    EVENTO_SESION_EXPIRADA,
    servicioAutenticacionPorDefecto,
  } from "./dominios/autenticacion/servicios/servicioAutenticacion";
  import LayoutPanelPrincipal from "./compartido/layouts/LayoutPanelPrincipal.svelte";
  import {
    GRUPOS_NAVEGACION_SIDEBAR,
    RUTA_BASE_DESCARTADOS,
    RUTA_BASE_RECLUTAMIENTO_VACANTES,
    RUTA_BASE_VALIDACION_CANDIDATOS,
    type MigaDePan,
    type SubItemNavegacion,
  } from "./compartido/configuracion/navegacion.config";
  import VistaInicioSesion from "./dominios/autenticacion/vistas/VistaInicioSesion.svelte";
  import type { UsuarioAutenticado } from "./dominios/autenticacion/modelos/credenciales";
  import {
    almacenRrhh,
    etiquetaEstadoAnalisis,
  } from "./dominios/candidatos/servicios/almacenAnalisis.svelte";
  import {
    adaptarCandidato,
    adaptarVacante,
  } from "./dominios/candidatos/servicios/nodosEnVivo";
  import VistaValidacionDatosCandidato from "./dominios/candidatos/vistas/VistaValidacionDatosCandidato.svelte";
  import VistaVacantesEnVivo from "./dominios/candidatos/vistas/VistaVacantesEnVivo.svelte";
  import VistaValidacionCandidatosLogs from "./dominios/candidatos/vistas/VistaValidacionCandidatosLogs.svelte";
  import VistaResultadosCandidato from "./dominios/candidatos/vistas/VistaResultadosCandidato.svelte";
  import {
    buscarCandidatoReclutamientoEnVacante,
    buscarVacanteReclutamientoPorSlug,
  } from "./dominios/reclutamiento/modelos/modelosReclutamiento";
  import VistaDescartados from "./dominios/candidatos/vistas/VistaDescartados.svelte";
  import VistaVacantesReclutamiento from "./dominios/reclutamiento/vistas/VistaVacantesReclutamiento.svelte";
  import VistaDetalleVacanteReclutamiento from "./dominios/reclutamiento/vistas/VistaDetalleVacanteReclutamiento.svelte";
  import VistaExpedienteCandidatoReclutamiento from "./dominios/reclutamiento/vistas/VistaExpedienteCandidatoReclutamiento.svelte";

  // Ruta pública (sin sesión) a la que llega el candidato desde el correo.
  const RUTA_VALIDACION_DATOS = "/validar-datos";

  const itemPorDefecto = GRUPOS_NAVEGACION_SIDEBAR[0].subItems[0];

  let usuarioActivo = $state<UsuarioAutenticado | null>(null);
  let sesionVerificada = $state(false);
  let rutaNavegador = $state<string>("/login");
  let itemNavegacionActivo = $state<SubItemNavegacion>(itemPorDefecto);
  let totalEventosLog = $state<number>(0);
  let estadoMotor = $state<string>("Motor Orquestador Listo");

  type ModuloDashboardActivo = "candidatos_validacion" | "reclutamiento" | "descartados";

  /**
   * Analiza la URL actual y extrae el módulo activo y los slugs jerárquicos:
   * 1. Módulo Candidatos:
   *    - /dashboard/candidatos/validacion-logs
   *    - /dashboard/candidatos/validacion-logs/:vacanteSlug
   *    - /dashboard/candidatos/validacion-logs/:vacanteSlug/:candidatoSlug
   *    - /dashboard/candidatos/validacion-logs/:vacanteSlug/:candidatoSlug/resultados
   * 2. Módulo Reclutamiento:
   *    - /dashboard/reclutamiento/vacantes
   *    - /dashboard/reclutamiento/vacantes/:vacanteSlug
   *    - /dashboard/reclutamiento/vacantes/:vacanteSlug/:candidatoSlug
   */
  const estadoRutaSegmentada = $derived.by(() => {
    const limpia = rutaNavegador.replace(/\/+$/, "");
    if (!limpia.startsWith("/dashboard")) {
      return {
        esDashboard: false,
        modulo: "candidatos_validacion" as ModuloDashboardActivo,
        slugVacante: null as string | null,
        slugCandidato: null as string | null,
      };
    }

    if (limpia === RUTA_BASE_DESCARTADOS) {
      return {
        esDashboard: true,
        modulo: "descartados" as ModuloDashboardActivo,
        slugVacante: null,
        slugCandidato: null,
      };
    }

    // Rutas de Reclutamiento
    const prefijoReclutamiento = RUTA_BASE_RECLUTAMIENTO_VACANTES;
    if (
      limpia === "/dashboard/reclutamiento" ||
      limpia === prefijoReclutamiento
    ) {
      return {
        esDashboard: true,
        modulo: "reclutamiento" as ModuloDashboardActivo,
        slugVacante: null,
        slugCandidato: null,
      };
    }

    if (limpia.startsWith(`${prefijoReclutamiento}/`)) {
      const resto = limpia.slice(prefijoReclutamiento.length + 1);
      const partes = resto.split("/").filter(Boolean);
      return {
        esDashboard: true,
        modulo: "reclutamiento" as ModuloDashboardActivo,
        slugVacante: partes[0] ? decodeURIComponent(partes[0]) : null,
        slugCandidato: partes[1] ? decodeURIComponent(partes[1]) : null,
      };
    }

    // Rutas de Validación de Candidatos / Logs (n8n)
    const prefijoCandidatos = RUTA_BASE_VALIDACION_CANDIDATOS;
    if (limpia === "/dashboard" || limpia === prefijoCandidatos) {
      return {
        esDashboard: true,
        modulo: "candidatos_validacion" as ModuloDashboardActivo,
        slugVacante: null,
        slugCandidato: null,
      };
    }

    if (limpia.startsWith(`${prefijoCandidatos}/`)) {
      const resto = limpia.slice(prefijoCandidatos.length + 1);
      const partes = resto.split("/").filter(Boolean);
      return {
        esDashboard: true,
        modulo: "candidatos_validacion" as ModuloDashboardActivo,
        slugVacante: partes[0] ? decodeURIComponent(partes[0]) : null,
        slugCandidato: partes[1] ? decodeURIComponent(partes[1]) : null,
      };
    }

    return {
      esDashboard: true,
      modulo: "candidatos_validacion" as ModuloDashboardActivo,
      slugVacante: null,
      slugCandidato: null,
    };
  });

  // Tercer segmento "resultados": la página de resultados y score del candidato (en vez del flujo).
  const esVistaResultados = $derived(
    estadoRutaSegmentada.modulo === "candidatos_validacion" &&
      Boolean(estadoRutaSegmentada.slugCandidato) &&
      /\/resultados$/.test(rutaNavegador.replace(/\/+$/, "")),
  );

  // Cambia al entrar o salir de lista, vacante o detalle de candidato (no al alternar candidato).
  const claveVista = $derived(
    `${estadoRutaSegmentada.modulo}|${estadoRutaSegmentada.slugVacante ?? ""}|${estadoRutaSegmentada.slugCandidato ? (esVistaResultados ? "resultados" : "detalle") : ""}`,
  );

  // Derivados del módulo Candidatos: vacantes y candidatos reales (backend Go + análisis en vivo).
  // En esta ruta el primer segmento es el ID de la vacante en Computrabajo (el slug se repite entre
  // vacantes) y el segundo es "<indice>-<nombre>" de la persona.
  const idVacanteRuta = $derived(
    estadoRutaSegmentada.modulo === "candidatos_validacion"
      ? estadoRutaSegmentada.slugVacante
      : null,
  );

  const vacanteEnVivo = $derived(
    idVacanteRuta ? almacenRrhh.vacantePorId(idVacanteRuta) : undefined,
  );

  const sesionEnVivo = $derived(
    idVacanteRuta ? almacenRrhh.sesion(idVacanteRuta) : null,
  );

  const candidatoEnVivo = $derived(
    sesionEnVivo && estadoRutaSegmentada.slugCandidato
      ? sesionEnVivo.porSlug(estadoRutaSegmentada.slugCandidato)
      : undefined,
  );

  const vacanteAdaptada = $derived(
    vacanteEnVivo && sesionEnVivo
      ? adaptarVacante(vacanteEnVivo, sesionEnVivo.candidatos)
      : null,
  );

  const candidatoAdaptado = $derived(
    candidatoEnVivo ? adaptarCandidato(candidatoEnVivo) : null,
  );

  // Con la URL directa a una vacante o persona se cargan sus datos sin pasar por el listado.
  $effect(() => {
    if (
      estadoRutaSegmentada.esDashboard &&
      estadoRutaSegmentada.modulo === "candidatos_validacion"
    ) {
      void almacenRrhh.asegurarVacantes();
    }
  });

  $effect(() => {
    if (sesionEnVivo && almacenRrhh.vacantes.length > 0) {
      void sesionEnVivo.cargarGuardados();
    }
  });

  // Derivados del módulo Reclutamiento (Clasificación + Score IA)
  const vacanteReclutamientoActiva = $derived.by(() => {
    if (
      estadoRutaSegmentada.modulo === "reclutamiento" &&
      estadoRutaSegmentada.slugVacante
    ) {
      return (
        buscarVacanteReclutamientoPorSlug(estadoRutaSegmentada.slugVacante) ??
        null
      );
    }
    return null;
  });

  const expedienteCandidatoReclutamiento = $derived.by(() => {
    if (
      estadoRutaSegmentada.modulo === "reclutamiento" &&
      estadoRutaSegmentada.slugVacante &&
      estadoRutaSegmentada.slugCandidato
    ) {
      return buscarCandidatoReclutamientoEnVacante(
        estadoRutaSegmentada.slugVacante,
        estadoRutaSegmentada.slugCandidato,
      );
    }
    return null;
  });

  const migasDePanActivas = $derived.by((): MigaDePan[] => {
    if (estadoRutaSegmentada.modulo === "reclutamiento") {
      const migasReclutamiento: MigaDePan[] = [
        { etiqueta: "Dashboard", ruta: RUTA_BASE_RECLUTAMIENTO_VACANTES },
        { etiqueta: "Reclutamiento", ruta: RUTA_BASE_RECLUTAMIENTO_VACANTES },
        {
          etiqueta: "Vacantes y Ranking IA",
          ruta: RUTA_BASE_RECLUTAMIENTO_VACANTES,
        },
      ];

      if (vacanteReclutamientoActiva) {
        migasReclutamiento.push({
          etiqueta: vacanteReclutamientoActiva.titulo,
          ruta: `${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${vacanteReclutamientoActiva.slug}`,
        });
      }

      if (expedienteCandidatoReclutamiento) {
        migasReclutamiento.push({
          etiqueta: expedienteCandidatoReclutamiento.candidato.nombreCompleto,
        });
      }

      return migasReclutamiento;
    }

    if (estadoRutaSegmentada.modulo === "descartados") {
      return [
        { etiqueta: "Dashboard", ruta: RUTA_BASE_VALIDACION_CANDIDATOS },
        { etiqueta: "Candidatos", ruta: RUTA_BASE_VALIDACION_CANDIDATOS },
        { etiqueta: "Descartados", ruta: RUTA_BASE_DESCARTADOS },
      ];
    }

    const migas: MigaDePan[] = [
      { etiqueta: "Dashboard", ruta: RUTA_BASE_VALIDACION_CANDIDATOS },
      { etiqueta: "Candidatos", ruta: RUTA_BASE_VALIDACION_CANDIDATOS },
      {
        etiqueta: "Reclutamiento",
        ruta: RUTA_BASE_VALIDACION_CANDIDATOS,
      },
    ];

    if (vacanteEnVivo) {
      migas.push({
        etiqueta: vacanteEnVivo.titulo,
        ruta: `${RUTA_BASE_VALIDACION_CANDIDATOS}/${vacanteEnVivo.id}`,
      });
    }

    if (candidatoEnVivo) {
      migas.push({
        etiqueta: esVistaResultados
          ? `${candidatoEnVivo.nombre} — Resultados`
          : candidatoEnVivo.nombre,
      });
    }

    return migas;
  });

  const seccionEncabezado = $derived.by(() => {
    if (estadoRutaSegmentada.modulo === "reclutamiento") {
      return "Reclutamiento";
    }
    return itemNavegacionActivo.seccion;
  });

  const tituloEncabezado = $derived.by(() => {
    if (estadoRutaSegmentada.modulo === "reclutamiento") {
      if (expedienteCandidatoReclutamiento) {
        return `Expediente — ${expedienteCandidatoReclutamiento.candidato.nombreCompleto}`;
      }
      if (vacanteReclutamientoActiva) {
        return vacanteReclutamientoActiva.titulo;
      }
      return "Vacantes y Ranking IA";
    }

    if (candidatoEnVivo) {
      return `${esVistaResultados ? "Resultados" : "Flujo en vivo"} — ${candidatoEnVivo.nombre}`;
    }
    if (vacanteEnVivo) {
      return vacanteEnVivo.titulo;
    }
    return itemNavegacionActivo.etiqueta;
  });

  function sincronizarItemSidebarConRuta(ruta: string): void {
    for (const grupo of GRUPOS_NAVEGACION_SIDEBAR) {
      for (const sub of grupo.subItems) {
        if (ruta.startsWith(sub.ruta)) {
          itemNavegacionActivo = sub;
          return;
        }
      }
    }
  }

  function navegarHacia(nuevaRuta: string, reemplazar = false): void {
    const rutaNormalizada =
      nuevaRuta === "/dashboard" ? RUTA_BASE_VALIDACION_CANDIDATOS : nuevaRuta;
    rutaNavegador = rutaNormalizada;
    sincronizarItemSidebarConRuta(rutaNormalizada);
    if (reemplazar) {
      window.history.replaceState({}, "", rutaNormalizada);
    } else {
      window.history.pushState({}, "", rutaNormalizada);
    }
  }

  onMount(() => {
    const inicial = window.location.pathname;
    if (inicial === "/dashboard") {
      navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS, true);
    } else if (inicial.startsWith("/dashboard")) {
      rutaNavegador = inicial;
      sincronizarItemSidebarConRuta(inicial);
    } else if (inicial.startsWith(RUTA_VALIDACION_DATOS)) {
      rutaNavegador = inicial;
    } else {
      rutaNavegador = "/login";
    }

    const manejarPopState = () => {
      const actual = window.location.pathname;
      if (actual === "/dashboard") {
        navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS, true);
      } else {
        rutaNavegador = actual;
        sincronizarItemSidebarConRuta(actual);
      }
    };

    // La sesión vive en una cookie del servidor: se consulta antes de mostrar nada del panel.
    const manejarSesionExpirada = () => {
      if (usuarioActivo === null) return;
      usuarioActivo = null;
      navegarHacia("/login", true);
      toast.error("Tu sesión venció. Inicia sesión de nuevo.");
    };
    window.addEventListener(EVENTO_SESION_EXPIRADA, manejarSesionExpirada);

    if (rutaNavegador.startsWith(RUTA_VALIDACION_DATOS)) {
      sesionVerificada = true; // página pública del candidato: no usa sesión
    } else {
      void servicioAutenticacionPorDefecto.obtenerSesion().then((usuario) => {
        usuarioActivo = usuario;
        if (!usuario && rutaNavegador.startsWith("/dashboard")) {
          navegarHacia("/login", true);
        } else if (usuario && !rutaNavegador.startsWith("/dashboard")) {
          navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS, true);
        }
        sesionVerificada = true;
      });
    }

    window.addEventListener("popstate", manejarPopState);
    return () => {
      window.removeEventListener("popstate", manejarPopState);
      window.removeEventListener(EVENTO_SESION_EXPIRADA, manejarSesionExpirada);
    };
  });

  function manejarIngresoExitoso(usuario: UsuarioAutenticado): void {
    usuarioActivo = usuario;
    navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS);
  }

  function manejarSeleccionRuta(item: SubItemNavegacion): void {
    itemNavegacionActivo = item;
    navegarHacia(item.ruta);
  }

  function manejarCierreSesion(): void {
    usuarioActivo = null;
    navegarHacia("/login");
    void servicioAutenticacionPorDefecto.cerrarSesion();
  }
</script>

<Toaster
  position="top-right"
  toastOptions={{
    style:
      "border-radius: 16px; background: #ffffff; color: #2c3e50; font-family: Poppins, sans-serif; font-size: 13px; font-weight: 500; padding: 12px 16px; box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 15px 30px -5px rgba(0, 0, 0, 0.2);",
  }}
/>

{#if rutaNavegador.startsWith(RUTA_VALIDACION_DATOS)}
  <VistaValidacionDatosCandidato />
{:else if !sesionVerificada}
  <!-- Se espera la respuesta de /api/auth/yo para no mostrar el panel (ni pedir datos) sin sesión -->
{:else if !estadoRutaSegmentada.esDashboard}
  <VistaInicioSesion alAutenticarExitosamente={manejarIngresoExitoso} />
{:else}
  <LayoutPanelPrincipal
    rutaActiva={rutaNavegador}
    seccionActiva={seccionEncabezado}
    tituloActivo={tituloEncabezado}
    migasDePan={migasDePanActivas}
    nombreUsuario={usuarioActivo?.nombreCompleto ?? ""}
    rolUsuario={usuarioActivo?.cargo ?? ""}
    {estadoMotor}
    {totalEventosLog}
    alCambiarRuta={manejarSeleccionRuta}
    alNavegarMiga={(ruta) => navegarHacia(ruta)}
    alCerrarSesion={manejarCierreSesion}
  >
    {#key claveVista}
      <div
        class="transicion-vista"
        in:fly={{ y: 12, duration: 260, delay: 140 }}
        out:fade={{ duration: 140 }}
      >
        {#if estadoRutaSegmentada.modulo === "reclutamiento"}
          {#if expedienteCandidatoReclutamiento}
            <VistaExpedienteCandidatoReclutamiento
              vacante={expedienteCandidatoReclutamiento.vacante}
              candidato={expedienteCandidatoReclutamiento.candidato}
              alVolverAVacante={() =>
                navegarHacia(
                  `${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${expedienteCandidatoReclutamiento.vacante.slug}`,
                )}
              alCambiarCandidatoEnVacante={(slugCand) =>
                navegarHacia(
                  `${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${expedienteCandidatoReclutamiento.vacante.slug}/${slugCand}`,
                )}
            />
          {:else if vacanteReclutamientoActiva}
            <VistaDetalleVacanteReclutamiento
              vacante={vacanteReclutamientoActiva}
              alVolverAVacantes={() =>
                navegarHacia(RUTA_BASE_RECLUTAMIENTO_VACANTES)}
              alVerExpedienteCandidato={(slugCand) =>
                navegarHacia(
                  `${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${vacanteReclutamientoActiva.slug}/${slugCand}`,
                )}
            />
          {:else}
            <VistaVacantesReclutamiento
              alEntrarAVacante={(slugVac) =>
                navegarHacia(`${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${slugVac}`)}
              alEntrarACandidatoDirecto={(slugVac, slugCand) =>
                navegarHacia(
                  `${RUTA_BASE_RECLUTAMIENTO_VACANTES}/${slugVac}/${slugCand}`,
                )}
            />
          {/if}
        {:else if estadoRutaSegmentada.modulo === "descartados"}
          <VistaDescartados
            alAbrirFlujoCandidato={(idVac, slugCand) =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVac}/${slugCand}`,
              )}
            alAbrirResultadosCandidato={(idVac, slugCand) =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVac}/${slugCand}/resultados`,
              )}
            alIrAReclutamiento={() => navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS)}
          />
        {:else if esVistaResultados && sesionEnVivo && idVacanteRuta}
          <VistaResultadosCandidato
            candidato={candidatoEnVivo}
            cargando={almacenRrhh.cargando || sesionEnVivo.cargandoGuardados}
            alVolver={() =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVacanteRuta}`,
              )}
            alVerFlujo={() =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVacanteRuta}/${estadoRutaSegmentada.slugCandidato}`,
              )}
          />
        {:else if vacanteAdaptada && candidatoAdaptado && candidatoEnVivo && sesionEnVivo}
          <VistaValidacionCandidatosLogs
            vacanteActiva={vacanteAdaptada}
            candidatoActivo={candidatoAdaptado}
            enVivo={{
              candidato: candidatoEnVivo,
              analisisActivo: sesionEnVivo.activo,
              etiquetaEstado: etiquetaEstadoAnalisis(sesionEnVivo.estado),
              alDetenerAnalisis: () => void sesionEnVivo.detener(),
            }}
            alVolverAVacantes={() =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${vacanteAdaptada.slug}`,
              )}
            alCambiarCandidatoEnVacante={(slugVac, slugCand) =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${slugVac}/${slugCand}`,
              )}
            alActualizarContadorLogs={(c) => (totalEventosLog = c)}
            alActualizarEstadoMotor={(e) => (estadoMotor = e)}
          />
        {:else}
          <VistaVacantesEnVivo
            idVacanteFiltrada={estadoRutaSegmentada.slugVacante}
            alSeleccionarVacante={(idVac) =>
              navegarHacia(`${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVac}`)}
            alVerTodasLasVacantes={() =>
              navegarHacia(RUTA_BASE_VALIDACION_CANDIDATOS)}
            alVerDescartados={() => navegarHacia(RUTA_BASE_DESCARTADOS)}
            alAbrirFlujoCandidato={(idVac, slugCand) =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVac}/${slugCand}`,
              )}
            alAbrirResultadosCandidato={(idVac, slugCand) =>
              navegarHacia(
                `${RUTA_BASE_VALIDACION_CANDIDATOS}/${idVac}/${slugCand}/resultados`,
              )}
          />
        {/if}
      </div>
    {/key}
  </LayoutPanelPrincipal>
{/if}

<style>
  /* Las vistas entrante y saliente se apilan en la misma celda para evitar saltos de layout. */
  :global(.contenido-vista) {
    display: grid !important;
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr);
  }

  .transicion-vista {
    grid-area: 1 / 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
  }
</style>
