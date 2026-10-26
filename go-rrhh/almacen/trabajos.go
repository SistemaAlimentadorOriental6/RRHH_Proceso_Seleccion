package almacen

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// tipoReanudar es el tipo de trabajo que continúa el análisis de un candidato cuando entrega sus datos.
const tipoReanudar = "reanudar_postulacion"

// maxIntentosReanudar son las veces que se intenta una reanudación antes de darla por fallida.
const maxIntentosReanudar = 3

// esperaReintento es lo que se espera antes de reintentar una reanudación que falló.
const esperaReintento = 2 * time.Minute

// encolarReanudacion agrega el trabajo que continuará el análisis del candidato, en la misma
// transacción que guarda sus datos: o se guardan los dos o ninguno.
func (a *Almacen) encolarReanudacion(ctx context.Context, tx *sql.Tx, trabajo, ofertaID, postulacion []byte, ahora time.Time) error {
	opciones, err := json.Marshal(map[string]string{"postulacion_id": idTexto(postulacion)})
	if err != nil {
		return err
	}
	return exec(ctx, tx,
		`INSERT INTO trabajos (id, tipo, oferta_id, regla_version_id, opciones, estado, cancelar, intentos, creado_en, actualizado_en)
		 VALUES (?,?,?,?,?, 'pendiente', 0, 0, ?, ?)`,
		trabajo, tipoReanudar, ofertaID, a.reglaID, string(opciones), ahora, ahora)
}

// TrabajoReanudacion es una reanudación pendiente que el trabajador reclamó.
type TrabajoReanudacion struct {
	ID            []byte
	PostulacionID string
	Intentos      int
}

// ReclamarReanudacion toma la reanudación pendiente más antigua y la reserva por lease. Devuelve
// nil si no hay ninguna (o si otro proceso se adelantó).
func (a *Almacen) ReclamarReanudacion(ctx context.Context, trabajador string, lease time.Duration) (*TrabajoReanudacion, error) {
	ahora := time.Now().UTC()
	var id []byte
	var opciones string
	var intentos int
	err := a.db.QueryRowContext(ctx,
		`SELECT id, opciones, intentos FROM trabajos
		  WHERE tipo = ? AND estado = 'pendiente' AND (bloqueado_hasta IS NULL OR bloqueado_hasta < ?)
		  ORDER BY creado_en LIMIT 1`, tipoReanudar, ahora).Scan(&id, &opciones, &intentos)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := a.db.ExecContext(ctx,
		`UPDATE trabajos SET estado = 'en_curso', intentos = intentos + 1, worker_id = ?, bloqueado_hasta = ?, actualizado_en = ?
		  WHERE id = ? AND estado = 'pendiente'`, trabajador, ahora.Add(lease), ahora, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil // otro trabajador lo reclamó primero
	}
	var o struct {
		PostulacionID string `json:"postulacion_id"`
	}
	if err := json.Unmarshal([]byte(opciones), &o); err != nil || o.PostulacionID == "" {
		_ = a.TerminarReanudacion(ctx, &TrabajoReanudacion{ID: id, Intentos: maxIntentosReanudar}, fmt.Errorf("trabajo sin postulación"))
		return nil, nil
	}
	return &TrabajoReanudacion{ID: id, PostulacionID: o.PostulacionID, Intentos: intentos + 1}, nil
}

// TerminarReanudacion cierra el trabajo: completado si fallo es nil. Si falló y quedan intentos
// vuelve a pendiente para dentro de unos minutos; si no, queda en error.
func (a *Almacen) TerminarReanudacion(ctx context.Context, t *TrabajoReanudacion, fallo error) error {
	ahora := time.Now().UTC()
	if fallo == nil {
		_, err := a.db.ExecContext(ctx,
			`UPDATE trabajos SET estado = 'completado', error = NULL, worker_id = NULL, bloqueado_hasta = NULL, terminado_en = ?, actualizado_en = ? WHERE id = ?`,
			ahora, ahora, t.ID)
		return err
	}
	mensaje := texto(fallo.Error(), 500)
	if t.Intentos >= maxIntentosReanudar {
		_, err := a.db.ExecContext(ctx,
			`UPDATE trabajos SET estado = 'error', error = ?, worker_id = NULL, bloqueado_hasta = NULL, terminado_en = ?, actualizado_en = ? WHERE id = ?`,
			mensaje, ahora, ahora, t.ID)
		return err
	}
	_, err := a.db.ExecContext(ctx,
		`UPDATE trabajos SET estado = 'pendiente', error = ?, worker_id = NULL, bloqueado_hasta = ?, actualizado_en = ? WHERE id = ?`,
		mensaje, ahora.Add(esperaReintento), ahora, t.ID)
	return err
}

// RecuperarReanudaciones devuelve a pendiente las que quedaron "en curso" con el lease vencido
// (el proceso se cayó a mitad). Tras maxIntentosReanudar pasan a error.
func (a *Almacen) RecuperarReanudaciones(ctx context.Context) error {
	ahora := time.Now().UTC()
	_, err := a.db.ExecContext(ctx,
		`UPDATE trabajos
		    SET estado = IF(intentos >= ?, 'error', 'pendiente'),
		        error = IF(intentos >= ?, 'el proceso se detuvo antes de terminar la reanudación', error),
		        terminado_en = IF(intentos >= ?, ?, terminado_en),
		        worker_id = NULL, bloqueado_hasta = NULL, actualizado_en = ?
		  WHERE tipo = ? AND estado = 'en_curso' AND bloqueado_hasta < ?`,
		maxIntentosReanudar, maxIntentosReanudar, maxIntentosReanudar, ahora, ahora, tipoReanudar, ahora)
	return err
}
