package almacen

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"go-rrhh/modelos"
)

// GuardarCandidato guarda al candidato tal como quedó al terminar su análisis: persona,
// postulación, perfil, archivos, una fila por cada validación consultada y su evaluación de
// idoneidad con los motivos. Todo en una sola transacción: o queda completo o no queda nada.
func (a *Almacen) GuardarCandidato(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato) error {
	_, err := a.guardar(ctx, oferta, c, time.Now().UTC(), false)
	return err
}

// ImportarCandidato guarda un candidato leído de los JSON que ya existían. Devuelve true si
// escribió algo. Si su postulación ya está completa en la base no hace nada (así importar dos veces
// no duplica el historial). momento es la fecha que se registra como de la consulta (la del archivo
// de origen).
func (a *Almacen) ImportarCandidato(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato, momento time.Time) (bool, error) {
	return a.guardar(ctx, oferta, c, momento.UTC(), true)
}

// identidad devuelve la clave con la que se reconoce a la persona y su cédula. Sin cédula válida
// se usa la URL de su ficha de Computrabajo.
//
// shortcut: la persona sin cédula queda provisional bajo la URL; si luego se lee su cédula se crea
// otra persona y la provisional queda huérfana. Consolidarlas cuando haya datos reales de ese caso.
func identidad(c *modelos.Candidato) (clave, cedula string) {
	cedula = soloDigitos(c.Cedula)
	if len(cedula) >= 6 {
		return "cc:" + cedula, cedula
	}
	if u := strings.TrimSpace(c.URLDetalle); u != "" {
		return "url:" + u, ""
	}
	return "", ""
}

// estadoPostulacion resume dónde quedó el candidato.
func estadoPostulacion(c *modelos.Candidato) (etapa, dictamen string, pendiente bool) {
	ev := c.EvaluacionIdoneidad
	switch {
	case modelos.EnPausa(c.EstadoProceso):
		// En pausa por un dato que falta: no es un rechazo ni un error, sigue pendiente.
		return c.EstadoProceso, "pendiente", true
	case ev == nil:
		return "perfil", "pendiente", true
	case ev.VerificacionPendiente:
		return "pendiente", "pendiente", true
	case ev.DescarteTemprano:
		etapa = "descartado_temprano"
	default:
		etapa = "completado"
	}
	if ev.EsApto {
		return etapa, "apto", false
	}
	return etapa, "no_apto", false
}

// triEstado guarda una regla como sin evaluar (NULL) si no llegó a comprobarse.
func triEstado(cumple bool, detalle string) sql.NullBool {
	if noVerificado(detalle) {
		return sql.NullBool{}
	}
	return nullBool(cumple)
}

// tipoMotivo clasifica un motivo de descarte comparándolo con los detalles de cada regla, que son
// de donde salió el texto; no se adivina por palabras.
func tipoMotivo(motivo string, ev *modelos.EvaluacionIdoneidad) string {
	switch {
	case strings.HasPrefix(motivo, "Verificación pendiente"):
		return "pendiente"
	case strings.HasPrefix(motivo, "Género"):
		return "genero"
	case strings.HasPrefix(motivo, "Cédula"):
		return "cedula"
	case motivo == ev.DetalleEPS:
		return "eps"
	case motivo == ev.DetalleRunt:
		return "runt"
	case motivo == ev.DetalleSimit:
		return "simit"
	}
	// Texto no reconocido: se atribuye a la primera regla que no se cumple.
	switch {
	case !ev.CumpleGenero:
		return "genero"
	case !ev.CumpleEPS:
		return "eps"
	case !ev.CumpleRunt:
		return "runt"
	case !ev.CumpleSimit:
		return "simit"
	}
	return "pendiente"
}

