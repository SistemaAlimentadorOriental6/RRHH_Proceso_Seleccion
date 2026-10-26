<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    PlayIcon,
    StopIcon,
    ReloadIcon,
    CheckmarkCircle02Icon,
    Alert02Icon,
    Cancel01Icon,
    SparklesIcon,
    EyeIcon
  } from '@hugeicons/core-free-icons';
  import toast from 'svelte-french-toast';
  import { tick, untrack } from 'svelte';
  import {
    ESCENARIOS_SIMULACION,
    NODOS_BASE_FLUJO_VALIDACION,
    type DefinicionNodoValidacion,
    type ModoSimulacionNodo
  } from '../modelos/flujoValidacion';
  import {
    CATALOGO_VACANTES_EVALUACION,
    construirEscenarioDesdeCandidato,
    type CandidatoEvaluadoEnVacante,
    type VacanteConCandidatos
  } from '../modelos/catalogoVacantesCandidatos';
  import {
    MotorFlujoValidacion,
    construirNodosIniciales
  } from '../servicios/motorFlujoValidacion';
  import type { CandidatoEnVivo } from '../modelos/analisisEnVivo';
  import {
    construirNodosEnVivo,
    esCandidatoDePrueba,
    type ContactoDetectado,
    type EnvioCorreo
  } from '../servicios/nodosEnVivo';
  import { obtenerEstadoSolicitud, type EstadoSolicitudApi } from '../servicios/clienteRrhh';
  import { almacenRrhh } from '../servicios/almacenAnalisis.svelte';
  import NodoFlujoN8n from '../componentes/NodoFlujoN8n.svelte';
  import ModalDetalleNodoValidacion from '../componentes/ModalDetalleNodoValidacion.svelte';
  import ModalFotoCandidato from '../componentes/ModalFotoCandidato.svelte';

  interface Propiedades {
    vacanteActiva?: VacanteConCandidatos;
    candidatoActivo?: CandidatoEvaluadoEnVacante;
    alVolverAVacantes?: () => void;
    alCambiarCandidatoEnVacante?: (slugVacante: string, slugCandidato: string) => void;
    alActualizarContadorLogs?: (cantidad: number) => void;
    alActualizarEstadoMotor?: (estado: string) => void;
    /**
     * Modo real: el lienzo muestra el análisis que hace el backend (SSE) en vez de la simulación.
     * `vacanteActiva`/`candidatoActivo` llegan adaptados desde los mismos datos reales.
     */
    enVivo?: {
      candidato: CandidatoEnVivo;
      analisisActivo: boolean;
      /** Texto del estado del análisis de la vacante (p. ej. "Análisis completado"). */
      etiquetaEstado: string;
      alDetenerAnalisis: () => void;
    };
  }

  let {
    vacanteActiva = CATALOGO_VACANTES_EVALUACION[0],
    candidatoActivo = CATALOGO_VACANTES_EVALUACION[0].candidatos[0],
    alVolverAVacantes,
    alCambiarCandidatoEnVacante,
    alActualizarContadorLogs,
    alActualizarEstadoMotor,
    enVivo
  }: Propiedades = $props();

  const ANCHO_NODO = 282;
  // Altura (en el mundo) a la que se anclan los puertos: alineada con la fila icono + título del nodo
  const Y_PUERTO = 71;
  const ALTO_ESTIMADO_NODO = 300;
  const ZOOM_MIN = 0.3;
  const ZOOM_MAX = 1.35;

  const escenarioActual = $derived(
    construirEscenarioDesdeCandidato(vacanteActiva, candidatoActivo)
  );

  let nodos = $state<DefinicionNodoValidacion[]>(
    construirNodosIniciales(ESCENARIOS_SIMULACION[0].configuracionNodos)
  );
  let idNodoSeleccionado = $state<string>('obtencion-cv');
  let idNodoActivoEjecucion = $state<string | null>(null);
  let ejecucionEnCurso = $state(false);
  let velocidad = $state<number>(1);
  let seguimientoCamaraActivo = $state<boolean>(true);
  let totalEventosEmitidos = $state<number>(0);
  let dictamenPipeline = $state<
    'en_espera' | 'ejecutando' | 'apto_aprobado' | 'detenido_critico'
  >('en_espera');

  // Rama de contacto de la persona de prueba: el backend hace todo por su cuenta (detecta el contacto, envía
  // el correo y el WhatsApp, recibe el formulario y reanuda). Aquí solo se LEE lo que ya pasó, al abrir el
  // candidato y cada vez que el backend avisa un cambio; abrir el flujo no dispara ningún envío.
  let contactoDetectado = $state<ContactoDetectado | undefined>(undefined);

  const claveContacto = $derived.by(() => {
    const c = enVivo?.candidato;
    const id = c?.resultado?.postulacion_id;
    if (!c || !id || !esCandidatoDePrueba(c)) return null;
    return `${id}|${almacenRrhh.versionCambios}`;
  });

  const canalDe = (estado: EstadoSolicitudApi['envio_correo'], medio: string): EnvioCorreo | undefined =>
    estado === 'enviada'
      ? { estado: 'enviado', yaEnviado: false }
      : estado === 'fallida'
        ? { estado: 'error', mensaje: `No fue posible enviar el ${medio}` }
        : estado === 'pendiente'
          ? { estado: 'enviando' }
          : undefined;

  $effect(() => {
    const clave = claveContacto;
    if (!clave) {
      untrack(() => (contactoDetectado = undefined));
      return;
    }
    const idPostulacion = clave.split('|')[0];
    const control = new AbortController();
    untrack(() => {
      if (!contactoDetectado) contactoDetectado = { estado: 'cargando' };
    });
    obtenerEstadoSolicitud(idPostulacion, control.signal)
      .then((e) => {
        contactoDetectado = e.existe
          ? {
              estado: 'listo',
              correo: e.correo,
              telefono: e.telefono,
              envio: canalDe(e.envio_correo, 'correo'),
              envioWhatsapp: canalDe(e.envio_whatsapp, 'WhatsApp'),
              formulario: { completado: e.completada, cedula: e.cedula, edad: e.edad }
            }
          : {
              estado: 'error',
              mensaje: 'Este candidato aún no pasó por la rama de contacto: vuelve a analizar la vacante con «forzar».'
            };
      })
      .catch((e: unknown) => {
        if (e instanceof DOMException && e.name === 'AbortError') return;
        contactoDetectado = { estado: 'error', mensaje: e instanceof Error ? e.message : 'No se pudo leer la solicitud' };
      });
    return () => control.abort();
  });

  const nodosEnVivo = $derived(
    enVivo ? construirNodosEnVivo(enVivo.candidato, contactoDetectado) : null
  );
  let slugEnVivoPrevio: string | null = null;
  let nodoEnfocadoPrevio: string | null = null;

  // Modo real: los nodos reflejan lo que informa el backend. Se conserva la posición que el usuario
  // le dio a cada tarjeta mientras sigue en la misma persona; al cambiar de persona se reencuadra.
  $effect(() => {
    const vivos = nodosEnVivo;
    const actual = enVivo?.candidato;
    if (!vivos || !actual) return;

    untrack(() => {
      const cambioPersona = actual.slug !== slugEnVivoPrevio;
      const posiciones = new Map(
        nodos.map((n) => [n.id, { x: n.posicionX, y: n.posicionY }])
      );
      nodos = vivos.map((n) => {
        const pos = cambioPersona ? undefined : posiciones.get(n.id);
        return pos ? { ...n, posicionX: pos.x, posicionY: pos.y } : n;
      });

      const activo = vivos.find(
        (n) => n.estado === 'ejecutando' || n.estado === 'reintentando'
      );
      idNodoActivoEjecucion = activo?.id ?? null;
      ejecucionEnCurso = actual.fase === 'analizando';
      dictamenPipeline =
        actual.fase === 'analizando'
          ? 'ejecutando'
          : actual.fase === 'en_cola'
            ? 'en_espera'
            : actual.resultado?.es_apto
              ? 'apto_aprobado'
              : 'detenido_critico';

      if (cambioPersona) {
        slugEnVivoPrevio = actual.slug;
        nodoEnfocadoPrevio = null;
        // Se enfoca el primer paso con problema (si lo hay) para que el resumen inferior lo explique.
        idNodoSeleccionado = (vivos.find((n) => n.estado === 'detenido_error') ?? vivos[0]).id;
        void tick().then(encuadrarTodoElFlujo);
      } else if (activo && activo.id !== nodoEnfocadoPrevio) {
        nodoEnfocadoPrevio = activo.id;
        idNodoSeleccionado = activo.id;
        if (seguimientoCamaraActivo) {
          const nodoPosicionado = nodos.find((n) => n.id === activo.id);
          if (nodoPosicionado) enfocarNodoEnCamara(nodoPosicionado);
        }
      }

      totalEventosEmitidos = actual.registro.length;
      alActualizarContadorLogs?.(totalEventosEmitidos);
    });
  });

  $effect(() => {
    if (!enVivo) return;
    const c = enVivo.candidato;
    alActualizarEstadoMotor?.(
      enVivo.analisisActivo
        ? `Analizando en vivo — ${c.nombre}`
        : enVivo.etiquetaEstado
    );
  });

  $effect(() => {
    if (enVivo) return; // en modo real no hay escenario simulado que reiniciar
    const config = candidatoActivo.configuracionNodos;
    motor.detener();
    ejecucionEnCurso = false;
    idNodoActivoEjecucion = null;
    nodos = construirNodosIniciales(config);
    totalEventosEmitidos = 0;
    dictamenPipeline = 'en_espera';
  });

  // Estado de los Modales Glassmorphism (Detalle de Validación y Foto del Candidato)
  let modalDetalleAbierto = $state<boolean>(false);
  let idNodoEnModal = $state<string>('obtencion-cv');
  let pestanaModalInicial = $state<'datos' | 'evidencia' | 'json'>('datos');
  let modalFotoCandidatoAbierto = $state<boolean>(false);

  const nodoParaModal = $derived(
    nodos.find((n) => n.id === idNodoEnModal) ?? nodos[0] ?? null
  );

  // Estado de cámara del lienzo infinito 2D (Pan & Zoom estilo n8n)
  let contenedorLienzoRef = $state<HTMLDivElement | null>(null);
  let cabeceraRef = $state<HTMLElement | null>(null);
  let panX = $state<number>(36);
  let panY = $state<number>(34);
  let escalaZoom = $state<number>(0.6);
  let transicionCamaraSuave = $state<boolean>(false);

  // Estado de arrastre del lienzo (Panning)
  let arrastrandoLienzo = $state<boolean>(false);
  let inicioPanPunteroX = 0;
  let inicioPanPunteroY = 0;
  let panInicialX = 0;
  let panInicialY = 0;

  // Estado de arrastre de un nodo individual
  let idNodoArrastrando = $state<string | null>(null);
  let inicioNodoPunteroX = 0;
  let inicioNodoPunteroY = 0;
  let nodoPosInicialX = 0;
  let nodoPosInicialY = 0;

  const motor = new MotorFlujoValidacion();

  // Encuadre inicial: en cuanto el lienzo existe, el grafo completo queda a la vista
  $effect(() => {
    if (!contenedorLienzoRef) return;
    untrack(() => {
      encuadrarTodoElFlujo();
    });
  });

  // La rama alterna "sin cédula" es un mockup: no cuenta en el avance del flujo.
  const nodosFlujo = $derived(nodos.filter((n) => !n.esRama));

  const totalCompletados = $derived(
    nodosFlujo.filter((n) => n.estado === 'completado').length
  );

  const totalReintentosActivos = $derived(
    nodos.reduce(
      (acum, n) => acum + (n.intentoActual > 1 ? n.intentoActual - 1 : 0),
      0
    )
  );

  const porcentajeProgreso = $derived(
    Math.round((totalCompletados / nodosFlujo.length) * 100)
  );

  const nodoEnFoco = $derived(
    nodos.find((n) => n.id === (idNodoActivoEjecucion ?? idNodoSeleccionado)) ??
      nodos[0]
  );

  /**
   * Ruta de cada cable entre nodos consecutivos. En la misma fila usa una curva Bézier suave;
   * cuando el flujo salta a la fila siguiente baja por el pasillo entre filas con esquinas
   * redondeadas (estilo n8n) para no cruzar por encima de los nodos.
   */
  function construirRutaCable(
    x1: number,
    y1: number,
    x2: number,
    y2: number,
    destinoY: number
  ): string {
    const deltaX = x2 - x1;
    const deltaY = y2 - y1;

    if (deltaX >= 40) {
      const tensor = Math.max(60, Math.abs(deltaX) * 0.5);
      return `M ${x1} ${y1} C ${x1 + tensor} ${y1}, ${x2 - tensor} ${y2}, ${x2} ${y2}`;
    }

    const r = 22;
    if (deltaY < 2 * r + 20) {
      return `M ${x1} ${y1} C ${x1 + 120} ${y1}, ${x2 - 120} ${y2}, ${x2} ${y2}`;
    }

    // Salto de fila: salida -> pasillo horizontal entre filas -> entrada
    const xa = x1 + 36;
    const xb = x2 - 36;
    const yPasillo = Math.max(y1 + 2 * r + 4, destinoY - 52);
    const sentido = xb < xa ? -1 : 1;
    return [
      `M ${x1} ${y1}`,
      `L ${xa - r} ${y1}`,
      `Q ${xa} ${y1} ${xa} ${y1 + r}`,
      `L ${xa} ${yPasillo - r}`,
      `Q ${xa} ${yPasillo} ${xa + sentido * r} ${yPasillo}`,
      `L ${xb - sentido * r} ${yPasillo}`,
      `Q ${xb} ${yPasillo} ${xb} ${yPasillo + r}`,
      `L ${xb} ${y2 - r}`,
      `Q ${xb} ${y2} ${xb + r} ${y2}`,
      `L ${x2} ${y2}`
    ].join(' ');
  }

  type EstadoConexion = 'inactiva' | 'fluyendo' | 'reintentando' | 'completada' | 'cortada';

  /**
   * Aristas del grafo. El flujo principal es consecutivo; "Cumplimiento de edad" (si existe) abre además
   * la rama alterna: Sí -> validaciones, No (sin edad ni cédula conocidas) -> contacto -> mensaje ->
   * formulario, que vuelve a ese mismo paso.
   */
  const aristas = $derived.by(() => {
    const lista: {
      origen: DefinicionNodoValidacion;
      destino: DefinicionNodoValidacion;
      etiqueta?: 'Sí' | 'No';
      /** false = camino que no se tomó para este candidato (se dibuja apagado). */
      tomada: boolean;
    }[] = [];
    const porId = new Map(nodos.map((n) => [n.id, n]));
    const condCedula = porId.get('cond-cedula');
    const sinCedula = condCedula?.estado === 'completado' && condCedula.payloadSalidaExito.tieneCedula === false;
    // La rama de contacto también se toma para la persona de prueba, aunque tenga cédula.
    const ramaContacto = condCedula?.estado === 'completado' && condCedula.payloadSalidaExito.ramaContacto === true;

    for (let i = 0; i < nodosFlujo.length - 1; i++) {
      const origen = nodosFlujo[i];
      const esSi = origen.id === 'cumplimiento-edad';
      lista.push({ origen, destino: nodosFlujo[i + 1], etiqueta: esSi ? 'Sí' : undefined, tomada: !(esSi && sinCedula) });
    }

    const condContacto = porId.get('cond-contacto');
    const obtener = porId.get('obtener-contacto');
    const notificar = porId.get('notificar-contacto');
    const whatsapp = porId.get('notificar-whatsapp');
    const edadNodo = porId.get('cumplimiento-edad');
    if (edadNodo && condContacto && obtener && notificar) {
      lista.push(
        { origen: edadNodo, destino: condContacto, etiqueta: 'No', tomada: ramaContacto },
        { origen: condContacto, destino: obtener, etiqueta: 'Sí', tomada: ramaContacto },
        { origen: obtener, destino: notificar, tomada: ramaContacto }
      );
      // El WhatsApp es un envío independiente del correo: ambos salen del nodo de contacto.
      if (whatsapp) lista.push({ origen: obtener, destino: whatsapp, tomada: ramaContacto });

      // Cualquiera de los dos envíos lleva al formulario; al completarse se retoma en "Cumplimiento de edad".
      const formulario = porId.get('completar-formulario');
      const edad = porId.get('cumplimiento-edad');
      if (formulario && edad) {
        lista.push({ origen: notificar, destino: formulario, tomada: ramaContacto });
        if (whatsapp) lista.push({ origen: whatsapp, destino: formulario, tomada: ramaContacto });
        lista.push({ origen: formulario, destino: edad, tomada: ramaContacto });
      }
    }
    return lista;
  });

  const idsConSalida = $derived(new Set(aristas.map((a) => a.origen.id)));

  const conexionesBezier = $derived.by(() => {
    return aristas.map(({ origen, destino, etiqueta, tomada }) => {
      const x1 = origen.posicionX + ANCHO_NODO;
      const y1 = origen.posicionY + Y_PUERTO;
      const x2 = destino.posicionX;
      const y2 = destino.posicionY + Y_PUERTO;

      let estadoConexion: EstadoConexion = 'inactiva';

      if (!tomada) {
        estadoConexion = 'inactiva';
      } else if (destino.id === 'cond-cedula' && destino.estado === 'completado') {
        // El condicional se evalúa aunque el perfil no haya sido "aprobado" (p. ej. sin cédula).
        estadoConexion = 'completada';
      } else if (origen.estado === 'detenido_error' || destino.estado === 'omitido') {
        estadoConexion = 'cortada';
      } else if (
        origen.estado === 'completado' &&
        (destino.estado === 'ejecutando' || destino.estado === 'reintentando')
      ) {
        estadoConexion = destino.estado === 'reintentando' ? 'reintentando' : 'fluyendo';
      } else if (origen.estado === 'completado' && destino.estado === 'completado') {
        estadoConexion = 'completada';
      } else if (origen.estado === 'completado') {
        estadoConexion = 'fluyendo';
      }

      return {
        id: `cable-${origen.id}-${destino.id}`,
        origen,
        destino,
        etiqueta,
        tomada,
        rutaSvg: construirRutaCable(x1, y1, x2, y2, destino.posicionY),
        puntoMedioX: (x1 + x2) / 2,
        puntoMedioY: (y1 + y2) / 2,
        estadoConexion
      };
    });
  });

  function abrirModalDetalleNodo(
    nodo: DefinicionNodoValidacion,
    pestana: 'datos' | 'evidencia'
  ): void {
    idNodoSeleccionado = nodo.id;
    idNodoEnModal = nodo.id;
    pestanaModalInicial = pestana;
    modalDetalleAbierto = true;
  }

  function limitarZoom(valor: number): number {
    return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Number(valor.toFixed(3))));
  }

  function enfocarNodoEnCamara(nodo: DefinicionNodoValidacion): void {
    if (!contenedorLienzoRef || !seguimientoCamaraActivo) return;
    const rect = contenedorLienzoRef.getBoundingClientRect();
    const escala = limitarZoom(Math.max(escalaZoom, 0.85));

    const nodoCentroX = nodo.posicionX + ANCHO_NODO / 2;
    const nodoCentroY = nodo.posicionY + ALTO_ESTIMADO_NODO / 2;

    transicionCamaraSuave = true;
    escalaZoom = escala;
    panX = Math.round(rect.width / 2 - nodoCentroX * escala);
    panY = Math.round(rect.height / 2 + 30 - nodoCentroY * escala);
  }

  /** Ajusta zoom y posición para que todo el grafo quepa entre la barra superior y los controles. */
  function encuadrarTodoElFlujo(): void {
    if (!contenedorLienzoRef || nodos.length === 0) return;
    const rect = contenedorLienzoRef.getBoundingClientRect();
    const margenIzquierdo = 40;
    const margenDerecho = 40;
    const margenSuperior = Math.max(96, (cabeceraRef?.offsetHeight ?? 0) + 36);
    const margenInferior = 84;

    const minX = Math.min(...nodos.map((n) => n.posicionX));
    const maxX = Math.max(...nodos.map((n) => n.posicionX + ANCHO_NODO));
    const minY = Math.min(...nodos.map((n) => n.posicionY));
    const maxY = Math.max(...nodos.map((n) => n.posicionY + ALTO_ESTIMADO_NODO));

    const anchoGrafo = maxX - minX;
    const altoGrafo = maxY - minY;
    const anchoDisponible = rect.width - margenIzquierdo - margenDerecho;
    const altoDisponible = rect.height - margenSuperior - margenInferior;

    const escala = limitarZoom(
      Math.min(anchoDisponible / anchoGrafo, altoDisponible / altoGrafo, 1)
    );

    transicionCamaraSuave = true;
    escalaZoom = escala;
    panX = Math.round(
      margenIzquierdo + (anchoDisponible - anchoGrafo * escala) / 2 - minX * escala
    );
    panY = Math.round(
      margenSuperior + (altoDisponible - altoGrafo * escala) / 2 - minY * escala
    );
  }

  function autoOrganizarGrafo(): void {
    const mapaBase = new Map(
      (nodosEnVivo ?? NODOS_BASE_FLUJO_VALIDACION).map((b) => [
        b.id,
        { x: b.posicionX, y: b.posicionY }
      ])
    );
    nodos = nodos.map((n) => {
      const pos = mapaBase.get(n.id);
      return pos ? { ...n, posicionX: pos.x, posicionY: pos.y } : n;
    });
    encuadrarTodoElFlujo();
    toast.success('Nodos organizados suavemente.');
  }

  /** Cambia el zoom manteniendo fijo el punto de pantalla indicado (como n8n o Figma). */
  function zoomHaciaPunto(nuevaEscala: number, puntoX: number, puntoY: number): void {
    const escala = limitarZoom(nuevaEscala);
    const mundoX = (puntoX - panX) / escalaZoom;
    const mundoY = (puntoY - panY) / escalaZoom;
    escalaZoom = escala;
    panX = Math.round(puntoX - mundoX * escala);
    panY = Math.round(puntoY - mundoY * escala);
  }

  function ajustarZoom(factor: number): void {
    if (!contenedorLienzoRef) return;
    const rect = contenedorLienzoRef.getBoundingClientRect();
    transicionCamaraSuave = true;
    zoomHaciaPunto(escalaZoom * factor, rect.width / 2, rect.height / 2);
  }

  function manejarRuedaLienzo(evento: WheelEvent): void {
    evento.preventDefault();
    if (!contenedorLienzoRef) return;
    transicionCamaraSuave = false;
    const rect = contenedorLienzoRef.getBoundingClientRect();
    zoomHaciaPunto(
      escalaZoom * Math.exp(-evento.deltaY * 0.0015),
      evento.clientX - rect.left,
      evento.clientY - rect.top
    );
  }

  function iniciarArrastreLienzo(evento: PointerEvent): void {
    if (idNodoArrastrando) return;
    transicionCamaraSuave = false;
    arrastrandoLienzo = true;
    inicioPanPunteroX = evento.clientX;
    inicioPanPunteroY = evento.clientY;
    panInicialX = panX;
    panInicialY = panY;
  }

  function iniciarArrastreNodo(idNodo: string, evento: PointerEvent): void {
    const nodoObj = nodos.find((n) => n.id === idNodo);
    if (!nodoObj) return;
    transicionCamaraSuave = false;
    idNodoArrastrando = idNodo;
    inicioNodoPunteroX = evento.clientX;
    inicioNodoPunteroY = evento.clientY;
    nodoPosInicialX = nodoObj.posicionX;
    nodoPosInicialY = nodoObj.posicionY;
  }

  function manejarMovimientoPunteroGlobal(evento: PointerEvent): void {
    if (idNodoArrastrando) {
      const dx = (evento.clientX - inicioNodoPunteroX) / escalaZoom;
      const dy = (evento.clientY - inicioNodoPunteroY) / escalaZoom;

      nodos = nodos.map((n) =>
        n.id === idNodoArrastrando
          ? {
              ...n,
              posicionX: Math.round(nodoPosInicialX + dx),
              posicionY: Math.round(nodoPosInicialY + dy)
            }
          : n
      );
      return;
    }

    if (arrastrandoLienzo) {
      const dx = evento.clientX - inicioPanPunteroX;
      const dy = evento.clientY - inicioPanPunteroY;
      panX = Math.round(panInicialX + dx);
      panY = Math.round(panInicialY + dy);
    }
  }

  function finalizarArrastreGlobal(): void {
    idNodoArrastrando = null;
    arrastrandoLienzo = false;
  }

  function seleccionarCandidatoDeVacante(nuevoSlugCandidato: string): void {
    if (ejecucionEnCurso) {
      motor.detener();
      ejecucionEnCurso = false;
    }
    alCambiarCandidatoEnVacante?.(vacanteActiva.slug, nuevoSlugCandidato);
  }

  function cambiarModoNodoIndividual(
    idNodo: string,
    nuevoModo: ModoSimulacionNodo
  ): void {
    nodos = nodos.map((n) =>
      n.id === idNodo ? { ...n, modoSimulacion: nuevoModo } : n
    );
  }

  function reiniciarPipeline(): void {
    motor.detener();
    ejecucionEnCurso = false;
    idNodoActivoEjecucion = null;
    const posicionesActuales = new Map(
      nodos.map((n) => [n.id, { x: n.posicionX, y: n.posicionY }])
    );
    nodos = construirNodosIniciales(escenarioActual.configuracionNodos).map(
      (n) => {
        const pos = posicionesActuales.get(n.id);
        return pos ? { ...n, posicionX: pos.x, posicionY: pos.y } : n;
      }
    );
    totalEventosEmitidos = 0;
    dictamenPipeline = 'en_espera';
    alActualizarContadorLogs?.(0);
    alActualizarEstadoMotor?.('Motor Orquestador Listo');
    encuadrarTodoElFlujo();
  }

  async function iniciarSimulacionN8n(): Promise<void> {
    if (ejecucionEnCurso) return;

    ejecucionEnCurso = true;
    dictamenPipeline = 'ejecutando';
    totalEventosEmitidos = 0;
    alActualizarEstadoMotor?.('Validando flujo en vivo...');

    await motor.ejecutarFlujo(nodos, velocidad, {
      alActualizarNodos: (actualizados) => {
        nodos = actualizados;
      },
      alRegistrarLog: () => {
        totalEventosEmitidos += 1;
        alActualizarContadorLogs?.(totalEventosEmitidos);
      },
      alCambiarNodoActivo: (idActivo) => {
        idNodoActivoEjecucion = idActivo;
        if (idActivo) {
          idNodoSeleccionado = idActivo;
          const nodoObj = nodos.find((n) => n.id === idActivo);
          if (nodoObj && seguimientoCamaraActivo) {
            enfocarNodoEnCamara(nodoObj);
          }
        }
      },
      alFinalizarFlujo: (res) => {
        ejecucionEnCurso = false;
        idNodoActivoEjecucion = null;
        if (seguimientoCamaraActivo) encuadrarTodoElFlujo();
        if (res.completadoConExito) {
          dictamenPipeline = 'apto_aprobado';
          alActualizarEstadoMotor?.('Candidato Apto Validado (14/14)');
          toast.success(
            `¡Validación completada! 14/14 nodos aprobados (${res.totalReintentos} reintentos recuperados)`
          );
        } else {
          dictamenPipeline = 'detenido_critico';
          alActualizarEstadoMotor?.(
            `Detenido en ${res.nodoDetenido?.titulo ?? 'Validación'}`
          );
          toast.error(
            `Detenido en ${res.nodoDetenido?.titulo}: ${res.nodoDetenido?.mensajeFalloDefinitivo}`
          );
        }
      }
    });
  }

  function detenerSimulacion(): void {
    motor.detener();
    ejecucionEnCurso = false;
    idNodoActivoEjecucion = null;
    dictamenPipeline = 'detenido_critico';
    alActualizarEstadoMotor?.('Ejecución detenida');
    toast.error('Flujo detenido manualmente.');
  }
