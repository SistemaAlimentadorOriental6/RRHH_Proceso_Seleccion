import type {
  CredencialesAcceso,
  ResultadoAutenticacion,
  UsuarioAutenticado
} from '../modelos/credenciales';

/** En desarrollo Vite redirige /api al backend (puerto 8080); VITE_API_URL permite apuntar a otro host. */
const BASE_API: string = (import.meta.env.VITE_API_URL as string | undefined) ?? '';

/** Evento que se emite cuando el servidor responde 401 a una petición con la sesión ya abierta. */
export const EVENTO_SESION_EXPIRADA = 'rrhh:sesion-expirada';

/**
 * Contrato de servicio de autenticación (Principio de Inversión de Dependencias - DIP).
 */
export interface IServicioAutenticacion {
  iniciarSesion(credenciales: CredencialesAcceso): Promise<ResultadoAutenticacion>;
  /** El usuario de la sesión abierta (cookie), o null si no hay o venció. */
  obtenerSesion(): Promise<UsuarioAutenticado | null>;
  cerrarSesion(): Promise<void>;
}

interface UsuarioApi {
  id: string;
  correo: string;
  nombre: string;
  cargo: string;
}

function aUsuario(u: UsuarioApi): UsuarioAutenticado {
  return { id: u.id, correoElectronico: u.correo, nombreCompleto: u.nombre, cargo: u.cargo };
}

/** Implementación contra el backend Go (/api/auth/*), desacoplada de la capa de presentación. */
export class ServicioAutenticacion implements IServicioAutenticacion {
  async iniciarSesion(credenciales: CredencialesAcceso): Promise<ResultadoAutenticacion> {
    let respuesta: Response;
    try {
      respuesta = await fetch(`${BASE_API}/api/auth/login`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          correo: credenciales.correoElectronico.trim(),
          contrasena: credenciales.contrasena
        })
      });
    } catch {
      return { exitoso: false, mensaje: 'No hay conexión con el servidor RRHH (¿está corriendo en el puerto 8080?)' };
    }
    const cuerpo = (await respuesta.json().catch(() => ({}))) as Partial<UsuarioApi> & { error?: string };
    if (!respuesta.ok) {
      return { exitoso: false, mensaje: cuerpo.error ?? `El servidor respondió ${respuesta.status}` };
    }
    return { exitoso: true, mensaje: 'Sesión iniciada.', usuario: aUsuario(cuerpo as UsuarioApi) };
  }

  async obtenerSesion(): Promise<UsuarioAutenticado | null> {
    try {
      const respuesta = await fetch(`${BASE_API}/api/auth/yo`, { credentials: 'include' });
      if (!respuesta.ok) return null;
      return aUsuario((await respuesta.json()) as UsuarioApi);
    } catch {
      return null;
    }
  }

  async cerrarSesion(): Promise<void> {
    try {
      await fetch(`${BASE_API}/api/auth/logout`, { method: 'POST', credentials: 'include' });
    } catch {
      /* sin conexión: la cookie vence sola y el usuario ya salió de la pantalla */
    }
  }
}

export const servicioAutenticacionPorDefecto: IServicioAutenticacion =
  new ServicioAutenticacion();