// vincularFichaConPersona mueve a persona la postulación de esa ficha si estaba guardada bajo otra
// persona y persona todavía no tiene postulación en la vacante. La persona anterior se borra si
// queda sin postulaciones.
func (a *Almacen) vincularFichaConPersona(ctx context.Context, tx *sql.Tx, ofertaID []byte, urlDetalle string, persona []byte) error {
	var postulacion, personaVieja []byte
	err := tx.QueryRowContext(ctx,
		`SELECT po.id, po.persona_id FROM postulaciones po JOIN perfiles pf ON pf.postulacion_id = po.id
		  WHERE po.oferta_id = ? AND pf.url_detalle = ? AND po.persona_id <> ? LIMIT 1`,
		ofertaID, urlDetalle, persona).Scan(&postulacion, &personaVieja)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var yaTiene int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM postulaciones WHERE persona_id = ? AND oferta_id = ?`, persona, ofertaID).Scan(&yaTiene); err != nil {
		return err
	}
	if yaTiene > 0 {
		return nil // dos postulaciones distintas de la misma persona en la vacante: no se mezclan solas
	}
	if err := exec(ctx, tx, `UPDATE postulaciones SET persona_id = ? WHERE id = ?`, persona, postulacion); err != nil {
		return err
	}
	if err := exec(ctx, tx,
		`DELETE FROM personas WHERE id = ? AND NOT EXISTS (SELECT 1 FROM postulaciones po WHERE po.persona_id = ?)`,
		personaVieja, personaVieja); err != nil {
		return err
	}
	return historialTx(ctx, tx, postulacion, "persona.vinculada", map[string]any{"detalle": "la ficha pasó a la persona con cédula"})
}

func (a *Almacen) guardar(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato, momento time.Time, soloNueva bool) (bool, error) {
	clave, cedula := identidad(c)
	if clave == "" {
		return false, errors.New("el candidato no tiene cédula ni URL de ficha: no se puede identificar")
	}
	ofertaID, err := a.asegurarOferta(ctx, oferta)
	if err != nil {
		return false, err
	}
	hmacPersona := a.claves.Pseudonimo(clave)

	// Una importación no repite lo que ya está en la base, salvo que sus validaciones se guardaron
	// antes de existir la columna datos: esas no se pueden devolver tal cual y se vuelven a guardar.
	mejorarLegado := false
	if soloNueva {
		var existe, sinDatos int
		err := a.db.QueryRowContext(ctx,
			`SELECT COUNT(DISTINCT po.id), COUNT(v.id)
			   FROM postulaciones po JOIN personas pe ON pe.id = po.persona_id
			   LEFT JOIN validaciones v ON v.postulacion_id = po.id AND v.datos IS NULL
			  WHERE pe.cedula_hmac = ? AND po.oferta_id = ?`, hmacPersona, ofertaID).Scan(&existe, &sinDatos)
		if err != nil {
			return false, err
		}
		if existe > 0 && sinDatos == 0 {
			return false, nil
		}
		mejorarLegado = existe > 0
	}

	cifrada, err := a.claves.Cifrar(cedula)
	if err != nil {
		return false, err
	}
	personaID, err := nuevoID()
	if err != nil {
		return false, err
	}
	postulacionID, err := nuevoID()
	if err != nil {
		return false, err
	}
	archivos := a.prepararArchivos(c)
	vals := a.validaciones(c, momento)
	validacionIDs := make([][]byte, len(vals))
	for i := range vals {
		if validacionIDs[i], err = nuevoID(); err != nil {
			return false, err
		}
	}
	ev := c.EvaluacionIdoneidad
	var evaluacionID []byte
	if ev != nil {
		if evaluacionID, err = nuevoID(); err != nil {
			return false, err
		}
	}
	etapa, dictamen, pendiente := estadoPostulacion(c)

	creada := false
	var postulacionFinal []byte
	err = a.enTx(ctx, func(tx *sql.Tx) error {
		creada = false

		// 1. Persona (única por cédula) y postulación (única por persona y vacante).
		if err := exec(ctx, tx,
			`INSERT INTO personas (id, cedula_hmac, cedula_cifrada, creado_en, actualizado_en) VALUES (?,?,?,?,?)
			 ON DUPLICATE KEY UPDATE actualizado_en = VALUES(actualizado_en)`,
			personaID, hmacPersona, cifrada, momento, momento); err != nil {
			return err
		}
		var persona []byte
		if err := tx.QueryRowContext(ctx, `SELECT id FROM personas WHERE cedula_hmac = ?`, hmacPersona).Scan(&persona); err != nil {
			return err
		}

		// Si esta misma ficha ya estaba guardada bajo otra persona (la provisional de cuando no se
		// conocía la cédula) esa postulación pasa a la persona con cédula, en vez de duplicarla.
		if cedula != "" && c.URLDetalle != "" {
			if err := a.vincularFichaConPersona(ctx, tx, ofertaID, c.URLDetalle, persona); err != nil {
				return err
			}
		}

		postulacion := postulacionID
		err := tx.QueryRowContext(ctx, `SELECT id FROM postulaciones WHERE persona_id = ? AND oferta_id = ?`,
			persona, ofertaID).Scan(&postulacion)
		switch {
		case err == nil:
			if err := exec(ctx, tx,
				`UPDATE postulaciones SET etapa_actual = ?, dictamen = ?, activa = 1, verificacion_pendiente = ?, error_proceso = ?,
				   actualizado_en = ? WHERE id = ?`,
				etapa, dictamen, pendiente, texto(c.ErrorProceso, 255), momento, postulacion); err != nil {
				return err
			}
			if mejorarLegado {
				if err := exec(ctx, tx, `DELETE FROM validaciones WHERE postulacion_id = ? AND datos IS NULL`, postulacion); err != nil {
					return err
				}
			}
		case errors.Is(err, sql.ErrNoRows):
			creada = true
			if err := exec(ctx, tx,
				`INSERT INTO postulaciones (id, persona_id, oferta_id, trabajo_id, etapa_actual, dictamen, activa,
				   verificacion_pendiente, error_proceso, creado_en, actualizado_en)
				 VALUES (?,?,?,NULL,?,?,1,?,?,?,?)`,
				postulacion, persona, ofertaID, etapa, dictamen, pendiente, texto(c.ErrorProceso, 255), momento, momento); err != nil {
				return err
			}
		default:
			return err
		}
		postulacionFinal = postulacion

		// Si el candidato salió de la pausa por otra vía (se halló su cédula, o se reprocesó) el
		// enlace que se le mandó ya no debe servir.
		if !modelos.EnPausa(etapa) {
			res, err := tx.ExecContext(ctx,
				`UPDATE solicitudes_datos SET estado = 'revocada', actualizada_en = ?
				  WHERE postulacion_id = ? AND estado IN ('pendiente_envio','enviada','abierta')`, momento, postulacion)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				if err := historialTx(ctx, tx, postulacion, "solicitud.revocada", map[string]any{"motivo": "el análisis siguió por otra vía"}); err != nil {
					return err
				}
			}
		}

		// 2. Perfil (edad: NULL si no se conoce; no pisa la ya guardada). genero_inferido es el género que el sistema usó; el declarado y su origen no
		// llegan hasta aquí (el candidato solo trae el resultado final).
		if err := exec(ctx, tx,
			`INSERT INTO perfiles (postulacion_id, indice, nombre, slug_persona, genero_inferido, edad, url_detalle, url_cv, url_foto, leido_en)
			 VALUES (?,?,?,?,?,?,?,?,?,?)
			 ON DUPLICATE KEY UPDATE indice = VALUES(indice), nombre = VALUES(nombre), slug_persona = VALUES(slug_persona),
			   genero_inferido = VALUES(genero_inferido), edad = COALESCE(VALUES(edad), edad), url_detalle = VALUES(url_detalle),
			   url_cv = VALUES(url_cv), url_foto = VALUES(url_foto), leido_en = VALUES(leido_en)`,
			postulacion, c.Indice, texto(c.Nombre, 200), texto(a.slugPersona(c.Nombre), 200), texto(c.Genero, 30),
			edadParaGuardar(c.Edad), texto(c.URLDetalle, 500), texto(c.URLDescargaCV, 500), texto(c.URLFoto, 500), momento); err != nil {
			return err
		}

		// 3. Archivos: se reemplaza el conjunto por el actual (las capturas se sobrescriben en cada análisis).
		if err := exec(ctx, tx, `DELETE FROM archivos WHERE postulacion_id = ?`, postulacion); err != nil {
			return err
		}
		var textoID []byte
		for _, f := range archivos {
			if err := exec(ctx, tx,
				`INSERT INTO archivos (id, postulacion_id, tipo, fuente_codigo, etapa, ruta, sha256, bytes, creado_en) VALUES (?,?,?,?,?,?,?,?,?)`,
				f.id, postulacion, f.tipo, f.fuente, f.etapa, f.ruta, f.sha256, f.bytes, momento); err != nil {
				return err
			}
			if f.tipo == "texto_perfil" {
				textoID = f.id
			}
		}
		if textoID != nil {
			if err := exec(ctx, tx, `UPDATE perfiles SET texto_perfil_archivo_id = ? WHERE postulacion_id = ?`, textoID, postulacion); err != nil {
				return err
			}
		}

		// 4. Validaciones: se agregan al historial (cada consulta es una fila nueva).
		for i, v := range vals {
			if err := exec(ctx, tx,
				`INSERT INTO validaciones (id, postulacion_id, fuente_codigo, trabajo_id, resultado, codigo_estado, detalle,
				   datos, intentos, duracion_ms, consultado_en)
				 VALUES (?,?,?,NULL,?,?,?,?,1,NULL,?)`,
				validacionIDs[i], postulacion, v.fuente, v.resultado, texto(v.codigo, 60), texto(v.detalle, 16000),
				sql.NullString{String: v.datos, Valid: v.datos != ""}, momento); err != nil {
				return err
			}
			if v.insertar == nil { // fuente sin tabla de detalle: el JSON de la validación basta
				continue
			}
			if err := v.insertar(ctx, tx, validacionIDs[i]); err != nil {
				return err
			}
		}

		// 5. Evaluación de idoneidad: la nueva pasa a ser la vigente y la anterior queda en el historial.
		if ev != nil {
			if err := exec(ctx, tx, `UPDATE evaluaciones_idoneidad SET vigente = 0 WHERE postulacion_id = ? AND vigente = 1`, postulacion); err != nil {
				return err
			}
			if err := exec(ctx, tx,
				`INSERT INTO evaluaciones_idoneidad (id, postulacion_id, regla_version_id, trabajo_id, es_apto, genero, cumple_genero,
				   cumple_eps, cumple_runt, cumple_simit, detalle_eps, detalle_runt, detalle_simit,
				   descarte_temprano, verificacion_pendiente, vigente, evaluado_en)
				 VALUES (?,?,?,NULL,?,?,?,?,?,?,?,?,?,?,?,1,?)`,
				evaluacionID, postulacion, a.reglaID, ev.EsApto, texto(ev.Genero, 30), nullBool(ev.CumpleGenero),
				triEstado(ev.CumpleEPS, ev.DetalleEPS), triEstado(ev.CumpleRunt, ev.DetalleRunt),
				triEstado(ev.CumpleSimit, ev.DetalleSimit),
				texto(ev.DetalleEPS, 16000), texto(ev.DetalleRunt, 16000), texto(ev.DetalleSimit, 16000),
				ev.DescarteTemprano, ev.VerificacionPendiente, momento); err != nil {
				return err
			}
			for _, m := range ev.MotivosDescarte {
				motivoID, err := nuevoID()
				if err != nil {
					return err
				}
				if err := exec(ctx, tx,
					`INSERT INTO postulacion_motivos (id, evaluacion_id, tipo, detalle) VALUES (?,?,?,?)`,
					motivoID, evaluacionID, tipoMotivo(m, ev), texto(m, 16000).String); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	c.PostulacionID = idTexto(postulacionFinal)
	return creada || mejorarLegado, nil
}
