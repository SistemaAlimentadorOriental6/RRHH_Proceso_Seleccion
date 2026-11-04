export type Genero = 'masculino' | 'femenino';

export interface ParametrosAceptacion {
  edadMinima: number;
  edadMaxima: number;
  epsAceptadas: string[];
  aceptaReintegros: boolean;
  generosAceptados: Genero[];
  licenciasAceptadas: string[];
  /** Experiencia mínima como conductor, en meses (18 = 1 año y 6 meses). */
  experienciaMinimaMeses: number | null;
  /** Tiempo mínimo inscrito en el RUNT, en meses. */
  runtAntiguedadMeses: number | null;
  /** Versión de reglas en la base; solo la asigna el servidor. */
  version?: number;
}

export const LICENCIAS_DISPONIBLES = ['A1', 'A2', 'B1', 'B2', 'B3', 'C1', 'C2', 'C3'];

// Catálogo nacional de EPS (contributivo y subsidiado). Las 4 primeras son las autorizadas en Medellín (Supersalud, mar-2026).
export const GRUPOS_EPS: { titulo: string; eps: string[] }[] = [
  {
    titulo: 'Operando',
    eps: [
      'Nueva EPS', 'Salud Total', 'EPS Sura', 'Savia Salud', 'EPS Sanitas', 'Coosalud', 'EPS SOS', 'Famisanar',
      'Compensar', 'Aliansalud', 'Capital Salud', 'Mutual Ser', 'Asmet Salud', 'Emssanar', 'Cajacopi Atlántico',
      'Comfenalco Valle', 'Fundación Salud Mía', 'EPS Familiar de Colombia', 'Capresoca', 'Comfachocó',
      'Comfaoriente', 'Comfamiliar Huila', 'Comfamiliar Nariño'
    ]
  }
];

export const GENEROS_DISPONIBLES: { id: Genero; etiqueta: string }[] = [
  { id: 'masculino', etiqueta: 'Masculino' },
  { id: 'femenino', etiqueta: 'Femenino' }
];

export const PARAMETROS_POR_DEFECTO: ParametrosAceptacion = {
  edadMinima: 22,
  edadMaxima: 55,
  epsAceptadas: ['Nueva EPS', 'Salud Total', 'EPS Sura', 'Savia Salud'],
  aceptaReintegros: false,
  generosAceptados: ['masculino', 'femenino'],
  licenciasAceptadas: ['C1', 'C2'],
  experienciaMinimaMeses: 24,
  runtAntiguedadMeses: 24
};

/** Texto legible de una duración en meses: 18 -> "1 año y 6 meses". */
export function formatearDuracion(meses: number | null): string {
  if (meses === null) return '?';
  const a = Math.floor(meses / 12);
  const m = meses % 12;
  const partes = [];
  if (a > 0) partes.push(`${a} ${a === 1 ? 'año' : 'años'}`);
  if (m > 0 || a === 0) partes.push(`${m} ${m === 1 ? 'mes' : 'meses'}`);
  return partes.join(' y ');
}

export type ErroresParametros = Partial<Record<keyof ParametrosAceptacion, string>>;

const esNumero = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v);

/** Errores por campo; un objeto vacío significa que los parámetros son válidos y no hay nada vacío. */
export function validarParametros(p: ParametrosAceptacion): ErroresParametros {
  const e: ErroresParametros = {};

  // Un input numérico vacío llega como null: no es un entero válido.
  if (!esNumero(p.edadMinima)) e.edadMinima = 'La edad inicial es obligatoria.';
  else if (!Number.isInteger(p.edadMinima) || p.edadMinima < 18) e.edadMinima = 'La edad inicial debe ser un entero de 18 o más.';

  if (!esNumero(p.edadMaxima)) e.edadMaxima = 'La edad final es obligatoria.';
  else if (!Number.isInteger(p.edadMaxima) || p.edadMaxima > 99) e.edadMaxima = 'La edad final debe ser un entero de máximo 99.';
  else if (!e.edadMinima && p.edadMaxima < p.edadMinima) e.edadMaxima = 'La edad final no puede ser menor a la inicial.';

  const TOPE_MESES = 720; // 60 años

  if (!esNumero(p.experienciaMinimaMeses)) e.experienciaMinimaMeses = 'La experiencia mínima es obligatoria (0 si no se exige).';
  else if (!Number.isInteger(p.experienciaMinimaMeses) || p.experienciaMinimaMeses < 0 || p.experienciaMinimaMeses > TOPE_MESES)
    e.experienciaMinimaMeses = 'La experiencia debe estar entre 0 y 60 años, en meses completos.';

  if (!esNumero(p.runtAntiguedadMeses)) e.runtAntiguedadMeses = 'El tiempo inscrito en el RUNT es obligatorio (0 si no se exige).';
  else if (!Number.isInteger(p.runtAntiguedadMeses) || p.runtAntiguedadMeses < 0 || p.runtAntiguedadMeses > TOPE_MESES)
    e.runtAntiguedadMeses = 'El tiempo en el RUNT debe estar entre 0 y 60 años, en meses completos.';

  if (p.generosAceptados.length === 0) e.generosAceptados = 'Selecciona al menos un género.';
  if (p.licenciasAceptadas.length === 0) e.licenciasAceptadas = 'Selecciona al menos una categoría de licencia.';
  if (p.epsAceptadas.length === 0) e.epsAceptadas = 'Selecciona al menos una EPS.';

  return e;
}
