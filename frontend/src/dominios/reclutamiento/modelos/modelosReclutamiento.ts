export interface LicenciaRuntCandidato {
  numeroLicencia: string;
  categoria: string;
  estado: 'ACTIVA' | 'VENCIDA' | 'SUSPENDIDA';
  fechaExpedicion: string;
  fechaVencimiento: string;
  antiguedadAnios: number;
}

export interface ExpedienteRuntCandidato {
  encontrado: boolean;
  nombreCompleto: string;
  tipoDocumento: string;
  numeroDocumento: string;
  nroInscripcionRunt: string;
  fechaInscripcion: string;
  estadoConductor: 'ACTIVO' | 'INACTIVO';
  estadoPazYSalvo: 'A PAZ Y SALVO' | 'CON PENDIENTES';
  numeroPazYSalvo: string;
  restriccionesConductor: string;
  licencias: LicenciaRuntCandidato[];
}

export interface ExpedienteAdresCandidato {
  encontrado: boolean;
  tipoDocumento: string;
  numeroDocumento: string;
  nombres: string;
  apellidos: string;
  estadoAfiliacion: 'ACTIVO' | 'RETIRADO' | 'SUSPENDIDO';
  entidadEps: string;
  regimen: 'CONTRIBUTIVO' | 'SUBSIDIADO';
  tipoAfiliado: 'COTIZANTE' | 'BENEFICIARIO' | 'CABEZA DE FAMILIA';
  fechaAfiliacionEfectiva: string;
  departamentoMunicipio: string;
}

export interface MultaComparendoSimit {
  numeroResolucion: string;
  fechaComparendo: string;
  codigoInfraccion: string;
  descripcionInfraccion: string;
  secretariaTransito: string;
  estado: 'En cobro coactivo' | 'Pendiente de pago' | 'Acuerdo de pago vigente';
  valorTotal: string;
}

export interface ExpedienteSimitCandidato {
  consultadoExitosamente: boolean;
  poseePendientes: boolean;
  cantidadMultas: number;
  cantidadComparendos: number;
  cantidadAcuerdosPago: number;
  totalDeudaFormateado: string;
  totalEnAcuerdosFormateado: string;
  totalAPagarNumerico: number;
  mensajeEstado: string;
  detalleComparendos: MultaComparendoSimit[];
}

export interface ExpedienteAntecedentesDisciplinarios {
  procuraduriaEstado: 'SIN ANTECEDENTES' | 'CON ANTECEDENTES';
  procuraduriaCertificado: string;
  contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL' | 'REPORTADO';
  contraloriaCertificado: string;
  policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES' | 'PENDIENTES';
  sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU' | 'ALERTA';
  ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC' | 'ALERTA';
  reintegroSao6Estado: string;
}

export interface ClasificacionPrimariaCandidato {
  esApto: boolean;
  etiquetaEstado: 'Aceptado (Apto)' | 'Rechazado (No Apto)';
  cumpleGenero: boolean;
  detalleGenero: string;
  cumpleEdad: boolean;
  detalleEdad: string;
  cumpleEps: boolean;
  detalleEps: string;
  cumpleRunt: boolean;
  detalleRunt: string;
  cumpleSimit: boolean;
  detalleSimit: string;
  cumpleAntecedentes: boolean;
  detalleAntecedentes: string;
  motivosRechazo: string[];
}

export interface EvaluacionScoreIA {
  analizadoPorIA: boolean;
  posicionRanking?: number;
  scoreTotal: number;
  scoreExperiencia: number; // 0 - 30 pts
  scoreConduccionNormativo: number; // 0 - 30 pts
  scoreEstabilidad: number; // 0 - 20 pts
  scoreAfinidad: number; // 0 - 20 pts
  dictamenIA: 'Apto Prioritario' | 'Apto Recomendado' | 'Apto con Reservas';
  porQueEsElIndicado: string;
  resumenPerfil: string;
  fortalezasClave: string[];
  habilidadesDetectadas: string[];
  empresasAnteriores: string[];
  nivelEducativo: string;
  observacionesRiesgo: string;
  modeloUtilizado: string;
  tiempoInferenciaSegundos: number;
}

export interface CandidatoReclutamiento {
  id: string;
  slug: string;
  nombreCompleto: string;
  cedula: string;
  edad: number;
  genero: 'Hombre' | 'Mujer';
  telefono: string;
  correo: string;
  municipioResidencia: string;
  cargoActualOanterior: string;
  anosExperienciaConduccion: number;
  categoriaLicenciaPrincipal: string;
  fechaPostulacion: string;
  fotoUrl: string;
  extractoHojaDeVida: string;
  clasificacionPrimaria: ClasificacionPrimariaCandidato;
  expediente: {
    runt: ExpedienteRuntCandidato;
    adres: ExpedienteAdresCandidato;
    simit: ExpedienteSimitCandidato;
    antecedentes: ExpedienteAntecedentesDisciplinarios;
  };
  evaluacionIA?: EvaluacionScoreIA;
}

export interface VacanteReclutamiento {
  id: string;
  slug: string;
  codigoRequisicion: string;
  titulo: string;
  areaOperativa: string;
  sede: string;
  tipoContrato: string;
  salarioReferencia: string;
  licenciaExigida: string;
  experienciaMinima: string;
  fechaApertura: string;
  cuposDisponibles: number;
  estadoVacante: 'Ranking IA Completado' | 'En Evaluación Primaria';
  reglasScoringIA: {
    pesoExperiencia: number;
    pesoConduccionNormativo: number;
    pesoEstabilidad: number;
    pesoAfinidad: number;
    descripcionRegla: string;
  };
  candidatos: CandidatoReclutamiento[];
}

