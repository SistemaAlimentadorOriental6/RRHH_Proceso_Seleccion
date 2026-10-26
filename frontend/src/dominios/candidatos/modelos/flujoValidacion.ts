export type EstadoNodoValidacion =
  | 'pendiente'
  | 'ejecutando'
  | 'reintentando'
  | 'completado'
  | 'detenido_error'
  | 'omitido';

export type ModoSimulacionNodo =
  | 'exito_directo'
  | 'recuperar_en_reintento'
  | 'fallo_definitivo';

export type NivelLogEjecucion =
  | 'info'
  | 'exito'
  | 'reintento'
  | 'error_critico';

export interface RegistroLogEjecucion {
  id: string;
  marcaTiempo: string;
  idNodo: string;
  ordenNodo: number;
  nombreNodo: string;
  nivel: NivelLogEjecucion;
  intento: number;
  maxIntentos: number;
  mensaje: string;
  detalleTecnico?: string;
}

export interface CampoDestacadoNodo {
  etiqueta: string;
  valorExito: string;
  valorError: string;
}

export interface EvidenciaVisualNodo {
  entidadOficial: string;
  portalUrl: string;
  codigoRadicado: string;
  tituloCertificado: string;
  selloExito: string;
  selloError: string;
  observacionLegal: string;
}

export interface DefinicionNodoValidacion {
  id: string;
  orden: number;
  codigo: string;
  titulo: string;
  subtitulo: string;
  categoria: string;
  claveIcono: string;
  colorAcento: string;
  posicionX: number;
  posicionY: number;
  maxIntentos: number;
  duracionBaseMs: number;
  modoSimulacion: ModoSimulacionNodo;
  estado: EstadoNodoValidacion;
  intentoActual: number;
  duracionRealMs: number;
  resumenSalida: string;
  mensajeErrorTransitorio: string;
  mensajeFalloDefinitivo: string;
  camposDestacados: CampoDestacadoNodo[];
  evidenciaVisual: EvidenciaVisualNodo;
  payloadSalidaExito: Record<string, unknown>;
  payloadSalidaError: Record<string, unknown>;
  /** Solo en modo real: texto del estado que muestra la tarjeta ("Sin verificar", "Alerta"…). */
  etiquetaEstadoVivo?: string;
  /** Solo en modo real: URL de la captura de pantalla que tomó el sistema al consultar la fuente. */
  capturaUrl?: string;
  /** Nodo de decisión (Sí/No): no es una consulta a una fuente, no muestra intentos. */
  esCondicional?: boolean;
  /** Nodo de la rama alterna "sin cédula" (mockup de frontend): no cuenta en el avance del flujo. */
  esRama?: boolean;
}

export interface DatosCandidatoEscenario {
  nombre: string;
  cedula: string;
  edad: number;
  genero: string;
  licencia: string;
  experienciaAnios: number;
  fotoUrl: string;
  municipio: string;
  telefono: string;
  correo: string;
  eps: string;
  regimenEps: string;
  archivoHojaVida: string;
  /** URL de descarga de la hoja de vida original (solo en modo en vivo y si existe). */
  urlHojaVida?: string;
  archivoPerfil: string;
  resumenPerfil: string;
  empresasPrevias: string[];
}

export interface EscenarioValidacion {
  id: string;
  nombre: string;
  descripcion: string;
  vacante: string;
  candidato: DatosCandidatoEscenario;
  configuracionNodos: Record<string, ModoSimulacionNodo>;
}

/**
 * Genera un retrato SVG estilizado y limpio como Data URI para que siempre cargue instantáneamente
 * sin depender de conexión externa.
 */
function crearRetratoSvgDataUri(
  iniciales: string,
  colorFondo1: string,
  colorFondo2: string,
  colorCamisa: string,
  colorPiel: string,
  colorCabello: string
): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 320" fill="none">
    <defs>
      <linearGradient id="bg" x1="0" y1="0" x2="320" y2="320" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="${colorFondo1}"/>
        <stop offset="100%" stop-color="${colorFondo2}"/>
      </linearGradient>
    </defs>
    <rect width="320" height="320" rx="48" fill="url(#bg)"/>
    <circle cx="160" cy="160" r="136" stroke="rgba(255,255,255,0.35)" stroke-width="2"/>
    <!-- Hombros y uniforme de operador -->
    <path d="M56 320C56 254 100 222 160 222C220 222 264 254 264 320H56Z" fill="${colorCamisa}"/>
    <path d="M132 224L160 272L188 224" fill="#ffffff"/>
    <path d="M153 245H167L164 310H156L153 245Z" fill="#4cc253"/>
    <!-- Cuello y rostro -->
    <rect x="140" y="182" width="40" height="48" rx="16" fill="${colorPiel}"/>
    <ellipse cx="160" cy="136" rx="54" ry="62" fill="${colorPiel}"/>
    <!-- Cabello -->
    <path d="M106 128C106 86 128 66 160 66C192 66 214 86 214 128C206 104 188 94 160 94C132 94 114 104 106 128Z" fill="${colorCabello}"/>
    <!-- Ojos y sonrisa amable -->
    <circle cx="140" cy="136" r="5" fill="#1e293b"/>
    <circle cx="180" cy="136" r="5" fill="#1e293b"/>
    <path d="M144 162C150 170 170 170 176 162" stroke="#1e293b" stroke-width="4" stroke-linecap="round"/>
    <!-- Insignia verificada SAO6 -->
    <rect x="208" y="24" width="88" height="32" rx="16" fill="rgba(255,255,255,0.92)"/>
    <text x="252" y="45" text-anchor="middle" fill="#2c3e50" font-family="Inter, sans-serif" font-weight="700" font-size="13">${iniciales} • SAO6</text>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

/**
 * 14 nodos de validación con coordenadas 2D amplias y toda la información real estructurada.
 */
export const NODOS_BASE_FLUJO_VALIDACION: Omit<
  DefinicionNodoValidacion,
  'estado' | 'intentoActual' | 'duracionRealMs' | 'resumenSalida'
