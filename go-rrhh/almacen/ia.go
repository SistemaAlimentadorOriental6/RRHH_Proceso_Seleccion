package almacen

import (
	"context"
	"database/sql"
	"encoding/json"

	"go-rrhh/modelos"
)

// GuardarEvaluacionIA guarda la calificación de la IA de una postulación y la deja como la vigente
// (la anterior queda en el historial). Se ata a la evaluación de idoneidad vigente, que debe existir.
// El documento completo va en la columna sintesis: de ahí se reconstruye al leer.
func (a *Almacen) GuardarEvaluacionIA(ctx context.Context, postulacionID string, ev *modelos.EvaluacionIA) error {
	postulacion, err := idBytes(postulacionID)
	if err != nil {
		return err
	}
	id, err := nuevoID()
	if err != nil {
		return err
	}
	doc, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	evaluadoEn := ev.EvaluadoEn
	return a.enTx(ctx, func(tx *sql.Tx) error {
		var idoneidad []byte
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM evaluaciones_idoneidad WHERE postulacion_id = ? AND vigente = 1`, postulacion).Scan(&idoneidad); err != nil {
			return err
		}
		if err := exec(ctx, tx, `UPDATE evaluaciones_ia SET vigente = 0 WHERE postulacion_id = ? AND vigente = 1`, postulacion); err != nil {
			return err
		}
		return exec(ctx, tx,
			`INSERT INTO evaluaciones_ia (id, postulacion_id, evaluacion_id, modelo, prompt_version, cargo,
			   score_total, score_experiencia, score_conduccion_normativo, score_estabilidad, score_afinidad,
			   dictamen, por_que, fortalezas, alerta, nivel_educativo, anos_experiencia, habilidades, observaciones,
			   sintesis, duracion_ms, evaluado_en, vigente)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			id, postulacion, idoneidad, texto(ev.Modelo, 60).String, texto(ev.PromptVersion, 20).String, texto(ev.Cargo, 255).String,
			ev.ScoreTotal, ev.ScoreExperiencia, ev.ScoreConduccionNormativo, ev.ScoreEstabilidad, ev.ScoreAfinidad,
			texto(ev.Dictamen, 60).String, texto(ev.PorQueEsElHombre, 16000), jsonLista(ev.Fortalezas), texto(ev.Alerta, 16000),
			texto(ev.NivelEducativo, 60), ev.AnosExperiencia, jsonLista(ev.Habilidades), texto(ev.Observaciones, 16000),
			string(doc), ev.DuracionMS, evaluadoEn)
	})
}

// leerEvaluacionesIA devuelve a cada candidato su calificación vigente de la IA. indice relaciona
// cada postulación con su posición en candidatos; cond es la condición sobre el alias po.
func (a *Almacen) leerEvaluacionesIA(ctx context.Context, cond string, indice map[string]int, candidatos []modelos.Candidato, args ...any) error {
	filas, err := a.db.QueryContext(ctx,
		`SELECT ia.postulacion_id, ia.sintesis
		   FROM evaluaciones_ia ia JOIN postulaciones po ON po.id = ia.postulacion_id
		  WHERE ia.vigente = 1 AND `+cond, args...)
	if err != nil {
		return err
	}
	defer filas.Close()
	for filas.Next() {
		var postulacion []byte
		var doc sql.NullString
		if err := filas.Scan(&postulacion, &doc); err != nil {
			return err
		}
		i, ok := indice[string(postulacion)]
		if !ok || !doc.Valid {
			continue
		}
		var ev modelos.EvaluacionIA
		if json.Unmarshal([]byte(doc.String), &ev) == nil {
			candidatos[i].EvaluacionIA = &ev
		}
	}
	return filas.Err()
}