function crearRetratoCandidatoSvg(
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
    <circle cx="160" cy="160" r="136" stroke="rgba(255,255,255,0.38)" stroke-width="2"/>
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

export const CATALOGO_VACANTES_RECLUTAMIENTO: VacanteReclutamiento[] = [
  {
    id: 'rec-vac-01',
    slug: 'operador-buses-alimentadores-metro-medellin',
    codigoRequisicion: 'REQ-SAO6-2026-014',
    titulo: 'Operador de Buses Alimentadores - Metro de Medellín',
    areaOperativa: 'Operaciones y Conducción Masiva',
    sede: 'Patio Oriental SAO6 — Medellín',
    tipoContrato: 'Término Indefinido Directo',
    salarioReferencia: '$2.480.000 + Auxilio Operativo + Prestaciones',
    licenciaExigida: 'C2 o C3 Activa (Antigüedad >= 2 años)',
    experienciaMinima: '2 años en vehículos pesados o servicio público',
    fechaApertura: '12 Oct 2026',
    cuposDisponibles: 12,
    estadoVacante: 'Ranking IA Completado',
    reglasScoringIA: {
      pesoExperiencia: 30,
      pesoConduccionNormativo: 30,
      pesoEstabilidad: 20,
      pesoAfinidad: 20,
      descripcionRegla:
        'Evalúa únicamente a los candidatos Aceptados (Aptos) cruzando su hoja de vida, continuidad laboral en flotas de transporte masivo, antigüedad de licencia C2/C3 en RUNT y comportamiento vial en SIMIT.'
    },
    candidatos: [
      {
        id: 'cand-rec-101',
        slug: 'juan-carlos-tamayo-ossa',
        nombreCompleto: 'Juan Carlos Tamayo Ossa',
        cedula: '98.575.875',
        edad: 42,
        genero: 'Hombre',
        telefono: '+57 310 448 9210',
        correo: 'jctamayo.conductor@gmail.com',
        municipioResidencia: 'Bello, Antioquia',
        cargoActualOanterior: 'Operador de Padrón y Alimentador SITVA',
        anosExperienciaConduccion: 9,
        categoriaLicenciaPrincipal: 'C2 Activa (14.2 años)',
        fechaPostulacion: '14 Oct 2026, 08:30 AM',
        fotoUrl: crearRetratoCandidatoSvg('JCT', '#e8f8ea', '#c8f0ce', '#14532d', '#ecc59c', '#1f2937'),
        extractoHojaDeVida:
          'Conductor profesional de transporte masivo y colectivo con 9 años de experiencia certificada operando buses padrón a gas natural vehicular (GNV) y buses alimentadores en cuencas del Valle de Aburrá. Experto en conducción preventiva, atención a usuarios del sistema integrado Metro, cumplimiento estricto de tablas horarias y alistamiento preoperacional.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '42 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'EPS SURA — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C2 ACTIVA — 14.2 años de antigüedad (Vence 2029-07-09)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'JUAN CARLOS TAMAYO OSSA',
            tipoDocumento: 'C.C.',
            numeroDocumento: '98575875',
            nroInscripcionRunt: '10004829104',
            fechaInscripcion: '18/05/2010',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '492817462910',
            restriccionesConductor: 'Ninguna restricción médica o técnica registrada',
            licencias: [
              {
                numeroLicencia: '05001000849102',
                categoria: 'C2',
                estado: 'ACTIVA',
                fechaExpedicion: '12/06/2012',
                fechaVencimiento: '09/07/2029',
                antiguedadAnios: 14.2
              },
              {
                numeroLicencia: '05001000849102',
                categoria: 'B2',
                estado: 'ACTIVA',
                fechaExpedicion: '12/06/2012',
                fechaVencimiento: '09/07/2029',
                antiguedadAnios: 14.2
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '98575875',
            nombres: 'JUAN CARLOS',
            apellidos: 'TAMAYO OSSA',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'EPS SURAMERICANA S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '01/03/2018',
            departamentoMunicipio: 'ANTIOQUIA - MEDELLÍN'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-8841209',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-391042',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Elegible directo — Sin desvinculaciones previas desfavorables'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 1,
          scoreTotal: 95,
          scoreExperiencia: 29,
          scoreConduccionNormativo: 30,
          scoreEstabilidad: 18,
          scoreAfinidad: 18,
          dictamenIA: 'Apto Prioritario',
          porQueEsElIndicado:
            'Es el candidato #1 para la vacante porque combina 9 años de experiencia directa operando buses alimentadores y padrones en el sistema SITVA con un historial vial impecable ($0 en SIMIT, 0 comparendos históricos) y una licencia C2 con 14.2 años de antigüedad vigente hasta 2029. Además, registra estabilidades laborales superiores a 4 años por empresa y certificaciones vigentes en conducción preventiva y manejo de flota GNV.',
          resumenPerfil:
            'Operador senior de transporte masivo con dominio comprobado en rutas alimentadoras de alta pendiente y tráfico denso en Medellín.',
          fortalezasClave: [
            '14.2 años de antigüedad en licencia C2 activa (Vence en julio de 2029)',
            'Récord SIMIT 100% limpio ($0 COP en multas y comparendos)',
            '9 años de experiencia continua en operación de buses alimentadores y padrones',
            'Certificación SENA en Operación Segura de Vehículos Masivos y Seguridad Vial'
          ],
          habilidadesDetectadas: [
            'Conducción Preventiva SITVA',
            'Operación Buses Padrón GNV',
            'Alistamiento Preoperacional',
            'Servicio al Usuario Metro',
            'Control de Tiempos y Frecuencias'
          ],
          empresasAnteriores: [
            'Conducciones Oriente S.A.S. (4.8 años)',
            'Masivo de Occidente S.A. (4.2 años)'
          ],
          nivelEducativo: 'Técnico Laboral en Operación de Transporte Masivo (SENA)',
          observacionesRiesgo: 'Sin alertas operativas ni jurídicas. Disponibilidad inmediata para curva de ruta.',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.8
        }
      },
      {
        id: 'cand-rec-102',
        slug: 'julian-andres-castaneda-alzate',
        nombreCompleto: 'Julián Andrés Castañeda Álzate',
        cedula: '71.269.493',
        edad: 47,
        genero: 'Hombre',
        telefono: '+57 314 782 1940',
        correo: 'julian.castaneda71@hotmail.com',
        municipioResidencia: 'Medellín, Antioquia',
        cargoActualOanterior: 'Conductor de Bus Urbano y Especial',
        anosExperienciaConduccion: 6,
        categoriaLicenciaPrincipal: 'C2 Activa (14.4 años)',
        fechaPostulacion: '14 Oct 2026, 10:15 AM',
        fotoUrl: crearRetratoCandidatoSvg('JAC', '#e8f8ea', '#d0f0d6', '#1e293b', '#f2cba5', '#2c3e50'),
        extractoHojaDeVida:
          'Conductor profesional con 6 años de experiencia reciente en operación de buses urbanos de pasajeros de 9 y 12 metros en el área metropolitana. Sólido conocimiento de nomenclatura urbana de Medellín, protocolos de embarque seguro de pasajeros, reporte de novedades mecánicas en patio y cero siniestralidad vial.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '47 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'EPS SURA — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C2 ACTIVA — 14.4 años de antigüedad (Vence 2027-07-04)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'JULIAN ANDRES CASTANEDA ALZATE',
            tipoDocumento: 'C.C.',
            numeroDocumento: '71269493',
            nroInscripcionRunt: '10002918471',
            fechaInscripcion: '11/02/2010',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '391847201928',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05001000391842',
                categoria: 'C2',
                estado: 'ACTIVA',
                fechaExpedicion: '04/05/2011',
                fechaVencimiento: '04/07/2027',
                antiguedadAnios: 14.4
              },
              {
                numeroLicencia: '05001000391842',
                categoria: 'C1',
                estado: 'VENCIDA',
                fechaExpedicion: '10/03/2008',
                fechaVencimiento: '10/03/2011',
                antiguedadAnios: 17.6
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '71269493',
            nombres: 'JULIAN ANDRES',
            apellidos: 'CASTANEDA ALZATE',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'EPS SURAMERICANA S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '15/08/2020',
            departamentoMunicipio: 'ANTIOQUIA - MEDELLÍN'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-7731904',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-291044',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Elegible — Sin bloqueos disciplinarios en SAO6'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 2,
          scoreTotal: 89,
          scoreExperiencia: 25,
          scoreConduccionNormativo: 29,
          scoreEstabilidad: 18,
          scoreAfinidad: 17,
          dictamenIA: 'Apto Prioritario',
          porQueEsElIndicado:
            'Ocupa la posición #2 gracias a su excelente madurez operativa (14.4 años con licencia C2 activa), cero infracciones en SIMIT y 6 años continuos manejando buses urbanos de alta capacidad en corredores orientales de Medellín. Su perfil destaca por baja rotación laboral (3 años promedio por empresa) y cumplimiento íntegro de los 14 filtros normativos.',
          resumenPerfil:
            'Conductor de transporte urbano consolidado, con alta confiabilidad normativa y conocimiento de rutas orientales.',
          fortalezasClave: [
            'Licencia C2 activa con 14.4 años de antigüedad comprobada en RUNT',
            'Paz y salvo total en SIMIT ($0 COP) y RUNT sin restricciones',
            '6 años de experiencia específica en buses urbanos de 9 y 12 metros',
            'Estabilidad laboral sólida en empresas de transporte público de Medellín'
          ],
          habilidadesDetectadas: [
            'Conducción de Buses 12m',
            'Rutas Cuenca Oriental',
            'Mecánica Básica Preventiva',
            'Atención Cordial al Pasajero'
          ],
          empresasAnteriores: [
            'Transportes Aranjuez Santa Cruz S.A. (3.5 años)',
            'Cootransmede S.A.S. (2.5 años)'
          ],
          nivelEducativo: 'Bachiller Académico + Curso Seguridad Vial Pesados',
          observacionesRiesgo: 'Licencia C2 vence en julio de 2027 (más de 1 año de margen operativo).',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.9
        }
      },
      {
        id: 'cand-rec-103',
        slug: 'ferney-alonso-bedoya-giraldo',
        nombreCompleto: 'Ferney Alonso Bedoya Giraldo',
        cedula: '71.789.312',
        edad: 39,
        genero: 'Hombre',
        telefono: '+57 301 612 8841',
        correo: 'ferney.bedoya.c3@gmail.com',
        municipioResidencia: 'Itagüí, Antioquia',
        cargoActualOanterior: 'Conductor de Bus Intermunicipal y Empresarial',
        anosExperienciaConduccion: 5,
        categoriaLicenciaPrincipal: 'C3 Activa (8.5 años)',
        fechaPostulacion: '15 Oct 2026, 09:05 AM',
        fotoUrl: crearRetratoCandidatoSvg('FAB', '#eff6ff', '#dbeafe', '#1e3a8a', '#e9c199', '#111827'),
        extractoHojaDeVida:
          'Conductor con licencia C3 activa y 5 años de experiencia operando buses de transporte empresarial e intermunicipal de corta distancia en Antioquia. Cuenta con acuerdo de pago vigente en SIMIT por $480.000 COP al día, excelente trato al pasajero y disponibilidad para turnos rotativos de madrugada o cierre.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '39 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'NUEVA EPS — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C3 ACTIVA — 8.5 años de antigüedad (Vence 2028-11-20)',
          cumpleSimit: true,
          detalleSimit: 'Acuerdo de pago vigente por $480.000 COP (Permitido <= $1.000.000)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'FERNEY ALONSO BEDOYA GIRALDO',
            tipoDocumento: 'C.C.',
            numeroDocumento: '71789312',
            nroInscripcionRunt: '10005920184',
            fechaInscripcion: '22/08/2014',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '582910492817',
            restriccionesConductor: 'Uso de lentes correctivos durante la conducción',
            licencias: [
              {
                numeroLicencia: '05360000918273',
                categoria: 'C3',
                estado: 'ACTIVA',
                fechaExpedicion: '20/11/2018',
                fechaVencimiento: '20/11/2028',
                antiguedadAnios: 8.5
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '71789312',
            nombres: 'FERNEY ALONSO',
            apellidos: 'BEDOYA GIRALDO',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'NUEVA EPS S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '10/01/2021',
            departamentoMunicipio: 'ANTIOQUIA - ITAGÜÍ'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 1,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$480.000 COP',
            totalAPagarNumerico: 0,
            mensajeEstado:
              'Posee 1 acuerdo de pago al día por $480.000 COP (Dentro del tope permitido de $1.000.000 COP).',
            detalleComparendos: [
              {
                numeroResolucion: 'AP-2025-0091824',
                fechaComparendo: '14/04/2025',
                codigoInfraccion: 'C14',
                descripcionInfraccion: 'Transitar por sitios restringidos o en horas prohibidas (Pico y Placa)',
                secretariaTransito: 'Secretaría de Movilidad de Medellín',
                estado: 'Acuerdo de pago vigente',
                valorTotal: '$480.000 COP'
              }
            ]
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-6629102',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-119283',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Elegible — Sin historial previo en SAO6'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 3,
          scoreTotal: 81,
          scoreExperiencia: 22,
          scoreConduccionNormativo: 25,
          scoreEstabilidad: 17,
          scoreAfinidad: 17,
          dictamenIA: 'Apto Recomendado',
          porQueEsElIndicado:
            'Se ubica en el puesto #3 porque posee licencia superior categoría C3 vigente hasta 2028 y 5 años de experiencia operando flota de pasajeros pesada. Aunque registra un acuerdo de pago en SIMIT por $480.000 COP, se encuentra al día y dentro del umbral permitido por SAO6 (<= $1.000.000), siendo una excelente opción para citación a prueba práctica de patio.',
          resumenPerfil:
            'Conductor con licencia máxima C3 y sólida destreza en maniobra de vehículos largos; acuerdo de pago SIMIT controlado.',
          fortalezasClave: [
            'Licencia categoría C3 activa con vigencia amplia hasta noviembre de 2028',
            '5 años de experiencia en transporte empresarial e intermunicipal de pasajeros',
            'Acuerdo de pago SIMIT al día ($480.000 COP <= $1.000.000 COP permitido)'
          ],
          habilidadesDetectadas: [
            'Licencia C3 Articulados/Pesados',
            'Transporte Empresarial',
            'Inspección de Frenos Neumáticos',
            'Turnos Rotativos'
          ],
          empresasAnteriores: [
            'Expreso Belmira S.A.S. (3.0 años)',
            'Transportes Especiales del Sur (2.0 años)'
          ],
          nivelEducativo: 'Bachiller Técnico Industrial',
          observacionesRiesgo:
            'Verificar en entrevista soporte de última cuota pagada del acuerdo de pago AP-2025-0091824.',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.7
        }
      },
      {
        id: 'cand-rec-104',
        slug: 'diego-alejandro-yepes-monsalve',
        nombreCompleto: 'Diego Alejandro Yepes Monsalve',
        cedula: '1.037.619.420',
        edad: 31,
        genero: 'Hombre',
        telefono: '+57 321 904 3318',
        correo: 'diego.yepes.conductor@gmail.com',
        municipioResidencia: 'Copacabana, Antioquia',
        cargoActualOanterior: 'Conductor de Microbús y Camión Rígido',
        anosExperienciaConduccion: 3,
        categoriaLicenciaPrincipal: 'C2 Activa (3.2 años)',
        fechaPostulacion: '15 Oct 2026, 11:40 AM',
        fotoUrl: crearRetratoCandidatoSvg('DAY', '#fefce8', '#fef08a', '#334155', '#f1c69e', '#1e293b'),
        extractoHojaDeVida:
          'Conductor joven con 3 años de experiencia combinada entre microbús colectivo municipal y camión rígido de reparto urbano. Cuenta con licencia C2 activa de 3.2 años de antigüedad, afiliación activa a Salud Total EPS y cero comparendos en SIMIT. Busca integrarse al sistema alimentador Metro para desarrollar carrera operativa.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '31 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'SALUD TOTAL EPS — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C2 ACTIVA — 3.2 años de antigüedad (Vence 2026-12-18)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'DIEGO ALEJANDRO YEPES MONSALVE',
            tipoDocumento: 'C.C.',
            numeroDocumento: '1037619420',
            nroInscripcionRunt: '10008819204',
            fechaInscripcion: '14/07/2021',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '771928374610',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05212000192834',
                categoria: 'C2',
                estado: 'ACTIVA',
                fechaExpedicion: '18/08/2023',
                fechaVencimiento: '18/12/2026',
                antiguedadAnios: 3.2
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '1037619420',
            nombres: 'DIEGO ALEJANDRO',
            apellidos: 'YEPES MONSALVE',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'SALUD TOTAL ENTIDAD PROMOTORA DE SALUD',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '04/05/2023',
            departamentoMunicipio: 'ANTIOQUIA - COPACABANA'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-5519201',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-441029',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Elegible — Primer proceso en SAO6'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 4,
          scoreTotal: 74,
          scoreExperiencia: 18,
          scoreConduccionNormativo: 27,
          scoreEstabilidad: 14,
          scoreAfinidad: 15,
          dictamenIA: 'Apto Recomendado',
          porQueEsElIndicado:
            'Se posiciona en el lugar #4 entre los candidatos aceptados. Cumple todos los requisitos legales (C2 activa > 2 años, $0 en SIMIT y Salud Total activa), pero cuenta con menor trayectoria específica en buses pesados de 12 metros (3 años combinados entre microbús y camión). Ideal para ingresar con acompañamiento de instructor de patio.',
          resumenPerfil:
            'Conductor con récord vial limpio y cumplimiento normativo total; requiere apadrinamiento técnico en bus padrón.',
          fortalezasClave: [
            'Récord SIMIT completamente limpio ($0 COP)',
            'Cumplimiento del 100% de validaciones de seguridad y EPS Salud Total activa',
            'Alta disposición de aprendizaje y residencia cercana al norte/oriente'
          ],
          habilidadesDetectadas: [
            'Conducción Microbús Colectivo',
            'Reparto Urbano Pesado',
            'Cero Comparendos'
          ],
          empresasAnteriores: [
            'Cootracovi Copacabana (1.8 años)',
            'Distribuciones Antioquia S.A.S. (1.2 años)'
          ],
          nivelEducativo: 'Bachiller Académico',
          observacionesRiesgo:
            'Su licencia C2 vence en diciembre de 2026; debe programar refrendación antes de finalizar el trimestre.',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.6
        }
      },
      {
        id: 'cand-rec-105',
        slug: 'carlos-arturo-restrepo-velez',
        nombreCompleto: 'Carlos Arturo Restrepo Vélez',
        cedula: '15.512.011',
        edad: 39,
        genero: 'Hombre',
        telefono: '+57 311 509 7721',
        correo: 'carlos.restrepo15@yahoo.com',
        municipioResidencia: 'Medellín, Antioquia',
        cargoActualOanterior: 'Conductor de Transporte Urbano',
        anosExperienciaConduccion: 5,
        categoriaLicenciaPrincipal: 'C2 Activa (7.1 años)',
        fechaPostulacion: '13 Oct 2026, 04:20 PM',
        fotoUrl: crearRetratoCandidatoSvg('CAR', '#fef2f2', '#fecaca', '#334155', '#e5b88f', '#0f172a'),
        extractoHojaDeVida:
          'Conductor de servicio público urbano con 5 años de experiencia en rutas de Medellín. Posee licencia C2 vigente y afiliación activa a SURA EPS, pero presenta comparendos en cobro coactivo sin acuerdo de pago en la plataforma SIMIT.',
        clasificacionPrimaria: {
          esApto: false,
          etiquetaEstado: 'Rechazado (No Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '39 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'EPS SURA — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C2 ACTIVA — 7.1 años de antigüedad (Vence 2028-03-19)',
          cumpleSimit: false,
          detalleSimit: 'RECHAZADO: 2 multas en cobro coactivo por $2.480.000 COP sin acuerdo de pago',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: [
            'Registra 2 multas pendientes de pago en SIMIT por $2.480.000 COP en cobro coactivo (Excede tope y no posee acuerdo de pago vigente).'
          ]
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'CARLOS ARTURO RESTREPO VELEZ',
            tipoDocumento: 'C.C.',
            numeroDocumento: '15512011',
            nroInscripcionRunt: '10003819201',
            fechaInscripcion: '09/04/2015',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'CON PENDIENTES',
            numeroPazYSalvo: 'NO REGISTRA PAZ Y SALVO POR MULTAS ACTIVAS',
            restriccionesConductor: 'Ninguna restricción médica',
            licencias: [
              {
                numeroLicencia: '05001000661829',
                categoria: 'C2',
                estado: 'ACTIVA',
                fechaExpedicion: '19/03/2019',
                fechaVencimiento: '19/03/2028',
                antiguedadAnios: 7.1
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '15512011',
            nombres: 'CARLOS ARTURO',
            apellidos: 'RESTREPO VELEZ',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'EPS SURAMERICANA S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '20/09/2019',
            departamentoMunicipio: 'ANTIOQUIA - MEDELLÍN'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: true,
            cantidadMultas: 2,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$2.480.000 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 2480000,
            mensajeEstado:
              'RECHAZO AUTOMÁTICO: Registra 2 resoluciones en cobro coactivo por $2.480.000 COP sin acuerdo de pago.',
            detalleComparendos: [
              {
                numeroResolucion: 'RES-2024-884120',
                fechaComparendo: '11/09/2024',
                codigoInfraccion: 'C29',
                descripcionInfraccion: 'Conducir un vehículo a velocidad superior a la máxima permitida (Fotomulta)',
                secretariaTransito: 'Secretaría de Movilidad de Medellín',
                estado: 'En cobro coactivo',
                valorTotal: '$1.240.000 COP'
              },
              {
                numeroResolucion: 'RES-2025-119204',
                fechaComparendo: '03/02/2025',
                codigoInfraccion: 'D04',
                descripcionInfraccion: 'No detenerse ante una luz roja o amarilla de semáforo',
                secretariaTransito: 'Secretaría de Tránsito de Bello',
                estado: 'En cobro coactivo',
                valorTotal: '$1.240.000 COP'
              }
            ]
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-4192831',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-881920',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Sin vínculo previo en SAO6'
          }
        }
      },
      {
        id: 'cand-rec-106',
        slug: 'hector-fabio-quintero-rios',
        nombreCompleto: 'Héctor Fabio Quintero Ríos',
        cedula: '70.812.904',
        edad: 34,
        genero: 'Hombre',
        telefono: '+57 315 229 4100',
        correo: 'hfquintero.conductor@gmail.com',
        municipioResidencia: 'Envigado, Antioquia',
        cargoActualOanterior: 'Conductor de Taxi y Van Ejecutiva',
        anosExperienciaConduccion: 4,
        categoriaLicenciaPrincipal: 'C1 Activa (No cumple C2/C3)',
        fechaPostulacion: '13 Oct 2026, 02:10 PM',
        fotoUrl: crearRetratoCandidatoSvg('HFQ', '#fff7ed', '#fed7aa', '#475569', '#f1c27d', '#334155'),
        extractoHojaDeVida:
          'Conductor de servicio individual y transporte ejecutivo en camionetas y vans durante 4 años. Postulado a la vacante de operador de bus alimentador, pero únicamente cuenta con categoría C1 registrada en el RUNT.',
        clasificacionPrimaria: {
          esApto: false,
          etiquetaEstado: 'Rechazado (No Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '34 años (Rango permitido 24-58)',
          cumpleEps: false,
          detalleEps: 'SAVIA SALUD EPS — SUBSIDIADO (Entidad no autorizada para nómina)',
          cumpleRunt: false,
          detalleRunt: 'RECHAZADO: Solo posee categoría C1 (Se exige C2 o C3 activa con >= 2 años)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: [
            'No posee licencia categoría C2 ni C3 en RUNT (Únicamente registra categoría C1 para vehículos livianos).',
            'Entidad promotora de salud SAVIA SALUD EPS no se encuentra dentro del listado de EPS autorizadas por nómina.'
          ]
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'HECTOR FABIO QUINTERO RIOS',
            tipoDocumento: 'C.C.',
            numeroDocumento: '70812904',
            nroInscripcionRunt: '10006718290',
            fechaInscripcion: '19/10/2017',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '661728391042',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05266000441920',
                categoria: 'C1',
                estado: 'ACTIVA',
                fechaExpedicion: '12/04/2021',
                fechaVencimiento: '12/04/2027',
                antiguedadAnios: 5.5
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '70812904',
            nombres: 'HECTOR FABIO',
            apellidos: 'QUINTERO RIOS',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'ALIANZA MEDELLIN ANTIOQUIA EPS S.A.S. SAVIA SALUD',
            regimen: 'SUBSIDIADO',
            tipoAfiliado: 'CABEZA DE FAMILIA',
            fechaAfiliacionEfectiva: '01/11/2022',
            departamentoMunicipio: 'ANTIOQUIA - ENVIGADO'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-3109284',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-771829',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Sin vínculo previo en SAO6'
          }
        }
      }
    ]
  },
  {
    id: 'rec-vac-02',
    slug: 'conductor-padron-articulado-cuenca-6',
    codigoRequisicion: 'REQ-SAO6-2026-019',
    titulo: 'Conductor de Bus Padrón y Articulado - Cuenca 6',
    areaOperativa: 'Troncales y Corredores de Alta Capacidad',
    sede: 'Patio Norte Bello — Operación Metroplús',
    tipoContrato: 'Término Indefinido Directo',
    salarioReferencia: '$2.890.000 + Bono Troncal + Prestaciones',
    licenciaExigida: 'C3 Activa o C2 Senior (>= 3 años)',
    experienciaMinima: '3 años en vehículos pesados o articulados',
    fechaApertura: '14 Oct 2026',
    cuposDisponibles: 8,
    estadoVacante: 'Ranking IA Completado',
    reglasScoringIA: {
      pesoExperiencia: 30,
      pesoConduccionNormativo: 30,
      pesoEstabilidad: 20,
      pesoAfinidad: 20,
      descripcionRegla:
        'Prioriza candidatos Aceptados (Aptos) con licencia C3 o C2 de larga data, experiencia en buses de alta capacidad (>12 metros), cero comparendos y continuidad laboral en operadores troncales.'
    },
    candidatos: [
      {
        id: 'cand-rec-201',
        slug: 'mauricio-hernando-gomez-cardona',
        nombreCompleto: 'Mauricio Hernando Gómez Cardona',
        cedula: '71.384.219',
        edad: 45,
        genero: 'Hombre',
        telefono: '+57 312 889 4501',
        correo: 'mauricio.gomez.c3@gmail.com',
        municipioResidencia: 'Bello, Antioquia',
        cargoActualOanterior: 'Operador de Bus Articulado y Padrón GNV',
        anosExperienciaConduccion: 11,
        categoriaLicenciaPrincipal: 'C3 Activa (16.0 años)',
        fechaPostulacion: '15 Oct 2026, 07:50 AM',
        fotoUrl: crearRetratoCandidatoSvg('MHG', '#e8f8ea', '#bbf7d0', '#0f172a', '#e9c199', '#1e293b'),
        extractoHojaDeVida:
          'Operador máster con 11 años de experiencia conduciendo buses articulados y padrones en carriles exclusivos troncales. Instructor par en conducción económica a gas natural, maniobra en estaciones de transferencia y protocolos de seguridad operacional.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '45 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'EPS SURA — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C3 ACTIVA — 16.0 años de antigüedad (Vence 2029-11-15)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'MAURICIO HERNANDO GOMEZ CARDONA',
            tipoDocumento: 'C.C.',
            numeroDocumento: '71384219',
            nroInscripcionRunt: '10001192834',
            fechaInscripcion: '05/03/2010',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '991827364510',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05088000192831',
                categoria: 'C3',
                estado: 'ACTIVA',
                fechaExpedicion: '15/11/2010',
                fechaVencimiento: '15/11/2029',
                antiguedadAnios: 16.0
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '71384219',
            nombres: 'MAURICIO HERNANDO',
            apellidos: 'GOMEZ CARDONA',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'EPS SURAMERICANA S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '01/02/2016',
            departamentoMunicipio: 'ANTIOQUIA - BELLO'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-9918273',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-551928',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Excolaborador SAO6 2019-2022 con retiro voluntario y evaluación 98/100'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 1,
          scoreTotal: 97,
          scoreExperiencia: 30,
          scoreConduccionNormativo: 30,
          scoreEstabilidad: 19,
          scoreAfinidad: 18,
          dictamenIA: 'Apto Prioritario',
          porQueEsElIndicado:
            'Es el candidato #1 indiscutible para Cuenca 6: posee licencia C3 activa con 16 años de antigüedad, 11 años operando buses articulados y padrones, récord SIMIT en $0 COP y además fue colaborador de SAO6 entre 2019 y 2022 con retiro voluntario y calificación de desempeño sobresaliente (Reintegro Aprobado). Su curva de adaptación será inmediata.',
          resumenPerfil:
            'Operador máster de articulados con licencia C3 de 16 años y antecedente de desempeño sobresaliente en SAO6.',
          fortalezasClave: [
            '11 años de experiencia real en operación de buses articulados y padrones troncales',
            'Licencia C3 activa por 16 años (Vigencia hasta noviembre de 2029)',
            'Excolaborador SAO6 elegible para reintegro inmediato sin curva de inducción extensa',
            'Récord SIMIT impecable ($0 COP)'
          ],
          habilidadesDetectadas: [
            'Operación Bus Articulado 18m',
            'Aproximación Estaciones Metroplús',
            'Conducción Económica GNV',
            'Formador de Operadores'
          ],
          empresasAnteriores: [
            'Sistema Alimentador Oriental SAO6 S.A.S. (3.5 años)',
            'Metroplús Operación Troncal (7.5 años)'
          ],
          nivelEducativo: 'Tecnólogo en Seguridad Vial y Transporte',
          observacionesRiesgo: 'Cero riesgos detectados. Candidato prioritario para contratación inmediata.',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.7
        }
      },
      {
        id: 'cand-rec-202',
        slug: 'robinson-dario-lopez-aristizabal',
        nombreCompleto: 'Róbinson Darío López Aristizábal',
        cedula: '98.641.209',
        edad: 40,
        genero: 'Hombre',
        telefono: '+57 300 419 8273',
        correo: 'robinson.lopez98@gmail.com',
        municipioResidencia: 'Medellín, Antioquia',
        cargoActualOanterior: 'Conductor de Bus Padrón y Servicio Especial',
        anosExperienciaConduccion: 7,
        categoriaLicenciaPrincipal: 'C2 Activa (11.5 años)',
        fechaPostulacion: '15 Oct 2026, 09:30 AM',
        fotoUrl: crearRetratoCandidatoSvg('RDL', '#eff6ff', '#bfdbfe', '#1e40af', '#f0c8a0', '#111827'),
        extractoHojaDeVida:
          'Conductor con 7 años de experiencia en transporte masivo y buses tipo padrón de 12 metros. Destaca por puntualidad, manejo defensivo en corredores exclusivos y cumplimiento total de normatividad de tránsito.',
        clasificacionPrimaria: {
          esApto: true,
          etiquetaEstado: 'Aceptado (Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '40 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'SANITAS EPS — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: true,
          detalleRunt: 'Licencia C2 ACTIVA — 11.5 años de antigüedad (Vence 2028-09-12)',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: []
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'ROBINSON DARIO LOPEZ ARISTIZABAL',
            tipoDocumento: 'C.C.',
            numeroDocumento: '98641209',
            nroInscripcionRunt: '10004419283',
            fechaInscripcion: '19/06/2013',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '881726354910',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05001000771829',
                categoria: 'C2',
                estado: 'ACTIVA',
                fechaExpedicion: '12/03/2015',
                fechaVencimiento: '12/09/2028',
                antiguedadAnios: 11.5
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '98641209',
            nombres: 'ROBINSON DARIO',
            apellidos: 'LOPEZ ARISTIZABAL',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'ENTIDAD PROMOTORA DE SALUD SANITAS S.A.S.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '12/04/2019',
            departamentoMunicipio: 'ANTIOQUIA - MEDELLÍN'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-2219384',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-661920',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Elegible — Sin bloqueos'
          }
        },
        evaluacionIA: {
          analizadoPorIA: true,
          posicionRanking: 2,
          scoreTotal: 88,
          scoreExperiencia: 25,
          scoreConduccionNormativo: 29,
          scoreEstabilidad: 17,
          scoreAfinidad: 17,
          dictamenIA: 'Apto Prioritario',
          porQueEsElIndicado:
            'Ocupa la posición #2 por su sólida trayectoria de 7 años operando buses tipo padrón, licencia C2 con 11.5 años de antigüedad sin infracciones en SIMIT y afiliación activa en EPS Sanitas. Ideal para asignación directa a flota de buses padrón en Cuenca 6.',
          resumenPerfil:
            'Conductor senior de bus padrón con 11.5 años de licencia C2 y récord vial intacto.',
          fortalezasClave: [
            '7 años de experiencia específica en buses padrón de 12 metros',
            '11.5 años de antigüedad en licencia C2 activa hasta 2028',
            'Cero multas y cero comparendos en SIMIT ($0 COP)'
          ],
          habilidadesDetectadas: [
            'Operación Bus Padrón',
            'Manejo Defensivo Corredores',
            'Control de Aforo y Seguridad'
          ],
          empresasAnteriores: [
            'Coonatra Transporte Masivo (4.0 años)',
            'Sotrasanví S.A. (3.0 años)'
          ],
          nivelEducativo: 'Bachiller Académico + Diplomado Seguridad Vial',
          observacionesRiesgo: 'Habilitado para operar buses padrón; si se asigna a articulado requiere recategorización a C3.',
          modeloUtilizado: 'qwen2.5:14b-instruct • Motor Scoring RRHH',
          tiempoInferenciaSegundos: 1.8
        }
      },
      {
        id: 'cand-rec-203',
        slug: 'jhon-jairo-palacio-chavarria',
        nombreCompleto: 'Jhon Jairo Palacio Chavarría',
        cedula: '71.902.114',
        edad: 36,
        genero: 'Hombre',
        telefono: '+57 318 650 1192',
        correo: 'jjpalacio.transporte@gmail.com',
        municipioResidencia: 'Bello, Antioquia',
        cargoActualOanterior: 'Conductor de Camión y Buseta Colectiva',
        anosExperienciaConduccion: 2,
        categoriaLicenciaPrincipal: 'C2 Vencida (Inactiva en RUNT)',
        fechaPostulacion: '14 Oct 2026, 05:10 PM',
        fotoUrl: crearRetratoCandidatoSvg('JJP', '#fef2f2', '#fecaca', '#475569', '#d2a17c', '#1e293b'),
        extractoHojaDeVida:
          'Conductor con experiencia en transporte colectivo urbano y reparto. Se postula a Cuenca 6 pero su licencia categoría C2 figura con fecha de vencimiento expirada en el RUNT.',
        clasificacionPrimaria: {
          esApto: false,
          etiquetaEstado: 'Rechazado (No Apto)',
          cumpleGenero: true,
          detalleGenero: 'Hombre (Habilitado)',
          cumpleEdad: true,
          detalleEdad: '36 años (Rango permitido 24-58)',
          cumpleEps: true,
          detalleEps: 'NUEVA EPS — CONTRIBUTIVO (ACTIVO)',
          cumpleRunt: false,
          detalleRunt: 'RECHAZADO: Licencia C2 VENCIDA desde el 14/02/2026',
          cumpleSimit: true,
          detalleSimit: 'Sin multas ni comparendos ($0 COP)',
          cumpleAntecedentes: true,
          detalleAntecedentes: 'Procuraduría, Contraloría, Policía y SARLAFT sin registros',
          motivosRechazo: [
            'La licencia de conducción categoría C2 aparece en estado VENCIDA en el RUNT (Expiró el 14/02/2026).'
          ]
        },
        expediente: {
          runt: {
            encontrado: true,
            nombreCompleto: 'JHON JAIRO PALACIO CHAVARRIA',
            tipoDocumento: 'C.C.',
            numeroDocumento: '71902114',
            nroInscripcionRunt: '10007718291',
            fechaInscripcion: '10/09/2016',
            estadoConductor: 'ACTIVO',
            estadoPazYSalvo: 'A PAZ Y SALVO',
            numeroPazYSalvo: '551627384910',
            restriccionesConductor: 'Ninguna restricción registrada',
            licencias: [
              {
                numeroLicencia: '05088000551920',
                categoria: 'C2',
                estado: 'VENCIDA',
                fechaExpedicion: '14/02/2023',
                fechaVencimiento: '14/02/2026',
                antiguedadAnios: 3.6
              }
            ]
          },
          adres: {
            encontrado: true,
            tipoDocumento: 'CC',
            numeroDocumento: '71902114',
            nombres: 'JHON JAIRO',
            apellidos: 'PALACIO CHAVARRIA',
            estadoAfiliacion: 'ACTIVO',
            entidadEps: 'NUEVA EPS S.A.',
            regimen: 'CONTRIBUTIVO',
            tipoAfiliado: 'COTIZANTE',
            fechaAfiliacionEfectiva: '01/06/2022',
            departamentoMunicipio: 'ANTIOQUIA - BELLO'
          },
          simit: {
            consultadoExitosamente: true,
            poseePendientes: false,
            cantidadMultas: 0,
            cantidadComparendos: 0,
            cantidadAcuerdosPago: 0,
            totalDeudaFormateado: '$0 COP',
            totalEnAcuerdosFormateado: '$0 COP',
            totalAPagarNumerico: 0,
            mensajeEstado: 'El ciudadano no presenta multas ni comparendos pendientes en SIMIT.',
            detalleComparendos: []
          },
          antecedentes: {
            procuraduriaEstado: 'SIN ANTECEDENTES',
            procuraduriaCertificado: 'CERT-PGN-2026-1092834',
            contraloriaEstado: 'NO SE ENCUENTRA REPORTADO COMO RESPONSABLE FISCAL',
            contraloriaCertificado: 'SIBOR-CGR-2026-991023',
            policiaEstado: 'NO TIENE ASUNTOS PENDIENTES CON LAS AUTORIDADES JUDICIALES',
            sarlaftOnuEstado: '0 COINCIDENCIAS EN LISTA CONSOLIDADA ONU',
            ofacEstado: '0 COINCIDENCIAS EN SDN LIST OFAC',
            reintegroSao6Estado: 'Sin vínculo previo en SAO6'
          }
        }
      }
    ]
  }
];

