import {
  ETIQUETA_ETAPA,
  ORDEN_ETAPAS,
  type CandidatoEnVivo,
  type EstadoEtapa,
  type IdEtapa,
  type VacanteApi
} from '../modelos/analisisEnVivo';
import type {
  CandidatoEvaluadoEnVacante,
  VacanteConCandidatos
} from '../modelos/catalogoVacantesCandidatos';
import { ETIQUETA_VEREDICTO, veredictoDe } from './almacenAnalisis.svelte';
import { urlCaptura } from './clienteRrhh';
import type {
  DatosCandidatoEscenario,
  DefinicionNodoValidacion,
  EstadoNodoValidacion
} from '../modelos/flujoValidacion';

interface PlantillaNodo {
  subtitulo: string;
  categoria: string;
  claveIcono: string;
  entidad: string;
  portalUrl: string;
  sello: [exito: string, error: string];
}

const PLANTILLAS: Record<IdEtapa, PlantillaNodo> = {
  perfil: {
    subtitulo: 'Perfil de Computrabajo y hoja de vida original',
    categoria: 'Ingesta Documental',
    claveIcono: 'archivo',
    entidad: 'Portal Empresarial Computrabajo Colombia',
    portalUrl: 'https://empresa.co.computrabajo.com',
    sello: ['PERFIL Y HOJA DE VIDA DESCARGADOS', 'NO SE PUDO LEER EL PERFIL']
  },
  genero: {
    subtitulo: 'Requisito de la vacante: hombre (según el perfil)',
    categoria: 'Reglas de Perfil',
    claveIcono: 'usuario',
    entidad: 'Perfil del candidato en Computrabajo',
    portalUrl: 'https://empresa.co.computrabajo.com',
    sello: ['CUMPLE EL REQUISITO DE GÉNERO', 'NO CUMPLE EL REQUISITO DE GÉNERO']
  },
  reintegro: {
    subtitulo: 'Quien ya fue empleado no puede reingresar: el proceso termina',
    categoria: 'Reintegro',
    claveIcono: 'usuario',
    entidad: 'Administremos — SIGMA SAS',
    portalUrl: 'http://administremos.sigmansas.co',
    sello: ['SIN REINTEGRO PREVIO', 'REINTEGRO: YA FUE EMPLEADO, NO SE ADMITE']
  },
  runt: {
    subtitulo: 'Inscripción activa, antigüedad ≥ 2 años y licencia C2/C3 activa',
    categoria: 'Conducción',
    claveIcono: 'runt',
    entidad: 'Registro Único Nacional de Tránsito (RUNT)',
    portalUrl: 'https://portalpublico.runt.gov.co',
    sello: ['RUNT CONFORME', 'RUNT NO CONFORME O SIN VERIFICAR']
  },
  simit: {
    subtitulo: 'Multas y comparendos (tope $1.000.000 con acuerdo de pago)',
    categoria: 'Tránsito',
    claveIcono: 'simit',
    entidad: 'Federación Colombiana de Municipios — SIMIT',
    portalUrl: 'https://www.fcm.org.co/simit/#/estado-cuenta',
    sello: ['SIMIT CONFORME', 'SIMIT NO CONFORME O SIN VERIFICAR']
  },
  adres: {
    subtitulo: 'Afiliación activa a una EPS autorizada',
    categoria: 'Seguridad Social',
    claveIcono: 'documentoCheck',
    entidad: 'ADRES — Base de Datos Única de Afiliados (BDUA)',
    portalUrl: 'https://aplicaciones.adres.gov.co/BDUA_Internet/Pages/ConsultarAfiliadoWeb_2.aspx',
    sello: ['EPS ACTIVA Y AUTORIZADA', 'EPS NO CONFORME O SIN VERIFICAR']
  },
  procuraduria: {
    subtitulo: 'Antecedentes disciplinarios (informativo)',
    categoria: 'Antecedentes',
    claveIcono: 'balanza',
    entidad: 'Procuraduría General de la Nación',
    portalUrl: 'https://apps.procuraduria.gov.co/webcert/inicio.aspx?tpo=1',
    sello: ['SIN ANTECEDENTES', 'ALERTA DE ANTECEDENTES']
  },
  sarlaft: {
    subtitulo: 'Lista consolidada de sanciones ONU (informativo)',
    categoria: 'Listas Restrictivas',
    claveIcono: 'global',
    entidad: 'ONU — Consejo de Seguridad',
    portalUrl: 'https://scsanctions.un.org/consolidated-sp/',
    sello: ['SIN COINCIDENCIAS EN LISTA ONU', 'COINCIDENCIA EN LISTA ONU']
  },
  ia: {
    subtitulo: 'Calificación crítica de la hoja de vida contra los datos oficiales (Gemini)',
    categoria: 'Inteligencia Artificial',
    claveIcono: 'experiencia',
    entidad: 'Gemini — Google Vertex AI',
    portalUrl: 'https://cloud.google.com/vertex-ai',
    sello: ['CALIFICACIÓN DE IA GENERADA', 'LA IA NO PUDO CALIFICAR']
  },
  ofac: {
    subtitulo: 'Lista de sanciones OFAC EE. UU. (informativo)',
    categoria: 'Listas Restrictivas',
    claveIcono: 'escudo',
    entidad: 'OFAC — Departamento del Tesoro de EE. UU.',
    portalUrl: 'https://sanctionssearch.ofac.treas.gov/Default.aspx',
    sello: ['SIN COINCIDENCIAS EN OFAC', 'COINCIDENCIA EN OFAC']
  }
};