</script>

<svelte:window
  onpointermove={manejarMovimientoPunteroGlobal}
  onpointerup={finalizarArrastreGlobal}
/>

<div class="estudio-flujo-vuesax">
  <!-- Lienzo Interactivo 2D con estética minimalista DESING.md -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    bind:this={contenedorLienzoRef}
    class="viewport-lienzo"
    class:arrastrando-fondo={arrastrandoLienzo}
    onpointerdown={iniciarArrastreLienzo}
    onwheel={manejarRuedaLienzo}
  >
    <!-- Cuadrícula de puntos ultraligera -->
    <div
      class="rejilla-suave"
      style="
        background-position: {panX}px {panY}px;
        background-size: {26 * escalaZoom}px {26 * escalaZoom}px;
      "
    ></div>

    <!-- Barra Flotante Superior Minimalista y Compacta -->
    <header
      bind:this={cabeceraRef}
      class="barra-flotante-vuesax"
      onpointerdown={(e) => e.stopPropagation()}
    >
      <div class="bloque-contexto">
        {#if alVolverAVacantes}
          <button
            type="button"
            class="boton-volver-vacantes"
            onclick={alVolverAVacantes}
            title="Volver al listado de vacantes y personas evaluadas"
          >
            ← Vacantes
          </button>
        {/if}

        <button
          type="button"
          class="boton-avatar-candidato"
          onclick={() => (modalFotoCandidatoAbierto = true)}
          title="Ver foto y expediente de {escenarioActual.candidato.nombre}"
        >
          <img
            src={escenarioActual.candidato.fotoUrl}
            alt={escenarioActual.candidato.nombre}
          />
        </button>

        <select
          aria-label="Seleccionar persona evaluada en esta vacante"
          class="selector-escenario-vuesax input-premium"
          value={candidatoActivo.slug}
          disabled={ejecucionEnCurso && !enVivo}
          onchange={(e) =>
            seleccionarCandidatoDeVacante(
              (e.currentTarget as HTMLSelectElement).value
            )}
        >
          {#each vacanteActiva.candidatos as cand (cand.slug)}
            <option value={cand.slug}>
              {cand.datos.nombre.toUpperCase()} — {cand.etiquetaDictamen}
            </option>
          {/each}
        </select>

        <button
          type="button"
          class="boton-suave boton-expediente"
          onclick={() => (modalFotoCandidatoAbierto = true)}
        >
          <HugeiconsIcon icon={EyeIcon} size={14} strokeWidth={2} />
          <span>Expediente</span>
        </button>
      </div>

      <!-- Indicador Limpio de Progreso y Controles -->
      <div class="bloque-acciones">
        <div class="bloque-progreso">
          <div class="fila-progreso">
            <span class="etiqueta-suave">Avance</span>
            <strong class="valor-progreso">{totalCompletados}/{nodosFlujo.length}</strong>
            {#if totalReintentosActivos > 0}
              <span class="insignia-reintento">
                {totalReintentosActivos} reintentos
              </span>
            {/if}
          </div>
          <div class="pista-progreso">
            <div
              class="relleno-progreso"
              class:relleno-error={dictamenPipeline === 'detenido_critico'}
              style="width: {porcentajeProgreso}%;"
            ></div>
          </div>
        </div>

        {#if !enVivo}
          <div class="capsula-opciones">
            <button
              type="button"
              class="opcion-capsula"
              class:activa={velocidad === 1}
              onclick={() => (velocidad = 1)}
            >
              1x
            </button>
            <button
              type="button"
              class="opcion-capsula"
              class:activa={velocidad === 2}
              onclick={() => (velocidad = 2)}
            >
              2x
            </button>
          </div>
        {/if}

        <button
          type="button"
          class="boton-suave"
          class:activo-primario={seguimientoCamaraActivo}
          onclick={() => (seguimientoCamaraActivo = !seguimientoCamaraActivo)}
          title="Seguir automáticamente el nodo activo"
        >
          Auto-Foco
        </button>

        {#if enVivo}
          {#if enVivo.analisisActivo}
            <button
              type="button"
              class="boton-detener-flujo"
              onclick={enVivo.alDetenerAnalisis}
              title="Detener el análisis de la vacante (lo ya analizado queda guardado)"
            >
              <HugeiconsIcon icon={StopIcon} size={15} strokeWidth={2.3} />
              <span>Detener</span>
            </button>
          {/if}
        {:else if !ejecucionEnCurso}
          <button
            type="button"
            class="boton-ejecutar-flujo boton-premium"
            onclick={iniciarSimulacionN8n}
          >
            <HugeiconsIcon icon={PlayIcon} size={15} strokeWidth={2.3} />
            <span>Ejecutar</span>
          </button>
        {:else}
          <button
            type="button"
            class="boton-detener-flujo"
            onclick={detenerSimulacion}
          >
            <HugeiconsIcon icon={StopIcon} size={15} strokeWidth={2.3} />
            <span>Detener</span>
          </button>
        {/if}

        {#if !enVivo}
          <button
            type="button"
            class="boton-icono-suave"
            disabled={ejecucionEnCurso}
            onclick={reiniciarPipeline}
            title="Reiniciar flujo"
          >
            <HugeiconsIcon icon={ReloadIcon} size={15} strokeWidth={2} />
          </button>
        {/if}
      </div>
    </header>

    <!-- Mundo 2D Transformable (Cables Curvos Bézier + 14 Nodos Vuesax) -->
    <div
      class="mundo-2d"
      class:transicion-suave={transicionCamaraSuave}
      style="transform: translate3d({panX}px, {panY}px, 0) scale({escalaZoom});"
    >
      <svg class="capa-cables" width="2350" height="1200" viewBox="0 0 2350 1200">
        {#each conexionesBezier as cable (cable.id)}
          <path
            id={cable.id}
            d={cable.rutaSvg}
            class="cable-base cable-{cable.estadoConexion}"
            fill="none"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          />

          {#if cable.estadoConexion === 'fluyendo' || cable.estadoConexion === 'completada'}
            <path
              d={cable.rutaSvg}
              class="cable-activo-ok"
              fill="none"
              stroke-width="3"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
            <circle r="5" class="particula particula-ok">
              <animateMotion
                dur={cable.estadoConexion === 'fluyendo' ? '0.9s' : '2.4s'}
                repeatCount="indefinite"
                path={cable.rutaSvg}
              />
            </circle>
          {:else if cable.estadoConexion === 'reintentando'}
            <path
              d={cable.rutaSvg}
              class="cable-activo-reintento"
              fill="none"
              stroke-width="2.6"
              stroke-linecap="round"
            />
            <circle r="5" class="particula particula-reintento">
              <animateMotion
                dur="0.7s"
                repeatCount="indefinite"
                path={cable.rutaSvg}
              />
            </circle>
          {/if}

          {#if cable.etiqueta}
            <g transform="translate({cable.puntoMedioX}, {cable.puntoMedioY})">
              <rect
                x="-15"
                y="-10"
                width="30"
                height="20"
                rx="10"
                fill={cable.tomada ? '#4cc253' : '#e3e9ed'}
              />
              <text
                x="0"
                y="3.5"
                text-anchor="middle"
                fill={cable.tomada ? '#ffffff' : '#7b8a97'}
                font-size="10"
                font-weight="700"
              >
                {cable.etiqueta}
              </text>
            </g>
          {:else if cable.origen.estado === 'detenido_error'}
            <g transform="translate({cable.puntoMedioX}, {cable.puntoMedioY})">
              <rect
                x="-34"
                y="-10"
                width="68"
                height="20"
                rx="10"
                fill="#ef4444"
              />
              <text
                x="0"
                y="3.5"
                text-anchor="middle"
                fill="#ffffff"
                font-size="9"
                font-weight="700"
              >
                Detenido
              </text>
            </g>
          {/if}
        {/each}
      </svg>

      {#each nodos as nodo (nodo.id)}
        <NodoFlujoN8n
          {nodo}
          candidato={escenarioActual.candidato}
          seleccionado={idNodoSeleccionado === nodo.id}
          esUltimo={!idsConSalida.has(nodo.id)}
          {ejecucionEnCurso}
          alSeleccionar={(n) => (idNodoSeleccionado = n.id)}
          alCambiarModo={cambiarModoNodoIndividual}
          alIniciarArrastre={iniciarArrastreNodo}
          alAbrirDetalle={abrirModalDetalleNodo}
          alAbrirFotoCandidato={() => (modalFotoCandidatoAbierto = true)}
          modoEnVivo={Boolean(enVivo)}
        />
      {/each}
    </div>

    <!-- Píldora Flotante Inferior Izquierda con Acceso Directo a Toda la Información y Captura -->
    {#if nodoEnFoco}
      <div
        class="pildora-estado-inferior"
        onpointerdown={(e) => e.stopPropagation()}
      >
        {#if dictamenPipeline === 'apto_aprobado'}
          <span class="indicador-color ok">
            <HugeiconsIcon icon={CheckmarkCircle02Icon} size={15} strokeWidth={2.2} />
          </span>
          <strong class="titulo-pildora">Flujo completado ({totalCompletados}/{nodosFlujo.length}):</strong>
          <span class="detalle-pildora">Candidato apto para contratación</span>
        {:else if dictamenPipeline === 'detenido_critico'}
          <span class="indicador-color error">
            <HugeiconsIcon icon={Cancel01Icon} size={15} strokeWidth={2.2} />
          </span>
          <strong class="titulo-pildora">Detenido en #{nodoEnFoco.orden} {nodoEnFoco.titulo}:</strong>
          <span class="detalle-pildora">{nodoEnFoco.resumenSalida}</span>
        {:else if ejecucionEnCurso}
          <span class="indicador-color ok">
            <HugeiconsIcon icon={SparklesIcon} size={15} strokeWidth={2.2} />
          </span>
          <strong class="titulo-pildora">Validando #{nodoEnFoco.orden} {nodoEnFoco.titulo}:</strong>
          <span class="detalle-pildora">{nodoEnFoco.resumenSalida}</span>
        {:else}
          <span class="indicador-color neutro">
            <HugeiconsIcon icon={Alert02Icon} size={15} strokeWidth={2} />
          </span>
          <strong class="titulo-pildora">#{nodoEnFoco.orden} {nodoEnFoco.titulo}:</strong>
          <span class="detalle-pildora">{nodoEnFoco.subtitulo}</span>
        {/if}

        <button
          type="button"
          class="btn-abrir-detalle-pildora"
          onclick={() => abrirModalDetalleNodo(nodoEnFoco, 'datos')}
        >
          <HugeiconsIcon icon={EyeIcon} size={13} strokeWidth={2.1} />
          <span>Ver info y captura</span>
        </button>
      </div>
    {/if}

    <!-- Controles Flotantes Inferiores Derechos (Estilo Vuesax limpio) -->
    <div
      class="controles-camara-vuesax"
      onpointerdown={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        class="btn-zoom"
        onclick={() => ajustarZoom(1 / 1.2)}
        title="Alejar"
      >
        −
      </button>
      <span class="porcentaje-zoom">{Math.round(escalaZoom * 100)}%</span>
      <button
        type="button"
        class="btn-zoom"
        onclick={() => ajustarZoom(1.2)}
        title="Acercar"
      >
        +
      </button>
      <button
        type="button"
        class="btn-accion-camara"
        onclick={encuadrarTodoElFlujo}
      >
        Encuadrar
      </button>
      <button
        type="button"
        class="btn-accion-camara"
        onclick={autoOrganizarGrafo}
      >
        Organizar
      </button>
    </div>
  </div>

  <!-- Modal Flotante Glassmorphism con Toda la Información Devuelta y Captura Visual -->
  <ModalDetalleNodoValidacion
    abierto={modalDetalleAbierto}
    nodo={nodoParaModal}
    candidato={escenarioActual.candidato}
    vacante={escenarioActual.vacante}
    pestanaInicial={pestanaModalInicial}
    alCerrar={() => (modalDetalleAbierto = false)}
    alAbrirFotoCandidato={() => {
      modalDetalleAbierto = false;
      modalFotoCandidatoAbierto = true;
    }}
  />

  <!-- Modal Flotante Glassmorphism de Foto en Grande y Expediente del Candidato -->
  <ModalFotoCandidato
    abierto={modalFotoCandidatoAbierto}
    candidato={escenarioActual.candidato}
    vacante={escenarioActual.vacante}
    {totalCompletados}
    totalNodos={nodosFlujo.length}
    alCerrar={() => (modalFotoCandidatoAbierto = false)}
  />
</div>

<style>
  .estudio-flujo-vuesax {
    flex: 1;
    width: 100%;
    height: 100%;
    min-height: calc(100vh - 76px);
    display: flex;
    position: relative;
    overflow: hidden;
    background-color: var(--color-fondo-soft);
  }

  .viewport-lienzo {
    position: relative;
    flex: 1;
    width: 100%;
    height: 100%;
    overflow: hidden;
    cursor: grab;
    user-select: none;
    background-color: var(--color-fondo-soft);
  }

  .viewport-lienzo.arrastrando-fondo {
    cursor: grabbing;
  }

  .rejilla-suave {
    position: absolute;
    inset: 0;
    pointer-events: none;
    background-image: radial-gradient(rgba(44, 62, 80, 0.14) 1.2px, transparent 1.2px);
  }

  .mundo-2d {
    position: absolute;
    top: 0;
    left: 0;
    width: 2350px;
    height: 1200px;
    transform-origin: 0 0;
    will-change: transform;
  }

  .mundo-2d.transicion-suave {
    transition: transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  }

  .capa-cables {
    position: absolute;
    top: 0;
    left: 0;
    width: 2350px;
    height: 1200px;
    pointer-events: none;
    overflow: visible;
    z-index: 2;
  }

  .cable-base {
    stroke: #d3dce2;
    transition: stroke 0.25s ease;
  }

  .particula {
    transition: opacity 0.25s ease;
  }

  .particula-ok {
    fill: var(--color-primario);
    filter: drop-shadow(0 0 5px rgba(var(--vs-primary-raw), 0.9));
  }

  .particula-reintento {
    fill: #f59e0b;
    filter: drop-shadow(0 0 5px rgba(245, 158, 11, 0.9));
  }

  .cable-base.cable-completada,
  .cable-base.cable-fluyendo {
    stroke: rgba(var(--vs-primary-raw), 0.35);
  }

  .cable-base.cable-reintentando {
    stroke: rgba(245, 158, 11, 0.45);
  }

  .cable-base.cable-cortada {
    stroke: rgba(239, 68, 68, 0.35);
    stroke-dasharray: 5 5;
  }

  .cable-activo-ok {
    stroke: var(--color-primario);
    stroke-dasharray: 8 7;
    animation: fluirTrazo 0.6s linear infinite;
  }

  .cable-activo-reintento {
    stroke: #f59e0b;
    stroke-dasharray: 7 6;
    animation: fluirTrazo 0.45s linear infinite;
  }

  @keyframes fluirTrazo {
    to {
      stroke-dashoffset: -15;
    }
  }

  /* Barra Superior Flotante Minimalista */
  .barra-flotante-vuesax {
    position: absolute;
    top: 14px;
    left: 24px;
    right: 24px;
    z-index: 40;
    background: rgba(255, 255, 255, 0.94);
    backdrop-filter: blur(10px);
    border-radius: 18px;
    padding: 8px 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
    box-shadow: 0 12px 28px -12px rgba(44, 62, 80, 0.08);
  }

  .bloque-contexto {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .boton-volver-vacantes {
    padding: 8px 13px;
    border-radius: 12px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    flex-shrink: 0;
    transition: all 0.25s ease;
  }

  .boton-volver-vacantes:hover {
    background-color: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .boton-avatar-candidato {
    position: relative;
    width: 38px;
    height: 38px;
    border-radius: 12px;
    overflow: hidden;
    padding: 0;
    cursor: pointer;
    flex-shrink: 0;
    box-shadow: 0 6px 14px -6px rgba(44, 62, 80, 0.18);
  }

  .boton-avatar-candidato img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .selector-escenario-vuesax {
    padding: 7px 12px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 600;
    color: var(--color-texto-dark);
    cursor: pointer;
    max-width: 360px;
  }

  .boton-expediente {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  /* Progreso compacto */
  .bloque-progreso {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 140px;
  }

  .fila-progreso {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
  }

  .etiqueta-suave {
    font-size: 10.5px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .valor-progreso {
    font-size: 11.5px;
    font-weight: 700;
    color: var(--color-texto-dark);
  }

  .insignia-reintento {
    font-size: 9.5px;
    font-weight: 600;
    color: #d97706;
    background-color: rgba(245, 158, 11, 0.14);
    padding: 1px 6px;
    border-radius: 999px;
  }

  .pista-progreso {
    width: 100%;
    height: 5px;
    background-color: var(--color-fondo-soft);
    border-radius: 999px;
    overflow: hidden;
  }

  .relleno-progreso {
    height: 100%;
    background-color: var(--color-primario);
    border-radius: 999px;
    transition: width 0.3s ease;
  }

  .relleno-progreso.relleno-error {
    background-color: #ef4444;
  }

  /* Acciones derechas */
  .bloque-acciones {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .capsula-opciones {
    display: flex;
    background-color: var(--color-fondo-soft);
    padding: 3px;
    border-radius: 12px;
    gap: 2px;
  }

  .opcion-capsula {
    padding: 5px 10px;
    border-radius: 9px;
    font-size: 11px;
    font-weight: 600;
    color: var(--color-texto-grey);
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .opcion-capsula.activa {
    background-color: #ffffff;
    color: var(--color-texto-dark);
    box-shadow: 0 2px 8px rgba(44, 62, 80, 0.06);
  }

  .boton-suave {
    padding: 8px 12px;
    border-radius: 12px;
    font-size: 11.5px;
    font-weight: 600;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .boton-suave:hover {
    color: var(--color-texto-dark);
  }

  .boton-suave.activo-primario {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .boton-ejecutar-flujo {
    padding: 8px 15px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 700;
    border-radius: 12px;
  }

  .boton-detener-flujo {
    padding: 8px 14px;
    border-radius: 12px;
    background-color: #ef4444;
    color: #ffffff;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    box-shadow: 0 10px 20px -10px rgba(239, 68, 68, 0.5);
    transition: all 0.25s ease;
  }

  .boton-icono-suave {
    width: 36px;
    height: 36px;
    border-radius: 12px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .boton-icono-suave:hover:not(:disabled) {
    background-color: #ffffff;
    box-shadow: 0 6px 16px rgba(44, 62, 80, 0.08);
  }

  /* Píldora Inferior Izquierda Blanca y Limpia */
  .pildora-estado-inferior {
    position: absolute;
    bottom: 22px;
    left: 24px;
    z-index: 40;
    background: rgba(255, 255, 255, 0.94);
    backdrop-filter: blur(10px);
    padding: 8px 10px 8px 14px;
    border-radius: 999px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11.5px;
    box-shadow: 0 12px 30px -10px rgba(44, 62, 80, 0.1);
    max-width: 66%;
  }

  .indicador-color {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .indicador-color.ok {
    background-color: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
  }

  .indicador-color.error {
    background-color: rgba(239, 68, 68, 0.14);
    color: #ef4444;
  }

  .indicador-color.neutro {
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-grey);
  }

  .titulo-pildora {
    font-weight: 700;
    color: var(--color-texto-dark);
    white-space: nowrap;
  }

  .detalle-pildora {
    font-weight: 500;
    color: var(--color-texto-grey);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .btn-abrir-detalle-pildora {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 6px 12px;
    border-radius: 999px;
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
    cursor: pointer;
    flex-shrink: 0;
    transition: all 0.25s ease;
  }

  .btn-abrir-detalle-pildora:hover {
    background-color: var(--color-primario);
    color: #ffffff;
  }

  /* Controles de Cámara Inferiores Derechos */
  .controles-camara-vuesax {
    position: absolute;
    bottom: 22px;
    right: 24px;
    z-index: 40;
    background: rgba(255, 255, 255, 0.94);
    backdrop-filter: blur(10px);
    border-radius: 18px;
    padding: 6px;
    display: flex;
    align-items: center;
    gap: 5px;
    box-shadow: 0 12px 30px -10px rgba(44, 62, 80, 0.1);
  }

  .btn-zoom {
    width: 30px;
    height: 30px;
    border-radius: 10px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 15px;
    font-weight: 700;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-zoom:hover {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .porcentaje-zoom {
    font-size: 11px;
    font-weight: 600;
    color: var(--color-texto-grey);
    min-width: 40px;
    text-align: center;
  }

  .btn-accion-camara {
    padding: 6px 11px;
    border-radius: 10px;
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.25s ease;
  }

  .btn-accion-camara:hover {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }
</style>
