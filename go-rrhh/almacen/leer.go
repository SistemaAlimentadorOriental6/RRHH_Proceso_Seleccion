package almacen

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-rrhh/modelos"
)

// OfertaProcesada es una vacante con candidatos en la base.
type OfertaProcesada struct {
	IDComputrabajo string
	Titulo, Slug   string
	Total, Aptos   int
	ActualizadoEn  time.Time
}

// OfertasProcesadas lista las vacantes que tienen postulantes vigentes, con sus totales.
func (a *Almacen) OfertasProcesadas(ctx context.Context) ([]OfertaProcesada, error) {
	filas, err := a.db.QueryContext(ctx,
		`SELECT o.id_computrabajo, o.titulo, o.slug, COUNT(*), COALESCE(SUM(po.dictamen = 'apto'), 0), MAX(po.actualizado_en)
		   FROM ofertas o JOIN postulaciones po ON po.oferta_id = o.id AND po.activa = 1
		  GROUP BY o.id, o.id_computrabajo, o.titulo, o.slug`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	var salida []OfertaProcesada
	for filas.Next() {
		var o OfertaProcesada
		var actualizado sql.NullTime
		if err := filas.Scan(&o.IDComputrabajo, &o.Titulo, &o.Slug, &o.Total, &o.Aptos, &actualizado); err != nil {
			return nil, err
		}
		o.ActualizadoEn = actualizado.Time
		salida = append(salida, o)
	}
	return salida, filas.Err()
}

// CandidatosDeOferta devuelve los candidatos vigentes de una vacante, reconstruidos con la misma
// forma que tenían en candidatos.json. Una vacante desconocida devuelve una lista vacía.
func (a *Almacen) CandidatosDeOferta(ctx context.Context, idComputrabajo string) ([]modelos.Candidato, error) {
	var ofertaID []byte
	err := a.db.QueryRowContext(ctx, `SELECT id FROM ofertas WHERE id_computrabajo = ?`, idComputrabajo).Scan(&ofertaID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a.candidatosDe(ctx, ofertaID, true)
}

// CandidatosPrevios devuelve los candidatos ya analizados de la vacante (también los que dejaron
// de aparecer), indexados por la URL de su ficha. El análisis los reutiliza en vez de volver a consultar.
func (a *Almacen) CandidatosPrevios(ctx context.Context, oferta modelos.OfertaLaboral) (map[string]*modelos.Candidato, error) {
	ofertaID, err := a.asegurarOferta(ctx, oferta)
	if err != nil {
		return nil, err
	}
	lista, err := a.candidatosDe(ctx, ofertaID, false)
	if err != nil {
		return nil, err
	}
	previos := make(map[string]*modelos.Candidato, len(lista))
	for i := range lista {
		if u := lista[i].URLDetalle; u != "" {
			previos[u] = &lista[i]
		}
	}
	return previos, nil
}

// MarcarVigentes deja vigentes solo a los postulantes cuya ficha aparece en urls y retira a los
// demás de la vacante (su historial se conserva). Se llama al terminar un listado completo.
func (a *Almacen) MarcarVigentes(ctx context.Context, oferta modelos.OfertaLaboral, urls []string) error {
	ofertaID, err := a.asegurarOferta(ctx, oferta)
	if err != nil {
		return err
	}
	consulta := `UPDATE postulaciones po JOIN perfiles pf ON pf.postulacion_id = po.id
	                SET po.activa = COALESCE(pf.url_detalle IN (` + marcas(len(urls)) + `), 0)
	              WHERE po.oferta_id = ? AND pf.url_detalle IS NOT NULL`
	args := make([]any, 0, len(urls)+1)
	for _, u := range urls {
		args = append(args, u)
	}
	args = append(args, ofertaID)
	return a.enTx(ctx, func(tx *sql.Tx) error { return exec(ctx, tx, consulta, args...) })
}

// marcas devuelve n marcadores "?" separados por comas; con n = 0 devuelve NULL (nada coincide).
func marcas(n int) string {
	if n == 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// rutaAbsoluta devuelve la ruta que usaba el sistema: las guardadas relativas a descargas/ se
// completan con ese directorio y las que estaban fuera de él ya son absolutas.
func (a *Almacen) rutaAbsoluta(rel string) string {
	p := filepath.FromSlash(rel)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(a.descargas, p)
}

type filaCandidato struct {
	postulacion, cedulaCifrada, evaluacionID          []byte
	nacimientoCifrado                                 []byte // fecha declarada por el candidato, si la hay
	dictamen, etapa                                   string
	errorProceso, nombre, genero                      sql.NullString
	urlDetalle, urlCV, urlFoto                        sql.NullString
	indice                                            sql.NullInt64
	evEsApto, cumpleGenero, cumpleEPS                 sql.NullBool
	cumpleRunt, cumpleSimit, descarte, verifPendiente sql.NullBool
	evGenero, detalleEPS, detalleRunt, detalleSimit   sql.NullString
}

// candidatosDe reconstruye a los candidatos de una vacante.
func (a *Almacen) candidatosDe(ctx context.Context, ofertaID []byte, soloVigentes bool) ([]modelos.Candidato, error) {
	cond := "po.oferta_id = ?"
	if soloVigentes {
		cond += " AND po.activa = 1"
	}
	return a.candidatosDonde(ctx, cond, ofertaID)
}

// candidatosDonde reconstruye a los candidatos de las postulaciones que cumplen cond (una
// condición sobre el alias po de postulaciones) con cuatro consultas, sin importar cuántos sean:
// postulaciones con su perfil y evaluación vigente, motivos de descarte, la última validación de
// cada fuente y los archivos.
func (a *Almacen) candidatosDonde(ctx context.Context, cond string, args ...any) ([]modelos.Candidato, error) {
	filas, err := a.db.QueryContext(ctx,
		`SELECT po.id, po.dictamen, po.etapa_actual, po.error_proceso, pe.cedula_cifrada,
		        (SELECT dd.fecha_nacimiento_cifrada FROM datos_declarados dd
		          WHERE dd.postulacion_id = po.id ORDER BY dd.id DESC LIMIT 1),
		        pf.indice, pf.nombre, pf.genero_inferido, pf.url_detalle, pf.url_cv, pf.url_foto,
		        ev.id, ev.es_apto, ev.genero, ev.cumple_genero, ev.cumple_eps, ev.cumple_runt, ev.cumple_simit,
		        ev.detalle_eps, ev.detalle_runt, ev.detalle_simit, ev.descarte_temprano, ev.verificacion_pendiente
		   FROM postulaciones po
		   JOIN personas pe ON pe.id = po.persona_id
		   LEFT JOIN perfiles pf ON pf.postulacion_id = po.id
		   LEFT JOIN evaluaciones_idoneidad ev ON ev.postulacion_id = po.id AND ev.vigente = 1
		  WHERE `+cond+`
		  ORDER BY pf.indice, po.creado_en`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()

	var candidatos []modelos.Candidato
	indice := map[string]int{}     // postulación -> posición en candidatos
	evaluacion := map[string]int{} // evaluación vigente -> posición en candidatos
	for filas.Next() {
		var f filaCandidato
		if err := filas.Scan(&f.postulacion, &f.dictamen, &f.etapa, &f.errorProceso, &f.cedulaCifrada, &f.nacimientoCifrado,
			&f.indice, &f.nombre, &f.genero, &f.urlDetalle, &f.urlCV, &f.urlFoto,
			&f.evaluacionID, &f.evEsApto, &f.evGenero, &f.cumpleGenero, &f.cumpleEPS, &f.cumpleRunt, &f.cumpleSimit,
			&f.detalleEPS, &f.detalleRunt, &f.detalleSimit, &f.descarte, &f.verifPendiente); err != nil {
			return nil, err
		}
		c, err := a.armarCandidato(f)
		if err != nil {
			return nil, err
		}
		indice[string(f.postulacion)] = len(candidatos)
		if f.evaluacionID != nil {
			evaluacion[string(f.evaluacionID)] = len(candidatos)
		}
		candidatos = append(candidatos, c)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	filas.Close()
	if len(candidatos) == 0 {
		return candidatos, nil
	}

	// Motivos de descarte de la evaluación vigente, en el orden en que se guardaron (UUID v7).
	motivos, err := a.db.QueryContext(ctx,
		`SELECT m.evaluacion_id, m.detalle
		   FROM postulacion_motivos m
		   JOIN evaluaciones_idoneidad ev ON ev.id = m.evaluacion_id AND ev.vigente = 1
		   JOIN postulaciones po ON po.id = ev.postulacion_id
		  WHERE `+cond+`
		  ORDER BY m.id`, args...)
	if err != nil {
		return nil, err
	}
	defer motivos.Close()
	for motivos.Next() {
		var evID []byte
		var detalle string
		if err := motivos.Scan(&evID, &detalle); err != nil {
			return nil, err
		}
		if i, ok := evaluacion[string(evID)]; ok && candidatos[i].EvaluacionIdoneidad != nil {
			ev := candidatos[i].EvaluacionIdoneidad
			ev.MotivosDescarte = append(ev.MotivosDescarte, detalle)
		}
	}
	if err := motivos.Err(); err != nil {
		return nil, err
	}
	motivos.Close()

	// Última validación de cada fuente (la de mayor id: los UUID v7 crecen con el tiempo).
	vals, err := a.db.QueryContext(ctx,
		`SELECT v.postulacion_id, v.fuente_codigo, v.datos
		   FROM validaciones v
		   JOIN (SELECT postulacion_id, fuente_codigo, MAX(id) AS ultima
		           FROM validaciones
		          WHERE postulacion_id IN (SELECT po.id FROM postulaciones po WHERE `+cond+`)
		          GROUP BY postulacion_id, fuente_codigo) u ON u.ultima = v.id`, args...)
	if err != nil {
		return nil, err
	}
	defer vals.Close()
	for vals.Next() {
		var postulacion []byte
		var fuente string
		var datos sql.NullString
		if err := vals.Scan(&postulacion, &fuente, &datos); err != nil {
			return nil, err
		}
		if i, ok := indice[string(postulacion)]; ok && datos.Valid {
			asignarResultado(&candidatos[i], fuente, datos.String)
		}
	}
	if err := vals.Err(); err != nil {
		return nil, err
	}
	vals.Close()

	if err := a.leerEvaluacionesIA(ctx, cond, indice, candidatos, args...); err != nil {
		return nil, err
	}

	archivos, err := a.db.QueryContext(ctx,
		`SELECT ar.postulacion_id, ar.tipo, ar.etapa, ar.ruta
		   FROM archivos ar JOIN postulaciones po ON po.id = ar.postulacion_id
		  WHERE `+cond, args...)
	if err != nil {
		return nil, err
	}
	defer archivos.Close()
	for archivos.Next() {
		var postulacion []byte
		var tipo, ruta string
		var etapa sql.NullString
		if err := archivos.Scan(&postulacion, &tipo, &etapa, &ruta); err != nil {
			return nil, err
		}
		if i, ok := indice[string(postulacion)]; ok {
			a.asignarArchivo(&candidatos[i], tipo, etapa.String, ruta)
		}
	}
	return candidatos, archivos.Err()
}

// armarCandidato reconstruye los datos propios del candidato y su evaluación (sin motivos,
// validaciones ni archivos, que llegan en las consultas siguientes).
func (a *Almacen) armarCandidato(f filaCandidato) (modelos.Candidato, error) {
	cedula, err := a.claves.Descifrar(f.cedulaCifrada)
	if err != nil {
		return modelos.Candidato{}, fmt.Errorf("no se pudo descifrar la cédula (¿cambió RRHH_CLAVE_CIFRADO?): %w", err)
	}
	if cedula == "" {
		cedula = "No especificada" // lo mismo que mostraba el sistema cuando no se leyó la cédula
	}
	c := modelos.Candidato{
		Indice:        int(f.indice.Int64),
		Nombre:        f.nombre.String,
		Cedula:        cedula,
		URLDetalle:    f.urlDetalle.String,
		URLFoto:       f.urlFoto.String,
		URLDescargaCV: f.urlCV.String,
		Genero:        f.genero.String,
		ErrorProceso:  f.errorProceso.String,
		EsApto:        f.dictamen == "apto",
		PostulacionID: idTexto(f.postulacion),
	}
	// Solo los estados de pausa llegan al candidato; "completado", "pendiente"... son del flujo normal.
	if modelos.EnPausa(f.etapa) {
		c.EstadoProceso = f.etapa
	}
	if len(f.nacimientoCifrado) > 0 {
		if texto, err := a.claves.Descifrar(f.nacimientoCifrado); err == nil {
			if nacimiento, err := time.Parse("2006-01-02", texto); err == nil {
				c.Edad = edadEn(nacimiento, time.Now().UTC())
			}
		}
	}
	if f.evaluacionID != nil {
		ev := &modelos.EvaluacionIdoneidad{
			EsApto:                f.evEsApto.Bool,
			Genero:                f.evGenero.String,
			CumpleGenero:          f.cumpleGenero.Bool,
			CumpleEPS:             f.cumpleEPS.Bool,
			DetalleEPS:            f.detalleEPS.String,
			CumpleRunt:            f.cumpleRunt.Bool,
			DetalleRunt:           f.detalleRunt.String,
			CumpleSimit:           f.cumpleSimit.Bool,
			DetalleSimit:          f.detalleSimit.String,
			DescarteTemprano:      f.descarte.Bool,
			VerificacionPendiente: f.verifPendiente.Bool,
		}
		c.EvaluacionIdoneidad = ev
		c.EsApto = ev.EsApto
		if c.Genero == "" {
			c.Genero = ev.Genero
		}
	}
	return c, nil
}

// asignarResultado devuelve al candidato el resultado original de una fuente.
func asignarResultado(c *modelos.Candidato, fuente, datos string) {
	switch fuente {
	case "adres":
		var r modelos.ResultadoAdres
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoAdres = &r
		}
	case "runt":
		var r modelos.ResultadoRunt
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoRunt = &r
		}
	case "simit":
		var r modelos.ResultadoSimit
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoSimit = &r
		}
	case "procuraduria":
		var r modelos.ResultadoProcuraduria
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoProcuraduria = &r
		}
	case "ofac":
		var r modelos.ResultadoOfac
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoOfac = &r
		}
	case "sarlaft":
		var r modelos.ResultadoSarlaft
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoSarlaft = &r
		}
	case "reintegro":
		var r modelos.ResultadoReintegro
		if json.Unmarshal([]byte(datos), &r) == nil {
			c.ResultadoReintegro = &r
		}
	}
}

// asignarArchivo devuelve al candidato la ruta de cada archivo; el texto del perfil se lee del disco.
func (a *Almacen) asignarArchivo(c *modelos.Candidato, tipo, etapa, rel string) {
	ruta := a.rutaAbsoluta(rel)
	switch tipo {
	case "cv_pdf":
		c.RutaPDF = ruta
	case "cv_original":
		c.RutaCVOriginal = ruta
	case "perfil_pdf":
		c.RutaPerfilPDF = ruta
	case "foto":
		c.RutaFoto = ruta
	case "captura":
		if etapa != "" {
			if c.Capturas == nil {
				c.Capturas = map[string]string{}
			}
			c.Capturas[etapa] = ruta
		}
	case "texto_perfil":
		if b, err := os.ReadFile(ruta); err == nil {
			c.TextoPerfil = string(b)
		}
	}
}