const COLUMNAS = 5;
const SEPARACION_X = 392;
const SEPARACION_Y = 380;
// La rama "sin cédula" (mockup) va en una fila superior; el flujo principal queda debajo.
const Y_RAMA = 190;
// Persona de prueba: correo y WhatsApp salen en paralelo del mismo nodo de contacto, uno debajo y otro encima.
const SEPARACION_ENVIOS = 120;
const Y_FLUJO = Y_RAMA + SEPARACION_Y;

const ETIQUETA_ESTADO: Record<EstadoEtapa, string> = {
  ejecutando: 'Consultando',
  reintentando: 'Reintentando',
  ok: 'Aprobado',
  alerta: 'Alerta',
  rechazado: 'No cumple',
  error: 'Sin verificar',
  omitida: 'No consultado'
};

const ESTADO_NODO: Record<EstadoEtapa, EstadoNodoValidacion> = {
  ejecutando: 'ejecutando',
  reintentando: 'reintentando',
  ok: 'completado',
  alerta: 'completado',
  rechazado: 'detenido_error',
  error: 'detenido_error',
  omitida: 'omitido'
};

function resultadoDeEtapa(c: CandidatoEnVivo, id: IdEtapa): Record<string, unknown> {
  const r = c.resultado;
  const etapa = c.etapas[id];
  const base: Record<string, unknown> = etapa
    ? { estado: etapa.estado, detalle: etapa.detalle ?? null, ms: etapa.ms ?? null, reintentos: etapa.reintentos }
    : { estado: 'pendiente' };
  if (!r) return base;
  switch (id) {
    case 'perfil':
      return {
        ...base,
        cedula: r.cedula,
        genero: r.genero ?? null,
        hojaDeVidaOriginal: Boolean(r.ruta_cv_original),
        urlFicha: r.url_detalle
      };
    case 'genero':
      return { ...base, genero: r.evaluacion_idoneidad?.genero ?? r.genero ?? null, requisito: 'Hombre' };
    case 'reintegro':
      return { ...base, respuesta: r.reintegro ?? null };
    case 'runt':
      return { ...base, respuesta: r.runt ?? null };
    case 'simit':
      return { ...base, respuesta: r.simit ?? null };
    case 'adres':
      return { ...base, respuesta: r.adres ?? null };
    case 'procuraduria':
      return { ...base, respuesta: r.procuraduria ?? null };
    case 'sarlaft':
      return { ...base, respuesta: r.sarlaft_onu ?? null };
    case 'ofac':
      return { ...base, respuesta: r.ofac ?? null };
    case 'ia':
      return { ...base, respuesta: r.evaluacion_ia ?? null };
  }
}

function textoCorto(texto: string | undefined, max: number): string {
  if (!texto) return '—';
  return texto.length > max ? `${texto.slice(0, max - 1)}…` : texto;
}

/** RUNT en tres cards: inscripción, años de registro y categorías activas. Null si no hay respuesta del RUNT. */
function camposRunt(c: CandidatoEnVivo, detalle: string | undefined) {
  const runt = c.resultado?.runt;
  if (!runt) return null;
  const inscrito = runt.inscrito === true;
  // El detalle ya trae "11.3 años de registro"; si no, se calcula desde la fecha de inscripción.
  let anios = /([\d.,]+)\s*años/.exec(detalle ?? '')?.[1];
  const fecha = Date.parse(String(runt.fecha_inscripcion ?? ''));
  if (!anios && !Number.isNaN(fecha)) anios = ((Date.now() - fecha) / (365.25 * 864e5)).toFixed(1);
  // Vienen como "C2 (2028-07-22)": en la card solo el código; el vencimiento queda en "Ver toda la info".
  const categorias = runt.categorias_activas?.map((k) => k.replace(/\s*\(.*\)/, '')).join(' · ') || '—';
  const campo = (etiqueta: string, valor: string) => ({ etiqueta, valorExito: valor, valorError: valor });
  return [
    campo('Inscrito en RUNT', inscrito ? 'Sí' : 'No'),
    campo('Años de registro', anios ? `${anios} años` : '—'),
    campo('Categoría', categorias)
  ];
}

/** SIMIT en cuatro cards: paz y salvo, comparendos, multas y deuda. Null si no hay respuesta del SIMIT. */
function camposSimit(c: CandidatoEnVivo) {
  const simit = c.resultado?.simit;
  if (!simit) return null;
  const campo = (etiqueta: string, valor: string) => ({ etiqueta, valorExito: valor, valorError: valor });
  return [
    campo('Paz y salvo', simit.paz_y_salvo === true ? 'Sí' : 'No'),
    campo('Comparendos', String(simit.cantidad_comparendos ?? 0)),
    campo('Multas', String(simit.cantidad_multas ?? 0)),
    campo('Deuda total', String(simit.total_deuda || '$0'))
  ];
}

