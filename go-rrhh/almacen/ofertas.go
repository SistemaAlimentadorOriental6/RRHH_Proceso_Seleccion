package almacen

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go-rrhh/modelos"
)

// prefijoCarpeta marca las ofertas importadas desde las carpetas de descargas, de las que no se
// conoce el id de Computrabajo. En cuanto se analiza la misma vacante en vivo (mismo slug) se
// adopta ese registro y se le pone el id real.
const prefijoCarpeta = "carpeta:"

// maxIDOferta es el largo de ofertas.id_computrabajo (VARCHAR(32)).
const maxIDOferta = 32

// huellaCorta resume un texto largo en 16 caracteres hexadecimales estables.
func huellaCorta(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// IDCarpeta es el id provisional de una oferta importada desde su carpeta de descargas. Se resume
// con una huella porque el nombre de la carpeta es más largo que la columna.
func IDCarpeta(nombre string) string { return prefijoCarpeta + huellaCorta(nombre) }

// claveOferta devuelve el valor de ofertas.id_computrabajo: el id de Computrabajo si cabe en la
// columna; si falta o es demasiado largo, una clave corta derivada de él (o del slug).
func claveOferta(id, slug string) string {
	id = strings.TrimSpace(id)
	switch {
	case id == "":
		return "titulo:" + huellaCorta(slug)
	case len(id) > maxIDOferta:
		return "id:" + huellaCorta(id)
	}
	return id
}

// asegurarOferta devuelve el id de la oferta, creándola si hace falta.
func (a *Almacen) asegurarOferta(ctx context.Context, o modelos.OfertaLaboral) ([]byte, error) {
	slug := texto(a.slug(o.Titulo), 255).String
	idCT := claveOferta(o.ID, slug)

	a.mu.Lock()
	if id, ok := a.ofertas[idCT]; ok {
		a.mu.Unlock()
		return id, nil
	}
	a.mu.Unlock()

	titulo := texto(o.Titulo, 255).String
	ahora := time.Now().UTC()
	var id []byte
	err := a.enTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT id FROM ofertas WHERE id_computrabajo = ?`, idCT).Scan(&id)
		switch {
		case err == nil:
			_, err = tx.ExecContext(ctx, `UPDATE ofertas SET titulo = ?, slug = ?, actualizado_en = ? WHERE id = ?`,
				titulo, slug, ahora, id)
			return err
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}

		if !strings.HasPrefix(idCT, prefijoCarpeta) {
			res, err := tx.ExecContext(ctx,
				`UPDATE ofertas SET id_computrabajo = ?, titulo = ?, actualizado_en = ?
				  WHERE slug = ? AND id_computrabajo LIKE 'carpeta:%' LIMIT 1`, idCT, titulo, ahora, slug)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				return tx.QueryRowContext(ctx, `SELECT id FROM ofertas WHERE id_computrabajo = ?`, idCT).Scan(&id)
			}
		}

		nuevoOferta, err := nuevoID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ofertas (id, id_computrabajo, titulo, slug, excluida, creado_en, actualizado_en)
			 VALUES (?, ?, ?, ?, 0, ?, ?)
			 ON DUPLICATE KEY UPDATE titulo = VALUES(titulo), slug = VALUES(slug), actualizado_en = VALUES(actualizado_en)`,
			nuevoOferta, idCT, titulo, slug, ahora, ahora); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, `SELECT id FROM ofertas WHERE id_computrabajo = ?`, idCT).Scan(&id)
	})
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.ofertas[idCT] = id
	a.mu.Unlock()
	return id, nil
}
