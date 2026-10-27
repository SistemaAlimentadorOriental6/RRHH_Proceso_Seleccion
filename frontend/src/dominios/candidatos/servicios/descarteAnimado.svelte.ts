/**
 * Adi lleva las tarjetas: agarra la tarjeta de quien terminó, la comprime, la carga hasta su destino
 * y la suelta ahí. Hay dos destinos:
 *  - "descartar": la entrada "Descartados" de la barra lateral (la tarjeta se achica dentro de ella);
 *  - "analizar": el lugar de la tarjeta en la sección Analizados (la copia crece hasta encajar en él).
 * La tarjeta no se mueve sola: su copia sigue a Adi en cada cuadro (quien llama a `actualizar` es el
 * bucle de movimiento del avatar), así el recorrido siempre se completa y va a su ritmo.
 */

import { SvelteSet } from 'svelte/reactivity';

type Fase = 'agarrando' | 'llevando' | 'soltando' | 'listo';
export type ModoLlevar = 'descartar' | 'analizar';
export type ResultadoLlevar = 'iniciado' | 'ocupado' | 'sin-animacion';

interface Vuelo {
  el: HTMLElement;
  modo: ModoLlevar;
  /** Se pide en cada cuadro: el destino puede moverse (la grilla se reacomoda). */
  destinoEl: () => HTMLElement | null;
  alLlegar?: () => void;
  llego: boolean;
  fase: Fase;
  inicioFase: number;
  /** Centro y tamaño de la tarjeta al empezar, y la esquina donde Adi la agarra. */
  origen: { cx: number; cy: number; w: number; h: number };
  kirbyInicial: { cx: number; cy: number };
  /** Último punto donde iba sujeta (para soltarla desde ahí) y último destino conocido. */
  sujeta: { cx: number; cy: number };
  ultimoDestino: DOMRect | null;
  desplazado: boolean;
}

/**
 * Tarjetas que Adi ya llevó (o que ya no hay que llevar). Vive aquí y no en la pantalla: al cambiar de
 * pantalla y volver, la vista se crea de nuevo y, si esta lista se perdiera, volvería a llevar todo.
 */
export const despachados = new SvelteSet<string>();

/** Reactivo: el avatar lo observa para despertar su bucle y reaccionar. */
export const vuelo = $state<{ activo: boolean; fase: Fase | null; modo: ModoLlevar | null }>({
  activo: false,
  fase: null,
  modo: null
});

const TAM_KIRBY = 72;
const ESCALA_SUJETA = 0.2;
/** Dónde queda la tarjeta respecto al centro de Adi: colgando de su lado izquierdo, algo abajo. */
const COLGAR = { x: -26, y: 42 };
const MS_AGARRAR = 700;
const MS_SOLTAR = 550;
const MS_LISTO: Record<ModoLlevar, number> = { descartar: 1200, analizar: 700 };
const MS_MAXIMO_LLEVAR = 9000;

let actual: Vuelo | null = null;
let seguridad: ReturnType<typeof setTimeout> | undefined;

const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
const salida = (t: number) => 1 - (1 - t) ** 3; // cubic out
const rebote = (t: number) => 1 + 2.2 * (t - 1) ** 3 + 1.2 * (t - 1) ** 2; // back out

/** La entrada "Descartados" del menú (o, si no se ve, la barra lateral entera). */
export const destinoDescartados = (): HTMLElement | null =>
  document.querySelector<HTMLElement>('[data-nav="descartados"]') ?? document.querySelector<HTMLElement>('aside.barra-lateral');

/**
 * Empieza a llevar una tarjeta. 'ocupado' si Adi ya lleva otra (quien llama reintenta); 'sin-animacion'
 * si no hay con qué animarla (la tarjeta pasa sin viaje).
 */
