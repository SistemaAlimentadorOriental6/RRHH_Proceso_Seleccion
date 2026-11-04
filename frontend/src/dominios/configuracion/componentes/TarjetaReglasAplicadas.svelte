<script lang="ts">
  import { GENEROS_DISPONIBLES, formatearDuracion, type ParametrosAceptacion } from "../modelos/parametrosAceptacion";
  import { parametrosDeVersion } from "../servicios/almacenParametros.svelte";

  interface Propiedades {
    /** Versión de reglas con la que se evaluó a la persona; sin ella aún no tiene dictamen. */
    version: number | undefined;
  }

  let { version }: Propiedades = $props();

  let reglas = $state<ParametrosAceptacion | null>(null);
  let error = $state("");

  $effect(() => {
    const v = version;
    reglas = null;
    error = "";
    if (!v) return;
    parametrosDeVersion(v)
      .then((r) => { if (v === version) reglas = r; })
      .catch((e) => { if (v === version) error = e instanceof Error ? e.message : "No se pudieron leer las reglas."; });
  });

  const etiquetaGenero = (id: string) => GENEROS_DISPONIBLES.find((g) => g.id === id)?.etiqueta ?? id;
</script>

<section class="tarjeta">
  <div class="cabecera">
    <h3>Reglas aplicadas</h3>
    {#if version}<span class="chip-version">Versión v{version}</span>{/if}
  </div>
  {#if !version}
    <p class="aviso">Esta persona aún no tiene dictamen: se evaluará con las reglas vigentes cuando termine su análisis.</p>
  {:else if error}
    <p class="aviso aviso-error" role="alert">{error}</p>
  {:else if !reglas}
    <p class="aviso" aria-busy="true">Leyendo las reglas de la versión {version}…</p>
  {:else}
    <dl class="datos">
      <div><dt>Edad</dt><dd>{reglas.edadMinima} a {reglas.edadMaxima} años</dd></div>
      <div><dt>Género</dt><dd>{reglas.generosAceptados.map(etiquetaGenero).join(" y ")}</dd></div>
      <div><dt>Licencias</dt><dd>{reglas.licenciasAceptadas.join(" · ")}</dd></div>
      <div><dt>Tiempo en el RUNT</dt><dd>{formatearDuracion(reglas.runtAntiguedadMeses)} mínimo</dd></div>
      <div><dt>Experiencia mínima</dt><dd>{formatearDuracion(reglas.experienciaMinimaMeses)}</dd></div>
      <div><dt>Reintegros</dt><dd>{reglas.aceptaReintegros ? "Se aceptan" : "No se aceptan"}</dd></div>
      <div class="ancho"><dt>EPS aceptadas</dt><dd>{reglas.epsAceptadas.join(" · ")}</dd></div>
    </dl>
  {/if}
</section>

<style>
  .tarjeta {
    background: #ffffff;
    border-radius: 22px;
    padding: 20px 24px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .cabecera {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  h3 {
    font-size: 14px;
    font-weight: 700;
  }

  .chip-version {
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.14);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
  }

  .aviso {
    padding: 12px 14px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    font-size: 12.5px;
    line-height: 1.5;
    font-weight: 500;
  }

  .aviso-error {
    background: rgba(239, 68, 68, 0.08);
    color: #b42323;
  }

  .datos {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 10px 16px;
    margin: 0;
  }

  .datos div {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .datos .ancho {
    grid-column: 1 / -1;
  }

  dt {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-texto-grey);
  }

  dd {
    margin: 0;
    font-size: 12.5px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
</style>
