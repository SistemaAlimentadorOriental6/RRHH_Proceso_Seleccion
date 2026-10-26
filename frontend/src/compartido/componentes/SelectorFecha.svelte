<script lang="ts">
  import { HugeiconsIcon } from '@hugeicons/svelte';
  import { ArrowLeft01Icon, ArrowRight01Icon, Calendar03Icon } from '@hugeicons/core-free-icons';

  type IconoHuge = typeof ArrowLeft01Icon;

  interface Propiedades {
    id: string;
    etiqueta: string;
    /** Fecha en formato AAAA-MM-DD ('' si no hay). */
    valor?: string;
    icono?: IconoHuge;
    error?: string;
    /** Última fecha elegible (AAAA-MM-DD); por defecto hoy. */
    maximo?: string;
  }

  let { id, etiqueta, valor = $bindable(''), icono = undefined, error = '', maximo = '' }: Propiedades = $props();

  const MESES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre'];
  const DIAS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  const pad = (n: number) => String(n).padStart(2, '0');
  const iso = (a: number, m: number, d: number) => `${a}-${pad(m + 1)}-${pad(d)}`;

  const tope = $derived(maximo || new Date().toISOString().slice(0, 10));
  const inicial = $derived(valor ? new Date(`${valor}T00:00:00`) : new Date(`${tope}T00:00:00`));

  let abierto = $state(false);
  let vista = $state<'dias' | 'anios'>('dias');
  let anio = $state(new Date().getFullYear());
  let mes = $state(new Date().getMonth());
  let paginaAnios = $state(0); // primer año mostrado en la vista de años
  let raiz: HTMLDivElement | undefined = $state();

  let escrito = $state(valor ? valor.split('-').reverse().join('/') : '');

  // Al elegir en el calendario el texto se actualiza; mientras se escribe a medias no se toca.
  $effect(() => {
    if (valor) escrito = valor.split('-').reverse().join('/');
  });

  // Escritura manual: solo dígitos, con las barras puestas solas; valor queda vacío hasta que la fecha sea real.
  function escribir(e: Event & { currentTarget: HTMLInputElement }): void {
    const d = e.currentTarget.value.replace(/\D/g, '').slice(0, 8);
    escrito = [d.slice(0, 2), d.slice(2, 4), d.slice(4)].filter(Boolean).join('/');
    e.currentTarget.value = escrito;
    const [dd, mm, aaaa] = [+d.slice(0, 2), +d.slice(2, 4), +d.slice(4)];
    const f = new Date(aaaa, mm - 1, dd);
    const real = d.length === 8 && f.getFullYear() === aaaa && f.getMonth() === mm - 1 && f.getDate() === dd;
    valor = real && iso(aaaa, mm - 1, dd) <= tope ? iso(aaaa, mm - 1, dd) : '';
  }
  const huecos = $derived((new Date(anio, mes, 1).getDay() + 6) % 7); // semana desde el lunes
  const diasDelMes = $derived(new Date(anio, mes + 1, 0).getDate());

  function abrir(): void {
    if (!abierto) {
      anio = inicial.getFullYear();
      mes = inicial.getMonth();
      vista = 'dias';
      abierto = true;
    } else abierto = false;
  }

  function mover(delta: number): void {
    const f = new Date(anio, mes + delta, 1);
    anio = f.getFullYear();
    mes = f.getMonth();
  }

  function elegir(d: number): void {
    valor = iso(anio, mes, d);
    abierto = false;
  }

  function mostrarAnios(): void {
    paginaAnios = anio - 5;
    vista = 'anios';
  }
</script>

<svelte:window
  onpointerdown={(e) => { if (abierto && raiz && !raiz.contains(e.target as Node)) abierto = false; }}
  onkeydown={(e) => { if (e.key === 'Escape') abierto = false; }}
/>

