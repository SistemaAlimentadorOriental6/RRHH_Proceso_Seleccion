import { pedirJson } from '../../candidatos/servicios/clienteRrhh';
import type { ParametrosAceptacion } from '../modelos/parametrosAceptacion';

const RUTA = '/api/parametros-aceptacion';

export const almacenParametros = $state<{ guardados: ParametrosAceptacion | null; cargando: boolean; error: string }>({
  guardados: null,
  cargando: false,
  error: ''
});

/** Lee de la base los parámetros vigentes. El error queda en almacenParametros.error para que la pantalla lo muestre. */
export async function cargarParametros(): Promise<void> {
  almacenParametros.cargando = true;
  almacenParametros.error = '';
  try {
    almacenParametros.guardados = await pedirJson<ParametrosAceptacion>(RUTA);
  } catch (e) {
    almacenParametros.error = e instanceof Error ? e.message : 'No se pudieron leer los parámetros.';
  } finally {
    almacenParametros.cargando = false;
  }
}

// Una versión no cambia después de guardarse: se pide una sola vez (si falla, se vuelve a pedir la próxima).
const versiones = new Map<number, Promise<ParametrosAceptacion>>();

/** Reglas de una versión: con ellas se evaluó a quien muestra ese número. */
export function parametrosDeVersion(version: number): Promise<ParametrosAceptacion> {
  let p = versiones.get(version);
  if (!p) {
    p = pedirJson<ParametrosAceptacion>(`${RUTA}/versiones/${version}`);
    p.catch(() => versiones.delete(version));
    versiones.set(version, p);
  }
  return p;
}

/** Guarda en la base y devuelve lo que quedó guardado (ya normalizado). Lanza ErrorApi con el mensaje del servidor si falla. */
export async function guardarParametros(p: ParametrosAceptacion): Promise<ParametrosAceptacion> {
  const guardados = await pedirJson<ParametrosAceptacion>(RUTA, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify($state.snapshot(p))
  });
  almacenParametros.guardados = guardados;
  return guardados;
}
