import {
  NODOS_BASE_FLUJO_VALIDACION,
  type DefinicionNodoValidacion,
  type ModoSimulacionNodo,
  type RegistroLogEjecucion
} from '../modelos/flujoValidacion';

export interface CallbacksEjecucionFlujo {
  alActualizarNodos: (nodos: DefinicionNodoValidacion[]) => void;
  alRegistrarLog: (registro: RegistroLogEjecucion) => void;
  alCambiarNodoActivo: (idNodo: string | null) => void;
  alFinalizarFlujo: (resultado: {
    completadoConExito: boolean;
    nodoDetenido?: DefinicionNodoValidacion;
    totalReintentos: number;
    tiempoTotalMs: number;
  }) => void;
}

/**
 * Construye una copia limpia de los 14 nodos aplicando las reglas del escenario seleccionado.
 */
export function construirNodosIniciales(
  configuracionEscenario: Record<string, ModoSimulacionNodo> = {}
): DefinicionNodoValidacion[] {
  return NODOS_BASE_FLUJO_VALIDACION.map((base) => ({
    ...base,
    modoSimulacion: configuracionEscenario[base.id] ?? 'exito_directo',
    estado: 'pendiente',
    intentoActual: 0,
    duracionRealMs: 0,
    resumenSalida: 'En espera de ejecución'
  }));
}

function obtenerHoraFormateada(): string {
  const ahora = new Date();
  const hh = String(ahora.getHours()).padStart(2, '0');
  const mm = String(ahora.getMinutes()).padStart(2, '0');
  const ss = String(ahora.getSeconds()).padStart(2, '0');
  const ms = String(ahora.getMilliseconds()).padStart(3, '0');
  return `${hh}:${mm}:${ss}.${ms}`;
}

/**
 * Motor de orquestación paso a paso con soporte de reintentos automáticos y cortocircuito (SOLID - SRP).
 */
export class MotorFlujoValidacion {
  private abortado = false;

  public detener(): void {
    this.abortado = true;
  }

  private esperar(ms: number, factorVelocidad: number): Promise<void> {
    const tiempoAjustado = Math.max(120, Math.round(ms / factorVelocidad));
    return new Promise((resolver) => setTimeout(resolver, tiempoAjustado));
  }

