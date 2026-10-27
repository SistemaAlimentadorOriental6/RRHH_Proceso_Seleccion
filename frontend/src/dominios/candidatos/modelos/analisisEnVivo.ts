/** Contratos con el backend Go (go-rrhh) para vacantes de Computrabajo y su análisis en vivo (SSE). */

export interface VacanteApi {
  id: string;
  slug: string;
  titulo: string;
  url_candidatos: string;
  /** Inscritos reales en Computrabajo; null mientras no se ha contado o si el conteo falló. */
  total_candidatos: number | null;
  error_conteo?: string;
  excluida: boolean;
  procesada: boolean;
  /** De dónde vienen las personas; «correo» es la vacante virtual de las hojas de vida recibidas por el buzón. */
  origen?: 'computrabajo' | 'correo';
  /** Solo en vacantes agrupadas (p. ej. Operador/Conductor): ids de las vacantes reales que reúne. */
  ids_miembros?: string[];
  /** Solo en vacantes agrupadas: cuántas de las personas del total llegaron por correo. */
  total_correo?: number;
  /** Totales de lo ya analizado (de la base); ausente si la vacante no tiene nada analizado. */
  resumen?: ResumenAnalizadosApi;
}

export interface ResumenAnalizadosApi {
  postulados: number;
  aptos: number;
  /** Sin contar los de verificación pendiente. */
  no_aptos: number;
  pendientes: number;
}

export type IdEtapa =
  | 'perfil'
  | 'genero'
  | 'reintegro'
  | 'runt'
  | 'simit'
  | 'adres'
  | 'procuraduria'
  | 'sarlaft'
  | 'ofac'
  | 'ia';

/** Orden en que el flujo muestra (y el backend ejecuta) las etapas. */
export const ORDEN_ETAPAS: readonly IdEtapa[] = [
  'perfil',
  'genero',
  'reintegro',
  'adres',
  'runt',
  'simit',
  'procuraduria',
  'sarlaft',
  'ofac',
  'ia'
];

/** Etapas que definen si la persona es apta; el resto son alertas informativas. */
export const ETAPAS_NUCLEO: readonly IdEtapa[] = ['genero', 'reintegro', 'adres', 'runt', 'simit'];

export type EstadoEtapa =
  | 'ejecutando'
  | 'reintentando'
  | 'ok'
  | 'rechazado'
  | 'alerta'
  | 'error'
  | 'omitida';

export interface EtapaCandidato {
  estado: EstadoEtapa;
  detalle?: string;
  ms?: number;
  /** Veces que se reintentó la consulta (evento "reintentando"). */
  reintentos: number;
  /** Ruta en el servidor de la captura de pantalla de esta consulta (si se tomó). */
  captura?: string;
}

export type NivelLinea = 'info' | 'ok' | 'warn' | 'error';

export interface LineaRegistro {
  id: number;
  hora: string;
  etapa: IdEtapa | 'general';
  nivel: NivelLinea;
  texto: string;
}

/** Calificación de la IA sobre la hoja de vida y los datos oficiales (modelos.EvaluacionIA). */
export interface EvaluacionIaApi {
  score_total: number;
  score_experiencia: number;
  score_conduccion_normativo: number;
  score_estabilidad: number;
  score_afinidad: number;
  dictamen: string;
  por_que_es_el_hombre?: string;
  fortalezas?: string[];
  alerta?: string;
  nivel_educativo?: string;
  anos_experiencia?: number;
  habilidades?: string[];
  observaciones?: string;
  just_experiencia?: string;
  just_normativo?: string;
  just_estabilidad?: string;
  just_afinidad?: string;
  inconsistencias?: string[];
  experiencia?: { empresa: string; cargo: string; periodo?: string; duracion?: string; descripcion?: string }[];
  modelo?: string;
  evaluado_en?: string;
}

