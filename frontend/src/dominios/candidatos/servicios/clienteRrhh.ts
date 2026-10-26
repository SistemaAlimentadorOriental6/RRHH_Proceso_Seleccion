import toast from 'svelte-french-toast';
import { EVENTO_SESION_EXPIRADA } from '../../autenticacion/servicios/servicioAutenticacion';
import type {
  CandidatoApi,
  VacanteApi
} from '../modelos/analisisEnVivo';

/** En desarrollo Vite redirige /api al backend (puerto 8080); VITE_API_URL permite apuntar a otro host. */
const BASE_API: string = (import.meta.env.VITE_API_URL as string | undefined) ?? '';

export class ErrorApi extends Error {
  constructor(
    mensaje: string,
    readonly codigo: number
  ) {
    super(mensaje);
    this.name = 'ErrorApi';
  }
}

async function pedirJson<T>(ruta: string, init?: RequestInit): Promise<T> {
  let respuesta: Response;
  try {
    respuesta = await fetch(`${BASE_API}${ruta}`, init);
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ErrorApi('No hay conexión con el servidor RRHH (¿está corriendo en el puerto 8080?)', 0);
  }
  if (!respuesta.ok) {
    if (respuesta.status === 401) window.dispatchEvent(new Event(EVENTO_SESION_EXPIRADA));
    let mensaje = `El servidor respondió ${respuesta.status}`;
    try {
      const cuerpo = (await respuesta.json()) as { error?: string };
      if (cuerpo.error) mensaje = cuerpo.error;
    } catch {
      /* cuerpo no JSON: se deja el mensaje genérico */
    }
    throw new ErrorApi(mensaje, respuesta.status);
  }
  return (await respuesta.json()) as T;
}

/**
 * Vacantes activas en Computrabajo. Con contar=false responde rápido (sin abrir el listado de cada
 * vacante) y total_candidatos viene en null; con contar=true cuenta los inscritos reales y puede tardar.
 */
export function listarVacantes(contar: boolean, senal?: AbortSignal): Promise<VacanteApi[]> {
  return pedirJson<VacanteApi[]>(
    `/api/computrabajo/vacantes${contar ? '' : '?contar=0'}`,
    { signal: senal }
  );
}

/** Candidatos ya analizados y guardados de una vacante (por su id, no por slug). */
export function listarCandidatosGuardados(
  idVacante: string,
  senal?: AbortSignal
): Promise<CandidatoApi[]> {
  return pedirJson<CandidatoApi[]>(
    `/api/computrabajo/vacantes/${encodeURIComponent(idVacante)}/candidatos`,
    { signal: senal }
  );
}

export interface ContactoCandidatoApi {
  correo?: string;
  telefono?: string;
}

/**
 * Correo y celular detectados en la hoja de vida y el perfil. `candidato` es la cédula o el slug del
 * nombre sin el índice. El backend solo lo habilita para las personas de prueba (403 para el resto).
 */
export function obtenerContactoCandidato(
  idVacante: string,
  candidato: string,
  senal?: AbortSignal
): Promise<ContactoCandidatoApi> {
  return pedirJson<ContactoCandidatoApi>(
    `/api/computrabajo/vacantes/${encodeURIComponent(idVacante)}/candidatos/${encodeURIComponent(candidato)}/contacto`,
    { signal: senal }
  );
}

export interface SolicitudDatosApi extends ContactoCandidatoApi {
  correo_enviado: boolean;
  /** El correo ya se había enviado antes: el backend no lo repite. */
  ya_enviado?: boolean;
  whatsapp_enviado?: boolean;
  whatsapp_ya_enviado?: boolean;
  whatsapp_error?: string;
  mensaje: string;
}

/** Envía al candidato el correo que le pide completar sus datos en la plataforma (idempotente en el backend). */
export function solicitarDatosFaltantes(
  idVacante: string,
  candidato: string,
  senal?: AbortSignal
): Promise<SolicitudDatosApi> {
  return pedirJson<SolicitudDatosApi>(
    `/api/computrabajo/vacantes/${encodeURIComponent(idVacante)}/candidatos/${encodeURIComponent(candidato)}/solicitar-datos`,
    { method: 'POST', signal: senal }
  );
}

