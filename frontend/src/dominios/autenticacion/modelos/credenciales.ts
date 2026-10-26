export interface CredencialesAcceso {
  correoElectronico: string;
  contrasena: string;
}

/** La sesión vive en una cookie HttpOnly que pone el servidor: el frontend nunca ve el token. */
export interface UsuarioAutenticado {
  id: string;
  correoElectronico: string;
  nombreCompleto: string;
  cargo: string;
}

export interface ResultadoAutenticacion {
  exitoso: boolean;
  mensaje: string;
  usuario?: UsuarioAutenticado;
}
