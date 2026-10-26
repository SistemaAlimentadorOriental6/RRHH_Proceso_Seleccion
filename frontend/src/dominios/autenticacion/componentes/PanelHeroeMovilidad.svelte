<script lang="ts">
  interface Propiedades {
    urlImagen?: string;
    citaPrincipal?: string;
    lemaInferior?: string;
  }

  let {
    urlImagen = '/SAO6WALLPAPER.jpg',
    citaPrincipal = 'Movilizamos a las personas de la zona oriental y nororiental de Medellín, de manera cercana, segura y confiable.',
    lemaInferior = '¡MEDELLÍN CONDUCE EL CAMBIO!'
  }: Propiedades = $props();
</script>

<aside class="columna-heroe" aria-label="Mensaje institucional de movilidad">
  <div
    class="tarjeta-imagen"
    style="background-image: url('{urlImagen}');"
  >
    <div class="capa-degradado" aria-hidden="true"></div>

    <div class="contenido-superpuesto">
      <div class="icono-comillas" aria-hidden="true">“</div>

      <p class="texto-cita">
        {citaPrincipal}
      </p>

      <div class="pie-lema">
        <span class="linea-acento" aria-hidden="true"></span>
        <span class="texto-lema">{lemaInferior}</span>
      </div>
    </div>
  </div>
</aside>

<style>
  .columna-heroe {
    display: none;
    width: 50%;
    height: 100vh;
    padding: 20px 12px 20px 20px;
  }

  @media (min-width: 1024px) {
    .columna-heroe {
      display: flex;
    }
  }

  .tarjeta-imagen {
    position: relative;
    width: 100%;
    height: 100%;
    border-radius: 24px;
    overflow: hidden;
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    padding: 44px 42px;
    box-shadow:
      0 25px 50px -12px rgba(0, 0, 0, 0.22),
      0 12px 24px -8px rgba(0, 0, 0, 0.14);
  }

  .capa-degradado {
    position: absolute;
    inset: 0;
    background: linear-gradient(
      to top,
      rgba(0, 0, 0, 0.82) 0%,
      rgba(0, 0, 0, 0.42) 42%,
      rgba(0, 0, 0, 0.04) 100%
    );
    z-index: 1;
  }

  .contenido-superpuesto {
    position: relative;
    z-index: 2;
    max-width: 520px;
    color: #ffffff;
  }

  .icono-comillas {
    font-family: Georgia, 'Times New Roman', serif;
    font-size: 72px;
    font-weight: 700;
    line-height: 0.65;
    color: rgba(255, 255, 255, 0.55);
    margin-bottom: 10px;
    user-select: none;
  }

  .texto-cita {
    font-size: 27px;
    font-weight: 700;
    line-height: 1.32;
    letter-spacing: -0.015em;
    color: #ffffff;
    text-shadow: 0 2px 12px rgba(0, 0, 0, 0.25);
  }

  .pie-lema {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-top: 26px;
  }

  .linea-acento {
    width: 36px;
    height: 3.5px;
    border-radius: 999px;
    background-color: var(--color-primario);
    flex-shrink: 0;
  }

  .texto-lema {
    font-size: 11.5px;
    font-weight: 700;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: #ffffff;
  }
</style>
