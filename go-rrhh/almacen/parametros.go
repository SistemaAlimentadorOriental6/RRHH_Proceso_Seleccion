package almacen

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"go-rrhh/modelos"
)

// maxMeses es el tope de los tiempos de experiencia: 60 años.
const maxMeses = 720

// ParametrosAceptacion son los criterios que debe cumplir una hoja de vida para ser aceptada. Se guardan
// como una versión de reglas_idoneidad (migraciones 008 y 009); al guardar, Version se ignora.
type ParametrosAceptacion = modelos.ReglasIdoneidad

// VersionReglas resume una versión de las reglas para el historial.
type VersionReglas struct {
	Version      int        `json:"version"`
	VigenteDesde time.Time  `json:"vigenteDesde"`
	VigenteHasta *time.Time `json:"vigenteHasta,omitempty"` // nil = la vigente
}

// ErrVersionInexistente indica que no hay reglas con ese número de versión.
var ErrVersionInexistente = errors.New("no existe esa versión de reglas")

// ErrParametrosInvalidos envuelve el motivo por el que los parámetros no se aceptan (se muestra tal cual).
var ErrParametrosInvalidos = errors.New("parámetros inválidos")

// ErrMigracionPendiente indica que la base aún no tiene las columnas de las migraciones 008 y 009.
var ErrMigracionPendiente = errors.New("faltan aplicar las migraciones 008_parametros_aceptacion.sql y 009_meses_experiencia.sql en la base de datos")

var (
	generosValidos    = map[string]string{"masculino": "Hombre", "femenino": "Mujer"} // genero_requerido histórico
	licenciasValidas  = map[string]bool{"A1": true, "A2": true, "B1": true, "B2": true, "B3": true, "C1": true, "C2": true, "C3": true}
	maxEps            = 60
	maxLargoNombreEps = 60
)

// Normalizar limpia espacios y repetidos y devuelve el primer error de validación. Se valida aquí aunque
// la pantalla ya lo haga: la API es la que protege la base.
func Normalizar(p *ParametrosAceptacion) error {
	invalido := func(f string, a ...any) error {
		return fmt.Errorf("%w: %s", ErrParametrosInvalidos, fmt.Sprintf(f, a...))
	}
	if p.EdadMinima < 18 || p.EdadMinima > 99 {
		return invalido("la edad inicial debe estar entre 18 y 99")
	}
	if p.EdadMaxima < p.EdadMinima || p.EdadMaxima > 99 {
		return invalido("la edad final debe estar entre la inicial y 99")
	}
	if p.ExperienciaMinimaMeses < 0 || p.ExperienciaMinimaMeses > maxMeses {
		return invalido("la experiencia mínima debe estar entre 0 y 60 años")
	}
	if p.RuntAntiguedadMeses < 0 || p.RuntAntiguedadMeses > maxMeses {
		return invalido("el tiempo inscrito en el RUNT debe estar entre 0 y 60 años")
	}

	var err error
	if p.GenerosAceptados, err = sinRepetidos(p.GenerosAceptados, "género", 2, 20, func(s string) bool { _, ok := generosValidos[s]; return ok }); err != nil {
		return invalido("%v", err)
	}
	if p.LicenciasAceptadas, err = sinRepetidos(p.LicenciasAceptadas, "licencia", len(licenciasValidas), 3, func(s string) bool { return licenciasValidas[s] }); err != nil {
		return invalido("%v", err)
	}
	if p.EpsAceptadas, err = sinRepetidos(p.EpsAceptadas, "EPS", maxEps, maxLargoNombreEps, nil); err != nil {
		return invalido("%v", err)
	}
	return nil
}

// sinRepetidos recorta, descarta repetidos (sin distinguir mayúsculas) y exige que la lista no quede vacía.
func sinRepetidos(l []string, nombre string, max, largo int, valido func(string) bool) ([]string, error) {
	vistos := map[string]bool{}
	salida := make([]string, 0, len(l))
	for _, v := range l {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, fmt.Errorf("hay un valor de %s vacío", nombre)
		}
		if len([]rune(v)) > largo || (valido != nil && !valido(v)) {
			return nil, fmt.Errorf("%s no válido: %q", nombre, v)
		}
		if k := strings.ToLower(v); !vistos[k] {
			vistos[k] = true
			salida = append(salida, v)
		}
	}
	switch {
	case len(salida) == 0:
		return nil, fmt.Errorf("selecciona al menos un valor de %s", nombre)
	case len(salida) > max:
		return nil, fmt.Errorf("demasiados valores de %s", nombre)
	}
	return salida, nil
}

