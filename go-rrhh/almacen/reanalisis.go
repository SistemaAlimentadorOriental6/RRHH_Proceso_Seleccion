package almacen

import (
	"context"
	"database/sql"
	"time"

	"go-rrhh/modelos"
)

// DescartadosVigentes devuelve a los descartados con dictamen firme (no pendientes ni semilleros) de las
// postulaciones vigentes, con los resultados de sus consultas, para decidir a quién reanalizar.
func (a *Almacen) DescartadosVigentes(ctx context.Context) ([]modelos.Candidato, error) {
	return a.candidatosDonde(ctx, false,
		`po.activa = 1 AND po.dictamen = 'no_apto' AND po.verificacion_pendiente = 0
		 AND (po.clasificacion IS NULL OR po.clasificacion = 'descartado')`)
}

// ReanalisisEnCola devuelve las postulaciones con un reanálisis pendiente o en curso. Vive en la base y no
// en el navegador: todos los usuarios ven la misma cola aunque salgan de la pantalla o recarguen. Los
// encolados antes de existir la marca de origen se reconocen porque su postulación es un descartado firme
// (la cola del formulario es solo para personas en pausa).
func (a *Almacen) ReanalisisEnCola(ctx context.Context) ([]string, error) {
	filas, err := a.db.QueryContext(ctx,
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(t.opciones, '$.postulacion_id')) FROM trabajos t
		   LEFT JOIN postulaciones po
		     ON po.id = UNHEX(REPLACE(JSON_UNQUOTE(JSON_EXTRACT(t.opciones, '$.postulacion_id')), '-', ''))
		  WHERE t.tipo = ? AND t.estado IN ('pendiente', 'en_curso')
		    AND (JSON_UNQUOTE(JSON_EXTRACT(t.opciones, '$.origen')) = ?
		         OR (po.dictamen = 'no_apto' AND po.verificacion_pendiente = 0))
		  ORDER BY t.creado_en`, tipoReanudar, OrigenReanalisis)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	ids := []string{}
	for filas.Next() {
		var id sql.NullString
		if err := filas.Scan(&id); err != nil {
			return nil, err
		}
		if id.Valid {
			ids = append(ids, id.String)
		}
	}
	return ids, filas.Err()
}

// EncolarReanalisis pone a esas postulaciones en la cola de reanudaciones: el trabajador de segundo plano
// las vuelve a analizar desde ADRES con las reglas vigentes. Omite las que ya tienen una en cola y devuelve
// cuántas encoló.
func (a *Almacen) EncolarReanalisis(ctx context.Context, postulaciones []string) (int, error) {
	encoladas := 0
	err := a.enTx(ctx, func(tx *sql.Tx) error {
		encoladas = 0
		ahora := time.Now().UTC()
		for _, texto := range postulaciones {
			id, err := idBytes(texto)
			if err != nil {
				return err
			}
			var ofertaID []byte
			if err := tx.QueryRowContext(ctx, `SELECT oferta_id FROM postulaciones WHERE id = ?`, id).Scan(&ofertaID); err != nil {
				return err
			}
			var enCola int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM trabajos WHERE tipo = ? AND estado IN ('pendiente', 'en_curso')
				   AND JSON_UNQUOTE(JSON_EXTRACT(opciones, '$.postulacion_id')) = ?`, tipoReanudar, idTexto(id)).Scan(&enCola); err != nil {
				return err
			}
			if enCola > 0 {
				continue
			}
			trabajo, err := nuevoID()
			if err != nil {
				return err
			}
			if err := a.encolarReanudacion(ctx, tx, trabajo, ofertaID, id, ahora, OrigenReanalisis); err != nil {
				return err
			}
			encoladas++
		}
		return nil
	})
	return encoladas, err
}
