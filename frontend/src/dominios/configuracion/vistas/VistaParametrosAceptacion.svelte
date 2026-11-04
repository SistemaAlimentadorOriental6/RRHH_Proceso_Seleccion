<script lang="ts">
  import { onMount } from "svelte";
  import toast from "svelte-french-toast";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import { Settings02Icon, FloppyDiskIcon } from "@hugeicons/core-free-icons";
  import CasillaPersonalizada from "../../../compartido/componentes/CasillaPersonalizada.svelte";
  import CampoDuracion from "../../../compartido/componentes/CampoDuracion.svelte";
  import {
    GRUPOS_EPS,
    GENEROS_DISPONIBLES,
    LICENCIAS_DISPONIBLES,
    PARAMETROS_POR_DEFECTO,
    formatearDuracion,
    validarParametros,
    type Genero,
    type ParametrosAceptacion,
  } from "../modelos/parametrosAceptacion";
  import { almacenParametros, cargarParametros, guardarParametros } from "../servicios/almacenParametros.svelte";

  // Hasta que lleguen los de la base, el borrador muestra los valores por defecto (y no se puede guardar).
  let borrador = $state<ParametrosAceptacion>(structuredClone(PARAMETROS_POR_DEFECTO));

  async function cargar(): Promise<void> {
    await cargarParametros();
    if (almacenParametros.guardados) borrador = structuredClone($state.snapshot(almacenParametros.guardados));
    else toast.error(almacenParametros.error);
  }

  onMount(() => void cargar());

  let guardando = $state(false);
  // Los errores se muestran tras el primer intento de guardar; desde ahí se actualizan en vivo.
  let intentado = $state(false);

  const errores = $derived(validarParametros(borrador));
  const visibles = $derived(intentado ? errores : {});
  const hayCambios = $derived(
    almacenParametros.guardados !== null && JSON.stringify(borrador) !== JSON.stringify(almacenParametros.guardados),
  );

  // Las EPS guardadas que no están en el catálogo (p. ej. "Sura" de la versión 1) se muestran aparte para poder desmarcarlas.
  const catalogoEps = GRUPOS_EPS.flatMap((g) => g.eps);
  const gruposEps = $derived.by(() => {
    const otras = (almacenParametros.guardados?.epsAceptadas ?? []).filter((e) => !catalogoEps.includes(e));
    return otras.length ? [...GRUPOS_EPS, { titulo: "Guardadas fuera del catálogo", eps: otras }] : GRUPOS_EPS;
  });

  function alternar<T>(lista: T[], valor: T): T[] {
    return lista.includes(valor) ? lista.filter((v) => v !== valor) : [...lista, valor];
  }

  async function guardar(): Promise<void> {
    intentado = true;
    const mensajes = Object.values(errores);
    if (mensajes.length > 0) {
      toast.error(mensajes.length === 1 ? mensajes[0] : `Hay ${mensajes.length} campos por corregir.`);
      return;
    }
    guardando = true;
    try {
      borrador = structuredClone(await guardarParametros(borrador));
      intentado = false;
      toast.success(`Guardados como versión ${borrador.version}. Se aplican a las personas que se evalúen desde ahora.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "No se pudieron guardar los parámetros en la base de datos.");
    } finally {
      guardando = false;
    }
  }

  function restablecer(): void {
    borrador = structuredClone(PARAMETROS_POR_DEFECTO);
    intentado = false;
  }
</script>

<div class="contenedor-parametros">
  <section class="panel-superior">
    <div class="textos">
      <div class="fila-insignia">
        <span class="insignia-modulo">
          <HugeiconsIcon icon={Settings02Icon} size={13} strokeWidth={2.2} />
          <span>Configuración</span>
        </span>
        {#if almacenParametros.guardados?.version}
          <span class="chip" title="Cada persona queda con la versión con la que se evaluó">Versión vigente v{almacenParametros.guardados.version}</span>
        {/if}
        {#if hayCambios}
          <span class="chip chip-pendiente">Cambios sin guardar</span>
        {/if}
      </div>
      <h2 class="titulo-seccion">Parámetros de aceptación</h2>
      <p class="nota-version">Al guardar se crea una versión nueva: solo aplica a quien se evalúe después. Quien ya se evaluó conserva sus reglas, incluso si se vuelve a analizar.</p>
    </div>

    <div class="bloque-acciones">
      <div class="kpis">
        <div class="kpi"><span>Edad</span><strong>{borrador.edadMinima ?? "?"}–{borrador.edadMaxima ?? "?"} años</strong></div>
        <div class="kpi"><span>Experiencia</span><strong>{formatearDuracion(borrador.experienciaMinimaMeses)}</strong></div>
        <div class="kpi kpi-verde"><span>EPS</span><strong>{borrador.epsAceptadas.length}</strong></div>
      </div>
      <button type="button" class="boton-secundario" onclick={restablecer}>Restablecer</button>
      <button type="button" class="boton-premium boton-guardar" disabled={guardando || !hayCambios} onclick={guardar}>
        <HugeiconsIcon icon={FloppyDiskIcon} size={14} strokeWidth={2.2} />
        <span>{guardando ? "Guardando…" : "Guardar"}</span>
      </button>
    </div>
  </section>

  {#if almacenParametros.cargando}
    <div class="caja-aviso" aria-busy="true"><p>Leyendo los parámetros de la base de datos…</p></div>
  {:else if almacenParametros.error}
    <div class="caja-aviso caja-error" role="alert">
      <p>{almacenParametros.error}</p>
      <button type="button" class="boton-secundario" onclick={cargar}>Reintentar</button>
    </div>
  {/if}

  <div class="lista-parametros">
    <article class="tarjeta-parametro">
      <header class="cabecera-parametro">
        <h3>Edad, experiencia y RUNT</h3>
        <p>Rango de edad aceptado, años mínimos manejando y tiempo mínimo inscrito en el RUNT.</p>
      </header>
      <div class="bloque-suave fila-campos">
        <label>
          <span class="etiqueta">Edad inicio</span>
          <input class="input-premium campo" class:input-con-error={visibles.edadMinima} type="number" min="18" bind:value={borrador.edadMinima} />
          {#if visibles.edadMinima}<span class="mensaje-error" role="alert">{visibles.edadMinima}</span>{/if}
        </label>
        <label>
          <span class="etiqueta">Edad fin</span>
          <input class="input-premium campo" class:input-con-error={visibles.edadMaxima} type="number" min="18" bind:value={borrador.edadMaxima} />
          {#if visibles.edadMaxima}<span class="mensaje-error" role="alert">{visibles.edadMaxima}</span>{/if}
        </label>
        <CampoDuracion
          id="experiencia-minima"
          etiqueta="Experiencia mínima"
          bind:meses={borrador.experienciaMinimaMeses}
          conError={Boolean(visibles.experienciaMinimaMeses)}
          error={visibles.experienciaMinimaMeses}
        />
        <CampoDuracion
          id="runt-antiguedad"
          etiqueta="Tiempo inscrito en el RUNT"
          bind:meses={borrador.runtAntiguedadMeses}
          conError={Boolean(visibles.runtAntiguedadMeses)}
          error={visibles.runtAntiguedadMeses}
        />
      </div>
    </article>

    <article class="tarjeta-parametro">
      <header class="cabecera-parametro">
        <h3>Perfil del candidato</h3>
        <p>Género, categorías de licencia y política de reintegros.</p>
      </header>
      <div class="bloque-suave">
        <span class="etiqueta">Género</span>
        <div class="chips">
          {#each GENEROS_DISPONIBLES as g (g.id)}
            <button
              type="button"
              class="chip chip-opcion"
              class:chip-procesada={borrador.generosAceptados.includes(g.id)}
              aria-pressed={borrador.generosAceptados.includes(g.id)}
              onclick={() => (borrador.generosAceptados = alternar<Genero>(borrador.generosAceptados, g.id))}
            >{g.etiqueta}</button>
          {/each}
        </div>
        {#if visibles.generosAceptados}<span class="mensaje-error" role="alert">{visibles.generosAceptados}</span>{/if}
      </div>
      <div class="bloque-suave">
        <span class="etiqueta">Licencias</span>
        <div class="chips">
          {#each LICENCIAS_DISPONIBLES as l (l)}
            <button
              type="button"
              class="chip chip-opcion"
              class:chip-procesada={borrador.licenciasAceptadas.includes(l)}
              aria-pressed={borrador.licenciasAceptadas.includes(l)}
              onclick={() => (borrador.licenciasAceptadas = alternar(borrador.licenciasAceptadas, l))}
            >{l}</button>
          {/each}
        </div>
        {#if visibles.licenciasAceptadas}<span class="mensaje-error" role="alert">{visibles.licenciasAceptadas}</span>{/if}
      </div>
      <div class="bloque-suave">
        <CasillaPersonalizada bind:marcada={borrador.aceptaReintegros}>
          Aceptar reintegros (candidatos que ya trabajaron en la empresa)
        </CasillaPersonalizada>
      </div>
    </article>

    <article class="tarjeta-parametro">
      <header class="cabecera-parametro">
        <h3>EPS aceptadas</h3>
        <p>Solo se aceptan candidatos afiliados a las marcadas.</p>
      </header>
      {#if visibles.epsAceptadas}<span class="mensaje-error" role="alert">{visibles.epsAceptadas}</span>{/if}
      {#each gruposEps as grupo (grupo.titulo)}
        <div class="bloque-suave">
          <span class="etiqueta">{grupo.titulo}</span>
          <div class="grilla-eps">
            {#each grupo.eps as eps (eps)}
              <CasillaPersonalizada
                bind:marcada={
                  () => borrador.epsAceptadas.includes(eps),
                  () => (borrador.epsAceptadas = alternar(borrador.epsAceptadas, eps))
                }
              >{eps}</CasillaPersonalizada>
            {/each}
          </div>
        </div>
      {/each}
    </article>
  </div>
</div>

<style>
  .nota-version {
    margin-top: 6px;
    max-width: 560px;
    font-size: 12px;
    line-height: 1.45;
    color: var(--color-texto-grey);
  }

  .contenedor-parametros {
    padding: 26px 32px 42px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }

  .panel-superior {
    background: #ffffff;
    border-radius: 20px;
    padding: 14px 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 14px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .textos {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .fila-insignia {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .insignia-modulo {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 10px;
    border-radius: 999px;
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-size: 11px;
    font-weight: 700;
  }

  .titulo-seccion {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .bloque-acciones {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .kpis {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .kpi {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 12px;
    border-radius: 12px;
    background: var(--color-fondo-soft);
  }

  .kpi span {
    font-size: 10px;
    font-weight: 600;
    color: var(--color-texto-grey);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .kpi strong {
    font-size: 14px;
    font-weight: 700;
  }

  .kpi-verde {
    background: rgba(var(--vs-primary-raw), 0.12);
  }

  .kpi-verde strong {
    color: var(--color-primario);
  }

  .caja-aviso {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-radius: 18px;
    background: rgba(245, 158, 11, 0.1);
    color: #92590b;
    font-size: 13px;
    font-weight: 500;
  }

  .caja-aviso p {
    flex: 1;
  }

  .caja-error {
    background: rgba(239, 68, 68, 0.08);
    color: #b42323;
  }

  .mensaje-error {
    font-size: 12px;
    font-weight: 500;
    color: #ef4444;
    padding: 0 4px;
  }

  .lista-parametros {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .tarjeta-parametro {
    background: #ffffff;
    border-radius: 26px;
    padding: 22px 26px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    box-shadow: 0 12px 32px -12px rgba(44, 62, 80, 0.06);
  }

  .cabecera-parametro {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .cabecera-parametro h3 {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .cabecera-parametro p {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-grey);
  }

  .bloque-suave {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 18px;
    border-radius: 18px;
    background: var(--color-fondo-soft);
  }

  .fila-campos {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 14px;
  }

  .fila-campos label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1 1 180px;
  }

  .etiqueta {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-texto-grey);
    padding-left: 4px;
  }

  .campo {
    width: 100%;
    padding: 9px 14px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
    background: #ffffff;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 10px;
    border-radius: 999px;
    background: #ffffff;
    color: var(--color-texto-grey);
    font-size: 11px;
    font-weight: 600;
  }

  .chip-opcion {
    padding: 6px 14px;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .chip-procesada {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }

  .chip-pendiente {
    background: rgba(245, 158, 11, 0.12);
    color: #92590b;
  }

  .grilla-eps {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 14px;
  }

  .boton-guardar {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 18px;
    font-size: 12.5px;
    font-weight: 700;
  }

  .boton-secundario {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 9px 15px;
    border-radius: 14px;
    background: var(--color-fondo-soft);
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .boton-secundario:hover {
    background: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
  }
</style>