function construirNodoEtapa(c: CandidatoEnVivo, id: IdEtapa, posicion: number): DefinicionNodoValidacion {
  const plantilla = PLANTILLAS[id];
  const etapa = c.etapas[id];
  const estado: EstadoNodoValidacion = etapa ? ESTADO_NODO[etapa.estado] : 'pendiente';
  const intentos = etapa ? etapa.reintentos + 1 : 0;
  const detalle = etapa?.detalle;

  let resumen = 'En espera de ejecución';
  if (c.fase === 'completo' && !etapa) resumen = 'No se ejecutó';
  else if (etapa) {
    resumen =
      etapa.estado === 'ejecutando'
        ? `Consultando ${ETIQUETA_ETAPA[id]}…`
        : etapa.estado === 'reintentando'
          ? `Reintentando: ${detalle ?? 'la consulta falló'}`
          : (detalle ?? (etapa.estado === 'omitida' ? 'No consultado (descarte temprano)' : 'Sin detalle'));
  }

  const payload = resultadoDeEtapa(c, id);
  const rutaCaptura = etapa?.captura ?? c.resultado?.capturas?.[id];
  const col = posicion % COLUMNAS;
  const fila = Math.floor(posicion / COLUMNAS);

  return {
    id,
    orden: posicion + 1,
    codigo: `PASO_${String(posicion + 1).padStart(2, '0')}`,
    titulo: ETIQUETA_ETAPA[id],
    subtitulo: plantilla.subtitulo,
    categoria: plantilla.categoria,
    claveIcono: plantilla.claveIcono,
    colorAcento: '#4cc253',
    posicionX: 80 + col * SEPARACION_X,
    posicionY: Y_FLUJO + fila * SEPARACION_Y,
    maxIntentos: 2,
    duracionBaseMs: 0,
    modoSimulacion: 'exito_directo',
    estado,
    intentoActual: Math.min(intentos, 2),
    duracionRealMs: etapa?.ms ?? 0,
    resumenSalida: resumen,
    etiquetaEstadoVivo: etapa ? ETIQUETA_ESTADO[etapa.estado] : 'En cola',
    capturaUrl: rutaCaptura ? urlCaptura(rutaCaptura) : undefined,
    mensajeErrorTransitorio: detalle ?? 'La consulta falló y se reintenta',
    mensajeFalloDefinitivo: detalle ?? 'No se pudo completar esta validación',
    camposDestacados: (id === 'runt' && camposRunt(c, detalle)) || (id === 'simit' && camposSimit(c)) || [
      { etiqueta: 'Resultado', valorExito: detalle || '—', valorError: detalle || '—' },
      {
        etiqueta: 'Estado',
        valorExito: etapa?.estado === 'alerta' ? 'Con alerta' : 'Aprobado',
        valorError: etapa?.estado === 'error' ? 'Sin verificar' : 'No cumple'
      }
    ],
    evidenciaVisual: {
      entidadOficial: plantilla.entidad,
      portalUrl: plantilla.portalUrl,
      codigoRadicado: `EN-VIVO-${c.indice}-${id.toUpperCase()}`,
      tituloCertificado: 'RESULTADO DE LA CONSULTA REALIZADA POR EL SISTEMA',
      selloExito: plantilla.sello[0],
      selloError: plantilla.sello[1],
      observacionLegal:
        detalle ??
        'Consulta automatizada a la fuente oficial. Este registro muestra la respuesta recibida, no un certificado emitido por la entidad.'
    },
    payloadSalidaExito: payload,
    payloadSalidaError: payload
  };
}

interface DatosNodoMockup {
  id: string;
  orden: number;
  titulo: string;
  subtitulo: string;
  categoria: string;
  claveIcono: string;
  posicionX: number;
  posicionY: number;
  estado: EstadoNodoValidacion;
  etiqueta: string;
  resumen: string;
  campos: [string, string][];
  payload: Record<string, unknown>;
  condicional?: boolean;
  rama?: boolean;
  /** true = los datos vienen del backend (hoja de vida real), no se simulan en el navegador. */
  real?: boolean;
}

/** Nodo del flujo que todavía no existe en el backend: se arma solo con datos del frontend. */
function construirNodoMockup(c: CandidatoEnVivo, d: DatosNodoMockup): DefinicionNodoValidacion {
  const nodo = construirNodoSimulado(c, d);
  if (!d.real) return nodo;
  return {
    ...nodo,
    evidenciaVisual: {
      entidadOficial: 'Hoja de vida y perfil del candidato',
      portalUrl: 'internal://hoja-de-vida',
      codigoRadicado: `EN-VIVO-${c.indice}-${d.id.toUpperCase()}`,
      tituloCertificado: 'CONTACTO DETECTADO EN LA HOJA DE VIDA',
      selloExito: 'DETECTADO',
      selloError: 'NO ENCONTRADO',
      observacionLegal: 'Datos extraídos automáticamente del texto de la hoja de vida y del perfil del candidato.'
    },
    payloadSalidaExito: d.payload,
    payloadSalidaError: d.payload
  };
}

