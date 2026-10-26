<script lang="ts">
  import IconoMarcaDSI from '../../../compartido/componentes/IconoMarcaDSI.svelte';
  import PanelHeroeMovilidad from '../componentes/PanelHeroeMovilidad.svelte';
  import FormularioInicioSesion from '../componentes/FormularioInicioSesion.svelte';
  import type { UsuarioAutenticado } from '../modelos/credenciales';

  interface Propiedades {
    alAutenticarExitosamente?: (usuario: UsuarioAutenticado) => void;
  }

  let { alAutenticarExitosamente }: Propiedades = $props();
</script>

<main class="disposicion-login">
  <!-- Columna Izquierda: Fotografía del Bus Eléctrico SA06 y Cita -->
  <PanelHeroeMovilidad />

  <!-- Columna Derecha: Formulario de Login -->
  <section class="columna-acceso" aria-labelledby="titulo-bienvenida">
    <div class="decoracion-superior" aria-hidden="true"></div>

    <div class="espaciador-superior" aria-hidden="true"></div>

    <div class="contenedor-acceso">
      <header class="encabezado-acceso">
        <div class="marca-espaciado">
          <IconoMarcaDSI nombreMarca="Proceso de Selección y Contratación" />
        </div>

        <h1 id="titulo-bienvenida" class="titulo-bienvenida">
          <span>Bienvenido de nuevo</span>
          <span class="emoji-saludo" aria-hidden="true">👋</span>
        </h1>

        <p class="subtitulo-acceso">
          Ingresa tus credenciales para acceder al Proceso de Selección y Contratación
        </p>
      </header>

      <FormularioInicioSesion {alAutenticarExitosamente} />
    </div>

    <footer class="pie-copyright">
      <p>© 2026 Proceso de Selección y Contratación • SAO6</p>
    </footer>
  </section>
</main>

<style>
  .disposicion-login {
    min-height: 100vh;
    width: 100%;
    display: flex;
    background-color: #ffffff;
  }

  .columna-acceso {
    position: relative;
    width: 100%;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    align-items: center;
    padding: 32px 24px;
    background-color: #ffffff;
    overflow: hidden;
  }

  @media (min-width: 1024px) {
    .columna-acceso {
      width: 50%;
      padding: 48px 64px;
    }
  }

  .decoracion-superior {
    position: absolute;
    top: 0;
    right: 0;
    width: 256px;
    height: 256px;
    background-color: var(--color-fondo-soft);
    border-bottom-left-radius: 9999px;
    opacity: 0.55;
    pointer-events: none;
    z-index: 0;
  }

  .espaciador-superior {
    height: 16px;
  }

  .contenedor-acceso {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: 400px;
    display: flex;
    flex-direction: column;
    margin: auto 0;
    padding: 24px 0;
  }

  .encabezado-acceso {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    margin-bottom: 32px;
  }

  .marca-espaciado {
    margin-bottom: 32px;
  }

  .titulo-bienvenida {
    font-size: 30px;
    font-weight: 800;
    color: var(--color-texto-dark);
    letter-spacing: -0.025em;
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
  }

  .emoji-saludo {
    font-size: 28px;
    line-height: 1;
    display: inline-block;
    transform-origin: 70% 70%;
    animation: saludarMano 2.4s ease-in-out infinite;
  }

  @keyframes saludarMano {
    0%,
    60%,
    100% {
      transform: rotate(0deg);
    }
    15% {
      transform: rotate(14deg);
    }
    30% {
      transform: rotate(-8deg);
    }
    45% {
      transform: rotate(12deg);
    }
  }

  .subtitulo-acceso {
    font-size: 14px;
    font-weight: 400;
    color: var(--color-texto-grey);
    line-height: 1.6;
  }

  .pie-copyright {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: 400px;
    margin-top: 24px;
    font-size: 11px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }
</style>