export function llevarTarjeta(
  tarjeta: HTMLElement,
  modo: ModoLlevar,
  destinoEl: () => HTMLElement | null,
  alLlegar?: () => void
): ResultadoLlevar {
  if (actual) return 'ocupado';
  if (!destinoEl() && modo === 'descartar') return 'sin-animacion';
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) return 'sin-animacion';

  const r = tarjeta.getBoundingClientRect();
  const clon = tarjeta.cloneNode(true) as HTMLElement;
  clon.removeAttribute('data-slug');
  Object.assign(clon.style, {
    position: 'fixed',
    left: '0',
    top: '0',
    width: `${r.width}px`,
    height: `${r.height}px`,
    margin: '0',
    zIndex: '9990',
    pointerEvents: 'none',
    transformOrigin: 'center center',
    animation: 'none',
    transition: 'none',
    willChange: 'transform, opacity',
    transform: `translate(${r.left}px, ${r.top}px)`
  });
  document.body.appendChild(clon);

  const cx = r.left + r.width / 2;
  const cy = r.top + r.height / 2;
  actual = {
    el: clon,
    modo,
    destinoEl,
    alLlegar,
    llego: false,
    fase: 'agarrando',
    inicioFase: performance.now(),
    origen: { cx, cy, w: r.width, h: r.height },
    // Adi estaba en la esquina superior derecha de esa tarjeta.
    kirbyInicial: { cx: r.right - TAM_KIRBY / 2 - 14, cy: r.top - TAM_KIRBY * 0.55 + TAM_KIRBY / 2 },
    sujeta: { cx, cy },
    ultimoDestino: null,
    desplazado: false
  };
  vuelo.activo = true;
  vuelo.fase = 'agarrando';
  vuelo.modo = modo;
  // Por si el avatar no puede moverla (pestaña en segundo plano, etc.), nunca queda una copia colgada.
  seguridad = setTimeout(liberar, MS_AGARRAR + MS_MAXIMO_LLEVAR + MS_SOLTAR + MS_LISTO[modo] + 1500);
  return 'iniciado';
}

/** Rectángulo del destino ahora mismo (o el último conocido si el elemento ya no está). */
function rectDestino(): DOMRect | null {
  if (!actual) return null;
  const el = actual.destinoEl();
  if (el) actual.ultimoDestino = el.getBoundingClientRect();
  return actual.ultimoDestino;
}

/** Hacia dónde debe ir el centro de Adi ahora, o null si no hay nada que llevar. */
export function destinoAdi(): { cx: number; cy: number } | null {
  if (!actual) return null;
  if (actual.fase === 'agarrando') return actual.kirbyInicial;
  const d = rectDestino();
  if (!d) return actual.kirbyInicial;
  // Para que la tarjeta que cuelga de Adi quede sobre el destino.
  return { cx: d.left + d.width / 2 - COLGAR.x, cy: d.top + d.height / 2 - COLGAR.y };
}

function colocar(cx: number, cy: number, sx: number, sy: number, giro: number, opacidad = 1): void {
  if (!actual) return;
  const { w, h } = actual.origen;
  actual.el.style.transform = `translate(${cx - w / 2}px, ${cy - h / 2}px) scale(${sx}, ${sy}) rotate(${giro}deg)`;
  actual.el.style.opacity = String(opacidad);
}

function cambiarFase(fase: Fase, t: number): void {
  if (!actual) return;
  actual.fase = fase;
  actual.inicioFase = t;
  vuelo.fase = fase;
}