function construirNodoSimulado(c: CandidatoEnVivo, d: DatosNodoMockup): DefinicionNodoValidacion {
  return {
    id: d.id,
    orden: d.orden,
    codigo: `PASO_${String(d.orden).padStart(2, '0')}`,
    titulo: d.titulo,
    subtitulo: d.subtitulo,
    categoria: d.categoria,
    claveIcono: d.claveIcono,
    colorAcento: '#4cc253',
    posicionX: d.posicionX,
    posicionY: d.posicionY,
    maxIntentos: 1,
    duracionBaseMs: 0,
    modoSimulacion: 'exito_directo',
    estado: d.estado,
    intentoActual: d.estado === 'completado' ? 1 : 0,
    duracionRealMs: 0,
    resumenSalida: d.resumen,
    etiquetaEstadoVivo: d.etiqueta,
    mensajeErrorTransitorio: '',
    mensajeFalloDefinitivo: '',
    camposDestacados: d.campos.map(([etiqueta, valor]) => ({ etiqueta, valorExito: valor, valorError: valor })),
    evidenciaVisual: {
      entidadOficial: 'Simulación de frontend (sin backend)',
      portalUrl: 'internal://mockup-frontend',
      codigoRadicado: `MOCK-${c.indice}-${d.id.toUpperCase()}`,
      tituloCertificado: 'PASO SIMULADO: AÚN NO CONECTADO AL BACKEND',
      selloExito: 'SIMULADO',
      selloError: 'SIMULADO',
      observacionLegal:
        'Este paso es un mockup visual: los datos se generan en el navegador y no provienen de ninguna fuente real.'
    },
    payloadSalidaExito: { mockup: true, ...d.payload },
    payloadSalidaError: { mockup: true, ...d.payload },
    esCondicional: d.condicional,
    esRama: d.rama
  };
}

/** Contacto que el backend detecta en la hoja de vida del candidato (rama de contacto). */
export type ContactoDetectado =
  | { estado: 'cargando' }
  | {
      estado: 'listo';
      correo?: string;
      telefono?: string;
      envio?: EnvioCorreo;
      envioWhatsapp?: EnvioCorreo;
      /** Lo que el candidato declaró en el formulario (si ya lo envió). */
      formulario?: { completado: boolean; cedula?: string; edad?: number };
    }
  | { estado: 'error'; mensaje: string };

/** Resultado del mensaje (correo o WhatsApp) que pide al candidato completar sus datos. */
export type EnvioCorreo =
  | { estado: 'enviando' }
  | { estado: 'enviado'; yaEnviado: boolean }
  | { estado: 'error'; mensaje: string };

// Personas de prueba: van a la rama de contacto aunque su cédula aparezca en el perfil.
// Debe coincidir con nombresPrueba/cedulasPrueba de go-rrhh/servidor/solicitud_datos.go.
const CEDULAS_PRUEBA = new Set(['1035126774']);
const NOMBRES_PRUEBA = new Set(['daniel ochoa castrillon']);

const sinAcentos = (t: string) => t.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase().trim();

export function esCandidatoDePrueba(c: Pick<CandidatoEnVivo, 'nombre' | 'cedula'>): boolean {
  return CEDULAS_PRUEBA.has(c.cedula.replace(/\D/g, '')) || NOMBRES_PRUEBA.has(sinAcentos(c.nombre));
}

/**
 * Convierte el estado real de un candidato (eventos SSE o resultado guardado) en los nodos del
 * lienzo n8n: una tarjeta por etapa, con su estado, tiempo, reintentos y la respuesta cruda.
 *
 * Tras el perfil se inserta el condicional "¿Tiene cédula?" y después "Cumplimiento de edad". Quien no
 * tiene cédula tampoco tiene edad conocida (la edad solo sale del formulario), así que es en "Cumplimiento
 * de edad" donde el flujo se abre a la rama alterna: contacto -> mensaje (correo y WhatsApp) -> formulario,
 * que vuelve a ese mismo paso. Fuera de la persona de prueba, esa rama es solo un mockup del frontend.
 */
