import { pedirJson } from './clienteRrhh';

/** Resumen de la segunda oportunidad a los descartados (POST /api/descartados/reanalizar). */
export interface ResumenReanalisis {
  /** Versión de los parámetros con la que se reanaliza. */
  version: number;
  /** Descartados revisados. */
  total: number;
  /** Los que podrían cambiar de resultado: se reanalizan. */
  reanalizar: { postulacion_id: string; nombre: string; regla_version: number; motivo: string }[];
  /** Los que siguen descartados con lo que ya se sabe de ellos, agrupados por motivo. */
  omitidos: { motivo: string; cantidad: number }[];
  simulado: boolean;
  /** Cuántos quedaron en cola (los que ya estaban en cola no se repiten). */
  encolados: number;
}

/** Postulaciones con un reanálisis pendiente o en curso: la cola vive en el servidor y es la misma para todos. */
export async function colaReanalisis(): Promise<string[]> {
  return (await pedirJson<{ en_cola: string[] }>('/api/descartados/reanalisis')).en_cola ?? [];
}

/** Con simular=true solo calcula a quién reanalizar; sin él, además los pone en cola. */
export function reanalizarDescartados(simular: boolean): Promise<ResumenReanalisis> {
  return pedirJson<ResumenReanalisis>(`/api/descartados/reanalizar${simular ? '?simular=1' : ''}`, { method: 'POST' });
}