  public async ejecutarFlujo(
    nodosIniciales: DefinicionNodoValidacion[],
    factorVelocidad: number,
    callbacks: CallbacksEjecucionFlujo
  ): Promise<void> {
    this.abortado = false;
    const inicioGlobal = performance.now();
    let totalReintentos = 0;

    const nodos: DefinicionNodoValidacion[] = nodosIniciales.map((n) => ({
      ...n,
      estado: 'pendiente',
      intentoActual: 0,
      duracionRealMs: 0,
      resumenSalida: 'En cola del pipeline'
    }));

    callbacks.alActualizarNodos([...nodos]);

    for (let i = 0; i < nodos.length; i++) {
      if (this.abortado) {
        return;
      }

      const nodo = nodos[i];
      const inicioNodo = performance.now();
      callbacks.alCambiarNodoActivo(nodo.id);

      let superado = false;

      for (let intento = 1; intento <= nodo.maxIntentos; intento++) {
        if (this.abortado) {
          return;
        }

        nodo.intentoActual = intento;
        nodo.estado = intento === 1 ? 'ejecutando' : 'reintentando';
        nodo.resumenSalida =
          intento === 1
            ? `Ejecutando validación (${intento}/${nodo.maxIntentos})...`
            : `Reintentando operación (${intento}/${nodo.maxIntentos})...`;
        callbacks.alActualizarNodos([...nodos]);

        callbacks.alRegistrarLog({
          id: `${nodo.id}-int-${intento}-${Date.now()}`,
          marcaTiempo: obtenerHoraFormateada(),
          idNodo: nodo.id,
          ordenNodo: nodo.orden,
          nombreNodo: nodo.titulo,
          nivel: intento === 1 ? 'info' : 'reintento',
          intento,
          maxIntentos: nodo.maxIntentos,
          mensaje:
            intento === 1
              ? `Iniciando [${nodo.codigo}] ${nodo.titulo} (${nodo.subtitulo})`
              : `Ejecutando reintento automático #${intento} de ${nodo.maxIntentos} para ${nodo.titulo}...`
        });

        await this.esperar(nodo.duracionBaseMs, factorVelocidad);
        if (this.abortado) {
          return;
        }

        // Caso A: El nodo está configurado para fallar en el intento 1 y recuperarse en el intento 2
        if (nodo.modoSimulacion === 'recuperar_en_reintento' && intento === 1) {
          totalReintentos++;
          nodo.estado = 'reintentando';
          nodo.resumenSalida = `Fallo transitorio en intento 1/${nodo.maxIntentos}. Reintentando...`;
          callbacks.alActualizarNodos([...nodos]);

          callbacks.alRegistrarLog({
            id: `${nodo.id}-warn-${intento}-${Date.now()}`,
            marcaTiempo: obtenerHoraFormateada(),
            idNodo: nodo.id,
            ordenNodo: nodo.orden,
            nombreNodo: nodo.titulo,
            nivel: 'reintento',
            intento,
            maxIntentos: nodo.maxIntentos,
            mensaje: `Advertencia transitoria: ${nodo.mensajeErrorTransitorio}`,
            detalleTecnico: `Política de backoff activada -> Programando reintento ${intento + 1}/${nodo.maxIntentos}`
          });

          await this.esperar(520, factorVelocidad);
          continue;
        }

        // Caso B: El nodo está configurado con fallo definitivo (reintenta todos sus intentos y si sigue fallando detiene todo)
        if (nodo.modoSimulacion === 'fallo_definitivo') {
          if (intento < nodo.maxIntentos) {
            totalReintentos++;
            nodo.estado = 'reintentando';
            nodo.resumenSalida = `Intento ${intento}/${nodo.maxIntentos} falló. Reintentando...`;
            callbacks.alActualizarNodos([...nodos]);

            callbacks.alRegistrarLog({
              id: `${nodo.id}-retry-fail-${intento}-${Date.now()}`,
              marcaTiempo: obtenerHoraFormateada(),
              idNodo: nodo.id,
              ordenNodo: nodo.orden,
              nombreNodo: nodo.titulo,
              nivel: 'reintento',
              intento,
              maxIntentos: nodo.maxIntentos,
              mensaje: `Fallo en intento ${intento}/${nodo.maxIntentos}: ${nodo.mensajeErrorTransitorio}`,
              detalleTecnico: `Verificando nuevamente antes de declarar parada crítica (${intento + 1}/${nodo.maxIntentos})...`
            });

            await this.esperar(550, factorVelocidad);
            continue;
          }

          // Agotó todos los intentos o falló por regla excluyente -> Detener flujo (Circuit Breaker)
          nodo.duracionRealMs = Math.round(performance.now() - inicioNodo);
          nodo.estado = 'detenido_error';
          nodo.resumenSalida = nodo.mensajeFalloDefinitivo;

          // Marcar todos los nodos posteriores como omitidos por parada del pipeline
          for (let j = i + 1; j < nodos.length; j++) {
            nodos[j].estado = 'omitido';
            nodos[j].resumenSalida = `Omitido por parada en ${nodo.codigo} (${nodo.titulo})`;
          }

          callbacks.alActualizarNodos([...nodos]);

          callbacks.alRegistrarLog({
            id: `${nodo.id}-critico-${Date.now()}`,
            marcaTiempo: obtenerHoraFormateada(),
            idNodo: nodo.id,
            ordenNodo: nodo.orden,
            nombreNodo: nodo.titulo,
            nivel: 'error_critico',
            intento,
            maxIntentos: nodo.maxIntentos,
            mensaje: `FLUJO DETENIDO EN [${nodo.codigo}]: ${nodo.mensajeFalloDefinitivo}`,
            detalleTecnico: JSON.stringify(nodo.payloadSalidaError)
          });

          callbacks.alFinalizarFlujo({
            completadoConExito: false,
            nodoDetenido: nodo,
            totalReintentos,
            tiempoTotalMs: Math.round(performance.now() - inicioGlobal)
          });
          return;
        }

        // Caso C: Validación superada exitosamente
        superado = true;
        nodo.duracionRealMs = Math.round(performance.now() - inicioNodo);
        nodo.estado = 'completado';
        nodo.resumenSalida =
          intento > 1
            ? `Recuperado y validado en intento ${intento}/${nodo.maxIntentos}`
            : 'Validación aprobada sin novedades';
        callbacks.alActualizarNodos([...nodos]);

        callbacks.alRegistrarLog({
          id: `${nodo.id}-ok-${Date.now()}`,
          marcaTiempo: obtenerHoraFormateada(),
          idNodo: nodo.id,
          ordenNodo: nodo.orden,
          nombreNodo: nodo.titulo,
          nivel: 'exito',
          intento,
          maxIntentos: nodo.maxIntentos,
          mensaje:
            intento > 1
              ? `[OK TRAS REINTENTO ${intento}/${nodo.maxIntentos}] ${nodo.titulo} superado en ${nodo.duracionRealMs} ms`
              : `[OK] ${nodo.titulo} validado correctamente (${nodo.duracionRealMs} ms)`,
          detalleTecnico: JSON.stringify(nodo.payloadSalidaExito)
        });

        break;
      }

      if (!superado) {
        return;
      }
    }

    callbacks.alCambiarNodoActivo(null);
    callbacks.alFinalizarFlujo({
      completadoConExito: true,
      totalReintentos,
      tiempoTotalMs: Math.round(performance.now() - inicioGlobal)
    });
  }
}