export function construirNodosEnVivo(c: CandidatoEnVivo, contacto?: ContactoDetectado): DefinicionNodoValidacion[] {
  const colX = (col: number) => 80 + col * SEPARACION_X;

  // Posiciones del flujo: 0 perfil, 1 cédula, 2 edad (mockup), 3 en adelante las etapas reales.
  const nodos = ORDEN_ETAPAS.map((id, i) => construirNodoEtapa(c, id, i === 0 ? 0 : i + 2));

  const estadoPerfil = c.etapas.perfil?.estado;
  const perfilListo =
    c.fase === 'completo' ||
    (estadoPerfil !== undefined && estadoPerfil !== 'ejecutando' && estadoPerfil !== 'reintentando');
  const tieneCedula = Boolean(c.cedula?.trim());
  const prueba = esCandidatoDePrueba(c);
  const ramaActiva = perfilListo && (!tieneCedula || prueba);
  // Persona de prueba: la rama usa datos reales de la hoja de vida; el resto de sin cédula sigue simulado.
  const real = ramaActiva && prueba;
  const estadoRama: EstadoNodoValidacion = !perfilListo ? 'pendiente' : ramaActiva ? 'completado' : 'omitido';
  const etiquetaRama = (activa: string) => (!perfilListo ? 'En cola' : ramaActiva ? activa : 'No aplica');
  const resumenRama = (activa: string) =>
    !perfilListo ? 'En espera del perfil' : ramaActiva ? activa : 'No aplica: el candidato sí tiene cédula';

  // Persona de prueba: la edad solo existe cuando el candidato envió el formulario (el backend la calcula).
  // El resto: la edad que el backend leyó de la ficha de Computrabajo; si no viene, queda sin conocer.
  const formulario = real && contacto?.estado === 'listo' ? contacto.formulario : undefined;
  const edadReal = c.resultado?.edad ?? (formulario?.completado ? formulario.edad : undefined);
  const formularioRecibido = edadReal !== undefined;
  // Solo para los datos del formulario simulado (nunca se muestra como edad del candidato).
  const edadSimulada = real && formularioRecibido ? edadReal : 22 + (c.indice % 27);
  const edadNodo = edadReal || undefined;
  const sinEdad = !edadNodo;
  // Misma regla que el backend (RRHH_EDAD_MIN/MAX_CONTRATACION): mayor de 21 y menor de 50.
  const cumpleEdad = !!edadNodo && edadNodo > 21 && edadNodo < 50;

  const cumplimientoEdad = construirNodoMockup(c, {
    id: 'cumplimiento-edad',
    orden: 3,
    titulo: 'Cumplimiento de edad',
    subtitulo: 'Mayor de 21 y menor de 50; si no se conoce, se pide por mensaje',
    categoria: 'Regla de Negocio',
    claveIcono: 'calendario',
    posicionX: colX(2),
    posicionY: Y_FLUJO,
    estado: !perfilListo || sinEdad ? 'pendiente' : cumpleEdad ? 'completado' : 'detenido_error',
    etiqueta: !perfilListo || sinEdad ? 'En cola' : cumpleEdad ? 'Cumple' : 'No cumple',
    resumen: !perfilListo
      ? 'En espera del perfil'
      : sinEdad
        ? 'Sin edad conocida: la ficha de Computrabajo no la trae'
        : cumpleEdad
          ? `Edad ${edadNodo} años: dentro del rango`
          : `Edad ${edadNodo} años: fuera del rango`,
    campos: [
      ['Edad', sinEdad ? '—' : `${edadNodo} años`],
      ['Rango', 'Mayor de 21 y menor de 50']
    ],
    payload: { edad: edadNodo ?? null, rangoMinimo: 21, rangoMaximo: 50, cumple: cumpleEdad, pideDatos: ramaActiva, fuente: sinEdad ? 'Sin dato' : real && formularioRecibido ? 'Formulario del candidato' : 'Ficha de Computrabajo' }
  });

  // Contacto simulado (para las personas sin cédula que aún no se conectan al backend)
  const telefono = `300 555 ${String(1000 + (c.indice % 9000)).padStart(4, '0')}`;
  const correo = `${c.slug.replace(/^\d+-/, '').replace(/-/g, '.')}@correo-ejemplo.com`;

  // Persona de prueba: estado de la rama según lo que detecte el backend en la hoja de vida.
  const buscando = real && (!contacto || contacto.estado === 'cargando');
  const falloContacto = real && contacto?.estado === 'error' ? contacto.mensaje : undefined;
  const correoReal = real && contacto?.estado === 'listo' ? contacto.correo : undefined;
  const telefonoReal = real && contacto?.estado === 'listo' ? contacto.telefono : undefined;
  const hayContacto = Boolean(correoReal || telefonoReal);
  const envio = real && contacto?.estado === 'listo' ? contacto.envio : undefined;

  // Nodos de envío de la persona de prueba: cada canal (correo, WhatsApp) tiene su propio estado.
  const estadoCanal = (
    destino: string | undefined,
    envioCanal: EnvioCorreo | undefined,
    medio: string,
    sinDestino: string
  ): [EstadoNodoValidacion, string, string] => {
    if (!real || buscando) return ['pendiente', 'En cola', `En espera de detectar el ${medio === 'correo' ? 'correo' : 'teléfono'}`];
    if (!destino) return ['omitido', sinDestino, falloContacto ?? `No hay ${medio === 'correo' ? 'un correo' : 'un teléfono'} al que escribir`];
    if (!envioCanal || envioCanal.estado === 'enviando') return ['ejecutando', 'Enviando', `Enviando ${medio} a ${destino}`];
    if (envioCanal.estado === 'enviado') {
      return ['completado', envioCanal.yaEnviado ? 'Ya enviado' : 'Enviado', envioCanal.yaEnviado ? `El ${medio} a ${destino} ya se había enviado` : `${medio === 'correo' ? 'Correo' : 'WhatsApp'} enviado a ${destino}`];
    }
    return ['detenido_error', 'Error', envioCanal.mensaje];
  };
  const listo = real && contacto?.estado === 'listo' ? contacto : undefined;
  const [estadoEnvio, etiquetaEnvio, resumenEnvio] = estadoCanal(correoReal, envio, 'correo', 'Sin correo');
  const [estadoWa, etiquetaWa, resumenWa] = estadoCanal(telefonoReal, listo?.envioWhatsapp, 'WhatsApp', 'Sin teléfono');

  const condCedula = construirNodoMockup(c, {
    id: 'cond-cedula',
    orden: 2,
    titulo: '¿Tiene cédula?',
    subtitulo: 'Revisa la cédula en la hoja de vida y el perfil',
    categoria: 'Condicional',
    claveIcono: 'condicional',
    posicionX: colX(1),
    posicionY: Y_FLUJO,
    estado: perfilListo ? 'completado' : 'pendiente',
    etiqueta: !perfilListo ? 'En cola' : tieneCedula ? 'Sí tiene' : 'Sin cédula',
    resumen: !perfilListo
      ? 'En espera del perfil'
      : tieneCedula
        ? prueba
          ? 'Tiene cédula, pero (prueba) se trata como sin cédula: sus datos se piden en «Cumplimiento de edad»'
          : 'Tiene cédula: sigue a «Cumplimiento de edad»'
        : 'No tiene cédula: tampoco se conoce su edad; se le pedirá en «Cumplimiento de edad»',
    campos: [
      ['Cédula', tieneCedula ? c.cedula : 'No registra'],
      [
        'Ruta',
        !perfilListo ? 'Pendiente' : tieneCedula ? (prueba ? 'Sí → edad (pide datos)' : 'Sí → edad') : 'No → edad (pide datos)'
      ]
    ],
    payload: {
      tieneCedula,
      cedula: tieneCedula ? c.cedula : null,
      ruta: tieneCedula ? 'edad' : 'edad-con-contacto',
      // La rama de contacto (que sale de «Cumplimiento de edad») se toma sin cédula o, mientras dure la prueba, para la persona de prueba.
      ramaContacto: !tieneCedula || prueba
    },
    condicional: true
  });

  const condContacto = construirNodoMockup(c, {
    id: 'cond-contacto',
    orden: 10,
    titulo: '¿Tiene teléfono o correo?',
    subtitulo: 'Busca en el perfil de Computrabajo y en la hoja de vida',
    categoria: real ? 'Condicional · contacto' : 'Condicional · sin edad',
    claveIcono: 'condicional',
    posicionX: colX(3),
    posicionY: Y_RAMA,
    estado: real ? (buscando ? 'ejecutando' : hayContacto ? 'completado' : 'detenido_error') : estadoRama,
    etiqueta: real
      ? buscando
        ? 'Buscando'
        : falloContacto !== undefined
          ? 'Error'
          : hayContacto
            ? 'Sí tiene'
            : 'No encontrado'
      : etiquetaRama('Sí tiene'),
    resumen: real
      ? buscando
        ? 'Buscando correo y celular en la hoja de vida'
        : (falloContacto ?? (hayContacto ? 'Hay datos de contacto: se obtienen' : 'No hay correo ni celular en la hoja de vida'))
      : resumenRama('Hay teléfono y correo: se obtienen y se notifica'),
    campos: real
      ? [
          ['Correo', correoReal ? 'Con datos' : '—'],
          ['Celular', telefonoReal ? 'Con datos' : '—']
        ]
      : [
          ['Perfil Computrabajo', ramaActiva ? 'Con datos' : '—'],
          ['Hoja de vida', ramaActiva ? 'Con datos' : '—']
        ],
    payload: real
      ? { tieneCorreo: Boolean(correoReal), tieneTelefono: Boolean(telefonoReal), fuentes: ['Perfil Computrabajo', 'Hoja de vida'] }
      : { tieneTelefono: true, tieneCorreo: true, fuentes: ['Perfil Computrabajo', 'Hoja de vida'] },
    condicional: true,
    rama: true,
    real
  });

  const obtenerContacto = construirNodoMockup(c, {
    id: 'obtener-contacto',
    orden: 11,
    titulo: 'Obtener teléfono y correo',
    subtitulo: 'Extrae ambos datos del perfil y la hoja de vida',
    categoria: 'Contacto',
    claveIcono: 'contacto',
    posicionX: colX(4),
    posicionY: Y_RAMA,
    estado: real ? (buscando ? 'pendiente' : hayContacto ? 'completado' : 'omitido') : estadoRama,
    etiqueta: real ? (buscando ? 'En cola' : hayContacto ? 'Obtenido' : 'Sin datos') : etiquetaRama('Obtenido'),
    resumen: real
      ? hayContacto
        ? `Teléfono ${telefonoReal ?? '—'} · ${correoReal ?? '—'}`
        : 'Sin datos de contacto'
      : resumenRama(`Teléfono ${telefono} · ${correo}`),
    campos: real
      ? [
          ['Teléfono', telefonoReal ?? '—'],
          ['Correo', textoCorto(correoReal ?? '—', 26)]
        ]
      : [
          ['Teléfono', ramaActiva ? telefono : '—'],
          ['Correo', ramaActiva ? textoCorto(correo, 26) : '—']
        ],
    payload: real ? { telefono: telefonoReal ?? null, correo: correoReal ?? null } : { telefono, correo },
    rama: true,
    real
  });

  const notificar = construirNodoMockup(c, {
    id: 'notificar-contacto',
    orden: 12,
    titulo: real ? 'Enviar correo' : 'Enviar notificación',
    subtitulo: real ? 'Pide al candidato que complete sus datos por correo' : 'Avisa a Reclutamiento con el contacto del candidato',
    categoria: real ? 'Correo' : 'Notificación',
    claveIcono: 'notificacion',
    posicionX: colX(5),
    posicionY: real ? Y_RAMA + SEPARACION_ENVIOS : Y_RAMA,
    estado: real ? estadoEnvio : estadoRama,
    etiqueta: real ? etiquetaEnvio : etiquetaRama('Enviada'),
    resumen: real ? resumenEnvio : resumenRama(`Notificación enviada con teléfono y correo de ${c.nombre}`),
    campos: real
      ? [
          ['Destino', correoReal ? textoCorto(correoReal, 26) : '—'],
          ['Contenido', 'Solicitud de datos faltantes']
        ]
      : [
          ['Destino', ramaActiva ? 'Reclutamiento' : '—'],
          ['Contenido', ramaActiva ? 'Teléfono + correo' : '—']
        ],
    payload: real
      ? { destino: correoReal ?? null, asunto: 'Completa tus datos para continuar en el proceso de selección' }
      : {
          destino: 'Reclutamiento',
          asunto: `Candidato sin cédula: ${c.nombre}`,
          mensaje: `${c.nombre} no registra cédula. Contacto: ${telefono} · ${correo}`
        },
    rama: true,
    real
  });

  // Solo la persona de prueba: el WhatsApp es un envío aparte del correo, con su propio estado.
  const notificarWhatsapp = construirNodoMockup(c, {
    id: 'notificar-whatsapp',
    orden: 13,
    titulo: 'Enviar WhatsApp',
    subtitulo: 'Pide al candidato que complete sus datos por WhatsApp',
    categoria: 'WhatsApp',
    claveIcono: 'whatsapp',
    posicionX: colX(5),
    posicionY: Y_RAMA - SEPARACION_ENVIOS,
    estado: estadoWa,
    etiqueta: etiquetaWa,
    resumen: resumenWa,
    campos: [
      ['Destino', telefonoReal ?? '—'],
      ['Contenido', 'Solicitud de datos faltantes']
    ],
    payload: { destino: telefonoReal ?? null, plantilla: 'Solicitud de datos faltantes' },
    rama: true,
    real
  });

  // Tras enviar el correo o el WhatsApp el flujo espera a que el candidato complete el formulario y
  // solo entonces retoma desde "Cumplimiento de edad".
  // Persona de prueba: el nodo solo se completa si el backend ya guardó el formulario (llega `edad`).
  // El resto de candidatos sin cédula sigue siendo un mockup.
  const envioListo = estadoEnvio === 'completado' || estadoWa === 'completado';
  const estadoForm: EstadoNodoValidacion = !real
    ? estadoRama
    : formularioRecibido
      ? 'completado'
      : envioListo
        ? 'pendiente' // enviado: el candidato aún no responde
        : [estadoEnvio, estadoWa].some((e) => e === 'pendiente' || e === 'ejecutando')
        ? 'pendiente'
        : 'omitido';
  // Datos que el candidato aporta en el formulario (simulados, coherentes con la edad simulada).
  const cedulaFormulario = c.cedula || String(1_000_000_000 + ((c.indice * 7919) % 90_000_000));
  const nacimiento = new Date();
  nacimiento.setFullYear(nacimiento.getFullYear() - edadSimulada);
  nacimiento.setDate(nacimiento.getDate() - (c.indice % 300));
  const fechaNacimiento = nacimiento.toLocaleDateString('es-CO', { day: '2-digit', month: '2-digit', year: 'numeric' });
  const formularioCompleto = estadoForm === 'completado';
  const completarFormulario = construirNodoMockup(c, {
    id: 'completar-formulario',
    orden: 14,
    titulo: 'Completar formulario',
    subtitulo: 'El candidato aporta su cédula y fecha de nacimiento',
    categoria: 'Formulario',
    claveIcono: 'documentoCheck',
    posicionX: colX(6),
    posicionY: Y_RAMA,
    estado: estadoForm,
    etiqueta: !real
      ? etiquetaRama('Completado')
      : estadoForm === 'completado'
        ? 'Completado'
        : estadoForm === 'pendiente'
          ? envioListo
            ? 'Esperando al candidato'
            : 'En cola'
          : 'No aplica',
    resumen: !real
      ? resumenRama(`${c.nombre} completó el formulario con sus datos`)
      : estadoForm === 'completado'
        ? `${c.nombre} completó el formulario: sigue la validación de edad`
        : estadoForm === 'pendiente'
          ? envioListo
            ? 'Mensaje enviado: esperando que el candidato llene el formulario'
            : 'En espera del envío del correo o WhatsApp'
          : 'No se pudo enviar ni correo ni WhatsApp',
    campos: real
      ? [
          ['Cédula', formularioCompleto ? formulario?.cedula || c.cedula || '—' : '—'],
          ['Edad', formularioCompleto ? `${edadReal} años` : '—']
        ]
      : [
          ['Cédula', formularioCompleto ? cedulaFormulario : '—'],
          ['Fecha de nacimiento', formularioCompleto ? fechaNacimiento : '—']
        ],
    payload: real
      ? { completado: formularioCompleto, simulado: false, cedula: formularioCompleto ? formulario?.cedula || c.cedula || null : null, edad: edadReal ?? null, siguientePaso: 'cumplimiento-edad' }
      : {
          completado: formularioCompleto,
          simulado: true,
          cedula: formularioCompleto ? cedulaFormulario : null,
          fechaNacimiento: formularioCompleto ? fechaNacimiento : null,
          siguientePaso: 'cumplimiento-edad'
        },
    rama: true,
    real
  });

  // Flujo principal: perfil -> condicional -> resto de validaciones. La rama alterna va al final.
  return [
    nodos[0],
    condCedula,
    cumplimientoEdad,
    ...nodos.slice(1),
    condContacto,
    obtenerContacto,
    notificar,
    ...(real ? [notificarWhatsapp] : []),
    completarFormulario
  ];
}