/** Subconjunto del Candidato que serializa el backend (modelos.Candidato). */
export interface CandidatoApi {
  indice: number;
  nombre: string;
  cedula: string;
  url_detalle: string;
  foto_url?: string;
  ruta_pdf?: string;
  ruta_cv_original?: string;
  ruta_perfil_pdf?: string;
  genero?: string;
  sarlaft_onu?: { alerta: boolean; detalles: string } & Record<string, unknown>;
  ofac?: { alerta: boolean; estado: string; detalles?: string } & Record<string, unknown>;
  procuraduria?: { alerta: boolean; estado: string; detalles?: string } & Record<string, unknown>;
  adres?: ({ eps: string; regimen: string; municipio: string; departamento: string } & Record<string, unknown>) | null;
  runt?: ({ categorias_activas?: string[] | null } & Record<string, unknown>) | null;
  simit?: Record<string, unknown> | null;
  /** Administremos: si la cédula ya figura como empleado (informativo, no descarta). */
  reintegro?: {
    estado: string;
    alerta: boolean;
    detalles?: string;
    estado_empleado?: string;
    cargo?: string;
    fecha_ingreso?: string;
    fecha_retiro?: string;
    sanciones?: string;
    registros?: number;
  } | null;
  /** Captura de pantalla de cada etapa (id de etapa -> ruta en el servidor). */
  capturas?: Partial<Record<IdEtapa, string>>;
  es_apto: boolean;
  evaluacion_idoneidad?: {
    es_apto: boolean;
    genero: string;
    cumple_genero: boolean;
    cumple_eps: boolean;
    detalle_eps: string;
    cumple_runt: boolean;
    detalle_runt: string;
    cumple_simit: boolean;
    detalle_simit: string;
    motivos_descarte: string[] | null;
    descarte_temprano?: boolean;
    verificacion_pendiente?: boolean;
  };
  evaluacion_ia?: EvaluacionIaApi;
  error_proceso?: string;
  /**
   * Presente solo cuando el análisis está en pausa o se reanudó por falta de datos del candidato:
   * esperando_datos, sin_contacto, datos_recibidos, solicitud_vencida o identidad_en_revision.
   */
  estado_proceso?:
    | 'esperando_datos'
    | 'sin_contacto'
    | 'datos_recibidos'
    | 'solicitud_vencida'
    | 'identidad_en_revision';
  /** Identifica al candidato en la base de datos (para reenviar su solicitud o ver su historial). */
  postulacion_id?: string;
  /** Edad calculada con la fecha de nacimiento que declaró; la fecha no se expone. */
  edad?: number;
  slug?: string;
  cache?: boolean;
}

export type FaseCandidato = 'en_cola' | 'analizando' | 'completo';

export type Veredicto = 'en_cola' | 'analizando' | 'apto' | 'no_apto' | 'pendiente';

export interface CandidatoEnVivo {
  indice: number;
  /** Único dentro de la vacante: "<indice>-<nombre-en-slug>". */
  slug: string;
  nombre: string;
  cedula: string;
  urlDetalle: string;
  fotoUrl: string;
  fase: FaseCandidato;
  etapas: Partial<Record<IdEtapa, EtapaCandidato>>;
  registro: LineaRegistro[];
  resultado: CandidatoApi | null;
  desdeCache: boolean;
  /** Momento (ms) en que terminó su análisis en vivo; sirve para dejarlo un rato en su sitio antes de acomodarlo. */
  terminadoEn: number | null;
}

export type EstadoAnalisis =
  | 'inactivo'
  | 'conectando'
  | 'en_espera'
  | 'en_curso'
  | 'reconectando'
  | 'completado'
  | 'cancelado'
  | 'error';

export interface ResumenAnalisis {
  descubiertos: number;
  procesados: number;
  cache: number;
  aptos: number;
  no_aptos: number;
  errores: number;
  pendientes: number;
  descartes_tempranos: number;
  listado_completo: boolean;
}

export const RESUMEN_VACIO: ResumenAnalisis = {
  descubiertos: 0,
  procesados: 0,
  cache: 0,
  aptos: 0,
  no_aptos: 0,
  errores: 0,
  pendientes: 0,
  descartes_tempranos: 0,
  listado_completo: false
};

export const ETIQUETA_ETAPA: Record<IdEtapa, string> = {
  perfil: 'Perfil y hoja de vida',
  genero: 'Género',
  reintegro: 'Reintegro',
  runt: 'RUNT',
  simit: 'SIMIT',
  adres: 'EPS (ADRES)',
  procuraduria: 'Procuraduría',
  sarlaft: 'SARLAFT ONU',
  ofac: 'OFAC',
  ia: 'Validación con IA'
};