<div class="grupo-campo" bind:this={raiz}>
  <label for={id} class="etiqueta-campo">{etiqueta}</label>
  <div class="contenedor-input input-premium" class:input-con-error={Boolean(error)}>
    {#if icono}<span class="icono-izquierdo" aria-hidden="true"><HugeiconsIcon icon={icono} size={20} strokeWidth={1.8} /></span>{/if}
    <input {id} class="entrada" type="text" inputmode="numeric" autocomplete="off" placeholder="dd/mm/aaaa" maxlength="10" value={escrito} oninput={escribir} />
    <button type="button" class="abrir" aria-label="Abrir calendario" aria-haspopup="dialog" aria-expanded={abierto} onclick={abrir}>
      <HugeiconsIcon icon={Calendar03Icon} size={18} strokeWidth={1.8} />
    </button>
  </div>

  {#if abierto}
    <div class="modal" role="dialog" aria-label="Elegir fecha">
      <div class="cabecera">
        <button type="button" class="nav" aria-label="Anterior" onclick={() => (vista === 'dias' ? mover(-1) : (paginaAnios -= 12))}><HugeiconsIcon icon={ArrowLeft01Icon} size={18} strokeWidth={2} /></button>
        <button type="button" class="titulo" onclick={() => (vista === 'dias' ? mostrarAnios() : (vista = 'dias'))}>
          {vista === 'dias' ? `${MESES[mes]} ${anio}` : `${paginaAnios} – ${paginaAnios + 11}`}
        </button>
        <button type="button" class="nav" aria-label="Siguiente" onclick={() => (vista === 'dias' ? mover(1) : (paginaAnios += 12))}><HugeiconsIcon icon={ArrowRight01Icon} size={18} strokeWidth={2} /></button>
      </div>

      {#if vista === 'dias'}
        <div class="rejilla">
          {#each DIAS as d}<span class="dia-semana">{d}</span>{/each}
          {#each Array(huecos) as _}<span></span>{/each}
          {#each Array(diasDelMes) as _, i}
            {@const f = iso(anio, mes, i + 1)}
            <button type="button" class="dia" class:elegido={f === valor} disabled={f > tope} onclick={() => elegir(i + 1)}>{i + 1}</button>
          {/each}
        </div>
      {:else}
        <div class="rejilla anios">
          {#each Array(12) as _, i}
            {@const a = paginaAnios + i}
            <button type="button" class="dia" class:elegido={a === anio} disabled={a > Number(tope.slice(0, 4))} onclick={() => { anio = a; vista = 'dias'; }}>{a}</button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
  {#if error}<span class="mensaje-error" role="alert">{error}</span>{/if}
</div>

<style>
  .grupo-campo { position: relative; display: flex; flex-direction: column; gap: 6px; width: 100%; }
  .etiqueta-campo { font-size: 10px; font-weight: 700; letter-spacing: 0.06em; color: var(--color-texto-grey); text-transform: uppercase; padding-left: 4px; }
  .contenedor-input { position: relative; display: flex; align-items: center; width: 100%; border-radius: 14px; cursor: pointer; text-align: left; }
  .entrada { width: 100%; padding: 14px 48px 14px 48px; font-size: 14px; font-weight: 500; color: var(--color-texto-dark); background: transparent; }
  .entrada::placeholder { color: var(--color-texto-grey); }
  .abrir { position: absolute; right: 8px; width: 34px; height: 34px; border-radius: 10px; display: flex; align-items: center; justify-content: center; color: var(--color-texto-grey); cursor: pointer; }
  .abrir:hover { background: #fff; color: var(--color-primario); }
  .icono-izquierdo { position: absolute; left: 16px; color: var(--color-texto-grey); display: flex; }
  .mensaje-error { font-size: 12px; color: #e5484d; padding-left: 4px; }
  .modal { position: absolute; top: calc(100% + 6px); left: 0; z-index: 50; width: 300px; max-width: 100%; background: #fff; border-radius: 20px; padding: 16px; box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.2); }
  .cabecera { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
  .nav { width: 32px; height: 32px; border-radius: 10px; display: flex; align-items: center; justify-content: center; color: var(--color-texto-dark); cursor: pointer; }
  .nav:hover, .titulo:hover { background: var(--color-fondo-soft); }
  .titulo { font-size: 14px; font-weight: 700; color: var(--color-texto-navy); padding: 6px 12px; border-radius: 10px; cursor: pointer; }
  .rejilla { display: grid; grid-template-columns: repeat(7, 1fr); gap: 2px; }
  .rejilla.anios { grid-template-columns: repeat(3, 1fr); gap: 6px; }
  .dia-semana { font-size: 11px; font-weight: 700; color: var(--color-texto-grey); text-align: center; padding: 6px 0; }
  .dia { height: 34px; border-radius: 10px; font-size: 13px; font-weight: 500; color: var(--color-texto-dark); cursor: pointer; }
  .dia:hover:not(:disabled) { background: var(--color-fondo-soft); }
  .dia.elegido { background: var(--color-primario); color: #fff; font-weight: 700; }
  .dia:disabled { color: #d3d7dc; cursor: not-allowed; }
</style>