export function buscarVacanteReclutamientoPorSlug(
  slugVacante: string
): VacanteReclutamiento | undefined {
  return CATALOGO_VACANTES_RECLUTAMIENTO.find((v) => v.slug === slugVacante);
}

export function buscarCandidatoReclutamientoEnVacante(
  slugVacante: string,
  slugCandidato: string
): { vacante: VacanteReclutamiento; candidato: CandidatoReclutamiento } | null {
  const vacante = buscarVacanteReclutamientoPorSlug(slugVacante);
  if (!vacante) return null;
  const candidato = vacante.candidatos.find((c) => c.slug === slugCandidato);
  if (!candidato) return null;
  return { vacante, candidato };
}

export function obtenerCandidatosAceptadosOrdenadosPorScore(
  vacante: VacanteReclutamiento
): CandidatoReclutamiento[] {
  return vacante.candidatos
    .filter((c) => c.clasificacionPrimaria.esApto)
    .slice()
    .sort((a, b) => (b.evaluacionIA?.scoreTotal ?? 0) - (a.evaluacionIA?.scoreTotal ?? 0))
    .map((candidato, indice) => ({
      ...candidato,
      evaluacionIA: candidato.evaluacionIA
        ? {
            ...candidato.evaluacionIA,
            posicionRanking: indice + 1
          }
        : undefined
    }));
}

export function obtenerCandidatosRechazadosVacante(
  vacante: VacanteReclutamiento
): CandidatoReclutamiento[] {
  return vacante.candidatos.filter((c) => !c.clasificacionPrimaria.esApto);
}
