import {
  ESCENARIOS_SIMULACION,
  type DatosCandidatoEscenario,
  type EscenarioValidacion,
  type ModoSimulacionNodo
} from './flujoValidacion';

export type EstadoDictamenCandidato =
  | 'apto_prioritario'
  | 'con_reintento_recuperado'
  | 'rechazado_regla';

export interface CandidatoEvaluadoEnVacante {
  slug: string;
  idEscenario: string;
  estadoDictamen: EstadoDictamenCandidato;
  etiquetaDictamen: string;
  pasosSuperados: number;
  totalPasos: number;
  nodoObservacion: string;
  fechaUltimaEvaluacion: string;
  datos: DatosCandidatoEscenario;
  configuracionNodos: Record<string, ModoSimulacionNodo>;
}

export interface VacanteConCandidatos {
  id: string;
  slug: string;
  codigoRequisicion: string;
  titulo: string;
  areaOperativa: string;
  sede: string;
  tipoContrato: string;
  licenciaExigida: string;
  experienciaMinima: string;
  estadoVacante: 'Activa en Evaluación' | 'Preselección Cerrada';
  fechaApertura: string;
  cuposDisponibles: number;
  candidatos: CandidatoEvaluadoEnVacante[];
}

function crearFotoAdicionalSvg(
  iniciales: string,
  fondo1: string,
  fondo2: string,
  camisa: string,
  piel: string,
  cabello: string
): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 320" fill="none">
    <defs>
      <linearGradient id="bg" x1="0" y1="0" x2="320" y2="320" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="${fondo1}"/>
        <stop offset="100%" stop-color="${fondo2}"/>
      </linearGradient>
    </defs>
    <rect width="320" height="320" rx="48" fill="url(#bg)"/>
    <circle cx="160" cy="160" r="136" stroke="rgba(255,255,255,0.35)" stroke-width="2"/>
    <path d="M56 320C56 254 100 222 160 222C220 222 264 254 264 320H56Z" fill="${camisa}"/>
    <path d="M132 224L160 272L188 224" fill="#ffffff"/>
    <path d="M153 245H167L164 310H156L153 245Z" fill="#4cc253"/>
    <rect x="140" y="182" width="40" height="48" rx="16" fill="${piel}"/>
    <ellipse cx="160" cy="136" rx="54" ry="62" fill="${piel}"/>
    <path d="M106 128C106 86 128 66 160 66C192 66 214 86 214 128C206 104 188 94 160 94C132 94 114 104 106 128Z" fill="${cabello}"/>
    <circle cx="140" cy="136" r="5" fill="#1e293b"/>
    <circle cx="180" cy="136" r="5" fill="#1e293b"/>
    <path d="M144 162C150 170 170 170 176 162" stroke="#1e293b" stroke-width="4" stroke-linecap="round"/>
    <rect x="208" y="24" width="88" height="32" rx="16" fill="rgba(255,255,255,0.92)"/>
    <text x="252" y="45" text-anchor="middle" fill="#2c3e50" font-family="Inter, sans-serif" font-weight="700" font-size="13">${iniciales} • SAO6</text>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