// ParametrosVigentes lee la versión vigente de las reglas.
func (a *Almacen) ParametrosVigentes(ctx context.Context) (ParametrosAceptacion, error) {
	return leerVigente(a.db.QueryRowContext(ctx, consultaVigente))
}

// ParametrosVersion lee una versión cualquiera de las reglas (las versiones no cambian después de guardarse).
func (a *Almacen) ParametrosVersion(ctx context.Context, version int) (ParametrosAceptacion, error) {
	p, err := leerVigente(a.db.QueryRowContext(ctx, columnasReglas+` WHERE version = ?`, version))
	if errors.Is(err, sql.ErrNoRows) {
		return p, fmt.Errorf("%w: %d", ErrVersionInexistente, version)
	}
	return p, err
}

// VersionesParametros lista todas las versiones de reglas, de la más nueva a la más antigua.
func (a *Almacen) VersionesParametros(ctx context.Context) ([]VersionReglas, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT version, vigente_desde, vigente_hasta FROM reglas_idoneidad ORDER BY version DESC`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	lista := []VersionReglas{}
	for filas.Next() {
		var v VersionReglas
		var hasta sql.NullTime
		if err := filas.Scan(&v.Version, &v.VigenteDesde, &hasta); err != nil {
			return nil, err
		}
		if hasta.Valid {
			v.VigenteHasta = &hasta.Time
		}
		lista = append(lista, v)
	}
	return lista, filas.Err()
}

const columnasReglas = `SELECT version, eps_autorizadas, runt_categorias, edad_min, edad_max, acepta_reintegros,
	generos_aceptados, experiencia_min_meses, runt_antiguedad_meses FROM reglas_idoneidad`

const consultaVigente = columnasReglas + ` WHERE vigente_hasta IS NULL ORDER BY version DESC LIMIT 1`

func leerVigente(fila *sql.Row) (ParametrosAceptacion, error) {
	var (
		p                      ParametrosAceptacion
		eps, lic               string
		generos                sql.NullString
		emin, emax             sql.NullInt64
		reintegros             sql.NullBool
		experiencia, runtMeses sql.NullInt64
	)
	if err := fila.Scan(&p.Version, &eps, &lic, &emin, &emax, &reintegros, &generos, &experiencia, &runtMeses); err != nil {
		return p, traducirColumnas(err)
	}
	if err := json.Unmarshal([]byte(eps), &p.EpsAceptadas); err != nil {
		return p, fmt.Errorf("eps_autorizadas ilegible: %w", err)
	}
	if err := json.Unmarshal([]byte(lic), &p.LicenciasAceptadas); err != nil {
		return p, fmt.Errorf("runt_categorias ilegible: %w", err)
	}
	if !emin.Valid || !emax.Valid || !generos.Valid || !experiencia.Valid || !runtMeses.Valid {
		return p, ErrMigracionPendiente // las migraciones no llegaron a rellenar la versión vigente
	}
	if err := json.Unmarshal([]byte(generos.String), &p.GenerosAceptados); err != nil {
		return p, fmt.Errorf("generos_aceptados ilegible: %w", err)
	}
	p.EdadMinima, p.EdadMaxima = int(emin.Int64), int(emax.Int64)
	p.AceptaReintegros = reintegros.Bool
	p.ExperienciaMinimaMeses, p.RuntAntiguedadMeses = int(experiencia.Int64), int(runtMeses.Int64)
	return p, nil
}

// traducirColumnas convierte "Unknown column" (1054) en ErrMigracionPendiente.
func traducirColumnas(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1054 {
		return ErrMigracionPendiente
	}
	return err
}

// anios convierte meses a años con un decimal, para las columnas de años que siguen existiendo.
func anios(meses int) float64 {
	return float64(meses) / 12
}

// GuardarParametros crea una versión nueva de las reglas con los parámetros, cierra la anterior y deja
// una fila en auditoria. Lo que no se edita (SIMIT) se copia de la versión que estaba vigente.
func (a *Almacen) GuardarParametros(ctx context.Context, p ParametrosAceptacion, quien string) (ParametrosAceptacion, error) {
	if err := Normalizar(&p); err != nil {
		return p, err
	}
	eps, _ := json.Marshal(p.EpsAceptadas)
	lic, _ := json.Marshal(p.LicenciasAceptadas)
	gen, _ := json.Marshal(p.GenerosAceptados)
	requerido := sql.NullString{} // ambos géneros = sin requisito
	if len(p.GenerosAceptados) == 1 {
		requerido = sql.NullString{String: generosValidos[p.GenerosAceptados[0]], Valid: true}
	}

	nuevoUUID, err := nuevoID()
	if err != nil {
		return p, err
	}
	var guardada ParametrosAceptacion
	err = a.enTx(ctx, func(tx *sql.Tx) error {
		var version int
		var anterior []byte
		var simitMax int
		var simitDeuda float64
		// FOR UPDATE: dos guardados a la vez se ponen en fila en vez de crear dos versiones vigentes.
		if err := tx.QueryRowContext(ctx,
			`SELECT id, version, simit_max_multas, simit_deuda_max FROM reglas_idoneidad
			  WHERE vigente_hasta IS NULL ORDER BY version DESC LIMIT 1 FOR UPDATE`).Scan(&anterior, &version, &simitMax, &simitDeuda); err != nil {
			return err
		}
		ahora := time.Now().UTC()
		if err := exec(ctx, tx, `UPDATE reglas_idoneidad SET vigente_hasta = ? WHERE id = ?`, ahora, anterior); err != nil {
			return err
		}
		if err := exec(ctx, tx,
			`INSERT INTO reglas_idoneidad (id, version, genero_requerido, eps_autorizadas, runt_antiguedad_anios, runt_categorias,
			   simit_max_multas, simit_deuda_max, vigente_desde, vigente_hasta, creado_en,
			   edad_min, edad_max, acepta_reintegros, generos_aceptados,
			   experiencia_min_anios, experiencia_min_meses, runt_antiguedad_meses)
			 VALUES (?,?,?,?,?,?,?,?,?,NULL,?,?,?,?,?,?,?,?)`,
			nuevoUUID, version+1, requerido, string(eps), anios(p.RuntAntiguedadMeses), string(lic),
			simitMax, simitDeuda, ahora, ahora,
			p.EdadMinima, p.EdadMaxima, p.AceptaReintegros, string(gen),
			anios(p.ExperienciaMinimaMeses), p.ExperienciaMinimaMeses, p.RuntAntiguedadMeses); err != nil {
			return err
		}
		detalle, _ := json.Marshal(map[string]any{"version": version + 1, "anterior": version})
		if err := exec(ctx, tx,
			`INSERT INTO auditoria (quien, accion, objetivo_tipo, objetivo_id, detalle, creado_en) VALUES (?,?,?,?,?,?)`,
			texto(quien, 120), "parametros.guardar", "reglas_idoneidad", nuevoUUID, string(detalle), ahora); err != nil {
			return err
		}
		var err error
		guardada, err = leerVigente(tx.QueryRowContext(ctx, consultaVigente))
		return err
	})
	if err != nil {
		return p, traducirColumnas(err)
	}

	a.muRegla.Lock()
	a.reglaID = nuevoUUID // los jobs y evaluaciones nuevos apuntan a las reglas recién guardadas
	a.muRegla.Unlock()
	return guardada, nil
}

// reglaDeVersion es el id de la versión de reglas con la que se evaluó al candidato. La versión 0 (evaluado
// sin leer reglas de la base) queda con la vigente, como antes.
func (a *Almacen) reglaDeVersion(ctx context.Context, tx *sql.Tx, version int) ([]byte, error) {
	if version <= 0 {
		return a.reglaVigente(), nil
	}
	var id []byte
	err := tx.QueryRowContext(ctx, `SELECT id FROM reglas_idoneidad WHERE version = ?`, version).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %d", ErrVersionInexistente, version)
	}
	return id, err
}

// reglaVigente es el id de las reglas con las que se evalúa y se programa trabajo ahora.
func (a *Almacen) reglaVigente() []byte {
	a.muRegla.RLock()
	defer a.muRegla.RUnlock()
	return a.reglaID
}
