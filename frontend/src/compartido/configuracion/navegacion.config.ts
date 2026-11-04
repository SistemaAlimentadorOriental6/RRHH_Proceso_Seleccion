export interface MigaDePan {
  etiqueta: string;
  ruta?: string;
}

export interface SubItemNavegacion {
  id: string;
  etiqueta: string;
  ruta: string;
  seccion: string;
  descripcion: string;
  icono?: string;
}

export interface GrupoNavegacion {
  id: string;
  titulo: string;
  icono: string;
  subItems: SubItemNavegacion[];
}

export const RUTA_BASE_VALIDACION_CANDIDATOS =
  '/dashboard/candidatos/validacion-logs';

export const RUTA_BASE_RECLUTAMIENTO_VACANTES =
  '/dashboard/reclutamiento/vacantes';

export const RUTA_BASE_SEMILLERO = '/dashboard/semillero';
export const RUTA_BASE_DESCARTADOS = '/dashboard/descartados';
export const RUTA_BASE_PENDIENTES = '/dashboard/pendientes';
export const RUTA_BASE_PARAMETROS = '/dashboard/configuracion/parametros';

export const GRUPOS_NAVEGACION_SIDEBAR: GrupoNavegacion[] = [
  {
    id: 'candidatos',
    titulo: 'Candidatos',
    icono: 'candidatos',
    subItems: [
      {
        id: 'validacion-logs',
        etiqueta: 'Reclutamiento',
        ruta: RUTA_BASE_VALIDACION_CANDIDATOS,
        seccion: 'Candidatos',
        descripcion:
          'Vacantes evaluadas y orquestación paso a paso de validaciones estilo n8n',
        icono: 'flujo'
      },
      {
        id: 'semillero',
        etiqueta: 'Semillero',
        ruta: RUTA_BASE_SEMILLERO,
        seccion: 'Candidatos',
        descripcion: 'Candidatos para tener en cuenta en futuras vacantes',
        icono: 'flujo'
      },
      {
        id: 'pendientes',
        etiqueta: 'Pendientes',
        ruta: RUTA_BASE_PENDIENTES,
        seccion: 'Candidatos',
        descripcion: 'Candidatos con verificación pendiente (faltan datos o no se pudo verificar)',
        icono: 'flujo'
      },
      {
        id: 'descartados',
        etiqueta: 'Descartados',
        ruta: RUTA_BASE_DESCARTADOS,
        seccion: 'Candidatos',
        descripcion: 'Candidatos descartados',
        icono: 'flujo'
      }
    ]
  },
  {
    id: 'configuracion',
    titulo: 'Configuración',
    icono: 'configuracion',
    subItems: [
      {
        id: 'parametros',
        etiqueta: 'Parámetros',
        ruta: RUTA_BASE_PARAMETROS,
        seccion: 'Configuración',
        descripcion: 'Parámetros de aceptación de hojas de vida',
        icono: 'configuracion'
      }
    ]
  }
];