/** Datos del candidato en el formato que esperan las tarjetas y modales del lienzo. */
export function datosCandidatoDesdeVivo(c: CandidatoEnVivo): DatosCandidatoEscenario {
  const r = c.resultado;
  const adres = r?.adres ?? null;
  const categorias = r?.runt?.categorias_activas;
  const municipio = adres?.municipio
    ? `${adres.municipio}${adres.departamento ? `, ${adres.departamento}` : ''}`
    : 'Sin dato';

  return {
    nombre: c.nombre,
    cedula: c.cedula || 'Sin cédula',
    edad: 0,
    genero: r?.evaluacion_idoneidad?.genero ?? r?.genero ?? 'Sin dato',
    licencia: categorias && categorias.length > 0 ? categorias.join(', ') : 'Sin verificar',
    experienciaAnios: 0,
    fotoUrl: c.fotoUrl,
    municipio,
    telefono: 'Sin dato',
    correo: 'Sin dato',
    eps: adres?.eps ? String(adres.eps) : 'Sin verificar',
    regimenEps: adres?.regimen ? String(adres.regimen) : 'Sin verificar',
    archivoHojaVida: r?.ruta_cv_original ? (r.ruta_cv_original.split(/[\\/]/).pop() ?? '') : 'No disponible',
    urlHojaVida: r?.ruta_cv_original ? urlCaptura(r.ruta_cv_original) : undefined,
    archivoPerfil: r?.ruta_perfil_pdf ? (r.ruta_perfil_pdf.split(/[\\/]/).pop() ?? '') : 'No disponible',
    resumenPerfil: r?.evaluacion_idoneidad
      ? r.es_apto
        ? 'Cumple las reglas de la vacante: género, licencia C2/C3 en RUNT, SIMIT y EPS autorizada.'
        : (r.evaluacion_idoneidad.motivos_descarte ?? []).join(' · ') || 'No cumple los requisitos de la vacante.'
      : 'Análisis en curso: el dictamen aparece cuando terminan las validaciones.',
    empresasPrevias: []
  };
}

