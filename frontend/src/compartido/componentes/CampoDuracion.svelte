<script lang="ts">
  interface Propiedades {
    id: string;
    etiqueta: string;
    /** Duración total en meses; null cuando el campo está vacío. */
    meses?: number | null;
    error?: string;
    conError?: boolean;
  }

  let { id, etiqueta, meses = $bindable(null), error = '', conError = false }: Propiedades = $props();

  const anios = $derived(meses === null ? null : Math.floor(meses / 12));
  const resto = $derived(meses === null ? null : meses % 12);

  // Vacío = null. Un valor no entero se deja pasar para que la validación lo marque.
  function leer(texto: string): number | null {
    return texto.trim() === '' ? null : Number(texto);
  }

  function cambiarAnios(texto: string): void {
    const a = leer(texto);
    meses = a === null ? null : a * 12 + (resto ?? 0);
  }

  function cambiarMeses(texto: string): void {
    const m = leer(texto);
    meses = anios === null && m === null ? null : (anios ?? 0) * 12 + (m ?? 0);
  }
</script>

<div class="campo-duracion" class:con-error={conError}>
  <span class="etiqueta" id="{id}-etiqueta">{etiqueta}</span>
  <div class="fila">
    <label class="parte">
      <input
        class="input-premium"
        type="number"
        min="0"
        max="60"
        step="1"
        inputmode="numeric"
        aria-labelledby="{id}-etiqueta"
        value={anios ?? ''}
        oninput={(e) => cambiarAnios(e.currentTarget.value)}
      />
      <span class="sufijo">años</span>
    </label>
    <label class="parte">
      <input
        class="input-premium"
        type="number"
        min="0"
        max="11"
        step="1"
        inputmode="numeric"
        aria-labelledby="{id}-etiqueta"
        value={resto ?? ''}
        oninput={(e) => cambiarMeses(e.currentTarget.value)}
      />
      <span class="sufijo">meses</span>
    </label>
  </div>
  {#if error}<span class="mensaje-error" role="alert">{error}</span>{/if}
</div>

<style>
  .campo-duracion {
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

  .fila {
    display: flex;
    gap: 10px;
  }

  .parte {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    min-width: 0;
  }

  input {
    width: 100%;
    min-width: 0;
    padding: 9px 14px;
    font-size: 12.5px;
    font-weight: 500;
    color: var(--color-texto-dark);
    background: #ffffff;
    border-radius: 14px;
  }

  .sufijo {
    font-size: 11px;
    font-weight: 600;
    color: var(--color-texto-grey);
  }

  .con-error input {
    border-color: rgba(239, 68, 68, 0.5);
  }

  .mensaje-error {
    font-size: 12px;
    font-weight: 500;
    color: #ef4444;
    padding: 0 4px;
  }
</style>