>[] = [
  // Cuadrícula de 5 columnas: todas las filas fluyen de izquierda a derecha
  {
    id: 'obtencion-cv',
    orden: 1,
    codigo: 'NODO_01',
    titulo: 'Obtención de Hojas de Vida',
    subtitulo: 'Extracción CV PDF y datos de la vacante',
    categoria: 'Ingesta Documental',
    claveIcono: 'archivo',
    colorAcento: '#4cc253',
    posicionX: 80,
    posicionY: 150,
    maxIntentos: 3,
    duracionBaseMs: 620,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Timeout de lectura en sesión Computrabajo (HTTP 504)',
    mensajeFalloDefinitivo: 'No fue posible descargar el expediente PDF de la vacante tras 3 intentos.',
    camposDestacados: [
      { etiqueta: 'Fuente', valorExito: 'Computrabajo PDF', valorError: 'Error HTTP 504' },
      { etiqueta: 'Documentos', valorExito: 'CV + Ficha (3 págs)', valorError: '0 extraídos' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Portal Empresarial Computrabajo Colombia & Extractor OCR PDF',
      portalUrl: 'https://empresa.computrabajo.com.co/candidatos/expediente-pdf',
      codigoRadicado: 'EXP-CT-2026-098412',
      tituloCertificado: 'ACTA DE EXTRACCIÓN DOCUMENTAL Y FICHA DE POSTULACIÓN',
      selloExito: 'EXPEDIENTE PDF ÍNTEGRO Y LEGIBLE',
      selloError: 'ARCHIVO CORRUPTO O SESIÓN EXPIRADA',
      observacionLegal: 'Extracción automatizada de Hoja de Vida original y resumen de perfil del candidato para el cargo de Operador de Buses Alimentadores.'
    },
    payloadSalidaExito: {
      vacante: 'Operador de Buses Alimentadores - Metro de Medellín',
      fuente: 'Computrabajo Empresa (Sesión Autenticada)',
      archivoHojaVida: 'hoja_de_vida_original_candidato.pdf',
      archivoPerfilComputrabajo: 'perfil_computrabajo_candidato.pdf',
      paginasProcesadas: 3,
      extraccionOcrCompleta: true,
      camposIdentificados: [
        'Nombre Completo',
        'Cédula de Ciudadanía',
        'Fecha de Nacimiento',
        'Categoría de Licencia',
        'Historial Laboral en Transporte Público',
        'Datos de Contacto y Municipio'
      ]
    },
    payloadSalidaError: {
      codigoHTTP: 504,
      motivo: 'Sesión bloqueada o expediente PDF corrupto en origen'
    }
  },
  {
    id: 'sarlaft-onu',
    orden: 2,
    codigo: 'NODO_02',
    titulo: 'Validación SARLAFT ONU',
    subtitulo: 'Lista Consolidada Consejo de Seguridad',
    categoria: 'Listas Restrictivas',
    claveIcono: 'global',
    colorAcento: '#4cc253',
    posicionX: 472,
    posicionY: 150,
    maxIntentos: 3,
    duracionBaseMs: 480,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Sincronizando caché XML de sancionados ONU...',
    mensajeFalloDefinitivo: 'ALERTA CRÍTICA: Coincidencia positiva en lista de sanciones ONU.',
    camposDestacados: [
      { etiqueta: 'Registros ONU', valorExito: '1.010 cruzados', valorError: 'Coincidencia' },
      { etiqueta: 'Resultado', valorExito: '0 Coincidencias', valorError: 'Alerta SARLAFT' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Organización de las Naciones Unidas — Consejo de Seguridad (UNSC)',
      portalUrl: 'https://scsanctions.un.org/consolidated/',
      codigoRadicado: 'ONU-SARLAFT-2026-88210',
      tituloCertificado: 'CERTIFICADO DE CRUCE EN LISTA CONSOLIDADA DE SANCIONES ONU',
      selloExito: 'SIN COINCIDENCIAS EN LISTA ONU',
      selloError: 'ALERTA RESTRICTIVA ONU DETECTADA',
      observacionLegal: 'Verificación ejecutada conforme al manual SARLAFT/SAGRILAFT contra individuos y entidades sancionadas por el Consejo de Seguridad de las Naciones Unidas.'
    },
    payloadSalidaExito: {
      listaConsultada: 'UN Security Council Consolidated List (XML Oficial)',
      individuosVerificados: 742,
      entidadesVerificadas: 268,
      totalRegistrosAnalizados: 1010,
      coincidenciaExacta: false,
      coincidenciaFonetica: false,
      alertaSarlaft: false,
      dictamen: 'Sin coincidencias en la Lista Consolidada de Sanciones ONU (SARLAFT)',
      fechaVerificacion: '2026-10-16 10:25:14'
    },
    payloadSalidaError: {
      coincidenciaExacta: true,
      alertaSarlaft: true,
      accionInmediata: 'BLOQUEO_INMEDIATO_POR_CUMPLIMIENTO_SARLAFT'
    }
  },
  {
    id: 'sarlaft-ofac',
    orden: 3,
    codigo: 'NODO_03',
    titulo: 'Validación SARLAFT OFAC',
    subtitulo: 'Lista SDN / Clinton Tesoro EE. UU.',
    categoria: 'Listas Restrictivas',
    claveIcono: 'escudo',
    colorAcento: '#4cc253',
    posicionX: 864,
    posicionY: 150,
    maxIntentos: 3,
    duracionBaseMs: 650,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Latencia alta en portal sanctionssearch.ofac.treas.gov',
    mensajeFalloDefinitivo: 'Fallo definitivo en validación OFAC / Coincidencia en lista SDN.',
    camposDestacados: [
      { etiqueta: 'Lista SDN', valorExito: 'Score 0% (Limpio)', valorError: 'Score 100% Alerta' },
      { etiqueta: 'Estado OFAC', valorExito: 'Sin Coincidencias', valorError: 'Bloqueado SDN' }
    ],
    evidenciaVisual: {
      entidadOficial: 'U.S. Department of the Treasury — Office of Foreign Assets Control (OFAC)',
      portalUrl: 'https://sanctionssearch.ofac.treas.gov/',
      codigoRadicado: 'OFAC-SDN-2026-441902',
      tituloCertificado: 'REPORTE DE VERIFICACIÓN EN LISTA OFAC / SDN (LISTA CLINTON)',
      selloExito: 'SIN REGISTROS EN LISTA OFAC SDN',
      selloError: 'COINCIDENCIA POSITIVA EN LISTA SDN',
      observacionLegal: 'Consulta realizada por número de documento de identidad y nombres completos sobre las listas Specially Designated Nationals (SDN) y Non-SDN.'
    },
    payloadSalidaExito: {
      portal: 'OFAC Treasury Sanctions List Search',
      estado: 'SIN COINCIDENCIAS EN LISTA OFAC',
      coincidencia: false,
      alerta: false,
      totalCoincidencias: 0,
      umbralSimilitudAplicado: '85%',
      registrosEncontrados: [],
      detalles: 'Sin coincidencias en la lista de sanciones de la OFAC (EE. UU.)'
    },
    payloadSalidaError: {
      portal: 'OFAC Treasury Sanctions List Search',
      estado: 'ALERTA_OFAC_O_TIMEOUT_PERSISTENTE',
      alerta: true
    }
  },
  {
    id: 'policia-nacional',
    orden: 4,
    codigo: 'NODO_04',
    titulo: 'Antecedentes Policía',
    subtitulo: 'Consulta Judicial PONAL Colombia',
    categoria: 'Judicial',
    claveIcono: 'policia',
    colorAcento: '#4cc253',
    posicionX: 1256,
    posicionY: 150,
    maxIntentos: 3,
    duracionBaseMs: 700,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Token reCAPTCHA de antecedentes.policia.gov.co expirado',
    mensajeFalloDefinitivo: 'Candidato con requerimientos judiciales pendientes o portal PONAL caído.',
    camposDestacados: [
      { etiqueta: 'Art. 248 CN', valorExito: 'Sin Asuntos Pendientes', valorError: 'Requerido / Error' },
      { etiqueta: 'DIJIN / PONAL', valorExito: 'Verificado OK', valorError: 'Rechazado' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Policía Nacional de Colombia — Dirección de Investigación Criminal e INTERPOL',
      portalUrl: 'https://antecedentes.policia.gov.co:7005/WebJudicial/',
      codigoRadicado: 'PONAL-JUD-2026-773190',
      tituloCertificado: 'CONSULTA EN LÍNEA DE ANTECEDENTES JUDICIALES (ART. 248 C.P.)',
      selloExito: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
      selloError: 'REGISTRA REQUERIMIENTO JUDICIAL O FALLO DE CONEXIÓN',
      observacionLegal: 'En cumplimiento del Artículo 248 de la Constitución Política de Colombia, la Policía Nacional certifica el estado judicial actual del ciudadano consultado.'
    },
    payloadSalidaExito: {
      entidad: 'Policía Nacional de Colombia (DIJIN)',
      estadoJudicial: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
      fundamentoConstitucional: 'Artículo 248 de la Constitución Política de Colombia',
      alertaJudicial: false,
      validacionCaptcha: 'Superada automáticamente',
      fechaHoraExpedicion: '2026-10-16 10:25:19'
    },
    payloadSalidaError: {
      entidad: 'Policía Nacional de Colombia',
      estadoJudicial: 'REQUERIMIENTO_JUDICIAL_PENDIENTE_O_PORTAL_CAIDO',
      alertaJudicial: true
    }
  },
  {
    id: 'procuraduria-pgn',
    orden: 5,
    codigo: 'NODO_05',
    titulo: 'Procuraduría General',
    subtitulo: 'Antecedentes Disciplinarios SIRI',
    categoria: 'Disciplinario',
    claveIcono: 'balanza',
    colorAcento: '#4cc253',
    posicionX: 1648,
    posicionY: 150,
    maxIntentos: 3,
    duracionBaseMs: 760,
    modoSimulacion: 'recuperar_en_reintento',
    mensajeErrorTransitorio: 'Pregunta PGN no catalogada -> Recargando reto (#ImageButton1)',
    mensajeFalloDefinitivo: 'Inhabilidad disciplinaria vigente en SIRI o bloqueo del portal PGN.',
    camposDestacados: [
      { etiqueta: 'Certificado SIRI', valorExito: 'Sin Antecedentes', valorError: 'Inhabilidad Activa' },
      { etiqueta: 'Reto Seguridad', valorExito: 'Resuelto (Intento 2)', valorError: 'Fallo Reto PGN' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Procuraduría General de la Nación — División de Registro y Control (SIRI)',
      portalUrl: 'https://apps.procuraduria.gov.co/webcert/Certificado.aspx',
      codigoRadicado: 'SIRI-PGN-2026-5591024',
      tituloCertificado: 'CERTIFICADO ORDINARIO DE ANTECEDENTES DISCIPLINARIOS (SIRI)',
      selloExito: 'EL CIUDADANO NO PRESENTA ANTECEDENTES DISCIPLINARIOS',
      selloError: 'REGISTRA SANCIÓN O INHABILIDAD ESPECIAL VIGENTE',
      observacionLegal: 'Sistema de Información de Registro de Sanciones y Causas de Inhabilidad (SIRI). Incluye sanciones disciplinarias, penales, fiscales y pérdida de investidura.'
    },
    payloadSalidaExito: {
      entidad: 'Procuraduría General de la Nación',
      sistema: 'SIRI — Certificado de Antecedentes Ordinario',
      estado: 'SIN_ANTECEDENTES',
      alerta: false,
      coincidencia: false,
      retoSeguridadResuelto: '¿Cuál es la capital de Antioquia? -> medellin',
      detalles: 'El ciudadano NO registra antecedentes ni inhabilidades vigentes en Procuraduría.',
      sancionesDisciplinarias: 'Ninguna',
      inhabilidadesEspeciales: 'Ninguna'
    },
    payloadSalidaError: {
      entidad: 'Procuraduría General de la Nación',
      estado: 'INHABILIDAD_DISCIPLINARIA_VIGENTE',
      alerta: true
    }
  },

  {
    id: 'contraloria-cgr',
    orden: 6,
    codigo: 'NODO_06',
    titulo: 'Contraloría General',
    subtitulo: 'Boletín de Responsables Fiscales SIBOR',
    categoria: 'Control Fiscal',
    claveIcono: 'documentoCheck',
    colorAcento: '#4cc253',
    posicionX: 80,
    posicionY: 560,
    maxIntentos: 3,
    duracionBaseMs: 640,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Error 503 en microservicio de certificado fiscal CGR',
    mensajeFalloDefinitivo: 'Reportado en Boletín de Responsables Fiscales o fallo definitivo CGR.',
    camposDestacados: [
      { etiqueta: 'Sistema SIBOR', valorExito: 'No Reportado', valorError: 'Reportado Fiscal' },
      { etiqueta: 'Certificado CGR', valorExito: 'Vigente y Limpio', valorError: 'Bloqueo Fiscal' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Contraloría General de la República — Contraloría Delegada para Investigaciones',
      portalUrl: 'https://www.contraloria.gov.co/control-fiscal/responsabilidad-fiscal/certificado-de-antecedentes-fiscales',
      codigoRadicado: 'SIBOR-CGR-2026-309184',
      tituloCertificado: 'CERTIFICADO DE ANTECEDENTES FISCALES — SISTEMA SIBOR',
      selloExito: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
      selloError: 'REPORTADO EN EL BOLETÍN DE RESPONSABLES FISCALES',
      observacionLegal: 'Certificación expedida de conformidad con el Artículo 60 de la Ley 610 de 2000 sobre el Boletín de Responsables Fiscales.'
    },
    payloadSalidaExito: {
      entidad: 'Contraloría General de la República',
      sistema: 'SIBOR (Boletín de Responsables Fiscales)',
      estadoFiscal: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
      procesosFiscalesActivos: 0,
      alertaFiscal: false,
      codigoVerificacion: 'CGR-88412095-2026'
    },
    payloadSalidaError: {
      entidad: 'Contraloría General de la República',
      estadoFiscal: 'REPORTADO_EN_BOLETIN_DE_RESPONSABLES_FISCALES',
      alertaFiscal: true
    }
  },
  {
    id: 'validacion-edad',
    orden: 7,
    codigo: 'NODO_07',
    titulo: 'Validación de Edad',
    subtitulo: 'Verificación de rango etario operativo',
    categoria: 'Regla de Negocio',
    claveIcono: 'calendario',
    colorAcento: '#4cc253',
    posicionX: 472,
    posicionY: 560,
    maxIntentos: 2,
    duracionBaseMs: 420,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Fecha ilegible en página 1 -> Aplicando OCR secundario',
    mensajeFalloDefinitivo: 'Edad fuera del rango operativo permitido para la vacante.',
    camposDestacados: [
      { etiqueta: 'Edad Candidato', valorExito: '36 años (Cumple)', valorError: 'Fuera de rango' },
      { etiqueta: 'Rango Vacante', valorExito: '23 a 55 años', valorError: 'Exige 23-55 años' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Motor de Reglas Operativas SAO6 — Validación Demográfica y Cédula',
      portalUrl: 'internal://motor-reglas-rrhh/validacion-edad',
      codigoRadicado: 'RN-EDAD-2026-00412',
      tituloCertificado: 'DICTAMEN DE CUMPLIMIENTO DE RANGO ETARIO OPERATIVO',
      selloExito: 'CUMPLE RANGO DE EDAD PARA OPERACIÓN DE BUSES',
      selloError: 'FUERA DEL RANGO ETARIO DEFINIDO EN LA REQUISICIÓN',
      observacionLegal: 'Cálculo exacto a partir de la fecha de nacimiento extraída de la Cédula de Ciudadanía y la ficha de postulación.'
    },
    payloadSalidaExito: {
      fechaNacimientoDetectada: '1990-04-18',
      edadCalculadaAnios: 36,
      rangoMinimoVacante: 23,
      rangoMaximoVacante: 55,
      cumpleReglaEdad: true,
      fuenteVerificacion: 'Cédula de Ciudadanía + Ficha Computrabajo'
    },
    payloadSalidaError: {
      rangoPermitido: '23 a 55 años',
      cumpleReglaEdad: false,
      motivo: 'Edad fuera del perfil operativo exigido por la vacante'
    }
  },
  {
    id: 'validacion-reintegro',
    orden: 8,
    codigo: 'NODO_08',
    titulo: 'Validación de Reintegro',
    subtitulo: 'Cruce histórico con base interna SAO6',
    categoria: 'Historial Interno',
    claveIcono: 'reintegro',
    colorAcento: '#4cc253',
    posicionX: 864,
    posicionY: 560,
    maxIntentos: 3,
    duracionBaseMs: 520,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Reconectando al pool de base histórica Kactus RRHH...',
    mensajeFalloDefinitivo: 'Excolaborador con causal de retiro no elegible para reintegro.',
    camposDestacados: [
      { etiqueta: 'Base Kactus', valorExito: 'Sin Bloqueos', valorError: 'No Reintegrable' },
      { etiqueta: 'Estado RRHH', valorExito: 'Elegible Ingreso', valorError: 'Retiro Disciplinario' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Sistema Interno de Gestión Humana Kactus & Bitácora Operativa SAO6',
      portalUrl: 'internal://kactus-rrhh.sao6.local/consulta-reintegros',
      codigoRadicado: 'KACTUS-RH-2026-19044',
      tituloCertificado: 'CERTIFICADO DE CRUCE HISTÓRICO DE NÓMINA Y ELEGIBILIDAD DE REINTEGRO',
      selloExito: 'CANDIDATO ELEGIBLE — SIN NOVEDADES DISCIPLINARIAS INTERNAS',
      selloError: 'BLOQUEO DE REINGRESO POR CAUSAL DE RETIRO PREVIO',
      observacionLegal: 'Verificación sobre el maestro histórico de colaboradores, contratos previos, descargos disciplinarios y paz y salvo de dotación.'
    },
    payloadSalidaExito: {
      sistemaConsultado: 'Kactus HRM + Bitácora Disciplinaria SAO6',
      laboroAnteriormenteEnEmpresa: false,
      contratosPrevios: 0,
      bloqueoDisciplinarioInterno: false,
      elegibleParaContratacion: true,
      observacionRRHH: 'Nuevo ingreso habilitado sin restricciones históricas en el grupo empresarial.'
    },
    payloadSalidaError: {
      laboroAnteriormenteEnEmpresa: true,
      elegibleParaContratacion: false,
      motivo: 'Registra retiro previo con restricción definitiva de reingreso'
    }
  },
  {
    id: 'validacion-genero',
    orden: 9,
    codigo: 'NODO_09',
    titulo: 'Validación de Género',
    subtitulo: 'Criterio de perfil y análisis onomástico',
    categoria: 'Regla de Negocio',
    claveIcono: 'usuario',
    colorAcento: '#4cc253',
    posicionX: 1256,
    posicionY: 560,
    maxIntentos: 2,
    duracionBaseMs: 380,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Campo género no explícito -> Ejecutando inferencia onomástica',
    mensajeFalloDefinitivo: 'No cumple criterio de perfil configurado en la requisición.',
    camposDestacados: [
      { etiqueta: 'Género Perfil', valorExito: 'Hombre (Validado)', valorError: 'No coincide' },
      { etiqueta: 'Método', valorExito: 'Ficha + Onomástico', valorError: 'Inconsistente' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Clasificador Demográfico y Onomástico RRHH + ADRES BDUA',
      portalUrl: 'internal://clasificador-rrhh/onomastico-adres',
      codigoRadicado: 'ONOM-2026-66201',
      tituloCertificado: 'VALIDACIÓN DE PERFIL DEMOGRÁFICO Y AFILIACIÓN ADRES',
      selloExito: 'PERFIL DEMOGRÁFICO Y EPS VERIFICADOS CORRECTAMENTE',
      selloError: 'DISCREPANCIA EN PARÁMETROS DE LA REQUISICIÓN',
      observacionLegal: 'Validación cruzada entre la ficha del candidato, diccionario onomástico y registro activo en BDUA ADRES.'
    },
    payloadSalidaExito: {
      generoDetectado: 'Hombre',
      metodoDeterminacion: 'Ficha de postulación + Diccionario Onomástico + BDUA ADRES',
      afiliacionAdres: {
        estado: 'AFILIADO_VALIDO',
        eps: 'EPS SURAMERICANA S.A.',
        regimen: 'CONTRIBUTIVO',
        estadoAfiliacion: 'ACTIVO',
        tipoAfiliado: 'COTIZANTE',
        cumpleEpsAutorizada: true
      },
      cumpleRequisitoVacante: true
    },
    payloadSalidaError: {
      cumpleRequisitoVacante: false,
      motivo: 'No coincide con los parámetros configurados en la requisición activa'
    }
  },
  {
    id: 'tipo-licencia',
    orden: 10,
    codigo: 'NODO_10',
    titulo: 'Tipo de Licencia',
    subtitulo: 'Verificación de categoría C2 o C3 activa',
    categoria: 'Tránsito y Movilidad',
    claveIcono: 'licencia',
    colorAcento: '#4cc253',
    posicionX: 1648,
    posicionY: 560,
    maxIntentos: 3,
    duracionBaseMs: 520,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Validando homologación de categoría de servicio público...',
    mensajeFalloDefinitivo: 'El candidato solo posee categoría B1/C1 (Se exige C2 o C3 activa).',
    camposDestacados: [
      { etiqueta: 'Cat. Activa', valorExito: 'C2 Vigente (2028)', valorError: 'Solo C1 (Rechazado)' },
      { etiqueta: 'Exigencia', valorExito: 'C2 o C3 Público', valorError: 'No apto bus pesado' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Ministerio de Transporte — Validación de Categoría de Servicio Público',
      portalUrl: 'https://portalpublico.runt.gov.co/#/consulta-ciudadano-documento',
      codigoRadicado: 'LIC-C2C3-2026-99104',
      tituloCertificado: 'VERIFICACIÓN DE CATEGORÍA DE LICENCIA DE CONDUCCIÓN (C2 / C3)',
      selloExito: 'CATEGORÍA C2 ACTIVA — HABILITADO PARA BUSES ALIMENTADORES',
      selloError: 'CATEGORÍA INSUFICIENTE (POSEE C1/B1 — SE EXIGE C2 O C3)',
      observacionLegal: 'Para la conducción de buses alimentadores del sistema masivo se requiere licencia de conducción de servicio público categoría C2 o C3 con estado ACTIVA.'
    },
    payloadSalidaExito: {
      categoriasExigidasVacante: ['C2', 'C3'],
      categoriaPrincipalDetectada: 'C2',
      categoriasSecundariasActivas: ['A2', 'B2'],
      estadoLicencia: 'ACTIVA',
      fechaExpedicion: '2023-12-13',
      fechaVencimiento: '2026-12-13',
      organismoTransitoExpide: 'STRIA TTEyTTO ITAGUI',
      restriccionesConductor: 'CONDUCIR CON LENTES',
      licenciaRetenida: false,
      cumpleCategoriaBusesPesados: true
    },
    payloadSalidaError: {
      categoriasExigidasVacante: ['C2', 'C3'],
      categoriaPrincipalDetectada: 'C1',
      estadoLicencia: 'ACTIVA (Solo automóviles/camionetas)',
      cumpleCategoriaBusesPesados: false,
      motivoRechazo: 'La categoría C1 no autoriza la conducción de buses alimentadores de servicio público.'
    }
  },

  {
    id: 'validacion-runt',
    orden: 11,
    codigo: 'NODO_11',
    titulo: 'Validación con el RUNT',
    subtitulo: 'Registro activo y antigüedad >= 2 años',
    categoria: 'Tránsito y Movilidad',
    claveIcono: 'runt',
    colorAcento: '#4cc253',
    posicionX: 80,
    posicionY: 970,
    maxIntentos: 3,
    duracionBaseMs: 820,
    modoSimulacion: 'recuperar_en_reintento',
    mensajeErrorTransitorio: 'LibreCaptcha RUNT rechazado -> Re-ejecutando OCR solve.exe',
    mensajeFalloDefinitivo: 'No inscrito en RUNT o antigüedad menor a los 2 años exigidos.',
    camposDestacados: [
      { etiqueta: 'Estado RUNT', valorExito: 'Inscrito Activo', valorError: 'Sin antigüedad' },
      { etiqueta: 'Antigüedad', valorExito: '15.1 años (>= 2)', valorError: '1.1 años (< 2)' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Concesión RUNT 2.0 S.A.S. — Registro Único Nacional de Tránsito',
      portalUrl: 'https://portalpublico.runt.gov.co/#/consulta-ciudadano-documento',
      codigoRadicado: 'RUNT-MS-2450848-2026',
      tituloCertificado: 'CONSULTA DE CIUDADANO E HISTORIAL DE LICENCIAS EN RUNT PRO',
      selloExito: 'CONDUCTOR INSCRITO ACTIVO — ANTIGÜEDAD 15.1 AÑOS',
      selloError: 'NO CUMPLE ANTIGÜEDAD MÍNIMA DE 2 AÑOS EN C2/C3',
      observacionLegal: 'Extracción directa del microservicio CYRConsultaCiudadanoMS resolviendo LibreCaptcha mediante motor OCR local.'
    },
    payloadSalidaExito: {
      estado: 'INSCRITO_ACTIVO',
      inscrito: true,
      idPersonaRunt: '2450848',
      fechaInscripcionRunt: '2011-09-06T14:07:28.000-05:00',
      antiguedadVerificadaAnios: 15.1,
      minimoExigidoAnios: 2.0,
      estadoPersona: 'ACTIVA',
      estadoConductor: 'ACTIVO',
      categoriasActivas: ['C2 (Vence 2026-12-13)', 'A2 (Vence 2033-12-13)', 'B2 (Vence 2033-12-13)'],
      numeroPazYSalvoRunt: '901769261067',
      historialLicenciasRegistradas: [
        {
          numeroLicencia: '71799510',
          categorias: 'C2, A2, B2',
          fechaExpedicion: '2023-12-13',
          fechaVencimientoC2: '2026-12-13',
          estado: 'ACTIVA',
          organismoTransito: 'STRIA TTEyTTO ITAGUI'
        },
        {
          numeroLicencia: '71799510',
          categorias: 'C2, A2, B2',
          fechaExpedicion: '2020-07-23',
          fechaVencimientoC2: '2023-07-23',
          estado: 'INACTIVA (Histórica)',
          organismoTransito: 'SECRETARIA DISTRITAL DE MOVILIDAD DE BOGOTA'
        },
        {
          numeroLicencia: '71799510',
          categorias: 'C2, A2, B2',
          fechaExpedicion: '2017-07-06',
          fechaVencimientoC2: '2020-07-06',
          estado: 'INACTIVA (Histórica)',
          organismoTransito: 'SECRETARIA DISTRITAL DE MOVILIDAD DE BOGOTA'
        }
      ]
    },
    payloadSalidaError: {
      estado: 'ANTIGUEDAD_INSUFICIENTE',
      antiguedadVerificadaAnios: 1.1,
      minimoExigidoAnios: 2.0,
      motivo: 'El candidato tiene menos de 2 años de antigüedad con categoría C2/C3.'
    }
  },
  {
    id: 'validacion-simit',
    orden: 12,
    codigo: 'NODO_12',
    titulo: 'Validación SIMIT',
    subtitulo: 'Consulta estado de cuenta nacional FCM',
    categoria: 'Tránsito y Movilidad',
    claveIcono: 'simit',
    colorAcento: '#4cc253',
    posicionX: 472,
    posicionY: 970,
    maxIntentos: 3,
    duracionBaseMs: 780,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'SPA Angular de fcm.org.co/simit tardó en hidratar #txtBusqueda',
    mensajeFalloDefinitivo: 'Portal SIMIT inaccesible tras 3 intentos o bloqueo de consulta.',
    camposDestacados: [
      { etiqueta: 'Comparendos', valorExito: '0 registrados', valorError: '2 pendientes' },
      { etiqueta: 'Estado SIMIT', valorExito: 'Consulta Exitosa', valorError: 'Error / Deuda' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Federación Colombiana de Municipios — Sistema Integrado SIMIT',
      portalUrl: 'https://fcm.org.co/simit/#/estado-cuenta',
      codigoRadicado: 'SIMIT-FCM-2026-610298',
      tituloCertificado: 'ESTADO DE CUENTA NACIONAL DE INFRACCIONES DE TRÁNSITO (SIMIT)',
      selloExito: 'PAZ Y SALVO NACIONAL EN SIMIT — 0 COMPARENDOS',
      selloError: 'REGISTRA INFRACCIONES PENDIENTES DE PAGO EN SIMIT',
      observacionLegal: 'Consulta nacional unificada de comparendos, resoluciones sancionatorias y acuerdos de pago reportados por los organismos de tránsito del país.'
    },
    payloadSalidaExito: {
      entidad: 'Federación Colombiana de Municipios (SIMIT)',
      estado: 'PAZ_Y_SALVO',
      cantidadComparendos: 0,
      cantidadMultas: 0,
      cantidadAcuerdosPago: 0,
      totalDeudaActual: '$ 0',
      detalles: 'SIMIT: PAZ_Y_SALVO | Comparendos: 0 | Multas: 0 | Acuerdos: 0 | Deuda: $ 0'
    },
    payloadSalidaError: {
      entidad: 'Federación Colombiana de Municipios (SIMIT)',
      estado: 'CON_PENDIENTES',
      cantidadMultas: 2,
      totalDeudaActual: '$ 2.480.000'
    }
  },
  {
    id: 'validacion-multas',
    orden: 13,
    codigo: 'NODO_13',
    titulo: 'Validación de Multas',
    subtitulo: 'Tope $0 o acuerdo vigente <= $1.000.000',
    categoria: 'Regla Financiera',
    claveIcono: 'multas',
    colorAcento: '#4cc253',
    posicionX: 864,
    posicionY: 970,
    maxIntentos: 2,
    duracionBaseMs: 480,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'Verificando estado de resolución de acuerdo de pago en SIMIT...',
    mensajeFalloDefinitivo: 'RECHAZADO: Posee multas sin acuerdo de pago o deuda > $1.000.000.',
    camposDestacados: [
      { etiqueta: 'Deuda Total', valorExito: '$ 0 COP (Paz y Salvo)', valorError: '$ 2.480.000 COP' },
      { etiqueta: 'Política SAO6', valorExito: 'Cumple Tope <= $1M', valorError: 'Sin Acuerdo Pago' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Comité de Selección y Seguridad Vial SAO6 — Regla Financiera de Multas',
      portalUrl: 'internal://motor-reglas-rrhh/politica-multas-simit',
      codigoRadicado: 'POL-MUL-2026-77120',
      tituloCertificado: 'DICTAMEN DE CUMPLIMIENTO DE POLÍTICA DE MULTAS Y ACUERDOS DE PAGO',
      selloExito: 'APROBADO: $0 EN MULTAS Y PAZ Y SALVO VERIFICADO',
      selloError: 'RECHAZADO: DEUDA SUPERA $1.000.000 SIN ACUERDO DE PAGO VIGENTE',
      observacionLegal: 'Política corporativa: el aspirante no debe registrar multas pendientes ($0), o en su defecto debe contar con acuerdo de pago activo cuya deuda no supere $1.000.000 COP.'
    },
    payloadSalidaExito: {
      reglaCorporativa: 'Deuda $0 o Acuerdo de Pago vigente menor o igual a $1.000.000 COP',
      multasActivas: 0,
      acuerdosDePagoVigentes: 0,
      valorTotalDeuda: '$ 0 COP',
      topeMaximoPermitidoConAcuerdo: '$ 1.000.000 COP',
      cumplePoliticaMultas: true,
      dictamenFinanciero: 'APROBADO SIN RESTRICCIONES'
    },
    payloadSalidaError: {
      reglaCorporativa: 'Deuda $0 o Acuerdo de Pago vigente menor o igual a $1.000.000 COP',
      multasActivas: 2,
      acuerdosDePagoVigentes: 0,
      valorTotalDeuda: '$ 2.480.000 COP',
      infraccionesDetectadas: [
        'C02 — Estacionar en sitios prohibidos ($ 650.000)',
        'C29 — Conducir a velocidad superior a la máxima permitida ($ 1.830.000)'
      ],
      cumplePoliticaMultas: false,
      dictamenFinanciero: 'RECHAZADO POR DEUDA EN SIMIT SIN ACUERDO DE PAGO'
    }
  },
  {
    id: 'validacion-experiencia',
    orden: 14,
    codigo: 'NODO_14',
    titulo: 'Validación de Experiencia',
    subtitulo: 'Trayectoria en buses pesados y Scoring IA',
    categoria: 'Scoring IA Final',
    claveIcono: 'experiencia',
    colorAcento: '#4cc253',
    posicionX: 1256,
    posicionY: 970,
    maxIntentos: 3,
    duracionBaseMs: 880,
    modoSimulacion: 'exito_directo',
    mensajeErrorTransitorio: 'JSON truncado desde Ollama local -> Reparando estructura y reintentando',
    mensajeFalloDefinitivo: 'Sin experiencia demostrable en conducción de vehículos pesados / servicio público.',
    camposDestacados: [
      { etiqueta: 'Experiencia', valorExito: '6 años en Buses', valorError: '< 1 año pesado' },
      { etiqueta: 'Scoring IA', valorExito: '94 / 100 (Apto)', valorError: '38 / 100 (No Apto)' }
    ],
    evidenciaVisual: {
      entidadOficial: 'Agente Evaluador IA Local (Ollama qwen2.5) — Análisis de Trayectoria Laboral',
      portalUrl: 'internal://ollama-local:11434/api/generate',
      codigoRadicado: 'IA-SCORE-2026-99401',
      tituloCertificado: 'INFORME DE EVALUACIÓN DE EXPERIENCIA Y SCORING INTEGRAL DE CONTRATACIÓN',
      selloExito: 'CANDIDATO APTO PRIORITARIO — SCORE IA 94/100',
      selloError: 'DESCARTADO POR EXPERIENCIA INSUFICIENTE EN VEHÍCULOS PESADOS',
      observacionLegal: 'Análisis semántico de certificaciones laborales en transporte masivo, buses padrón/alimentadores y servicio público intermunicipal.'
    },
    payloadSalidaExito: {
      modeloIaUtilizado: 'qwen2.5:1.5b (Inferencia Local Ollama)',
      experienciaVerificadaBusesAnios: 6,
      empresasTransporteDetectadas: [
        'Sistema Alimentador Oriental SAO6 (Postulación)',
        'Coonatra — Cooperativa Nacional de Transportadores (3.5 años)',
        'Rápido Ochoa S.A. (2.5 años)'
      ],
      tiposVehiculoOperados: ['Bus Padrón', 'Bus Alimentador 50 pasajeros', 'Microbús Servicio Público'],
      puntajeExperiencia: '94 / 100',
      cumplimientoGlobal14Pasos: '14 / 14 Validaciones Aprobadas',
      recomendacionContratacion: 'APTO PRIORITARIO PARA CITACIÓN A PRUEBA TÉCNICA Y MÉDICA'
    },
    payloadSalidaError: {
      modeloIaUtilizado: 'qwen2.5:1.5b (Inferencia Local Ollama)',
      experienciaVerificadaBusesAnios: 0.5,
      puntajeExperiencia: '38 / 100',
      recomendacionContratacion: 'NO CUMPLE EXPERIENCIA MÍNIMA EN TRANSPORTE PÚBLICO DE PASAJEROS'
    }
  }
];

export const ESCENARIOS_SIMULACION: EscenarioValidacion[] = [
  {
    id: 'escenario-apto-con-reintentos',
    nombre: '1. Julián Andrés Castañeda — Flujo Completo Exitoso (Con Auto-Recuperación PGN y RUNT)',
    descripcion:
      'Recorre los 14 nodos con datos reales. Simula fallos transitorios en Procuraduría y RUNT que reintentan automáticamente y aprueban.',
    vacante: 'Operador de Buses Alimentadores - Metro de Medellín',
    candidato: {
      nombre: 'Julián Andrés Castañeda Álzate',
      cedula: '71.799.510',
      edad: 36,
      genero: 'Hombre',
      licencia: 'C2 Activa (Vence 2026-12-13)',
      experienciaAnios: 6,
      fotoUrl: crearRetratoSvgDataUri(
        'JC',
        '#e8f8ea',
        '#d0f0d4',
        '#1e293b',
        '#f1c27d',
        '#2c3e50'
      ),
      municipio: 'Medellín, Antioquia',
      telefono: '314 782 9104',
      correo: 'julian.castaneda89@correo.com',
      eps: 'EPS SURAMERICANA S.A.',
      regimenEps: 'CONTRIBUTIVO (COTIZANTE ACTIVO)',
      archivoHojaVida: 'hoja_de_vida_original_Julian_andres_Castañeda_alzate.pdf',
      archivoPerfil: 'perfil_computrabajo_Julian_andres_Castañeda_alzate.pdf',
      resumenPerfil:
        'Conductor profesional de servicio público con más de 15 años de registro en RUNT y 6 años de experiencia certificada operando buses alimentadores y transporte colectivo en el Valle de Aburrá. Cero comparendos en SIMIT y paz y salvo vigente.',
      empresasPrevias: [
        'Coonatra — Conductor Bus Alimentador (2022 - 2025)',
        'Rápido Ochoa — Operador Servicio Público (2019 - 2022)'
      ]
    },
    configuracionNodos: {
      'procuraduria-pgn': 'recuperar_en_reintento',
      'validacion-runt': 'recuperar_en_reintento'
    }
  },
  {
    id: 'escenario-parada-multas',
    nombre: '2. Jorge Luis Torres Pereira — Parada Crítica en Multas SIMIT (Reintenta y Corta)',
    descripcion:
      'Avanza hasta el nodo #13 (Validación de Multas), revalida 2 veces y al confirmar deuda de $2.480.000 sin acuerdo detiene el flujo.',
    vacante: 'Operador de Buses Alimentadores - Metro de Medellín',
    candidato: {
      nombre: 'Jorge Luis Torres Pereira',
      cedula: '1.035.442.819',
      edad: 31,
      genero: 'Hombre',
      licencia: 'C2 Activa (Vence 2027-08-19)',
      experienciaAnios: 4,
      fotoUrl: crearRetratoSvgDataUri(
        'JT',
        '#eef2ff',
        '#dbeafe',
        '#334155',
        '#e0ac69',
        '#1e293b'
      ),
      municipio: 'Bello, Antioquia',
      telefono: '301 554 2098',
      correo: 'jorge.torres.cond@correo.com',
      eps: 'SALUD TOTAL ENTIDAD PROMOTORA DE SALUD',
      regimenEps: 'CONTRIBUTIVO (ACTIVO)',
      archivoHojaVida: 'hoja_de_vida_original_Jorge_Luis_Torres_Pereira.pdf',
      archivoPerfil: 'perfil_computrabajo_Jorge_Luis_Torres_Pereira.pdf',
      resumenPerfil:
        'Operador de transporte urbano con 4 años de experiencia en rutas integradas. Posee licencia C2 vigente, pero registra 2 comparendos pendientes en SIMIT por $2.480.000 sin acuerdo de pago suscrito.',
      empresasPrevias: [
        'Transportes Hato Viejo — Conductor Bus Urbano (2021 - 2025)'
      ]
    },
    configuracionNodos: {
      'policia-nacional': 'recuperar_en_reintento',
      'validacion-multas': 'fallo_definitivo'
    }
  },
  {
    id: 'escenario-parada-licencia',
    nombre: '3. Sebastián Parra Avendaño — Parada Temprana en Tipo de Licencia (Solo C1)',
    descripcion:
      'En el nodo #10 (Tipo de Licencia) detecta categoría C1 en lugar de C2/C3 tras agotar los 3 intentos y corta los nodos siguientes.',
    vacante: 'Operador de Buses Alimentadores - Metro de Medellín',
    candidato: {
      nombre: 'Sebastián Parra Avendaño',
      cedula: '1.017.229.104',
      edad: 28,
      genero: 'Hombre',
      licencia: 'C1 Activa (No cumple C2/C3)',
      experienciaAnios: 2,
      fotoUrl: crearRetratoSvgDataUri(
        'SP',
        '#fef3c7',
        '#fde68a',
        '#1e293b',
        '#f1c27d',
        '#451a03'
      ),
      municipio: 'Itagüí, Antioquia',
      telefono: '320 918 4412',
      correo: 'sebastian.parra.av@correo.com',
      eps: 'NUEVA EPS S.A.',
      regimenEps: 'CONTRIBUTIVO (ACTIVO)',
      archivoHojaVida: 'hoja_de_vida_original_Sebastián_Parra_Avendaño.pdf',
      archivoPerfil: 'perfil_computrabajo_Sebastián_Parra_Avendaño.pdf',
      resumenPerfil:
        'Conductor de vehículos livianos y camionetas de reparto con 2 años de experiencia. Cuenta con antecedentes limpios y paz y salvo en SIMIT, pero aún no ha recategorizado su licencia a C2.',
      empresasPrevias: [
        'Logística del Aburrá — Conductor Reparto C1 (2023 - 2025)'
      ]
    },
    configuracionNodos: {
      'sarlaft-ofac': 'recuperar_en_reintento',
      'tipo-licencia': 'fallo_definitivo'
    }
  }
];
