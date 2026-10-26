<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import {
    Mail02Icon,
    LockIcon,
    ArrowRight01Icon
  } from '@hugeicons/core-free-icons';
  import toast from 'svelte-french-toast';
  import CampoEntrada from '../../../compartido/componentes/CampoEntrada.svelte';
  import BotonPrimario from '../../../compartido/componentes/BotonPrimario.svelte';
  import type {
    CredencialesAcceso,
    UsuarioAutenticado
  } from '../modelos/credenciales';
  import { validarCredencialesAcceso } from '../validadores/validadorCredenciales';
  import {
    servicioAutenticacionPorDefecto,
    type IServicioAutenticacion
  } from '../servicios/servicioAutenticacion';

  interface Propiedades {
    servicio?: IServicioAutenticacion;
    alAutenticarExitosamente?: (usuario: UsuarioAutenticado) => void;
    alOcurrirError?: (mensaje: string) => void;
  }

  let {
    servicio = servicioAutenticacionPorDefecto,
    alAutenticarExitosamente,
    alOcurrirError
  }: Propiedades = $props();

  let correoElectronico = $state('');
  let contrasena = $state('');
  let estaCargando = $state(false);
  let errores = $state<{ correoElectronico?: string; contrasena?: string }>({});

  async function manejarEnvioFormulario(evento: SubmitEvent): Promise<void> {
    evento.preventDefault();

    const credenciales: CredencialesAcceso = {
      correoElectronico,
      contrasena
    };

    const validacion = validarCredencialesAcceso(credenciales);
    errores = validacion.errores;

    if (!validacion.esValido) {
      if (validacion.errores.correoElectronico) {
        toast.error(validacion.errores.correoElectronico);
      } else if (validacion.errores.contrasena) {
        toast.error(validacion.errores.contrasena);
      }
      alOcurrirError?.(
        validacion.errores.correoElectronico ||
          validacion.errores.contrasena ||
          'Error de validación'
      );
      return;
    }

    estaCargando = true;
    try {
      const resultado = await servicio.iniciarSesion(credenciales);

      if (resultado.exitoso && resultado.usuario) {
        toast.success('¡Bienvenido! Sesión iniciada correctamente.');
        alAutenticarExitosamente?.(resultado.usuario);
      } else {
        const mensaje = resultado.mensaje || 'Credenciales inválidas.';
        toast.error(mensaje);
        alOcurrirError?.(mensaje);
      }
    } finally {
      estaCargando = false;
    }
  }
</script>

<form class="formulario-login" onsubmit={manejarEnvioFormulario} novalidate>
  <CampoEntrada
    id="campo-correo"
    etiqueta="Correo Electrónico"
    tipo="email"
    marcador="Ingresa tu correo"
    icono={Mail02Icon}
    autocompletado="email"
    bind:valor={correoElectronico}
    error={errores.correoElectronico}
    deshabilitado={estaCargando}
  />

  <CampoEntrada
    id="campo-contrasena"
    etiqueta="Contraseña"
    tipo="password"
    marcador="Ingresa tu contraseña"
    icono={LockIcon}
    autocompletado="current-password"
    bind:valor={contrasena}
    error={errores.contrasena}
    deshabilitado={estaCargando}
  />

  <BotonPrimario
    tipo="submit"
    cargando={estaCargando}
    deshabilitado={estaCargando}
    textoCargando="Cargando..."
  >
    <span>Iniciar Sesión</span>
    <span class="icono-flecha">
      <HugeiconsIcon icon={ArrowRight01Icon} size={20} strokeWidth={2.2} />
    </span>
  </BotonPrimario>

  <div class="bloque-proteccion">
    <p class="texto-proteccion">
      Este sitio está protegido y se aplican la
      <a href="#privacidad" class="enlace-legal">Política de privacidad</a>
      y los
      <a href="#terminos" class="enlace-legal">Términos de servicio</a>
      de Google.
    </p>
  </div>
</form>

<style>
  .formulario-login {
    display: flex;
    flex-direction: column;
    gap: 24px;
    width: 100%;
    max-width: 400px;
    margin: 0 auto;
  }

  .icono-flecha {
    display: inline-flex;
    align-items: center;
    transition: transform 0.25s ease;
  }

  :global(.boton-premium:hover) .icono-flecha {
    transform: translateX(4px);
  }

  .bloque-proteccion {
    margin-top: 2px;
    padding-top: 18px;
    border-top: 1px solid rgba(160, 166, 174, 0.16);
    text-align: center;
  }

  .texto-proteccion {
    font-size: 11px;
    line-height: 1.65;
    color: var(--color-texto-grey);
    max-width: 330px;
    margin: 0 auto;
  }

  .enlace-legal {
    color: var(--color-primario);
    font-weight: 600;
    text-decoration: none;
    transition: var(--transicion-vuesax);
  }

  .enlace-legal:hover {
    text-decoration: underline;
  }
</style>
