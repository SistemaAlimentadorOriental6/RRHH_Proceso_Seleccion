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

  porSlug(slug: string): CandidatoEnVivo | undefined {
    return this.candidatos.find((c) => c.slug === slug);
  }

  /**
   * Carga lo analizado antes, una sola vez por sesión. Se llama desde $effect: la bandera no es
   * reactiva y la comprobación va primero, así el efecto no se vuelve a disparar al cambiar estado
   * (antes, una vacante sin candidatos guardados pedía la lista en bucle).
   */
  async cargarGuardados(): Promise<void> {
    if (this.guardadosPedidos || this.activo) return;
    this.guardadosPedidos = true;
    this.cargandoGuardados = true;
    try {
      const guardados = await listarCandidatosGuardados(this.idVacante);
      // Si mientras tanto arrancó un análisis en vivo, sus datos mandan.
      if (this.activo || this.candidatos.length > 0) return;
      for (const r of guardados) this.porIndice[r.indice] = candidatoDesdeApi(r, true);
    } catch (error) {
      this.guardadosPedidos = false; // se reintenta en la siguiente visita
      this.mensaje = error instanceof Error ? error.message : 'No se pudieron leer los candidatos guardados';
    } finally {
      this.cargandoGuardados = false;
    }
  }

  /**
   * Vuelve a leer de la base los candidatos ya cargados y reemplaza los que cambiaron (p. ej. el
   * candidato envió su formulario o terminó su reanudación). No toca nada mientras corre un análisis.
   */
  async refrescar(): Promise<void> {
    if (!this.guardadosPedidos || this.activo) return;
    try {
      const guardados = await listarCandidatosGuardados(this.idVacante);
      if (this.activo) return;
      const sinIndice = (x: CandidatoApi) => JSON.stringify({ ...x, indice: 0 });
      for (const r of guardados) {
        // A la persona se la reconoce por su ficha: tras reubicarse en el listado su posición ya no
        // coincide con la guardada, y buscarla por posición duplicaba o pisaba a otra persona.
        const actual = Object.values(this.porIndice).find(
          (c) => (r.url_detalle && c.urlDetalle === r.url_detalle) || (c.indice === r.indice && c.nombre === r.nombre)
        );
        if (!actual) {
          // Analizado por otro medio: aparece sin recargar (si su posición está libre).
          if (!this.porIndice[r.indice]) this.porIndice[r.indice] = candidatoDesdeApi(r, true);
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
    this.idJob = idJob;
    this.conectar(urlEventosJob(idJob));
  }

  private conectar(url: string): void {
    this.conservados = new Map();
    if (!this.forzado) {
      for (const c of Object.values(this.porIndice)) {
        if (c.fase !== 'completo' || !c.resultado) continue;
        this.conservados.set(c.nombre, [...(this.conservados.get(c.nombre) ?? []), c]);
      }
    }
    const origen = new EventSource(url);
    this.origen = origen;

    const escuchar = (tipo: string, manejar: (datos: Record<string, unknown>) => void) =>
      origen.addEventListener(tipo, (e) => {
        try {
          manejar(JSON.parse((e as MessageEvent<string>).data) as Record<string, unknown>);
        } catch (error) {
          console.error(`Evento SSE ${tipo} ilegible`, error);
        }
      });

    origen.onopen = () => {
      if (this.estado === 'conectando' || this.estado === 'reconectando') this.estado = 'en_curso';
    };

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
      void this.alCerrarseElStream();
    };
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
    this.estado = estado;
    this.origen?.close();
    this.origen = null;
    // Quien quedó "analizando" cuando el flujo se cortó vuelve a la cola para no mostrarse colgado.
    for (const c of this.candidatos) {
      if (c.fase === 'analizando') c.fase = 'en_cola';
    }
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

    c.fase = 'analizando';
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

  private alCompletar(r: CandidatoApi): void {
    const c = this.obtener(r.indice, r.nombre);
    c.nombre = r.nombre;
    c.cedula = r.cedula && r.cedula !== 'No especificada' ? r.cedula : '';
    c.urlDetalle = r.url_detalle;
    if (r.foto_url) c.fotoUrl = r.foto_url;
    if (!r.cache && c.fase !== 'completo') c.terminadoEn = Date.now();
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

  get candidatos(): CandidatoEnVivo[] {
    // Una misma persona puede estar en dos vacantes del grupo con el mismo número: se muestra una vez (el
    // identificador debe ser único: la pantalla lo usa como clave).
    const vistos = new Set<string>();
    return this.sesiones
      .flatMap((s) => s.candidatos)
      .filter((c) => !vistos.has(c.slug) && vistos.add(c.slug))
      .sort((a, b) => a.indice - b.indice);
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

  get bloqueadoPor(): { id: string; titulo: string } | null {
    return this.bloqueo;
  }

  get cargandoGuardados(): boolean {
    return this.sesiones.some((s) => s.cargandoGuardados);
  }

  get resumen(): ResumenAnalisis {
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
  }

  porSlug(slug: string): CandidatoEnVivo | undefined {
    return this.candidatos.find((c) => c.slug === slug);
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

/** Estado compartido de la sección: lista de vacantes y una sesión de análisis por vacante. */
class AlmacenRrhh {
  private crudas = $state<VacanteApi[]>([]);
  /** Vacantes tal como se muestran: Operador y Conductor ya vienen reunidas en una sola. */
  readonly vacantes = $derived(agruparVacantes(this.crudas));
  cargando = $state(false);
  contando = $state(false);
  error = $state<string | null>(null);
  /** Aviso (no error): p. ej. la lista salió de la copia guardada porque hay un análisis en curso. */
  aviso = $state<string | null>(null);

  private sesiones = new Map<string, SesionAnalisis>();
  private grupos = new Map<string, SesionGrupo>();
  private solicitada = false;
  private cargaEnCurso: AbortController | null = null;

  private sesionReal(idVacante: string): SesionAnalisis {
    let s = this.sesiones.get(idVacante);
    if (!s) {
      s = new SesionAnalisis(idVacante);
      this.sesiones.set(idVacante, s);
      this.escucharCambios();
    }
    return s;
  }

  /** Sube con cada aviso del backend; las pantallas que leen estado propio (rama de contacto) lo observan. */
  versionCambios = $state(0);
  /** El servidor está revisando el correo y Computrabajo ahora mismo (una vez por minuto). */
  revisando = $state(false);
  private finRevision: ReturnType<typeof setTimeout> | undefined;
  private origenCambios: EventSource | null = null;
  private ultimoRefresco = 0;

  /**
   * Una sola conexión para toda la sección: cuando el backend avisa que un candidato envió su
   * formulario (o terminó su reanudación), cada sesión relee lo suyo. EventSource se reconecta solo.
   */
  private escucharCambios(): void {
    if (this.origenCambios || typeof EventSource === 'undefined') return;
    // Un análisis puede correr sin que esta pantalla lo haya iniciado: se detecta y se sigue en vivo.
    setInterval(() => void this.seguirAnalisisEnCurso(), 2000);
    const origen = new EventSource(urlCambios());
    this.origenCambios = origen;
    const refrescar = () => {
      this.versionCambios++;
      this.sesiones.forEach((s) => void s.refrescar());
    };
    origen.addEventListener('cambio', refrescar);
    for (const tipo of ['candidato.etapa', 'candidato.completo'] as const) {
      origen.addEventListener(tipo, (e) => {
        try {
          const datos = JSON.parse((e as MessageEvent<string>).data) as Record<string, unknown>;
          for (const s of this.sesiones.values()) if (s.alReanudar(tipo, datos)) break;
        } catch (error) {
          console.error(`Evento ${tipo} ilegible`, error);
        }
      });
    }
    origen.addEventListener('reanudacion.fallida', (e) => {
      try {
        const d = JSON.parse((e as MessageEvent<string>).data) as { intento?: number; mensaje?: string };
        toast.error(`No se pudo continuar el análisis tras el formulario (intento ${d.intento ?? '?'}): ${d.mensaje ?? 'error'}`);
      } catch {
        /* aviso ilegible: no hay nada que mostrar */
      }
    });
    origen.addEventListener('revision', (e) => {
      try {
        const d = JSON.parse((e as MessageEvent<string>).data) as { estado?: string };
        this.revisando = d.estado === 'inicio';
        clearTimeout(this.finRevision);
        // Si el aviso de "fin" se pierde, no se queda despierto para siempre.
        if (this.revisando) this.finRevision = setTimeout(() => (this.revisando = false), 3 * 60_000);
      } catch {
        /* aviso ilegible: se ignora */
      }
    });
    // Tras una reconexión pudo haberse perdido un aviso: se relee por si acaso.
    let primera = true;
    origen.onopen = () => {
      this.revisando = false;
      if (!primera) refrescar();
      primera = false;
    };
  }

  private async seguirAnalisisEnCurso(): Promise<void> {
    if (document.hidden) return;
    try {
      const job = await jobActivo();
      if (!job) {
        // Sin análisis en curso: lo que terminó por su cuenta (p. ej. el proceso automático, o las hojas
        // de vida del correo) se relee de la base, para que la lista no se quede con lo que había al entrar.
        if (Date.now() - this.ultimoRefresco > 10_000) {
          this.ultimoRefresco = Date.now();
          this.sesiones.forEach((s) => void s.refrescar());
        }
        return;
      }
      const sesion = [...this.sesiones.values()].find((s) => s.idVacante.toLowerCase() === job.oferta_id.toLowerCase());
      if (sesion && !sesion.activo) sesion.unirse(job.id);
    } catch {
      /* sin conexión: se reintenta en el siguiente ciclo */
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

  /** Primero la lista rápida (sin conteo) para pintar enseguida; luego se completa con los inscritos. */
  async cargarVacantes(): Promise<void> {
    this.cargaEnCurso?.abort();
    const control = new AbortController();
    this.cargaEnCurso = control;
    this.cargando = true;
    this.error = null;
    this.aviso = null;
    try {
      const rapida = await listarVacantes(false, control.signal);
      this.crudas = this.conservarConteos(rapida);
      this.cargando = false;

      this.contando = true;
      const completa = await listarVacantes(true, control.signal);
      this.crudas = completa;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      const mensaje = error instanceof Error ? error.message : 'No se pudieron cargar las vacantes';
      // Con vacantes ya pintadas, un fallo del conteo no es un error de la pantalla.
      if (this.crudas.length > 0) this.aviso = `No se pudo actualizar el conteo de inscritos: ${mensaje}`;
      else this.error = mensaje;
    } finally {
      if (this.cargaEnCurso === control) {
        this.cargando = false;
        this.contando = false;
        this.cargaEnCurso = null;
      }
    }
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