// ---- Adaptadores a los tipos que usa la vista del lienzo -------------------------------------

export function adaptarCandidato(c: CandidatoEnVivo): CandidatoEvaluadoEnVacante {
  const veredicto = veredictoDe(c);
  const completadas = ORDEN_ETAPAS.filter((id) => {
    const e = c.etapas[id]?.estado;
    return e === 'ok' || e === 'alerta';
  }).length;
  return {
    slug: c.slug,
    idEscenario: `en-vivo-${c.indice}`,
    estadoDictamen: veredicto === 'apto' ? 'apto_prioritario' : 'rechazado_regla',
    etiquetaDictamen: ETIQUETA_VEREDICTO[veredicto],
    pasosSuperados: completadas,
    totalPasos: ORDEN_ETAPAS.length,
    nodoObservacion: c.resultado?.evaluacion_idoneidad?.motivos_descarte?.join(' · ') ?? ETIQUETA_VEREDICTO[veredicto],
    fechaUltimaEvaluacion: c.registro.at(-1)?.hora ?? '',
    datos: datosCandidatoDesdeVivo(c),
    configuracionNodos: {}
  };
}

export function adaptarVacante(
  vacante: VacanteApi,
  candidatos: CandidatoEnVivo[]
): VacanteConCandidatos {
  return {
    id: vacante.id,
    slug: vacante.id, // la ruta usa el id: el slug de Computrabajo se repite entre vacantes
    codigoRequisicion: vacante.id.slice(0, 8),
    titulo: vacante.titulo,
    areaOperativa: 'Computrabajo',
    sede: '',
    tipoContrato: '',
    licenciaExigida: 'C2 o C3 activa (≥ 2 años)',
    experienciaMinima: '',
    estadoVacante: 'Activa en Evaluación',
    fechaApertura: '',
    cuposDisponibles: 0,
    candidatos: candidatos.map(adaptarCandidato)
  };
}
