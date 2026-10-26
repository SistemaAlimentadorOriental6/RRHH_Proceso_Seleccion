<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Mail02Icon,
    UserIcon,
    IdIcon,
    Calendar03Icon,
    Briefcase02Icon
  } from '@hugeicons/core-free-icons';
  import CampoEntrada from '../../../compartido/componentes/CampoEntrada.svelte';
  import SelectorFecha from '../../../compartido/componentes/SelectorFecha.svelte';
  import BotonPrimario from '../../../compartido/componentes/BotonPrimario.svelte';
  import { consultarSolicitudDatos, enviarDatosSolicitud } from '../servicios/clienteRrhh';

  // Ruta pública que llega por correo o WhatsApp: /validar-datos/:token
  // El token es la única credencial y no lleva datos personales. Lo que ya sabemos del candidato
  // (nombre, correo, cargo) se pide al servidor con ese token: la URL no se puede modificar para
  // cambiar a quién pertenecen los datos.
  const token = decodeURIComponent(window.location.pathname.split('/').filter(Boolean)[1] ?? '');

  const POLITICA_DATOS = 'https://sao6.com.co/pdf/politica-de-tratamiento-y-proteccion-de-datos-personales.pdf';

  type Fase = 'cargando' | 'listo' | 'completada' | 'vencida' | 'invalida' | 'sin_conexion';
  let fase = $state<Fase>('cargando');
  let mensajeFase = $state('');

  // Lo que viene del servidor (siempre de solo lectura).
  let nombre = $state('');
  let correo = $state('');
  let cargo = $state('');
  let edadMinima = $state(18);
  let politicaVersion = $state('');

  // Lo que escribe el candidato.
  let cedula = $state('');
  let fechaNacimiento = $state('');
  let autoriza = $state(false);
  let errores = $state<Record<string, string>>({});
  let errorGeneral = $state('');
  let enviando = $state(false);

  /** Última fecha de nacimiento posible: hoy menos la edad mínima. El calendario no deja elegir más allá. */
  const maximoNacimiento = $derived.by(() => {
    const hoy = new Date();
    hoy.setFullYear(hoy.getFullYear() - edadMinima);
    return hoy.toISOString().slice(0, 10);
  });

  const primerNombre = $derived(nombre.trim().split(/\s+/)[0] ?? '');

  async function cargar(senal?: AbortSignal): Promise<void> {
    fase = 'cargando';
    if (!token) {
      fase = 'invalida';
      mensajeFase = 'Este enlace no es válido.';
      return;
    }
    try {
      const { cuerpo } = await consultarSolicitudDatos(token, senal);
      mensajeFase = cuerpo.error ?? '';
      switch (cuerpo.estado) {
        case 'pendiente':
          nombre = cuerpo.nombre ?? '';
          correo = cuerpo.correo ?? '';
          cargo = cuerpo.cargo ?? '';
          edadMinima = cuerpo.edad_minima ?? 18;
          politicaVersion = cuerpo.politica_version ?? '';
          fase = 'listo';
          break;
        case 'completada':
          nombre = cuerpo.nombre ?? '';
          fase = 'completada';
          break;
        case 'vencida':
          fase = 'vencida';
          break;
        default:
          fase = 'invalida';
          mensajeFase = mensajeFase || 'No pudimos cargar tus datos. Inténtalo de nuevo en unos minutos.';
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      fase = 'sin_conexion';
    }
  }

  onMount(() => {
    const control = new AbortController();
    void cargar(control.signal);
    return () => control.abort();
  });

  function edadEn(nacimientoIso: string): number {
    const hoy = new Date();
    const n = new Date(`${nacimientoIso}T00:00:00`);
    let edad = hoy.getFullYear() - n.getFullYear();
    if (hoy.getMonth() < n.getMonth() || (hoy.getMonth() === n.getMonth() && hoy.getDate() < n.getDate())) edad--;
    return edad;
  }

  /** Mismas reglas que el servidor, para avisar antes de enviar; el servidor siempre vuelve a validar. */
  function validar(): Record<string, string> {
    const e: Record<string, string> = {};
    if (!/^\d{6,12}$/.test(cedula.replace(/[.\s,]/g, ''))) e.cedula = 'Ingresa solo los números de tu cédula (entre 6 y 12 dígitos)';
    if (!/^\d{4}-\d{2}-\d{2}$/.test(fechaNacimiento) || fechaNacimiento >= new Date().toISOString().slice(0, 10)) {
      e.fechaNacimiento = 'Ingresa una fecha de nacimiento válida';
    } else if (edadEn(fechaNacimiento) < edadMinima) {
      e.fechaNacimiento = `Debes tener al menos ${edadMinima} años para continuar con el proceso`;
    }
    if (!autoriza) e.autoriza = 'Debes autorizar el tratamiento de tus datos personales para continuar';
    return e;
  }

  async function enviar(evento: SubmitEvent): Promise<void> {
    evento.preventDefault();
    errorGeneral = '';
    errores = validar();
    if (Object.keys(errores).length > 0) return;

    enviando = true;
    try {
      const { codigo, cuerpo } = await enviarDatosSolicitud(token, {
        cedula,
        fecha_nacimiento: fechaNacimiento,
        autoriza,
        politica_version: politicaVersion
      });
      if (cuerpo.ok) {
        fase = 'completada';
      } else if (codigo === 422 && cuerpo.campos) {
        // El servidor es quien decide: sus mensajes se muestran junto al campo.
        errores = {
          ...(cuerpo.campos.cedula ? { cedula: cuerpo.campos.cedula } : {}),
          ...(cuerpo.campos.fecha_nacimiento ? { fechaNacimiento: cuerpo.campos.fecha_nacimiento } : {}),
          ...(cuerpo.campos.autoriza ? { autoriza: cuerpo.campos.autoriza } : {})
        };
        errorGeneral = cuerpo.error ?? '';
      } else if (cuerpo.estado === 'vencida') {
        fase = 'vencida';
        mensajeFase = cuerpo.error ?? '';
      } else if (cuerpo.estado === 'invalida') {
        fase = 'invalida';
        mensajeFase = cuerpo.error ?? '';
      } else {
        errorGeneral = cuerpo.error ?? 'No pudimos guardar tus datos. Inténtalo de nuevo en unos minutos.';
      }
    } catch {
      errorGeneral = 'No hay conexión con el servidor. Revisa tu internet e inténtalo de nuevo.';
    } finally {
      enviando = false;
    }
  }
</script>

<main class="pagina">
  <section class="tarjeta">
    <img class="logo" src="https://sao6.com.co/LOGOSAO6.svg" alt="SAO6" height="44" />

    {#if fase === 'cargando'}
      <p class="estado" role="status">Cargando tus datos…</p>
    {:else if fase === 'completada'}
      <div class="exito">
        <div class="sello">
          <span class="onda"></span><span class="onda onda2"></span>
          {#each Array(8) as _, i}<span class="punto" style="--ang: {i * 45}deg; --delay: {0.55 + (i % 3) * 0.05}s"></span>{/each}
          <svg class="circulo" viewBox="0 0 52 52" width="88" height="88" fill="none">
            <circle class="fondo" cx="26" cy="26" r="26" />
            <path class="palomita" d="M15 27l8 8 14-16" stroke="#fff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
        <h1 style="--i: 0">¡Gracias{primerNombre ? `, ${primerNombre}` : ''}!</h1>
        <p style="--i: 1">Recibimos tus datos correctamente. Continuaremos con tu proceso de selección y te contactaremos si necesitamos algo más.</p>
        {#if correo}<span class="aviso" style="--i: 2">Te escribiremos a <strong>{correo}</strong></span>{/if}
      </div>
    {:else if fase === 'vencida'}
      <div class="mensaje-estado">
        <h1>Este enlace venció</h1>
        <p>{mensajeFase || 'Pide a quien te contactó que te envíe uno nuevo.'}</p>
      </div>
    {:else if fase === 'invalida'}
      <div class="mensaje-estado">
        <h1>Enlace no válido</h1>
        <p>{mensajeFase || 'Este enlace no es válido.'}</p>
      </div>
    {:else if fase === 'sin_conexion'}
      <div class="mensaje-estado">
        <h1>No pudimos cargar tus datos</h1>
        <p>Revisa tu conexión a internet e inténtalo de nuevo.</p>
        <BotonPrimario tipo="button" alClick={() => void cargar()}>Reintentar</BotonPrimario>
      </div>
    {:else}
      <h1>Valida tus datos</h1>
      <p class="subtitulo">Confirma la información para poder seguir con tu proceso de selección.</p>

      <form onsubmit={enviar} novalidate>
        <CampoEntrada id="v-nombre" soloLectura etiqueta="Nombre completo" marcador="Tu nombre completo" icono={UserIcon} autocompletado="name" bind:valor={nombre} />
        <CampoEntrada id="v-correo" soloLectura etiqueta="Correo electrónico" tipo="email" marcador="tucorreo@ejemplo.com" icono={Mail02Icon} autocompletado="email" bind:valor={correo} />
        <CampoEntrada id="v-cedula" etiqueta="Cédula" marcador="Número de cédula" icono={IdIcon} bind:valor={cedula} error={errores.cedula} />
        <SelectorFecha id="v-fecha" etiqueta="Fecha de nacimiento" icono={Calendar03Icon} maximo={maximoNacimiento} bind:valor={fechaNacimiento} error={errores.fechaNacimiento} />
        <CampoEntrada id="v-cargo" soloLectura etiqueta="Cargo al que se postula" marcador="Cargo" icono={Briefcase02Icon} bind:valor={cargo} />

        <label class="autorizacion">
          <input type="checkbox" bind:checked={autoriza} />
          <span class="casilla" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="#fff" stroke-width="3.2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7.5" /></svg>
          </span>
          <span>Autorizo a Sistema Alimentador Oriental SAS (SAO6) el tratamiento de mis datos personales para fines del proceso de selección, conforme a la Ley 1581 de 2012 y a la <a href={POLITICA_DATOS} target="_blank" rel="noopener noreferrer">política de tratamiento y protección de datos personales</a>.</span>
        </label>
        {#if errores.autoriza}<p class="error">{errores.autoriza}</p>{/if}
        {#if errorGeneral}<p class="error" role="alert">{errorGeneral}</p>{/if}

        <BotonPrimario tipo="submit" cargando={enviando} textoCargando="Enviando...">Enviar datos</BotonPrimario>
      </form>
    {/if}
  </section>
</main>

<style>
  .pagina { min-height: 100vh; display: flex; justify-content: center; align-items: flex-start; padding: 40px 16px; background: #fff; }
  .tarjeta { width: 100%; max-width: 440px; display: flex; flex-direction: column; gap: 14px; }
  .logo { display: block; margin: 0 auto 8px; height: 44px; width: auto; }
  h1 { font-size: 22px; text-align: center; color: var(--color-texto-navy); }
  .subtitulo { text-align: center; font-size: 13px; color: var(--color-texto-grey); margin-bottom: 8px; }
  .estado { text-align: center; font-size: 14px; color: var(--color-texto-grey); padding: 24px 0; }
  .mensaje-estado { display: flex; flex-direction: column; gap: 14px; align-items: center; text-align: center; padding-top: 8px; }
  .mensaje-estado p { font-size: 14px; color: var(--color-texto-dark); line-height: 22px; max-width: 340px; }
  form { display: flex; flex-direction: column; gap: 18px; }
  .autorizacion { display: flex; gap: 10px; align-items: flex-start; font-size: 12px; line-height: 18px; color: var(--color-texto-dark); cursor: pointer; position: relative; }
  .autorizacion input { position: absolute; opacity: 0; width: 22px; height: 22px; margin: 0; cursor: pointer; }
  .casilla { width: 22px; height: 22px; border-radius: 8px; background: var(--color-fondo-soft); display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: var(--transicion-vuesax); }
  .casilla svg { opacity: 0; transform: scale(0.5); transition: var(--transicion-vuesax); }
  .autorizacion input:checked + .casilla { background: var(--color-primario); }
  .autorizacion input:checked + .casilla svg { opacity: 1; transform: scale(1); }
  .autorizacion a { color: var(--color-primario); font-weight: 600; text-decoration: none; }
  .autorizacion a:hover { text-decoration: underline; }
  /* Sin borde ni sombra al enfocar; los campos ya conocidos quedan bloqueados y sin cursor de edición. */
  :global(.pagina .input-premium:focus-within) { border-color: transparent; box-shadow: none; background: var(--color-fondo-soft); }
  :global(.pagina input[readonly]) { cursor: default; color: var(--color-texto-grey); }
  .error { font-size: 12px; color: #e5484d; margin-top: -8px; }
  .exito { display: flex; flex-direction: column; align-items: center; gap: 14px; text-align: center; padding-top: 8px; }
  .exito h1, .exito p, .aviso { opacity: 0; animation: subir 0.6s ease forwards; animation-delay: calc(0.7s + var(--i) * 0.15s); }
  .exito p { font-size: 14px; color: var(--color-texto-dark); line-height: 22px; max-width: 340px; }
  .aviso { font-size: 12px; color: #2e8b34; background: #e8f6e9; padding: 8px 16px; border-radius: 999px; word-break: break-all; }
  .sello { position: relative; width: 88px; height: 88px; margin-bottom: 8px; }
  .circulo { position: relative; z-index: 2; animation: pop 0.5s var(--curva-elastica) both; }
  .fondo { fill: var(--color-primario); }
  .palomita { stroke-dasharray: 40; stroke-dashoffset: 40; animation: trazar 0.45s ease 0.4s forwards; }
  .onda { position: absolute; inset: 0; border-radius: 50%; background: rgba(var(--vs-primary-raw), 0.25); animation: onda 1.8s ease-out 0.45s infinite; }
  .onda2 { animation-delay: 1.35s; }
  .punto { position: absolute; top: 50%; left: 50%; width: 8px; height: 8px; margin: -4px; border-radius: 50%; background: var(--color-primario); opacity: 0; animation: estallido 0.9s ease-out var(--delay) forwards; }
  .punto:nth-of-type(odd) { background: #a5e3a9; width: 6px; height: 6px; margin: -3px; }
  @keyframes pop { from { transform: scale(0); } to { transform: scale(1); } }
  @keyframes trazar { to { stroke-dashoffset: 0; } }
  @keyframes onda { from { transform: scale(1); opacity: 0.7; } to { transform: scale(1.9); opacity: 0; } }
  @keyframes estallido { 0% { opacity: 1; transform: rotate(var(--ang)) translateY(0); } 100% { opacity: 0; transform: rotate(var(--ang)) translateY(-70px); } }
  @keyframes subir { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: none; } }
  @media (prefers-reduced-motion: reduce) { .exito *, .exito { animation-duration: 0.01s !important; animation-delay: 0s !important; animation-iteration-count: 1 !important; } }
</style>