/** Estado de la rama de contacto de un candidato: lo que el backend envió y recibió por su cuenta. */
export interface EstadoSolicitudApi {
  /** false: el candidato todavía no pasó por la pausa (no hay solicitud). */
  existe: boolean;
  estado?: string;
  correo?: string;
  telefono?: string;
  envio_correo?: 'enviada' | 'fallida' | 'pendiente';
  envio_whatsapp?: 'enviada' | 'fallida' | 'pendiente';
  completada: boolean;
  /** La cédula que declaró el candidato, en claro. */
  cedula?: string;
  edad?: number;
}

/** Solo lee: el envío y la recepción del formulario ocurren en segundo plano en el backend. */
export function obtenerEstadoSolicitud(postulacionId: string, senal?: AbortSignal): Promise<EstadoSolicitudApi> {
  return pedirJson<EstadoSolicitudApi>(`/api/postulaciones/${encodeURIComponent(postulacionId)}/solicitud`, { signal: senal });
}

export interface OpcionesAnalisisApi {
  /** 0 = todos los postulados. */
  limite?: number;
  forzar?: boolean;
  completo?: boolean;
}

export function urlAnalisis(idVacante: string, opciones: OpcionesAnalisisApi): string {
  const q = new URLSearchParams();
  if (opciones.limite !== undefined) q.set('limite', String(opciones.limite));
  if (opciones.forzar) q.set('forzar', '1');
  if (opciones.completo) q.set('completo', '1');
  const consulta = q.toString();
  return `${BASE_API}/api/computrabajo/vacantes/${encodeURIComponent(idVacante)}/analizar${consulta ? `?${consulta}` : ''}`;
}

/** Eventos (en vivo y repetidos desde el inicio) de un análisis que ya existe; no crea uno nuevo. */
export function urlEventosJob(idJob: string): string {
  return `${BASE_API}/api/jobs/${encodeURIComponent(idJob)}/eventos`;
}

/** Descarga un .zip con los aptos de esas vacantes: aptos.csv (nombre, cédula, teléfono, correo) y sus hojas de vida. */
export function urlExportarAptos(idsVacantes: string[]): string {
  return `${BASE_API}/api/aptos/exportar?ids=${idsVacantes.map(encodeURIComponent).join(',')}`;
}

/** Descarga un .zip con la información de una persona: datos.csv (cédula, teléfono, correo, resultado) y sus hojas de vida. */
export function urlExportarCandidato(idsVacantes: string[], urlFicha: string): string {
  return `${BASE_API}/api/candidatos/exportar?ids=${idsVacantes.map(encodeURIComponent).join(',')}&ficha=${encodeURIComponent(urlFicha)}`;
}

const descargasEnCurso = new Set<string>();

/**
 * Descarga un .zip desde la API. Si el servidor responde con un error (o no es un zip) avisa con el
 * motivo en vez de guardar un archivo .json suelto, que es lo que haría un enlace directo.
 */
export async function descargarZip(url: string, nombreRespaldo: string): Promise<void> {
  if (descargasEnCurso.has(url)) return;
  descargasEnCurso.add(url);
  try {
    const respuesta = await fetch(url);
    if (!respuesta.ok || !(respuesta.headers.get('Content-Type') ?? '').includes('zip')) {
      let mensaje = `No se pudo descargar (error ${respuesta.status})`;
      try {
        const cuerpo = (await respuesta.json()) as { error?: string };
        if (cuerpo.error) mensaje = cuerpo.error === 'ruta no encontrada' ? 'El servidor no tiene esta descarga: reinícialo para que cargue la versión nueva' : cuerpo.error;
      } catch {
        /* el cuerpo no era JSON: se queda el mensaje genérico */
      }
      throw new Error(mensaje);
    }
    const disposicion = respuesta.headers.get('Content-Disposition') ?? '';
    const nombre = decodeURIComponent(
      /filename\*=UTF-8''([^;]+)/i.exec(disposicion)?.[1] ?? /filename="?([^";]+)"?/i.exec(disposicion)?.[1] ?? nombreRespaldo
    );
    const enlace = document.createElement('a');
    enlace.href = URL.createObjectURL(await respuesta.blob());
    enlace.download = nombre;
    document.body.appendChild(enlace);
    enlace.click();
    enlace.remove();
    setTimeout(() => URL.revokeObjectURL(enlace.href), 10_000);
  } catch (error) {
    toast.error(error instanceof Error ? error.message : 'No se pudo descargar');
  } finally {
    descargasEnCurso.delete(url);
  }
}

