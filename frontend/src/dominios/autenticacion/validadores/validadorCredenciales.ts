import type { CredencialesAcceso } from '../modelos/credenciales';

export interface ResultadoValidacionCredenciales {
  esValido: boolean;
  errores: {
    correoElectronico?: string;
    contrasena?: string;
  };
}

const PATRON_CORREO = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * Valida las credenciales de acceso antes de enviarlas al servicio de autenticación (SRP).
 */
export function validarCredencialesAcceso(
  credenciales: CredencialesAcceso
): ResultadoValidacionCredenciales {
  const errores: ResultadoValidacionCredenciales['errores'] = {};
  const correoLimpio = credenciales.correoElectronico.trim();

  if (!correoLimpio) {
    errores.correoElectronico = 'Ingresa tu correo electrónico para continuar.';
  } else if (!PATRON_CORREO.test(correoLimpio)) {
    errores.correoElectronico = 'El formato del correo electrónico no es válido.';
  }

  if (!credenciales.contrasena || credenciales.contrasena.trim().length === 0) {
    errores.contrasena = 'Ingresa tu contraseña para continuar.';
  } else if (credenciales.contrasena.length < 4) {
    errores.contrasena = 'La contraseña debe tener al menos 4 caracteres.';
  }

  return {
    esValido: Object.keys(errores).length === 0,
    errores
  };
}