/** Un cuadro del recorrido. `kirby` es su centro en pantalla y su velocidad (px/s). */
export function actualizar(t: number, kirby: { cx: number; cy: number; vx: number; vy: number }): void {
  if (!actual) return;
  const en = t - actual.inicioFase;
  const colgada = { cx: kirby.cx + COLGAR.x, cy: kirby.cy + COLGAR.y };

  if (actual.fase === 'agarrando') {
    // Se comprime mientras Adi la recoge.
    const p = Math.min(1, en / MS_AGARRAR);
    const e = rebote(salida(p) * 0.999);
    const escala = lerp(1, ESCALA_SUJETA, e);
    colocar(lerp(actual.origen.cx, colgada.cx, salida(p)), lerp(actual.origen.cy, colgada.cy, salida(p)), escala, escala, lerp(0, -6, p));
    actual.sujeta = colgada;
    if (p >= 1) cambiarFase('llevando', t);
    return;
  }

  if (actual.fase === 'llevando') {
    const meta = destinoAdi();
    const destino = actual.destinoEl();
    // Si el destino queda fuera de la pantalla, la página baja con suavidad para que se vea la llegada.
    if (!actual.desplazado && destino) {
      actual.desplazado = true;
      destino.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
    const cerca = meta ? en > 900 && Math.hypot(meta.cx - kirby.cx, meta.cy - kirby.cy) < 10 && Math.hypot(kirby.vx, kirby.vy) < 70 : true;
    // Se balancea según la velocidad de Adi, como algo que cuelga.
    const giro = Math.max(-20, Math.min(20, -kirby.vx * 0.03)) + Math.sin(t / 170) * 3;
    colocar(colgada.cx, colgada.cy, ESCALA_SUJETA, ESCALA_SUJETA, giro);
    actual.sujeta = colgada;
    if (cerca || en > MS_MAXIMO_LLEVAR) cambiarFase('soltando', t);
    return;
  }

  if (actual.fase === 'listo') {
    if (en > MS_LISTO[actual.modo]) liberar();
    return;
  }

  // Soltando.
  const p = Math.min(1, en / MS_SOLTAR);
  const e = salida(p);
  const d = rectDestino();
  if (actual.modo === 'descartar' || !d) {
    // Se achica hasta desaparecer dentro de la entrada del menú.
    const cx = d ? d.left + d.width / 2 : actual.sujeta.cx;
    const cy = d ? d.top + d.height / 2 : actual.sujeta.cy;
    const escala = lerp(ESCALA_SUJETA, 0.02, e);
    colocar(lerp(actual.sujeta.cx, cx, e), lerp(actual.sujeta.cy, cy, e), escala, escala, lerp(0, 14, p), 1 - p * 0.7);
  } else {
    // Crece hasta encajar justo en su lugar de la grilla.
    colocar(
      lerp(actual.sujeta.cx, d.left + d.width / 2, e),
      lerp(actual.sujeta.cy, d.top + d.height / 2, e),
      lerp(ESCALA_SUJETA, d.width / actual.origen.w, e),
      lerp(ESCALA_SUJETA, d.height / actual.origen.h, e),
      0
    );
  }
  if (p >= 1) soltada(t);
}

/** Llegó: la copia desaparece, el destino "recibe" la tarjeta; Adi sigue ahí un momento. */
function soltada(t: number): void {
  if (!actual) return;
  const { el, modo } = actual;
  el.remove();
  avisarLlegada();
  const destino = actual.destinoEl();
  destino?.animate(
    modo === 'descartar'
      ? [{ transform: 'scale(1)' }, { transform: 'scale(1.1)' }, { transform: 'scale(0.97)' }, { transform: 'scale(1)' }]
      : [{ transform: 'scale(0.98)' }, { transform: 'scale(1.025)' }, { transform: 'scale(1)' }],
    { duration: modo === 'descartar' ? 520 : 400, easing: 'ease-out' }
  );
  cambiarFase('listo', t);
}

function avisarLlegada(): void {
  if (!actual || actual.llego) return;
  actual.llego = true;
  actual.alLlegar?.();
}

/** Fin del recorrido: Adi queda libre para volver a su sitio. */
function liberar(): void {
  clearTimeout(seguridad);
  if (!actual) return;
  actual.el.remove();
  avisarLlegada(); // si algo cortó el viaje, la tarjeta real aparece igual
  actual = null;
  vuelo.activo = false;
  vuelo.fase = null;
  vuelo.modo = null;
}