export const CATALOGO_VACANTES_EVALUACION: VacanteConCandidatos[] = [
  {
    id: 'vac-01',
    slug: 'operador-buses-alimentadores-metro-medellin',
    codigoRequisicion: 'REQ-SAO6-2026-014',
    titulo: 'Operador de Buses Alimentadores - Metro de Medellín',
    areaOperativa: 'Operaciones y Conducción Masiva',
    sede: 'Patio Oriental SAO6 — Medellín',
    tipoContrato: 'Término Indefinido Directo',
    licenciaExigida: 'C2 o C3 Activa (>= 2 años)',
    experienciaMinima: '2 años en vehículos pesados',
    estadoVacante: 'Activa en Evaluación',
    fechaApertura: '12 Oct 2026',
    cuposDisponibles: 12,
    candidatos: [
      {
        slug: 'julian-andres-castaneda-alzate',
        idEscenario: ESCENARIOS_SIMULACION[0].id,
        estadoDictamen: 'con_reintento_recuperado',
        etiquetaDictamen: 'Apto Validado (Con Reintento PGN/RUNT)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones OK (Auto-recuperado en PGN y RUNT)',
        fechaUltimaEvaluacion: 'Hoy, 10:25 AM',
        datos: ESCENARIOS_SIMULACION[0].candidato,
        configuracionNodos: ESCENARIOS_SIMULACION[0].configuracionNodos
      },
      {
        slug: 'juan-carlos-tamayo-ossa',
        idEscenario: 'escenario-tamayo-directo',
        estadoDictamen: 'apto_prioritario',
        etiquetaDictamen: 'Apto Prioritario (14/14 Directo)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones superadas en primer intento',
        fechaUltimaEvaluacion: 'Hoy, 09:48 AM',
        datos: {
          nombre: 'Juan Carlos Tamayo Ossa',
          cedula: '98.575.875',
          edad: 42,
          genero: 'Hombre',
          licencia: 'C2 Activa (Vence 2029-07-09)',
          experienciaAnios: 9,
          fotoUrl: crearFotoAdicionalSvg(
            'JCT',
            '#e8f8ea',
            '#c8f0ce',
            '#0f172a',
            '#e0ac69',
            '#1e293b'
          ),
          municipio: 'Caldas / Medellín, Antioquia',
          telefono: '311 640 8821',
          correo: 'juancarlos.tamayo@correo.com',
          eps: 'SAVIA SALUD EPS S.A.S.',
          regimenEps: 'SUBSIDIADO (ACTIVO)',
          archivoHojaVida: 'perfil_computrabajo_JUAN_CARLOS_TAMAYO_OSSA.pdf',
          archivoPerfil: 'perfil_computrabajo_JUAN_CARLOS_TAMAYO_OSSA.pdf',
          resumenPerfil:
            'Conductor con 16.8 años de registro en RUNT y amplia trayectoria en transporte público de pasajeros en el sur del Valle de Aburrá. Cero comparendos en SIMIT y licencia C2 vigente hasta 2029.',
          empresasPrevias: [
            'Transportes Unidos La Estrella — Conductor Bus (2019 - 2026)',
            'Cootrasana — Operador Ruta Integrada (2016 - 2019)'
          ]
        },
        configuracionNodos: {}
      },
      {
        slug: 'jorge-luis-torres-pereira',
        idEscenario: ESCENARIOS_SIMULACION[1].id,
        estadoDictamen: 'rechazado_regla',
        etiquetaDictamen: 'Detenido en #13 Multas SIMIT',
        pasosSuperados: 12,
        totalPasos: 14,
        nodoObservacion: 'Registra deuda de $2.480.000 sin acuerdo de pago',
        fechaUltimaEvaluacion: 'Hoy, 09:15 AM',
        datos: ESCENARIOS_SIMULACION[1].candidato,
        configuracionNodos: ESCENARIOS_SIMULACION[1].configuracionNodos
      },
      {
        slug: 'sebastian-parra-avendano',
        idEscenario: ESCENARIOS_SIMULACION[2].id,
        estadoDictamen: 'rechazado_regla',
        etiquetaDictamen: 'Detenido en #10 Tipo de Licencia',
        pasosSuperados: 9,
        totalPasos: 14,
        nodoObservacion: 'Solo posee categoría C1 (Se exige C2 o C3 activa)',
        fechaUltimaEvaluacion: 'Ayer, 05:40 PM',
        datos: ESCENARIOS_SIMULACION[2].candidato,
        configuracionNodos: ESCENARIOS_SIMULACION[2].configuracionNodos
      }
    ]
  },
  {
    id: 'vac-02',
    slug: 'conductor-bus-padron-electrico-sao6',
    codigoRequisicion: 'REQ-SAO6-2026-019',
    titulo: 'Conductor de Bus Padrón Eléctrico — Flota Sostenible',
    areaOperativa: 'Electromovilidad y Corredor Verde',
    sede: 'Patio de Recarga Oriental — Medellín',
    tipoContrato: 'Término Indefinido Directo',
    licenciaExigida: 'C2 o C3 Activa (>= 3 años)',
    experienciaMinima: '3 años en buses padrón o articulados',
    estadoVacante: 'Activa en Evaluación',
    fechaApertura: '14 Oct 2026',
    cuposDisponibles: 6,
    candidatos: [
      {
        slug: 'filiberto-restrepo-macias',
        idEscenario: 'escenario-filiberto-padron',
        estadoDictamen: 'con_reintento_recuperado',
        etiquetaDictamen: 'Apto Validado (Reintento RUNT OK)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones OK (Captcha RUNT recuperado en intento 2)',
        fechaUltimaEvaluacion: 'Hoy, 10:05 AM',
        datos: {
          nombre: 'Filiberto Restrepo Macías',
          cedula: '71.981.992',
          edad: 39,
          genero: 'Hombre',
          licencia: 'C2 Activa (Vence 2028-04-21)',
          experienciaAnios: 8,
          fotoUrl: crearFotoAdicionalSvg(
            'FR',
            '#e8f8ea',
            '#d1fae5',
            '#1e293b',
            '#f1c27d',
            '#2c3e50'
          ),
          municipio: 'Medellín, Antioquia',
          telefono: '310 429 7731',
          correo: 'filiberto.restrepo@correo.com',
          eps: 'EPS SURA',
          regimenEps: 'CONTRIBUTIVO (COTIZANTE ACTIVO)',
          archivoHojaVida: 'hoja_de_vida_original_Filiberto_Restrepo_Macias.pdf',
          archivoPerfil: 'perfil_computrabajo_Filiberto_Restrepo_Macias.pdf',
          resumenPerfil:
            'Operador certificado en conducción preventiva de buses padrón de bajas emisiones con 8 años de trayectoria continua sin siniestralidad vial.',
          empresasPrevias: [
            'Conducciones Palenque Robledo — Operador Padrón (2020 - 2026)',
            'Bellanita de Transporte — Conductor Servicio Público (2017 - 2020)'
          ]
        },
        configuracionNodos: {
          'validacion-runt': 'recuperar_en_reintento'
        }
      },
      {
        slug: 'antonio-arias-meza',
        idEscenario: 'escenario-antonio-padron',
        estadoDictamen: 'apto_prioritario',
        etiquetaDictamen: 'Apto Prioritario (14/14 Directo)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones superadas sin novedades',
        fechaUltimaEvaluacion: 'Ayer, 04:20 PM',
        datos: {
          nombre: 'Antonio Arias Meza',
          cedula: '71.334.918',
          edad: 44,
          genero: 'Hombre',
          licencia: 'C3 Activa (Vence 2028-11-02)',
          experienciaAnios: 11,
          fotoUrl: crearFotoAdicionalSvg(
            'AA',
            '#eef2ff',
            '#e0e7ff',
            '#0f172a',
            '#e0ac69',
            '#1e293b'
          ),
          municipio: 'Envigado, Antioquia',
          telefono: '315 892 1045',
          correo: 'antonio.arias.meza@correo.com',
          eps: 'EPS SURAMERICANA S.A.',
          regimenEps: 'CONTRIBUTIVO (ACTIVO)',
          archivoHojaVida: 'hoja_de_vida_original_Antonio_Arias_Meza.pdf',
          archivoPerfil: 'perfil_computrabajo_Antonio_Arias_Meza.pdf',
          resumenPerfil:
            'Conductor C3 con 11 años de experiencia en transporte masivo e intermunicipal pesado. Paz y salvo integral en RUNT, SIMIT, Policía y Procuraduría.',
          empresasPrevias: [
            'Sotrasanvicente — Conductor Bus Pesado C3 (2018 - 2026)',
            'Expreso Belmira — Operador Intermunicipal (2014 - 2018)'
          ]
        },
        configuracionNodos: {}
      }
    ]
  },
  {
    id: 'vac-03',
    slug: 'conductor-reserva-operativa-cuenca-6',
    codigoRequisicion: 'REQ-SAO6-2026-022',
    titulo: 'Conductor de Reserva Operativa y Patio — Cuenca 6',
    areaOperativa: 'Maniobras de Patio y Relevo en Línea',
    sede: 'Terminal Oriente — Medellín',
    tipoContrato: 'Término Fijo Renovable',
    licenciaExigida: 'C2 Activa (>= 2 años)',
    experienciaMinima: '2 años en transporte urbano',
    estadoVacante: 'Activa en Evaluación',
    fechaApertura: '15 Oct 2026',
    cuposDisponibles: 4,
    candidatos: [
      {
        slug: 'yeferson-hurtado-mosquera',
        idEscenario: 'escenario-yeferson-reserva',
        estadoDictamen: 'con_reintento_recuperado',
        etiquetaDictamen: 'Apto Validado (Reintento PGN OK)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones OK (Reto Procuraduría superado al 2° intento)',
        fechaUltimaEvaluacion: 'Hoy, 08:30 AM',
        datos: {
          nombre: 'Yeferson Hurtado Mosquera',
          cedula: '1.028.019.442',
          edad: 29,
          genero: 'Hombre',
          licencia: 'C2 Activa (Vence 2027-10-15)',
          experienciaAnios: 4,
          fotoUrl: crearFotoAdicionalSvg(
            'YH',
            '#e8f8ea',
            '#dcfce7',
            '#1e293b',
            '#8d5524',
            '#0f172a'
          ),
          municipio: 'Medellín, Antioquia',
          telefono: '304 219 6501',
          correo: 'yeferson.hurtado@correo.com',
          eps: 'NUEVA EPS S.A.',
          regimenEps: 'CONTRIBUTIVO (ACTIVO)',
          archivoHojaVida: 'hoja_de_vida_original_yeferson_hurtado.pdf',
          archivoPerfil: 'perfil_computrabajo_yeferson_hurtado.pdf',
          resumenPerfil:
            'Operador de patio y relevo de rutas alimentadoras en la Cuenca 6 con 4 años de experiencia certificada y licencia C2 activa sin comparendos.',
          empresasPrevias: [
            'Coopetransa — Conductor Urbano C2 (2022 - 2026)'
          ]
        },
        configuracionNodos: {
          'procuraduria-pgn': 'recuperar_en_reintento'
        }
      },
      {
        slug: 'johan-alexander-zapata-cardona',
        idEscenario: 'escenario-johan-reserva',
        estadoDictamen: 'apto_prioritario',
        etiquetaDictamen: 'Apto Prioritario (14/14 Directo)',
        pasosSuperados: 14,
        totalPasos: 14,
        nodoObservacion: '14/14 validaciones aprobadas sin reintentos',
        fechaUltimaEvaluacion: 'Ayer, 03:10 PM',
        datos: {
          nombre: 'Johan Alexander Zapata Cardona',
          cedula: '1.036.654.219',
          edad: 33,
          genero: 'Hombre',
          licencia: 'C2 Activa (Vence 2028-02-18)',
          experienciaAnios: 5,
          fotoUrl: crearFotoAdicionalSvg(
            'JZ',
            '#f0fdf4',
            '#bbf7d0',
            '#334155',
            '#f1c27d',
            '#1e293b'
          ),
          municipio: 'Copacabana, Antioquia',
          telefono: '312 778 3910',
          correo: 'johan.zapata.c@correo.com',
          eps: 'EPS SURA',
          regimenEps: 'CONTRIBUTIVO (ACTIVO)',
          archivoHojaVida: 'hoja_de_vida_original_johan_alexander_zapata_cardona.pdf',
          archivoPerfil: 'perfil_computrabajo_johan_alexander_zapata_cardona.pdf',
          resumenPerfil:
            'Conductor de buses alimentadores con 5 años de experiencia continua en rutas del norte y oriente del Valle de Aburrá.',
          empresasPrevias: [
            'Cootracovi — Conductor Bus Urbano (2021 - 2026)'
          ]
        },
        configuracionNodos: {}
      }
    ]
  }
];

export function buscarVacantePorSlug(
  slugVacante: string
): VacanteConCandidatos | undefined {
  return CATALOGO_VACANTES_EVALUACION.find((v) => v.slug === slugVacante);
}

export function buscarCandidatoEnVacante(
  slugVacante: string,
  slugCandidato: string
): {
  vacante: VacanteConCandidatos;
  candidato: CandidatoEvaluadoEnVacante;
} | null {
  const vacante = buscarVacantePorSlug(slugVacante);
  if (!vacante) return null;
  const candidato = vacante.candidatos.find((c) => c.slug === slugCandidato);
  if (!candidato) return null;
  return { vacante, candidato };
}

export function construirEscenarioDesdeCandidato(
  vacante: VacanteConCandidatos,
  candidato: CandidatoEvaluadoEnVacante
): EscenarioValidacion {
  return {
    id: `${vacante.slug}__${candidato.slug}`,
    nombre: `${candidato.datos.nombre} — ${candidato.etiquetaDictamen}`,
    descripcion: candidato.nodoObservacion,
    vacante: vacante.titulo,
    candidato: candidato.datos,
    configuracionNodos: candidato.configuracionNodos
  };
}
