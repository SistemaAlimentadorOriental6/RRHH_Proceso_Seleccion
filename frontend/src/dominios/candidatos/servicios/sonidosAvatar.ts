/** Sonidos suaves del avatar, sintetizados con Web Audio (sin archivos). El silencio se recuerda en el navegador. */

export type Sonido = 'pop' | 'nuevo' | 'feliz' | 'celebrar' | 'triste' | 'enojado' | 'alerta' | 'boing';

const CLAVE = 'rrhh.avatar.silencio';
const VOLUMEN = 0.05;
const PAUSA_MIN_MS = 350; // con muchos candidatos a la vez, los sonidos no se amontonan

let contexto: AudioContext | null = null;
let ultimo = 0;

export function estaSilenciado(): boolean {
  try {
    return localStorage.getItem(CLAVE) === '1';
  } catch {
    return false;
  }
}

export function guardarSilencio(silenciado: boolean): void {
  try {
    localStorage.setItem(CLAVE, silenciado ? '1' : '0');
  } catch {
    /* sin almacenamiento: solo dura esta visita */
  }
}

function tono(ctx: AudioContext, freq: number, inicio: number, dur: number, tipo: OscillatorType, hasta?: number, vol = VOLUMEN): void {
  const osc = ctx.createOscillator();
  const ganancia = ctx.createGain();
  const t0 = ctx.currentTime + inicio;
  osc.type = tipo;
  osc.frequency.setValueAtTime(freq, t0);
  if (hasta) osc.frequency.exponentialRampToValueAtTime(hasta, t0 + dur);
  // Entrada y salida suaves para que no haya "clic" al empezar ni al terminar.
  ganancia.gain.setValueAtTime(0.0001, t0);
  ganancia.gain.exponentialRampToValueAtTime(vol, t0 + 0.015);
  ganancia.gain.exponentialRampToValueAtTime(0.0001, t0 + dur);
  osc.connect(ganancia).connect(ctx.destination);
  osc.start(t0);
  osc.stop(t0 + dur + 0.02);
}

const MELODIAS: Record<Sonido, (c: AudioContext) => void> = {
  pop: (c) => tono(c, 520, 0, 0.09, 'triangle', 760, 0.035),
  nuevo: (c) => tono(c, 880, 0, 0.12, 'sine'),
  feliz: (c) => {
    tono(c, 660, 0, 0.11, 'sine');
    tono(c, 880, 0.09, 0.16, 'sine');
  },
  celebrar: (c) => [523, 659, 784, 1047].forEach((f, i) => tono(c, f, i * 0.09, 0.2, 'triangle')),
  triste: (c) => tono(c, 440, 0, 0.3, 'sine', 330),
  enojado: (c) => tono(c, 170, 0, 0.24, 'sawtooth', 110, 0.03),
  alerta: (c) => {
    tono(c, 392, 0, 0.1, 'triangle');
    tono(c, 349, 0.11, 0.14, 'triangle');
  },
  boing: (c) => tono(c, 300, 0, 0.16, 'sine', 720)
};

/** Reproduce un sonido; no hace nada si está silenciado o el navegador aún no permite audio (sin gesto del usuario). */
export function sonar(nombre: Sonido, forzar = false): void {
  if (typeof AudioContext === 'undefined' || estaSilenciado()) return;
  const ahora = performance.now();
  if (!forzar && ahora - ultimo < PAUSA_MIN_MS) return;
  ultimo = ahora;
  try {
    contexto ??= new AudioContext();
    if (contexto.state === 'suspended') void contexto.resume();
    if (contexto.state === 'running') MELODIAS[nombre](contexto);
  } catch {
    /* sin audio disponible */
  }
}