/** Avisos del backend cuando el flujo de datos faltantes avanza (el candidato envió el formulario, terminó la reanudación). */
export function urlCambios(): string {
  return `${BASE_API}/api/cambios`;
}

/** URL con la que el navegador pide una captura guardada (el servidor solo entrega imágenes de su carpeta de descargas). */
export function urlCaptura(ruta: string): string {
  return `${BASE_API}/api/captura?ruta=${encodeURIComponent(ruta)}`;
}

export async function cancelarJob(idJob: string): Promise<void> {
  await pedirJson(`/api/jobs/${encodeURIComponent(idJob)}`, { method: 'DELETE' });
}

/**
 * Antes de abrir el stream se consulta la misma URL con fetch no es posible (sería un segundo análisis),
 * así que los errores previos al stream (409 otra vacante en curso, 404, 502…) llegan como un
 * error genérico de EventSource. Esta sonda ligera pregunta qué job está activo para explicarlo.
 */
export async function jobActivo(): Promise<{ id: string; oferta_id: string; titulo: string } | null> {
  const jobs = await pedirJson<{ id: string; oferta_id: string; titulo: string; estado: string }[]>('/api/jobs');
  return jobs.find((j) => j.estado === 'en_curso') ?? null;
}

export function estadoJob(idJob: string): Promise<{ estado: string }> {
  return pedirJson<{ estado: string }>(`/api/jobs/${encodeURIComponent(idJob)}`);
}

// ---- Página pública del candidato (datos faltantes) ----
// Se entra con un token de un solo uso que llega por correo o WhatsApp. No hay sesión: el token es la
// única credencial, así que no se envía nada más en la URL.

export interface SolicitudDatosPublica {
  /** pendiente: se puede completar; completada: ya entregó sus datos; vencida/invalida: el enlace no sirve. */
  estado: 'pendiente' | 'completada' | 'vencida' | 'invalida';
  nombre?: string;
  /** Enmascarado por el servidor ("da***@gmail.com"). */
  correo?: string;
  cargo?: string;
  vence_en?: string;
  politica_version?: string;
  edad_minima?: number;
  error?: string;
}

export interface DatosCandidatoEnvio {
  cedula: string;
  /** AAAA-MM-DD */
  fecha_nacimiento: string;
  autoriza: boolean;
  politica_version: string;
}

export interface RespuestaEnvioDatos {
  ok?: boolean;
  estado?: string;
  error?: string;
  /** Errores por campo (cedula, fecha_nacimiento, autoriza), cuando el servidor rechaza los datos. */
  campos?: Record<string, string>;
}

/**
 * A diferencia de pedirJson, aquí el cuerpo de un 404/410/422 importa (dice si el enlace venció, ya
 * se usó o qué campo está mal), así que se devuelve junto con el código en vez de lanzar un error.
 * Solo una falla de red se lanza.
 */
async function pedirPublico<T>(ruta: string, init?: RequestInit): Promise<{ codigo: number; cuerpo: T }> {
  let respuesta: Response;
  try {
    respuesta = await fetch(`${BASE_API}${ruta}`, init);
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ErrorApi('No hay conexión con el servidor', 0);
  }
  let cuerpo: T;
  try {
    cuerpo = (await respuesta.json()) as T;
  } catch {
    cuerpo = {} as T;
  }
  return { codigo: respuesta.status, cuerpo };
}

export function consultarSolicitudDatos(token: string, senal?: AbortSignal) {
  return pedirPublico<SolicitudDatosPublica>(`/api/validar-datos/${encodeURIComponent(token)}`, { signal: senal });
}

export function enviarDatosSolicitud(token: string, datos: DatosCandidatoEnvio, senal?: AbortSignal) {
  return pedirPublico<RespuestaEnvioDatos>(`/api/validar-datos/${encodeURIComponent(token)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(datos),
    signal: senal
  });
}
