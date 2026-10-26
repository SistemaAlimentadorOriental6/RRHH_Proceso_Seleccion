<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { EyeIcon, EyeOffIcon } from '@hugeicons/core-free-icons';

  type IconoHuge = typeof EyeIcon;

  interface Propiedades {
    id: string;
    etiqueta: string;
    valor?: string;
    tipo?: 'text' | 'email' | 'password' | 'date';
    marcador?: string;
    icono?: IconoHuge;
    error?: string;
    deshabilitado?: boolean;
    soloLectura?: boolean;
    autocompletado?: AutoFill;
  }

  let {
    id,
    etiqueta,
    valor = $bindable(''),
    tipo = 'text',
    marcador = '',
    icono = undefined,
    error = '',
    deshabilitado = false,
    soloLectura = false,
    autocompletado = 'off'
  }: Propiedades = $props();

  let mostrarContrasena = $state(false);

  const tipoEntradaActual = $derived(
    tipo === 'password' ? (mostrarContrasena ? 'text' : 'password') : tipo
  );

  function alternarVisibilidadContrasena(): void {
    mostrarContrasena = !mostrarContrasena;
  }
</script>

<div class="grupo-campo">
  {#if etiqueta}
    <label for={id} class="etiqueta-campo">
      {etiqueta}
    </label>
  {/if}

  <div
    class="contenedor-input input-premium"
    class:input-con-error={Boolean(error)}
  >
    {#if icono}
      <span class="icono-izquierdo" aria-hidden="true">
        <HugeiconsIcon icon={icono} size={20} strokeWidth={1.8} />
      </span>
    {/if}

    <input
      {id}
      bind:value={valor}
      type={tipoEntradaActual}
      placeholder={marcador}
      disabled={deshabilitado}
      readonly={soloLectura}
      autocomplete={autocompletado}
      class="elemento-input"
      class:con-icono-izquierdo={Boolean(icono)}
      class:con-icono-derecho={tipo === 'password'}
    />

    {#if tipo === 'password'}
      <button
        type="button"
        class="boton-visibilidad"
        onclick={alternarVisibilidadContrasena}
        aria-label={mostrarContrasena ? 'Ocultar contraseña' : 'Mostrar contraseña'}
        tabindex="-1"
      >
        <HugeiconsIcon
          icon={mostrarContrasena ? EyeOffIcon : EyeIcon}
          size={20}
          strokeWidth={1.8}
        />
      </button>
    {/if}
  </div>

  {#if error}
    <span class="mensaje-error" role="alert">{error}</span>
  {/if}
</div>

<style>
  .grupo-campo {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 100%;
  }

  .etiqueta-campo {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    padding-left: 4px;
  }

  .contenedor-input {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    border-radius: 14px;
  }

  .icono-izquierdo {
    position: absolute;
    left: 16px;
    color: var(--color-texto-grey);
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    z-index: 10;
    transition: var(--transicion-vuesax);
  }

  .contenedor-input:focus-within .icono-izquierdo {
    color: var(--color-primario);
  }

  .elemento-input {
    width: 100%;
    padding: 14px 16px;
    font-size: 14px;
    font-weight: 500;
    color: var(--color-texto-dark);
    background: transparent;
    z-index: 1;
  }

  .elemento-input::placeholder {
    color: var(--color-texto-grey);
    font-weight: 400;
    opacity: 0.85;
  }

  .elemento-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .elemento-input.con-icono-izquierdo {
    padding-left: 44px;
  }

  .elemento-input.con-icono-derecho {
    padding-right: 44px;
  }

  .boton-visibilidad {
    position: absolute;
    right: 16px;
    color: var(--color-texto-grey);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    z-index: 10;
    transition: var(--transicion-vuesax);
  }

  .boton-visibilidad:hover {
    color: var(--color-texto-dark);
  }

  .mensaje-error {
    font-size: 12px;
    font-weight: 500;
    color: #ef4444;
    padding: 0 4px;
    margin-top: 2px;
  }
</style>
