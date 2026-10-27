import toast from 'svelte-french-toast';
import {
  ETAPAS_NUCLEO,
  ETIQUETA_ETAPA,
  ORDEN_ETAPAS,
  RESUMEN_VACIO,
  type CandidatoApi,
  type CandidatoEnVivo,
  type EstadoAnalisis,
  type EstadoEtapa,
  type EtapaCandidato,
  type IdEtapa,
  type LineaRegistro,
  type NivelLinea,
  type ResumenAnalisis,
  type VacanteApi,
  type Veredicto
} from '../modelos/analisisEnVivo';
import {
  cancelarJob,
  estadoJob,
  jobActivo,
  listarCandidatosGuardados,
  listarVacantes,
  urlAnalisis,
  urlCambios,
  urlEventosJob,
  type OpcionesAnalisisApi
} from './clienteRrhh';

const MAX_LINEAS_REGISTRO = 300;

/** "Julián Andrés" -> "julian-andres". */
export function slugificar(texto: string): string {
  return texto
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function slugCandidato(indice: number, nombre: string): string {
  return `${indice}-${slugificar(nombre) || 'candidato'}`;
}

function horaActual(): string {
  const d = new Date();
  const dos = (n: number) => String(n).padStart(2, '0');
  return `${dos(d.getHours())}:${dos(d.getMinutes())}:${dos(d.getSeconds())}`;
}

function retratoIniciales(nombre: string): string {
  const iniciales = nombre
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('');
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 320"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#e8f8ea"/><stop offset="1" stop-color="#c8f0ce"/></linearGradient></defs><rect width="320" height="320" rx="48" fill="url(#g)"/><text x="160" y="190" text-anchor="middle" font-family="Poppins, sans-serif" font-weight="700" font-size="110" fill="#2c3e50">${iniciales}</text></svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

export function veredictoDe(c: CandidatoEnVivo): Veredicto {
  if (c.fase === 'en_cola') return 'en_cola';
  if (c.fase === 'analizando') return 'analizando';
  const ev = c.resultado?.evaluacion_idoneidad;
  if (ev?.verificacion_pendiente) return 'pendiente';
  return c.resultado?.es_apto ? 'apto' : 'no_apto';
}

export const ETIQUETA_VEREDICTO: Record<Veredicto, string> = {
  en_cola: 'En cola',
  analizando: 'Analizando…',
  apto: 'Apto',
  no_apto: 'No apto',
  pendiente: 'Verificación pendiente'
};

export function etiquetaEstadoAnalisis(estado: EstadoAnalisis): string {
  switch (estado) {
    case 'conectando':
      return 'Conectando con el servidor…';
    case 'en_espera':
      return 'En espera del navegador de Computrabajo…';
    case 'en_curso':
      return 'Analizando en vivo';
    case 'reconectando':
      return 'Reconectando…';
    case 'completado':
      return 'Análisis completado';
    case 'cancelado':
      return 'Análisis detenido';
    case 'error':
      return 'El análisis no pudo continuar';
    default:
      return 'Sin análisis en esta sesión';
  }
}

function esNoConsultado(detalle: string | undefined): boolean {
  return (detalle ?? '').startsWith('No consultado');
}

/**
 * Reconstruye el estado de cada etapa a partir del resultado final del candidato. Sirve para los
 * candidatos de caché o guardados (no hubo eventos en vivo) y para rellenar las etapas que el
 * descarte temprano nunca ejecutó.
 */
export function etapasDesdeResultado(
  r: CandidatoApi,
  previas: Partial<Record<IdEtapa, EtapaCandidato>> = {}
): Partial<Record<IdEtapa, EtapaCandidato>> {
  const etapas: Partial<Record<IdEtapa, EtapaCandidato>> = { ...previas };
  const poner = (id: IdEtapa, estado: EstadoEtapa, detalle?: string) => {
    const previa = etapas[id];
    // Lo observado en vivo (con tiempos y reintentos) tiene prioridad sobre lo reconstruido.
    if (previa && previa.estado !== 'ejecutando' && previa.estado !== 'reintentando') return;
    etapas[id] = { estado, detalle, ms: previa?.ms, reintentos: previa?.reintentos ?? 0, captura: previa?.captura };
  };

  const ev = r.evaluacion_idoneidad;
  const cedulaValida = (r.cedula ?? '').replace(/\D/g, '').length >= 6;
  const falloPerfil = (r.error_proceso ?? '').toLowerCase().includes('perfil');

  if (falloPerfil) {
    poner('perfil', 'error', r.error_proceso);
  } else {
    poner(
      'perfil',
      'ok',
      cedulaValida
        ? `Cédula ${r.cedula}${r.ruta_cv_original ? ' · hoja de vida descargada' : ''}`
        : 'Perfil leído, pero sin cédula'
    );
  }

  if (ev) {
    poner(
      'genero',
      ev.cumple_genero ? 'ok' : 'rechazado',
      ev.cumple_genero ? ev.genero : `Género (${ev.genero}) no cumple requisito (requerido: Hombre)`
    );

    const nucleo: [IdEtapa, boolean, string][] = [
      ['adres', ev.cumple_eps, ev.detalle_eps],
      ['runt', ev.cumple_runt, ev.detalle_runt],
      ['simit', ev.cumple_simit, ev.detalle_simit]
    ];
    for (const [id, cumple, detalle] of nucleo) {
      if (esNoConsultado(detalle)) poner(id, 'omitida', detalle);
      else if (cumple) poner(id, 'ok', detalle);
      else if (ev.verificacion_pendiente && (detalle.includes('no verificado') || detalle.includes('Sin información'))) {
        poner(id, 'error', detalle);
      } else poner(id, 'rechazado', detalle);
    }
  }

  const informativas: [IdEtapa, { alerta: boolean; detalles?: string; estado?: string } | null | undefined][] = [
    ['reintegro', r.reintegro],
    ['procuraduria', r.procuraduria],
    ['sarlaft', r.sarlaft_onu],
    ['ofac', r.ofac]
  ];
  for (const [id, res] of informativas) {
    if (!res) {
      poner(id, 'omitida', 'No consultado');
      continue;
    }
    const detalle = res.detalles ?? res.estado;
    // "NO_APLICA"/"NO_CONSULTADO_*" del motor anterior significa que nunca hubo cédula para consultar.
    if (res.estado?.startsWith('NO_APLICA') || res.estado?.startsWith('NO_CONSULTADO')) {
      poner(id, 'omitida', detalle);
    } else if (res.estado === 'ERROR_CONSULTA') {
      poner(id, 'error', detalle);
    } else if (id === 'reintegro' && res.alerta) {
      poner(id, 'rechazado', `${detalle}: no se admiten reintegros`); // regla que descarta, no un simple aviso
    } else poner(id, res.alerta ? 'alerta' : 'ok', detalle);
  }

  // La IA solo califica a quien cumplió todas las validaciones.
  if (r.evaluacion_ia) {
    poner('ia', 'ok', `Score ${r.evaluacion_ia.score_total}/100 · ${r.evaluacion_ia.dictamen}`);
  } else if (r.es_apto) {
    poner('ia', 'omitida', 'Sin calificar: la IA aún no evaluó a este candidato (se reintenta al reanalizar)');
  } else {
    poner('ia', 'omitida', 'No aplica: la IA solo califica a quien cumple todas las validaciones');
  }

  // Capturas guardadas en disco (candidatos de caché o ya analizados): se adjuntan a su etapa.
  for (const [id, ruta] of Object.entries(r.capturas ?? {}) as [IdEtapa, string][]) {
    const etapa = etapas[id];
    if (etapa && ruta && !etapa.captura) etapas[id] = { ...etapa, captura: ruta };
  }

  return etapas;
}

function nuevoCandidato(indice: number, nombre: string): CandidatoEnVivo {
  return {
    indice,
    slug: slugCandidato(indice, nombre),
    nombre,
    cedula: '',
    urlDetalle: '',
    fotoUrl: retratoIniciales(nombre),
    fase: 'en_cola',
    etapas: {},
    registro: [],
    resultado: null,
    desdeCache: false,
    terminadoEn: null
  };
}

function candidatoDesdeApi(r: CandidatoApi, desdeCache: boolean): CandidatoEnVivo {
  const c = nuevoCandidato(r.indice, r.nombre);
  c.cedula = r.cedula && r.cedula !== 'No especificada' ? r.cedula : '';
  c.urlDetalle = r.url_detalle;
  if (r.foto_url) c.fotoUrl = r.foto_url;
  c.fase = 'completo';
  c.resultado = r;
  c.desdeCache = desdeCache;
  c.etapas = etapasDesdeResultado(r);
  return c;
}

/** Análisis en vivo (SSE) de UNA vacante, más los candidatos ya guardados de corridas anteriores. */
export class SesionAnalisis {
  estado = $state<EstadoAnalisis>('inactivo');
  mensaje = $state<string | null>(null);
  idJob = $state<string | null>(null);
  resumen = $state<ResumenAnalisis>({ ...RESUMEN_VACIO });
  cargandoGuardados = $state(false);
  /** Por qué no se pudieron leer los candidatos guardados (se reintenta solo); null si se leyeron bien. */
  errorCarga = $state<string | null>(null);
  private reintentosCarga = 0;
  private refrescando = false;
  /** Versión de la última lista leída: un refresco sin cambios responde 304 y no se procesa. */
  private etag: string | null = null;
  /** El servidor está repitiendo lo ya ocurrido de un análisis en marcha (no es en vivo): no se anima. */
  repitiendo = $state(false);
  /** Último evento recibido del análisis: al reconectar se pide solo lo que siguió. */
  private ultimoEvento = 0;
  /** La conexión se soltó porque la pestaña quedó oculta; el análisis sigue en el servidor. */
  private pausada = false;
  /** Eventos recibidos que aún no se aplicaron: se aplican juntos, una vez por cuadro. */
  private pendientes: (() => void)[] = [];
  private loteProgramado = false;
  /** Otro análisis (de otra vacante) ocupa el único cupo; se muestra para poder abrirlo. */
  bloqueadoPor = $state<{ id: string; titulo: string } | null>(null);

  private porIndice = $state<Record<number, CandidatoEnVivo>>({});
  private guardadosPedidos = false;
  private origen: EventSource | null = null;
  private contadorLineas = 0;
  private terminado = false;
  private forzado = false;
  /** Lo ya analizado al empezar la corrida, por nombre: el listado se repite entero y no debe volver a "en cola". */
  private conservados = new Map<string, CandidatoEnVivo[]>();

  readonly candidatos = $derived.by(() => {
    // Defensa: el identificador (slug) es la clave de las tarjetas y no puede repetirse nunca.
    const vistos = new Set<string>();
    return Object.values(this.porIndice)
      .filter((c) => !vistos.has(c.slug) && vistos.add(c.slug))
      .sort((a, b) => a.indice - b.indice);
  });

  readonly activo = $derived(
    this.estado === 'conectando' ||
      this.estado === 'en_espera' ||
      this.estado === 'en_curso' ||
      this.estado === 'reconectando'
  );

  constructor(readonly idVacante: string) {}

  private readonly indiceSlug = $derived(new Map(this.candidatos.map((c) => [c.slug, c])));

  porSlug(slug: string): CandidatoEnVivo | undefined {
    return this.indiceSlug.get(slug);
  }

  /**
   * Carga lo analizado antes, una sola vez por sesión. Se llama desde $effect: la bandera no es
   * reactiva y la comprobación va primero, así el efecto no se vuelve a disparar al cambiar estado
   * (antes, una vacante sin candidatos guardados pedía la lista en bucle).
   */
  /** Lectura de guardados en curso: quien la pida mientras tanto (p. ej. unirse a un análisis) la espera. */
  private lecturaGuardados: Promise<void> | null = null;

  cargarGuardados(): Promise<void> {
    if (this.lecturaGuardados) return this.lecturaGuardados;
    if (this.guardadosPedidos || this.activo) return Promise.resolve();
    this.lecturaGuardados = this.leerGuardados().finally(() => (this.lecturaGuardados = null));
    return this.lecturaGuardados;
  }

  private async leerGuardados(): Promise<void> {
    this.guardadosPedidos = true;
    this.cargandoGuardados = true;
    try {
      const { candidatos: guardados, etag } = await listarCandidatosGuardados(this.idVacante);
      // Si mientras tanto arrancó un análisis en vivo, sus datos mandan.
      if (this.activo || this.candidatos.length > 0) return;
      this.etag = etag;
      for (const r of guardados ?? []) this.porIndice[r.indice] = candidatoDesdeApi(r, true);
      this.errorCarga = null;
      this.reintentosCarga = 0;
    } catch (error) {
      this.guardadosPedidos = false;
      this.errorCarga = error instanceof Error ? error.message : 'No se pudieron leer los candidatos guardados';
      this.mensaje = this.errorCarga;
      // Un fallo pasajero (servidor ocupado, reinicio) no debe dejar la vacante vacía hasta recargar.
      if (this.reintentosCarga < 8) {
        this.reintentosCarga++;
        setTimeout(() => void this.cargarGuardados(), 2000 * this.reintentosCarga);
      }
    } finally {
      this.cargandoGuardados = false;
    }
  }

  /**
   * Vuelve a leer de la base los candidatos ya cargados y reemplaza los que cambiaron (p. ej. el
   * candidato envió su formulario o terminó su reanudación). No toca nada mientras corre un análisis.
   */
  async refrescar(): Promise<void> {
    if (!this.guardadosPedidos || this.activo || this.refrescando) return; // sin solaparse: cada lectura pesa en la base
    this.refrescando = true;
    try {
      const { candidatos: guardados, etag } = await listarCandidatosGuardados(this.idVacante, this.etag);
      if (this.activo || !guardados) return; // sin cambios (304): nada que comparar ni repintar
      this.etag = etag;
      const sinIndice = (x: CandidatoApi) => JSON.stringify({ ...x, indice: 0 });
      for (const r of guardados) {
        // A la persona se la reconoce por su ficha: tras reubicarse en el listado su posición ya no
        // coincide con la guardada, y buscarla por posición duplicaba o pisaba a otra persona.
        const actual = Object.values(this.porIndice).find(
          (c) => (r.url_detalle && c.urlDetalle === r.url_detalle) || (c.indice === r.indice && c.nombre === r.nombre)
        );
        if (!actual) {
          // Analizado por otro medio: aparece sin recargar. Si su posición la ocupa otra persona (el listado
          // de Computrabajo se reordena al llegar postulados nuevos) va al final, y se reconoce por su ficha.
          if (!this.porIndice[r.indice]) this.porIndice[r.indice] = candidatoDesdeApi(r, true);
          else if (r.url_detalle) {
            const libre = Math.max(...Object.keys(this.porIndice).map(Number), 0) + 1;
            this.porIndice[libre] = candidatoDesdeApi({ ...r, indice: libre }, true);
          }
          continue;
        }
        if (actual.fase !== 'completo') continue;
        if (actual.resultado && sinIndice(actual.resultado) === sinIndice(r)) continue;
        const nuevo = candidatoDesdeApi({ ...r, indice: actual.indice }, true);
        nuevo.registro = actual.registro;
        nuevo.slug = actual.slug;
        nuevo.terminadoEn = actual.terminadoEn;
        this.porIndice[actual.indice] = nuevo;
      }
    } catch (error) {
      console.error('No se pudo refrescar los candidatos', error);
    } finally {
      this.refrescando = false;
    }
  }

  /**
   * Avance de una reanudación (el candidato entregó sus datos y el backend sigue con ADRES, RUNT, SIMIT...):
   * se aplica igual que si viniera del análisis en vivo. Devuelve false si el candidato no es de esta vacante.
   */
  alReanudar(tipo: 'candidato.etapa' | 'candidato.completo', d: Record<string, unknown>): boolean {
    const id = String(d.postulacion_id ?? '');
    const c = this.candidatos.find((x) => x.resultado?.postulacion_id === id);
    if (!c) return false;
    if (tipo === 'candidato.etapa') this.alEtapa({ ...d, indice: c.indice, nombre: c.nombre });
    else this.alCompletar(d as unknown as CandidatoApi);
    return true;
  }

  async iniciar(opciones: OpcionesAnalisisApi): Promise<void> {
    if (this.activo) return;
    this.mensaje = null;
    this.bloqueadoPor = null;
    this.terminado = false;
    this.estado = 'conectando';
    this.forzado = Boolean(opciones.forzar);
    this.ultimoEvento = 0;

    // Solo hay un análisis a la vez: si hay otro, se explica antes de abrir el stream (EventSource
    // no deja leer el cuerpo de un 409).
    try {
      const otro = await jobActivo();
      if (otro && otro.oferta_id.toLowerCase() !== this.idVacante.toLowerCase()) {
        this.estado = 'error';
        this.mensaje = `Hay un análisis en curso de otra vacante (${otro.titulo}). Solo se analiza una a la vez.`;
        this.bloqueadoPor = { id: otro.oferta_id, titulo: otro.titulo };
        return;
      }
    } catch (error) {
      this.estado = 'error';
      this.mensaje = error instanceof Error ? error.message : 'No se pudo contactar al servidor';
      return;
    }

    this.conectar(urlAnalisis(this.idVacante, opciones));
  }

  /**
   * Se une a un análisis que ya corre en el servidor (lo inició otra pestaña, o esta página se recargó):
   * repite lo ocurrido hasta ahora y sigue en vivo. A diferencia de iniciar, nunca crea un análisis nuevo.
   */
  unirse(idJob: string): void {
    if (this.activo) return;
    this.mensaje = null;
    this.bloqueadoPor = null;
    this.terminado = false;
    this.estado = 'conectando';
    if (this.idJob !== idJob) this.ultimoEvento = 0;
    this.idJob = idJob;
    this.conectar(urlEventosJob(idJob, this.ultimoEvento));
  }

  /**
   * Suelta la conexión en vivo sin dar el análisis por terminado (la pestaña quedó oculta): el navegador
   * solo permite 6 conexiones por servidor entre todas las pestañas y las ocultas no deben gastarlas.
   */
  pausar(): void {
    if (!this.origen || this.terminado) return;
    this.origen.close();
    this.origen = null;
    this.pausada = true;
    this.estado = 'reconectando';
  }

  /** Retoma la conexión soltada por pausar(): el servidor manda solo lo que pasó mientras tanto. */
  reanudar(): void {
    if (!this.pausada) return;
    this.pausada = false;
    if (!this.idJob) {
      this.finalizar('inactivo');
      return;
    }
    this.conectar(urlEventosJob(this.idJob, this.ultimoEvento));
  }

  private conectar(url: string): void {
    this.conservados = new Map();
    if (!this.forzado) {
      for (const c of Object.values(this.porIndice)) {
        if (c.fase !== 'completo' || !c.resultado) continue;
        this.conservados.set(c.nombre, [...(this.conservados.get(c.nombre) ?? []), c]);
      }
    }
    const origen = new EventSource(url, { withCredentials: true });
    this.origen = origen;
    // Hasta el evento repeticion.fin el servidor repite lo ya ocurrido; si no llega (servidor viejo), se
    // da por terminada la repetición a los pocos segundos.
    this.repitiendo = true;
    setTimeout(() => (this.repitiendo = false), 4000);

    // Los eventos no se aplican uno por uno: al unirse a un análisis llegan miles seguidos y cada uno
    // reordenaba y repintaba toda la pantalla. Se juntan y se aplican una vez por cuadro.
    const escuchar = (tipo: string, manejar: (datos: Record<string, unknown>) => void) =>
      origen.addEventListener(tipo, (e) => {
        const mensaje = e as MessageEvent<string>;
        const id = Number(mensaje.lastEventId);
        if (id > 0) this.ultimoEvento = id;
        let datos: Record<string, unknown>;
        try {
          datos = JSON.parse(mensaje.data) as Record<string, unknown>;
        } catch (error) {
          console.error(`Evento SSE ${tipo} ilegible`, error);
          return;
        }
        this.encolar(() => manejar(datos));
      });

    origen.onopen = () => {
      if (this.estado === 'conectando' || this.estado === 'reconectando') this.estado = 'en_curso';
    };

    escuchar('repeticion.fin', () => (this.repitiendo = false));
    escuchar('job.iniciado', (d) => {
      this.idJob = String(d.job_id ?? '');
      this.estado = 'en_curso';
    });
    escuchar('job.en_espera', (d) => {
      this.estado = 'en_espera';
      this.mensaje = String(d.motivo ?? 'En espera');
    });
    escuchar('candidato.descubierto', (d) => this.alDescubrir(Number(d.indice), String(d.nombre), d.cache === true));
    escuchar('candidato.etapa', (d) => this.alEtapa(d));
    escuchar('candidato.completo', (d) => this.alCompletar(d as unknown as CandidatoApi));
    escuchar('candidato.guardado', (d) => this.alGuardar(d));
    escuchar('resumen', (d) => (this.resumen = { ...RESUMEN_VACIO, ...(d as Partial<ResumenAnalisis>) }));
    escuchar('listado.completo', () => (this.resumen = { ...this.resumen, listado_completo: true }));
    escuchar('job.completado', (d) => {
      this.finalizar('completado');
      if (d.resumen) this.resumen = { ...RESUMEN_VACIO, ...(d.resumen as Partial<ResumenAnalisis>) };
      const r = this.resumen;
      toast.success(
        `Análisis completado: ${r.aptos} aptos, ${r.no_aptos} no aptos${r.pendientes ? `, ${r.pendientes} pendientes` : ''}.`
      );
    });
    escuchar('job.cancelado', () => {
      this.finalizar('cancelado');
      toast('Análisis detenido. Lo ya analizado quedó guardado.');
    });
    escuchar('job.error', (d) => {
      this.finalizar('error');
      this.mensaje = String(d.mensaje ?? 'El análisis terminó con errores');
      toast.error(this.mensaje);
    });

    origen.onerror = () => {
      if (this.terminado) return;
      if (origen.readyState === EventSource.CONNECTING) {
        this.estado = 'reconectando'; // el navegador reintenta solo y retoma con Last-Event-ID
        return;
      }
      // Lo que llegó antes del cierre (p. ej. job.completado) se aplica antes de decidir cómo terminó.
      this.encolar(() => void this.alCerrarseElStream());
    };
  }

  private encolar(aplicar: () => void): void {
    this.pendientes.push(aplicar);
    if (this.loteProgramado) return;
    this.loteProgramado = true;
    // Con la pestaña oculta requestAnimationFrame no corre: igual se aplican, sin esperar a que vuelva.
    if (document.hidden) setTimeout(() => this.aplicarLote(), 100);
    else requestAnimationFrame(() => this.aplicarLote());
  }

  private aplicarLote(): void {
    this.loteProgramado = false;
    const lote = this.pendientes;
    this.pendientes = [];
    for (const aplicar of lote) {
      try {
        aplicar();
      } catch (error) {
        console.error('No se pudo aplicar un evento del análisis', error);
      }
    }
  }

  /** El stream se cerró sin evento final: se consulta el job para saber cómo terminó. */
  private async alCerrarseElStream(): Promise<void> {
    this.origen?.close();
    this.origen = null;
    if (this.terminado) return;
    try {
      if (this.idJob) {
        const job = await estadoJob(this.idJob);
        if (job.estado === 'completado') return this.finalizar('completado');
        if (job.estado === 'cancelado') return this.finalizar('cancelado');
        if (job.estado === 'error') return this.finalizar('error');
      }
    } catch {
      /* sin conexión: se informa abajo */
    }
    this.finalizar('error');
    this.mensaje ??=
      'No se pudo iniciar o mantener el análisis. Revisa que la vacante siga activa, las credenciales de Computrabajo y que no haya otro análisis en curso.';
    toast.error(this.mensaje);
  }

  private finalizar(estado: EstadoAnalisis): void {
    this.terminado = true;
    this.pausada = false;
    this.estado = estado;
    this.origen?.close();
    this.origen = null;
    // Quien quedó "analizando" cuando el flujo se cortó vuelve a la cola para no mostrarse colgado.
    for (const c of this.candidatos) {
      if (c.fase === 'analizando') c.fase = 'en_cola';
    }
    // Al terminar se relee la base: deja la lista igual a lo guardado (p. ej. quien ya no está postulado).
    void this.refrescar();
  }

  async detener(): Promise<void> {
    if (!this.idJob || !this.activo) return;
    try {
      await cancelarJob(this.idJob);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'No se pudo detener el análisis');
    }
  }

  /** Cierra la conexión sin cancelar el job (el servidor sigue y se puede retomar). */
  desconectar(): void {
    this.terminado = true;
    this.pausada = false;
    this.origen?.close();
    this.origen = null;
    if (this.activo) this.estado = 'inactivo';
  }

  // ---- eventos ---------------------------------------------------------------------------------

  private obtener(indice: number, nombre: string): CandidatoEnVivo {
    if (!this.porIndice[indice]) this.porIndice[indice] = nuevoCandidato(indice, nombre);
    return this.porIndice[indice];
  }

  private linea(c: CandidatoEnVivo, etapa: LineaRegistro['etapa'], nivel: NivelLinea, texto: string): void {
    c.registro.push({ id: ++this.contadorLineas, hora: horaActual(), etapa, nivel, texto });
    if (c.registro.length > MAX_LINEAS_REGISTRO) c.registro.splice(0, c.registro.length - MAX_LINEAS_REGISTRO);
  }

  private alDescubrir(indice: number, nombre: string, deCache: boolean): void {
    // Un candidato ya analizado sigue con su resultado mientras el listado lo vuelve a recorrer (su
    // posición puede haber cambiado si llegaron nuevos); el resultado guardado lo confirma enseguida.
    const guardado = this.conservados.get(nombre)?.shift();
    if (guardado) {
      if (this.porIndice[guardado.indice] === guardado) delete this.porIndice[guardado.indice];
      guardado.indice = indice;
      this.porIndice[indice] = guardado;
      return;
    }
    const previo = this.porIndice[indice];
    if (previo && previo.nombre === nombre) {
      // Reanálisis: se limpia lo anterior para ver el flujo otra vez desde cero.
      previo.fase = 'en_cola';
      previo.etapas = {};
      previo.registro = [];
      previo.desdeCache = false;
    } else {
      this.porIndice[indice] = nuevoCandidato(indice, nombre);
      // Una persona que el servidor ya tiene guardada no es una novedad: solo se confirma enseguida.
      this.porIndice[indice].desdeCache = deCache;
    }
    this.linea(this.porIndice[indice], 'general', 'info', 'Descubierto en el listado de Computrabajo');
  }

  private alEtapa(d: Record<string, unknown>): void {
    const c = this.obtener(Number(d.indice), String(d.nombre));
    const id = String(d.etapa) as IdEtapa;
    const estadoApi = String(d.estado);
    const detalle = d.detalle ? String(d.detalle) : undefined;
    const ms = typeof d.ms === 'number' ? d.ms : undefined;
    const etiqueta = ETIQUETA_ETAPA[id] ?? id;

    // Un aviso de guardado llega después del resultado: no devuelve la tarjeta a "analizando".
    if (!(d.etapa === 'guardado' && c.fase === 'completo')) c.fase = 'analizando';
    const previa = c.etapas[id];
    const reintentos = (previa?.reintentos ?? 0) + (estadoApi === 'reintentando' ? 1 : 0);
    const mapa: Record<string, EstadoEtapa> = {
      iniciada: 'ejecutando',
      reintentando: 'reintentando',
      ok: 'ok',
      rechazado: 'rechazado',
      alerta: 'alerta',
      error: 'error',
      omitida: 'omitida'
    };
    const estado = mapa[estadoApi];
    if (!estado) return;
    const captura = typeof d.captura === 'string' && d.captura ? d.captura : previa?.captura;
    c.etapas[id] = { estado, detalle: detalle ?? previa?.detalle, ms: ms ?? previa?.ms, reintentos, captura };

    const tiempo = ms ? ` (${ms} ms)` : '';
    switch (estado) {
      case 'ejecutando':
        this.linea(c, id, 'info', `Consultando ${etiqueta}…`);
        break;
      case 'reintentando':
        this.linea(c, id, 'warn', `Reintentando ${etiqueta}: ${detalle ?? 'la consulta falló'}`);
        break;
      case 'ok':
        this.linea(c, id, 'ok', `${etiqueta} aprobado${detalle ? ` — ${detalle}` : ''}${tiempo}`);
        break;
      case 'alerta':
        this.linea(c, id, 'warn', `${etiqueta}: ALERTA${detalle ? ` — ${detalle}` : ''}${tiempo}`);
        break;
      case 'rechazado':
        this.linea(c, id, 'error', `${etiqueta} no cumple${detalle ? ` — ${detalle}` : ''}${tiempo}`);
        break;
      case 'error':
        this.linea(c, id, 'error', `${etiqueta} no pudo verificarse${detalle ? ` — ${detalle}` : ''}${tiempo}`);
        break;
      default:
        break;
    }
  }

  /** El resultado ya se mostró; ahora quedó en la base y trae el id de su postulación. */
  private alGuardar(d: Record<string, unknown>): void {
    const c = this.porIndice[Number(d.indice)];
    const id = typeof d.postulacion_id === 'string' ? d.postulacion_id : '';
    if (!c?.resultado || !id || c.resultado.postulacion_id === id) return;
    c.resultado = { ...c.resultado, postulacion_id: id };
  }

  private alCompletar(r: CandidatoApi): void {
    const c = this.obtener(r.indice, r.nombre);
    c.nombre = r.nombre;
    c.cedula = r.cedula && r.cedula !== 'No especificada' ? r.cedula : '';
    c.urlDetalle = r.url_detalle;
    if (r.foto_url) c.fotoUrl = r.foto_url;
    // Lo que llega repetido al unirse a un análisis ya en marcha no terminó "ahora": no se anima.
    if (!r.cache && c.fase !== 'completo' && !this.repitiendo) c.terminadoEn = Date.now();
    c.resultado = r;
    c.desdeCache = Boolean(r.cache);
    c.etapas = etapasDesdeResultado(r, c.etapas);
    c.fase = 'completo';

    const ev = r.evaluacion_idoneidad;
    const origen = r.cache ? ' (resultado guardado)' : '';
    if (ev?.verificacion_pendiente) {
      this.linea(c, 'general', 'warn', `VERIFICACIÓN PENDIENTE${origen}: ${(ev.motivos_descarte ?? []).join('; ')}`);
    } else if (r.es_apto) {
      this.linea(c, 'general', 'ok', `APTO${origen}: cumple las ${ETAPAS_NUCLEO.length} reglas`);
    } else {
      this.linea(c, 'general', 'error', `NO APTO${origen}: ${(ev?.motivos_descarte ?? []).join('; ') || 'no cumple los requisitos'}`);
    }
  }
}

const ID_GRUPO_OPERADOR_CONDUCTOR = 'grupo-operador-conductor';

/** Une en una sola vacante virtual todas las de Operador o Conductor para analizarlas juntas. */
function agruparVacantes(crudas: VacanteApi[]): VacanteApi[] {
  // Las hojas de vida del correo son todas para conductor: van dentro de este grupo, no aparte.
  const miembros = crudas.filter((v) => v.origen === 'correo' || /\b(operador|conductor)/i.test(v.titulo));
  if (miembros.length < 2) return crudas;
  const grupo: VacanteApi = {
    id: ID_GRUPO_OPERADOR_CONDUCTOR,
    slug: 'operador-conductor',
    titulo: 'Operador / Conductor',
    url_candidatos: miembros[0].url_candidatos,
    total_candidatos: miembros.some((m) => m.total_candidatos === null)
      ? null
      : miembros.reduce((suma, m) => suma + (m.total_candidatos ?? 0), 0),
    excluida: miembros.every((m) => m.excluida),
    procesada: miembros.some((m) => m.procesada),
    ids_miembros: miembros.map((m) => m.id),
    resumen: miembros.some((m) => m.resumen)
      ? miembros.reduce(
          (suma, m) => ({
            postulados: suma.postulados + (m.resumen?.postulados ?? 0),
            aptos: suma.aptos + (m.resumen?.aptos ?? 0),
            no_aptos: suma.no_aptos + (m.resumen?.no_aptos ?? 0),
            pendientes: suma.pendientes + (m.resumen?.pendientes ?? 0)
          }),
          { postulados: 0, aptos: 0, no_aptos: 0, pendientes: 0 }
        )
      : undefined,
    total_correo: miembros.filter((m) => m.origen === 'correo').reduce((suma, m) => suma + (m.total_candidatos ?? 0), 0)
  };
  const resultado: VacanteApi[] = [];
  for (const v of crudas) {
    if (v === miembros[0]) resultado.push(grupo);
    else if (!miembros.includes(v)) resultado.push(v);
  }
  return resultado;
}

/** Misma interfaz que SesionAnalisis, pero reúne varias vacantes: candidatos fusionados y análisis en serie. */
class SesionGrupo {
  private corriendo = $state(false);
  private cancelado = false;
  private estadoFinal = $state<EstadoAnalisis>('inactivo');
  private mensajeFinal = $state<string | null>(null);
  private bloqueo = $state<{ id: string; titulo: string } | null>(null);

  constructor(private miembros: () => { vacante: VacanteApi; sesion: SesionAnalisis }[]) {}

  private get sesiones(): SesionAnalisis[] {
    return this.miembros().map((m) => m.sesion);
  }

  private get actual(): SesionAnalisis | undefined {
    return this.sesiones.find((s) => s.activo);
  }

  // Memorizado ($derived): antes era un getter que unía y ordenaba a todos en cada lectura, y la pantalla
  // y el avatar lo leen muchas veces por cuadro.
  readonly candidatos = $derived.by((): CandidatoEnVivo[] => {
    // Una misma persona puede estar en dos vacantes del grupo con el mismo número: se muestra una vez (el
    // identificador debe ser único: la pantalla lo usa como clave).
    const vistos = new Set<string>();
    return this.sesiones
      .flatMap((s) => s.candidatos)
      .filter((c) => !vistos.has(c.slug) && vistos.add(c.slug))
      .sort((a, b) => a.indice - b.indice);
  });

  /** Alguna vacante del grupo repite lo ya ocurrido (no es en vivo). */
  get repitiendo(): boolean {
    return this.sesiones.some((s) => s.repitiendo);
  }

  get activo(): boolean {
    return this.corriendo || this.actual !== undefined;
  }

  get estado(): EstadoAnalisis {
    return this.actual?.estado ?? (this.corriendo ? 'en_curso' : this.estadoFinal);
  }

  get mensaje(): string | null {
    return this.actual?.mensaje ?? this.mensajeFinal;
  }

  get errorCarga(): string | null {
    return this.sesiones.find((s) => s.errorCarga)?.errorCarga ?? null;
  }

  get bloqueadoPor(): { id: string; titulo: string } | null {
    return this.bloqueo;
  }

  get cargandoGuardados(): boolean {
    return this.sesiones.some((s) => s.cargandoGuardados);
  }

  readonly resumen = $derived.by((): ResumenAnalisis => {
    const suma = { ...RESUMEN_VACIO };
    const lista = this.sesiones;
    for (const { resumen } of lista) {
      suma.descubiertos += resumen.descubiertos;
      suma.procesados += resumen.procesados;
      suma.cache += resumen.cache;
      suma.aptos += resumen.aptos;
      suma.no_aptos += resumen.no_aptos;
      suma.errores += resumen.errores;
      suma.pendientes += resumen.pendientes;
      suma.descartes_tempranos += resumen.descartes_tempranos;
    }
    suma.listado_completo = lista.length > 0 && lista.every((s) => s.resumen.listado_completo);
    return suma;
  });

  private readonly indiceSlug = $derived(new Map(this.candidatos.map((c) => [c.slug, c])));

  porSlug(slug: string): CandidatoEnVivo | undefined {
    return this.indiceSlug.get(slug);
  }

  async cargarGuardados(): Promise<void> {
    await Promise.all(this.sesiones.map((s) => s.cargarGuardados()));
  }

  /** El servidor solo admite un análisis a la vez: cada vacante real espera a que termine la anterior. */
  async iniciar(opciones: OpcionesAnalisisApi): Promise<void> {
    if (this.activo) return;
    this.corriendo = true;
    this.cancelado = false;
    this.bloqueo = null;
    this.mensajeFinal = null;
    this.estadoFinal = 'completado';
    try {
      const espera = (ms: number) => new Promise((r) => setTimeout(r, ms));
      for (const { vacante, sesion } of this.miembros()) {
        if (vacante.excluida) continue;
        if (this.cancelado) break;
        // Si otro análisis (p. ej. el proceso automático) tiene ocupado el servidor, se espera su turno
        // en vez de abandonar el resto de las vacantes del grupo.
        for (let intento = 0; ; intento++) {
          await sesion.iniciar(opciones);
          while (sesion.activo) await espera(1000);
          const ocupado = sesion.estado === 'error' && sesion.bloqueadoPor !== null;
          if (!ocupado || this.cancelado || intento >= 240) break;
          await espera(15000);
        }
        if (sesion.estado === 'error') {
          // Un fallo en una vacante no detiene las demás: se sigue y se avisa al final.
          this.estadoFinal = 'error';
          this.mensajeFinal ??= sesion.mensaje;
          this.bloqueo ??= sesion.bloqueadoPor;
          continue;
        }
        if (sesion.estado === 'cancelado') this.cancelado = true;
      }
      if (this.cancelado) this.estadoFinal = 'cancelado';
    } finally {
      this.corriendo = false;
    }
  }

  async detener(): Promise<void> {
    this.cancelado = true;
    await this.actual?.detener();
  }
}

/** Cuánto espera una pestaña oculta antes de soltar sus conexiones en vivo (cambiar de pestaña un momento no las corta). */
const PAUSA_PESTANA_OCULTA_MS = 15_000;
/** Si el aviso de que terminó la actualización de la lista no llega, se deja de mostrar "actualizando". */
const PLAZO_CONTEO_MS = 7 * 60_000;

/** Estado compartido de la sección: lista de vacantes y una sesión de análisis por vacante. */
class AlmacenRrhh {
  private crudas = $state<VacanteApi[]>([]);
  /** Vacantes tal como se muestran: Operador y Conductor ya vienen reunidas en una sola. */
  readonly vacantes = $derived(agruparVacantes(this.crudas));
  cargando = $state(false);
  /** El servidor está actualizando la lista (conteos de inscritos) en segundo plano. */
  contando = $state(false);
  error = $state<string | null>(null);
  /** Aviso (no error): p. ej. no se pudo actualizar el conteo de inscritos. */
  aviso = $state<string | null>(null);
  /** Cuándo leyó el servidor la lista que se muestra (ms); null hasta la primera carga. */
  listaLeidaEn = $state<number | null>(null);
  /** Estado de la conexión en vivo con el servidor, para que la pantalla lo diga en vez de parecer colgada. */
  conexion = $state<'conectando' | 'en_vivo' | 'reconectando' | 'pausada'>('conectando');

  private sesiones = new Map<string, SesionAnalisis>();
  private grupos = new Map<string, SesionGrupo>();
  private solicitada = false;
  private cargaEnCurso: AbortController | null = null;
  private finConteo: ReturnType<typeof setTimeout> | undefined;

  private sesionReal(idVacante: string): SesionAnalisis {
    let s = this.sesiones.get(idVacante);
    if (!s) {
      s = new SesionAnalisis(idVacante);
      this.sesiones.set(idVacante, s);
      this.escucharCambios();
      // Si su análisis ya estaba corriendo (el aviso de inicio llegó antes de abrir esta vacante), se une.
      if (this.cambiosYaAbierto) this.programarSincronizacion();
    }
    return s;
  }

  private sincronizacionProgramada = false;

  /** Varias sesiones nuevas a la vez (la lista de vacantes) se ponen al día con una sola consulta. */
  private programarSincronizacion(): void {
    if (this.sincronizacionProgramada) return;
    this.sincronizacionProgramada = true;
    setTimeout(() => {
      this.sincronizacionProgramada = false;
      void this.sincronizarAnalisis();
    }, 50);
  }

  /** Sube con cada aviso del backend; las pantallas que leen estado propio (rama de contacto) lo observan. */
  versionCambios = $state(0);
  /** El servidor está revisando el correo y Computrabajo ahora mismo (una vez por minuto). */
  revisando = $state(false);
  private finRevision: ReturnType<typeof setTimeout> | undefined;
  private cambiosIniciado = false;
  private cambiosYaAbierto = false;
  private cambios: EventSource | null = null;
  private temporizadorOculta: ReturnType<typeof setTimeout> | undefined;

  /**
   * Una sola conexión para toda la sección, y nada de sondeos: el backend avisa cuando empieza o
   * termina un análisis, cuando cambia la lista de vacantes y cuando un candidato envió su formulario
   * (o terminó su reanudación); cada pantalla relee solo lo que cambió.
   */
  private escucharCambios(): void {
    if (this.cambiosIniciado || typeof EventSource === 'undefined') return;
    this.cambiosIniciado = true;
    document.addEventListener('visibilitychange', () => this.alCambiarVisibilidad());
    this.conectarCambios();
  }

  /**
   * El navegador solo abre 6 conexiones a la vez por servidor, compartidas entre todas las pestañas, y
   * cada conexión en vivo ocupa una mientras dure. Una pestaña oculta las suelta y al volver las retoma
   * (el servidor le manda lo que pasó mientras tanto): varias pestañas ya no bloquean a la que se usa.
   */
  private alCambiarVisibilidad(): void {
    clearTimeout(this.temporizadorOculta);
    if (document.hidden) {
      this.temporizadorOculta = setTimeout(() => {
        this.cambios?.close();
        this.cambios = null;
        this.conexion = 'pausada';
        this.sesiones.forEach((s) => s.pausar());
      }, PAUSA_PESTANA_OCULTA_MS);
      return;
    }
    this.sesiones.forEach((s) => s.reanudar());
    if (!this.cambios) this.conectarCambios(); // al abrir se pone al día (análisis en curso y cambios)
  }

  private conectarCambios(): void {
    const origen = new EventSource(urlCambios(), { withCredentials: true });
    this.cambios = origen;
    const refrescar = () => {
      this.versionCambios++;
      this.sesiones.forEach((s) => void s.refrescar());
    };
    const leer = <T>(e: Event): T | null => {
      try {
        return JSON.parse((e as MessageEvent<string>).data) as T;
      } catch {
        return null;
      }
    };
    origen.addEventListener('cambio', () => {
      refrescar();
      void this.actualizarVacantes(); // los totales de cada vacante salen de la lista
    });
    origen.addEventListener('job', (e) => {
      const d = leer<{ id?: string; oferta_id?: string; estado?: string }>(e);
      if (d?.id && d.oferta_id) this.alCambiarJob(d.id, d.oferta_id, d.estado ?? '');
    });
    origen.addEventListener('vacantes', (e) => {
      const d = leer<{ error?: string; parcial?: boolean }>(e);
      if (d?.error) {
        this.contando = false;
        if (this.crudas.length > 0) this.aviso = `No se pudo actualizar la lista de vacantes: ${d.error}`;
        return;
      }
      // "parcial": llegó el conteo de algunas vacantes y sigue contando; sin él, la lista quedó completa.
      void this.actualizarVacantes(!d?.parcial);
    });
    for (const tipo of ['candidato.etapa', 'candidato.completo'] as const) {
      origen.addEventListener(tipo, (e) => {
        const datos = leer<Record<string, unknown>>(e);
        if (!datos) return console.error(`Evento ${tipo} ilegible`);
        for (const s of this.sesiones.values()) if (s.alReanudar(tipo, datos)) break;
      });
    }
    origen.addEventListener('reanudacion.fallida', (e) => {
      const d = leer<{ intento?: number; mensaje?: string }>(e);
      if (d) toast.error(`No se pudo continuar el análisis tras el formulario (intento ${d.intento ?? '?'}): ${d.mensaje ?? 'error'}`);
    });
    origen.addEventListener('revision', (e) => {
      const d = leer<{ estado?: string }>(e);
      if (!d) return;
      this.revisando = d.estado === 'inicio';
      clearTimeout(this.finRevision);
      // Si el aviso de "fin" se pierde, no se queda despierto para siempre.
      if (this.revisando) this.finRevision = setTimeout(() => (this.revisando = false), 3 * 60_000);
    });
    // Al abrir (y tras una reconexión, en la que pudo perderse un aviso) se pone al día una sola vez.
    origen.onopen = () => {
      this.conexion = 'en_vivo';
      this.revisando = false;
      if (this.cambiosYaAbierto) {
        refrescar();
        void this.actualizarVacantes();
      }
      this.cambiosYaAbierto = true;
      void this.sincronizarAnalisis();
    };
    // Si el servidor respondió con un error (reinicio, proxy sin backend, sesión) el navegador cierra el
    // EventSource para siempre y no reintenta: se abre uno nuevo, y las noticias dejarían de llegar.
    origen.onerror = () => {
      if (this.cambios === origen) this.conexion = 'reconectando';
      if (origen.readyState !== EventSource.CLOSED) return; // CONNECTING: el navegador reintenta solo
      origen.close();
      if (this.cambios !== origen) return; // se soltó a propósito (pestaña oculta)
      this.cambios = null;
      setTimeout(() => {
        if (!this.cambios && !document.hidden) this.conectarCambios();
      }, 3000);
    };
  }

  /** Empezó o terminó un análisis (lo haya lanzado esta pantalla, otra o el proceso automático). */
  private alCambiarJob(idJob: string, idOferta: string, estado: string): void {
    const sesion = this.sesionDeOferta(idOferta);
    if (estado === 'en_curso') {
      if (sesion && !sesion.activo) void this.unirseConGuardados(sesion, idJob);
      return;
    }
    // Terminó: la sesión que lo seguía relee al cerrar su stream; si nadie lo seguía, se relee aquí.
    if (sesion && !sesion.activo) void sesion.refrescar();
    void this.actualizarVacantes();
  }

  private sesionDeOferta(idOferta: string): SesionAnalisis | undefined {
    const buscado = idOferta.toLowerCase();
    return [...this.sesiones.values()].find((s) => s.idVacante.toLowerCase() === buscado);
  }

  /** Antes de unirse se cargan los ya analizados: así nadie con resultado aparece "en cola" mientras tanto. */
  private async unirseConGuardados(sesion: SesionAnalisis, idJob: string): Promise<void> {
    await sesion.cargarGuardados();
    sesion.unirse(idJob);
  }

  /** Un análisis puede estar corriendo sin que esta pantalla lo haya iniciado: se sigue en vivo. */
  private async sincronizarAnalisis(): Promise<void> {
    try {
      const job = await jobActivo();
      const sesion = job ? this.sesionDeOferta(job.oferta_id) : undefined;
      if (job && sesion && !sesion.activo) await this.unirseConGuardados(sesion, job.id);
    } catch {
      /* sin conexión: el EventSource reconecta y vuelve a intentarlo al abrir */
    }
  }

  /** Sesión de una vacante; el id de una real agrupada (o el del grupo) devuelve la sesión del grupo. */
  sesion(idVacante: string): SesionAnalisis | SesionGrupo {
    const vacante = this.vacantePorId(idVacante);
    if (!vacante?.ids_miembros) return this.sesionReal(idVacante);
    let g = this.grupos.get(vacante.id);
    if (!g) {
      const ids = vacante.ids_miembros;
      g = new SesionGrupo(() =>
        this.crudas
          .filter((c) => ids.includes(c.id))
          .map((c) => ({ vacante: c, sesion: this.sesionReal(c.id) }))
      );
      this.grupos.set(vacante.id, g);
    }
    return g;
  }

  vacantePorId(id: string): VacanteApi | undefined {
    const buscado = id.toLowerCase();
    return this.vacantes.find(
      (v) => v.id.toLowerCase() === buscado || v.ids_miembros?.some((m) => m.toLowerCase() === buscado)
    );
  }

  /** Carga las vacantes una sola vez por sesión del navegador (para recargar, usar cargarVacantes). */
  asegurarVacantes(): Promise<void> {
    if (this.solicitada) return Promise.resolve();
    this.solicitada = true;
    return this.cargarVacantes();
  }

  /**
   * El servidor responde al instante con la última lista que tiene; si la está actualizando (conteos de
   * inscritos) avisa al terminar y aquí se vuelve a pedir. refrescar=true (botón) pide esa actualización.
   */
  async cargarVacantes(refrescar = false): Promise<void> {
    this.solicitada = true;
    this.escucharCambios();
    this.cargaEnCurso?.abort();
    const control = new AbortController();
    this.cargaEnCurso = control;
    this.cargando = true;
    this.error = null;
    this.aviso = null;
    try {
      const { vacantes, actualizando, leidaEn } = await listarVacantes(refrescar, control.signal);
      this.crudas = this.conservarConteos(vacantes);
      this.prepararSesiones();
      this.listaLeidaEn = leidaEn;
      this.marcarConteo(actualizando);
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      const mensaje = error instanceof Error ? error.message : 'No se pudieron cargar las vacantes';
      if (this.crudas.length > 0) this.aviso = `No se pudo actualizar la lista de vacantes: ${mensaje}`;
      else this.error = mensaje;
    } finally {
      if (this.cargaEnCurso === control) {
        this.cargando = false;
        this.cargaEnCurso = null;
      }
    }
  }

  /** Relee la lista sin mostrar "cargando" (llegó un aviso de que cambió). */
  private async actualizarVacantes(listaCompleta = false): Promise<void> {
    if (this.cargaEnCurso || !this.solicitada) return;
    try {
      const { vacantes, actualizando, leidaEn } = await listarVacantes(false);
      this.crudas = this.conservarConteos(vacantes);
      this.prepararSesiones();
      this.listaLeidaEn = leidaEn;
      this.marcarConteo(actualizando && !listaCompleta);
      this.aviso = null;
      this.error = null;
    } catch (error) {
      console.error('No se pudo actualizar la lista de vacantes', error);
    }
  }

  /**
   * Crea las sesiones (y las de los grupos) apenas llega la lista, fuera de cualquier cálculo reactivo.
   * Svelte no registra como dependencia una señal creada dentro del mismo cálculo que la lee: si una
   * sesión nacía dentro del $derived del grupo o del render de la tarjeta, esa vista quedaba congelada
   * (el grupo Operador / Conductor no mostraba a nadie aunque el análisis avanzaba).
   */
  private prepararSesiones(): void {
    for (const v of this.crudas) this.sesionReal(v.id);
    for (const v of this.vacantes) this.sesion(v.id);
  }

  private marcarConteo(actualizando: boolean): void {
    this.contando = actualizando;
    clearTimeout(this.finConteo);
    if (actualizando) this.finConteo = setTimeout(() => (this.contando = false), PLAZO_CONTEO_MS);
  }

  private conservarConteos(nuevas: VacanteApi[]): VacanteApi[] {
    return nuevas.map((v) => {
      const previa = this.crudas.find((p) => p.id === v.id);
      return v.total_candidatos === null && previa ? { ...v, total_candidatos: previa.total_candidatos } : v;
    });
  }
}

export const almacenRrhh = new AlmacenRrhh();

export { ORDEN_ETAPAS };
