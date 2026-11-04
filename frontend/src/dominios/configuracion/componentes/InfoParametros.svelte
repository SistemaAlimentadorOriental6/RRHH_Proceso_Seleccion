<script lang="ts">
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import { InformationCircleIcon } from "@hugeicons/core-free-icons";
  import { GENEROS_DISPONIBLES, formatearDuracion, type ParametrosAceptacion } from "../modelos/parametrosAceptacion";
  import { almacenParametros, cargarParametros, parametrosDeVersion } from "../servicios/almacenParametros.svelte";

  // Botón con un globo que muestra los parámetros vigentes (con los que se reanaliza) y qué cambió frente a
  // la versión anterior. Se abre al pasar el mouse, con el teclado o con un clic (en pantallas táctiles).
  let abierto = $state(false);
  let fijo = $state(false);
  let anterior = $state<ParametrosAceptacion | null>(null);
  let pedido = false;

  const vigentes = $derived(almacenParametros.guardados);

  async function cargar(): Promise<void> {
    if (pedido) return;
    pedido = true;
    if (!almacenParametros.guardados) await cargarParametros();
    const v = almacenParametros.guardados?.version ?? 0;
    if (v > 1) {
      try {
        anterior = await parametrosDeVersion(v - 1);
      } catch {
        anterior = null; // sin la anterior se muestran solo los vigentes
      }
    }
    if (almacenParametros.error) pedido = false; // se reintenta la próxima vez que se abra
  }

  function mostrar(): void {
    abierto = true;
    void cargar();
  }

  const etiquetaGenero = (id: string) => GENEROS_DISPONIBLES.find((g) => g.id === id)?.etiqueta ?? id;
  const lista = (l: string[]) => (l.length ? l.join(", ") : "—");

  const filas = $derived.by(() => {
    if (!vigentes) return [];
    const textos = (p: ParametrosAceptacion) => ({
      edad: `${p.edadMinima} a ${p.edadMaxima} años`,
      genero: p.generosAceptados.map(etiquetaGenero).join(" y "),
      licencias: lista(p.licenciasAceptadas),
      runt: `${formatearDuracion(p.runtAntiguedadMeses)} mínimo`,
      experiencia: `${formatearDuracion(p.experienciaMinimaMeses)} (aún no se aplica)`,
      reintegros: p.aceptaReintegros ? "Se aceptan" : "No se aceptan",
      eps: `${p.epsAceptadas.length} aceptadas: ${lista(p.epsAceptadas)}`,
    });
    const ahora = textos(vigentes);
    const antes = anterior ? textos(anterior) : null;
    const nombres: [keyof typeof ahora, string][] = [
      ["edad", "Edad"],
      ["genero", "Género"],
      ["licencias", "Licencias"],
      ["runt", "Tiempo en el RUNT"],
      ["reintegros", "Reintegros"],
      ["eps", "EPS"],
      ["experiencia", "Experiencia"],
    ];
    return nombres.map(([clave, nombre]) => ({
      nombre,
      valor: ahora[clave],
      antes: antes && antes[clave] !== ahora[clave] ? antes[clave] : null,
    }));
  });
  const cambios = $derived(filas.filter((f) => f.antes !== null).length);
</script>

<div class="info-parametros" role="presentation" onmouseenter={mostrar} onmouseleave={() => (abierto = fijo)}>
  <button
    type="button"
    class="boton-info"
    aria-label="Ver los parámetros con los que se reanaliza"
    aria-expanded={abierto}
    aria-controls="globo-parametros"
    onfocus={mostrar}
    onblur={() => (abierto = fijo)}
    onclick={() => {
      fijo = !fijo;
      abierto = fijo;
      if (fijo) void cargar();
    }}
  >
    <HugeiconsIcon icon={InformationCircleIcon} size={18} strokeWidth={2} />
  </button>

  {#if abierto}
    <div id="globo-parametros" class="globo" role="tooltip">
      {#if !vigentes}
        <p class="nota">{almacenParametros.error || "Leyendo los parámetros…"}</p>
      {:else}
        <div class="cabecera">
          <strong>Parámetros vigentes v{vigentes.version}</strong>
          {#if anterior}
            <span class="chip" class:chip-cambio={cambios > 0}>
              {cambios === 0 ? `Igual que la v${anterior.version}` : `${cambios} ${cambios === 1 ? "cambio" : "cambios"} frente a la v${anterior.version}`}
            </span>
          {/if}
        </div>
        <dl>
          {#each filas as f (f.nombre)}
            <div class:cambio={f.antes !== null}>
              <dt>{f.nombre}</dt>
              <dd>
                {f.valor}
                {#if f.antes !== null}<span class="antes">antes: {f.antes}</span>{/if}
              </dd>
            </div>
          {/each}
        </dl>
        <p class="nota">
          Se reanaliza con estas reglas solo a quien podría cambiar de resultado. Se editan en Configuración › Parámetros.
        </p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .info-parametros {
    position: relative;
    display: inline-flex;
  }

  .boton-info {
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-info:hover,
  .boton-info[aria-expanded="true"] {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .boton-info:focus-visible {
    outline: 2px solid var(--color-primario);
    outline-offset: 2px;
  }

  .globo {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 50;
    width: min(380px, calc(100vw - 32px));
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 16px;
    border-radius: 18px;
    background: #ffffff;
    box-shadow: 0 18px 40px -12px rgba(44, 62, 80, 0.28);
    text-align: left;
  }

  .cabecera {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 6px;
  }

  .cabecera strong {
    font-size: 13px;
    color: var(--color-texto-dark);
  }

  .chip {
    padding: 2px 9px;
    border-radius: 999px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-grey);
    font-size: 10.5px;
    font-weight: 700;
  }

  .chip-cambio {
    background: rgba(245, 158, 11, 0.14);
    color: #b9700a;
  }

  dl {
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  dl div {
    display: grid;
    grid-template-columns: 110px 1fr;
    gap: 8px;
    padding: 5px 8px;
    border-radius: 10px;
  }

  dl div.cambio {
    background: rgba(245, 158, 11, 0.1);
  }

  dt {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-texto-grey);
    padding-top: 2px;
  }

  dd {
    margin: 0;
    font-size: 12px;
    font-weight: 600;
    color: var(--color-texto-dark);
    overflow-wrap: anywhere;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .antes {
    font-size: 11px;
    font-weight: 500;
    color: #b9700a;
  }

  .nota {
    margin: 0;
    font-size: 11px;
    line-height: 1.45;
    color: var(--color-texto-grey);
  }
</style>
