package almacen

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-rrhh/modelos"
)

// Resultados de una validación (columna validaciones.resultado).
const (
	resOK        = "ok"
	resRechazado = "rechazado" // la regla de negocio no se cumple
	resError     = "error"     // la consulta falló: no hay evidencia a favor ni en contra
	resAlerta    = "alerta"    // coincidencia en listas o antecedentes (informativo)
)

// valFuente es una validación lista para guardar: el encabezado común y el detalle de su fuente.
type valFuente struct {
	fuente    string
	resultado string
	codigo    string // estado crudo del portal
	detalle   string
	datos     string // resultado original de la fuente (JSON): con él se reconstruye al candidato
	insertar  func(ctx context.Context, tx *sql.Tx, validacionID []byte) error
}

// jsonDe serializa el resultado original de una fuente.
func jsonDe(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// validaciones arma las validaciones que el candidato trae de las fuentes consultadas. Las fuentes
// que no se consultaron (descarte temprano) no generan fila.
func (a *Almacen) validaciones(c *modelos.Candidato, momento time.Time) []valFuente {
	ev := c.EvaluacionIdoneidad
	var v []valFuente
	if r := c.ResultadoAdres; r != nil {
		v = append(v, valAdres(r, ev))
	}
	if r := c.ResultadoRunt; r != nil {
		v = append(v, valRunt(r, ev, momento))
	}
	if r := c.ResultadoSimit; r != nil {
		v = append(v, valSimit(r, ev))
	}
	if r := c.ResultadoProcuraduria; r != nil {
		v = append(v, valProcuraduria(r))
	}
	if r := c.ResultadoOfac; r != nil {
		v = append(v, valOfac(r))
	}
	if r := c.ResultadoSarlaft; r != nil {
		v = append(v, valSarlaft(r))
	}
	// Solo si la base ya tiene la fuente (migración 005): sin ella, guardar rompería la llave foránea.
	if r := c.ResultadoReintegro; r != nil && a.fuentes["reintegro"] {
		v = append(v, valReintegro(r))
	}
	return v
}

// resultadoReintegro: quien ya fue empleado queda rechazado (no se admiten reintegros).
func resultadoReintegro(r *modelos.ResultadoReintegro) string {
	switch {
	case r.Estado == modelos.ReintegroErrorConsulta:
		return resError
	case r.Alerta:
		return resRechazado
	}
	return resOK
}

func valReintegro(r *modelos.ResultadoReintegro) valFuente {
	return valFuente{
		fuente: "reintegro", resultado: resultadoReintegro(r), codigo: r.Estado, detalle: r.Detalles, datos: jsonDe(r),
	}
}

func exec(ctx context.Context, tx *sql.Tx, consulta string, args ...any) error {
	_, err := tx.ExecContext(ctx, consulta, args...)
	return err
}

// jsonLista serializa una lista; una lista vacía o nula se guarda como [].
func jsonLista[T any](l []T) string {
	if len(l) == 0 {
		return "[]"
	}
	b, err := json.Marshal(l)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// cumpleSegun decide si una regla se cumple: manda el dictamen del clasificador (que es lo que
// decidió la aptitud) salvo que esa regla no se haya evaluado; entonces se usa lo que dice la fuente.
func cumpleSegun(ev *modelos.EvaluacionIdoneidad, detalle string, delClasificador func() bool, delaFuente bool) bool {
	if ev != nil && !noVerificado(detalle) {
		return delClasificador()
	}
	return delaFuente
}

// ---- ADRES ----

func resultadoAdres(r *modelos.ResultadoAdres, ev *modelos.EvaluacionIdoneidad) (resultado string, cumple sql.NullBool, mantenimiento bool) {
	mantenimiento = r.Estado == "PORTAL_EN_MANTENIMIENTO"
	if mantenimiento || strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" {
		return resError, sql.NullBool{}, mantenimiento // sin verificar: no es un "no cumple"
	}
	var detalle string
	if ev != nil {
		detalle = ev.DetalleEPS
	}
	ok := cumpleSegun(ev, detalle, func() bool { return ev.CumpleEPS }, r.CumpleEPS)
	if ok {
		return resOK, nullBool(true), false
	}
	return resRechazado, nullBool(false), false
}

func valAdres(r *modelos.ResultadoAdres, ev *modelos.EvaluacionIdoneidad) valFuente {
	resultado, cumple, mant := resultadoAdres(r, ev)
	return valFuente{
		fuente: "adres", resultado: resultado, codigo: r.Estado, detalle: r.Detalles, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			return exec(ctx, tx,
				`INSERT INTO val_adres (validacion_id, estado_portal, nombres, apellidos, departamento, municipio,
				   eps, regimen, estado_afiliacion, tipo_afiliado, fecha_afiliacion, cumple_eps, en_mantenimiento)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				id, texto(r.Estado, 60), texto(r.Nombres, 120), texto(r.Apellidos, 120), texto(r.Departamento, 120),
				texto(r.Municipio, 120), texto(r.EPS, 120), texto(r.Regimen, 60), texto(r.EstadoAfiliacion, 60),
				texto(r.TipoAfiliado, 60), nullFecha(r.FechaAfiliacion), cumple, mant)
		},
	}
}

// ---- RUNT ----

func resultadoRunt(r *modelos.ResultadoRunt, ev *modelos.EvaluacionIdoneidad) (string, sql.NullBool) {
	switch {
	case strings.HasPrefix(r.Estado, "ERROR"), r.Estado == "NO_CONSULTADO_SIN_CEDULA",
		r.Estado == "SIN_APELLIDO", r.Estado == "APELLIDO_NO_COINCIDE":
		return resError, sql.NullBool{}
	}
	var detalle string
	if ev != nil {
		detalle = ev.DetalleRunt
	}
	ok := cumpleSegun(ev, detalle, func() bool { return ev.CumpleRunt }, r.Estado == "INSCRITO_ACTIVO" && r.Inscrito)
	if ok {
		return resOK, nullBool(true)
	}
	return resRechazado, nullBool(false)
}

func valRunt(r *modelos.ResultadoRunt, ev *modelos.EvaluacionIdoneidad, momento time.Time) valFuente {
	resultado, _ := resultadoRunt(r, ev)
	detalle := r.Error
	return valFuente{
		fuente: "runt", resultado: resultado, codigo: r.Estado, detalle: detalle, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			inscripcion := nullFecha(r.FechaInscripcion)
			var antiguedad sql.NullString
			if inscripcion.Valid {
				anios := momento.Sub(inscripcion.Time).Hours() / 24 / 365.25
				if anios < 0 {
					anios = 0
				}
				antiguedad = sql.NullString{String: fmt.Sprintf("%.2f", anios), Valid: true}
			}
			if err := exec(ctx, tx,
				`INSERT INTO val_runt (validacion_id, estado_portal, inscrito, id_persona_runt, apellido_consultado,
				   fecha_inscripcion, estado_persona, conductor_activo, estado_conductor, tiene_licencias,
				   categorias_activas, tiene_multas, paz_y_salvo, numero_paz_y_salvo, antiguedad_anios)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				id, texto(r.Estado, 60), r.Inscrito, texto(r.IDPersona, 60), texto(r.ApellidoConsultado, 120),
				inscripcion, texto(r.EstadoPersona, 60), r.ConductorActivo, texto(r.EstadoConductor, 60),
				r.TieneLicencias, jsonLista(r.CategoriasActivas), r.TieneMultas, r.PazYSalvo,
				texto(r.NumeroPazYSalvo, 60), antiguedad); err != nil {
				return err
			}
			for _, lic := range r.Licencias {
				if err := insertarLicencia(ctx, tx, id, lic); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func insertarLicencia(ctx context.Context, tx *sql.Tx, validacionID []byte, lic modelos.LicenciaRunt) error {
	licID, err := nuevoID()
	if err != nil {
		return err
	}
	if err := exec(ctx, tx,
		`INSERT INTO runt_licencias (id, validacion_id, numero_licencia, fecha_expedicion, estado, retenida, ot_expide, restricciones)
		 VALUES (?,?,?,?,?,?,?,?)`,
		licID, validacionID, texto(lic.NumeroLicencia, 60), nullFecha(lic.FechaExpedicion), texto(lic.Estado, 40),
		lic.Retenida, texto(lic.OTExpide, 120), texto(lic.Restricciones, 16000)); err != nil {
		return err
	}
	vistas := map[string]bool{}
	insertar := func(categoria, vence, restricciones, limitaciones string) error {
		categoria = strings.ToUpper(strings.TrimSpace(categoria))
		if categoria == "" || vistas[categoria] {
			return nil
		}
		vistas[categoria] = true
		catID, err := nuevoID()
		if err != nil {
			return err
		}
		return exec(ctx, tx,
			`INSERT INTO licencia_categorias (id, licencia_id, categoria, fecha_vencimiento, restricciones, limitaciones)
			 VALUES (?,?,?,?,?,?)`,
			catID, licID, texto(categoria, 10).String, nullFecha(vence), texto(restricciones, 16000), texto(limitaciones, 16000))
	}
	for _, d := range lic.DetallesCategorias {
		if err := insertar(d.Categoria, d.FechaVencimiento, d.Restricciones, d.Limitaciones); err != nil {
			return err
		}
	}
	for _, c := range lic.Categorias {
		if err := insertar(c, "", "", ""); err != nil {
			return err
		}
	}
	return nil
}

// ---- SIMIT ----

func resultadoSimit(r *modelos.ResultadoSimit, ev *modelos.EvaluacionIdoneidad) (string, sql.NullBool) {
	if strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" || r.Estado == "SIN_CEDULA" {
		return resError, sql.NullBool{}
	}
	var detalle string
	if ev != nil {
		detalle = ev.DetalleSimit
	}
	ok := cumpleSegun(ev, detalle, func() bool { return ev.CumpleSimit }, r.CantidadMultas == 0 && (r.PazYSalvo || r.CantidadComparendos == 0))
	if ok {
		return resOK, nullBool(true)
	}
	return resRechazado, nullBool(false)
}

func valSimit(r *modelos.ResultadoSimit, ev *modelos.EvaluacionIdoneidad) valFuente {
	resultado, cumple := resultadoSimit(r, ev)
	return valFuente{
		fuente: "simit", resultado: resultado, codigo: r.Estado, detalle: r.Detalles, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			return exec(ctx, tx,
				`INSERT INTO val_simit (validacion_id, estado_portal, paz_y_salvo, comparendos, multas, acuerdos,
				   deuda_texto, deuda_total, cumple)
				 VALUES (?,?,?,?,?,?,?,?,?)`,
				id, texto(r.Estado, 60), r.PazYSalvo, r.CantidadComparendos, r.CantidadMultas, r.CantidadAcuerdos,
				texto(r.TotalDeuda, 60), nullMonto(r.TotalDeuda), cumple)
		},
	}
}

// ---- Informativas: Procuraduría, OFAC, SARLAFT ----

func resultadoInformativo(tecnico, alerta bool) string {
	switch {
	case tecnico:
		return resError
	case alerta:
		return resAlerta
	}
	return resOK
}

func valProcuraduria(r *modelos.ResultadoProcuraduria) valFuente {
	tecnico := strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" || r.Estado == "SIN_CEDULA"
	return valFuente{
		fuente: "procuraduria", resultado: resultadoInformativo(tecnico, r.Alerta), codigo: r.Estado, detalle: r.Detalles, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			return exec(ctx, tx,
				`INSERT INTO val_procuraduria (validacion_id, estado_portal, coincidencia, con_antecedentes, detalle)
				 VALUES (?,?,?,?,?)`,
				id, texto(r.Estado, 60), r.Coincidencia, r.Estado == "CON_ANTECEDENTES", texto(r.Detalles, 16000))
		},
	}
}

func valOfac(r *modelos.ResultadoOfac) valFuente {
	tecnico := strings.HasPrefix(r.Estado, "ERROR") || strings.HasPrefix(r.Estado, "NO_CONSULTADO") || r.Estado == "SIN_CEDULA"
	return valFuente{
		fuente: "ofac", resultado: resultadoInformativo(tecnico, r.Alerta), codigo: r.Estado, detalle: r.Detalles, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			return exec(ctx, tx,
				`INSERT INTO val_ofac (validacion_id, estado_portal, total_coincidencias, registros) VALUES (?,?,?,?)`,
				id, texto(r.Estado, 60), r.TotalCoincidencias, jsonLista(r.Registros))
		},
	}
}

func valSarlaft(r *modelos.ResultadoSarlaft) valFuente {
	tecnico := r.TipoCoincidencia == "LISTA_NO_DISPONIBLE"
	return valFuente{
		fuente: "sarlaft", resultado: resultadoInformativo(tecnico, r.Alerta), codigo: r.TipoCoincidencia, detalle: r.Detalles, datos: jsonDe(r),
		insertar: func(ctx context.Context, tx *sql.Tx, id []byte) error {
			return exec(ctx, tx,
				`INSERT INTO val_sarlaft (validacion_id, tipo_coincidencia, id_onu, nombre_sancionado) VALUES (?,?,?,?)`,
				id, texto(r.TipoCoincidencia, 40), texto(r.IDONU, 40), texto(r.NombreSancionado, 255))
		},
	}
}
