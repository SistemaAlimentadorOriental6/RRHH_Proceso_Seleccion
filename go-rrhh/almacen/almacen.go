// Package almacen guarda en MySQL/MariaDB lo que el sistema produce al analizar candidatos.
// El esquema está en migraciones/001_inicial.sql.
package almacen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"go-rrhh/configuracion"
)

// Almacen es el acceso a la base. Es seguro para uso concurrente.
type Almacen struct {
	db          *sql.DB
	claves      *claves
	descargas   string
	slug        func(string) string // título de vacante -> slug
	slugPersona func(string) string // nombre -> slug
	reglaID     []byte              // reglas de idoneidad vigentes
	fuentes     map[string]bool     // códigos de la tabla fuentes (las nuevas llegan con migraciones opcionales)

	mu      sync.Mutex
	ofertas map[string][]byte // id_computrabajo -> id
}

// versionEsquema es la última migración que el código necesita ya aplicada.
const versionEsquema = 6

// Abrir conecta con la base descrita en la configuración y comprueba que tenga el esquema.
// slug y slugPersona convierten un título de vacante y un nombre en sus slugs
// (servidor.SlugVacante y servidor.SlugPersona).
func Abrir(ctx context.Context, cfg *configuracion.Configuracion, slug, slugPersona func(string) string) (*Almacen, error) {
	k, err := nuevasClaves(cfg.ClavePseudonimos, cfg.ClaveCifrado)
	if err != nil {
		return nil, fmt.Errorf("claves de privacidad: %w", err)
	}
	if cfg.MySQLUsuario == "" || cfg.MySQLBase == "" {
		return nil, errors.New("faltan RRHH_MYSQL_USUARIO o RRHH_MYSQL_BASE en el .env")
	}
	puerto := cfg.MySQLPuerto
	if puerto == "" {
		puerto = "3306"
	}

	mc := mysql.NewConfig()
	mc.User = cfg.MySQLUsuario
	mc.Passwd = cfg.MySQLClave
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(cfg.MySQLHost, puerto)
	mc.DBName = cfg.MySQLBase
	mc.ParseTime = true
	mc.Loc = time.UTC // todas las fechas se guardan en UTC
	mc.Collation = "utf8mb4_unicode_ci"
	mc.Timeout = 10 * time.Second
	mc.ReadTimeout = 30 * time.Second
	mc.WriteTimeout = 30 * time.Second

	db, err := sql.Open("mysql", mc.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("abriendo la conexión: %w", err)
	}
	// El análisis guarda mientras las pantallas leen: con 10 conexiones las lecturas hacían fila y
	// retrasaban el guardado de cada candidato. Las ociosas se conservan para no reabrir TCP a la base remota.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	a, err := nuevo(ctx, db, k, cfg.DirectorioDescargas, slug, slugPersona)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return a, nil
}

func nuevo(ctx context.Context, db *sql.DB, k *claves, descargas string, slug, slugPersona func(string) string) (*Almacen, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("no se pudo conectar con la base de datos: %w", err)
	}

	var version sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migraciones`).Scan(&version)
	switch {
	case err != nil:
		return nil, fmt.Errorf("no se pudo leer schema_migraciones (¿se aplicó almacen/migraciones/001_inicial.sql?): %w", err)
	case !version.Valid:
		return nil, errors.New("schema_migraciones existe pero está vacía: la migración 001_inicial.sql no llegó a su última sentencia (el INSERT que registra la versión)")
	case version.Int64 < versionEsquema:
		return nil, fmt.Errorf("el esquema está en la versión %d y se necesita la %d: aplique las migraciones pendientes de almacen/migraciones/ en orden", version.Int64, versionEsquema)
	}

	// El número de versión no basta: una migración puede haberse registrado sin aplicarse entera.
	if faltan, err := columnasFaltantes(ctx, db); err != nil {
		return nil, fmt.Errorf("verificando las columnas de la base: %w", err)
	} else if len(faltan) > 0 {
		return nil, fmt.Errorf("a la base le faltan columnas (%s): ejecute completos 002 a 006 de almacen/migraciones/, que son seguros de repetir", strings.Join(faltan, ", "))
	}

	a := &Almacen{db: db, claves: k, descargas: descargas, slug: slug, slugPersona: slugPersona, ofertas: map[string][]byte{}, fuentes: map[string]bool{}}
	if filas, err := db.QueryContext(ctx, `SELECT codigo FROM fuentes`); err == nil {
		for filas.Next() {
			var codigo string
			if filas.Scan(&codigo) == nil {
				a.fuentes[codigo] = true
			}
		}
		_ = filas.Close()
	}
	err = db.QueryRowContext(ctx,
		`SELECT id FROM reglas_idoneidad WHERE vigente_hasta IS NULL ORDER BY version DESC LIMIT 1`).Scan(&a.reglaID)
	if err != nil {
		return nil, fmt.Errorf("la base no tiene reglas de idoneidad vigentes: %w", err)
	}
	return a, nil
}

// columnasNecesarias son las columnas agregadas por la migración 002, de las que depende el guardado.
var columnasNecesarias = []string{
	"perfiles.indice", "perfiles.nombre", "perfiles.slug_persona", "perfiles.url_foto",
	"archivos.etapa", "validaciones.datos", "postulaciones.activa",
	"solicitudes_datos.token_hash", "datos_declarados.cedula_hmac", "postulacion_historial.tipo",
	"usuarios.clave_hash", "sesiones.token_hash", "perfiles.edad",
	"evaluaciones_idoneidad.genero", "evaluaciones_idoneidad.detalle_eps",
	"evaluaciones_idoneidad.detalle_runt", "evaluaciones_idoneidad.detalle_simit",
}

// columnasFaltantes devuelve cuáles de columnasNecesarias no existen en la base.
func columnasFaltantes(ctx context.Context, db *sql.DB) ([]string, error) {
	filas, err := db.QueryContext(ctx,
		`SELECT CONCAT(table_name, '.', column_name) FROM information_schema.columns
		  WHERE table_schema = DATABASE() AND table_name IN ('perfiles', 'archivos', 'evaluaciones_idoneidad', 'validaciones', 'postulaciones',
		                                   'solicitudes_datos', 'datos_declarados', 'postulacion_historial', 'usuarios', 'sesiones')`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	existen := map[string]bool{}
	for filas.Next() {
		var c string
		if err := filas.Scan(&c); err != nil {
			return nil, err
		}
		existen[strings.ToLower(c)] = true
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	var faltan []string
	for _, c := range columnasNecesarias {
		if !existen[c] {
			faltan = append(faltan, c)
		}
	}
	return faltan, nil
}

// Cerrar libera las conexiones.
func (a *Almacen) Cerrar() error { return a.db.Close() }

// idTexto devuelve un id binario como UUID en texto ("" si no es un UUID).
func idTexto(b []byte) string {
	u, err := uuid.FromBytes(b)
	if err != nil {
		return ""
	}
	return u.String()
}

// idBytes convierte un UUID en texto al id binario con el que se guarda.
func idBytes(s string) ([]byte, error) {
	u, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("identificador inválido: %w", err)
	}
	return u[:], nil
}

// nuevoID genera un UUID v7: ordenable por tiempo, así los índices crecen por el final.
func nuevoID() ([]byte, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return u[:], nil
}

// enTx ejecuta fn en una transacción y la repite si MySQL aborta por interbloqueo o espera de
// bloqueo (varios candidatos se guardan a la vez). fn debe poder ejecutarse más de una vez.
func (a *Almacen) enTx(ctx context.Context, fn func(*sql.Tx) error) error {
	var err error
	for intento := 1; intento <= 3; intento++ {
		err = a.unaTx(ctx, fn)
		if !reintentable(err) {
			return err
		}
		select {
		case <-time.After(time.Duration(intento) * 100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (a *Almacen) unaTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func reintentable(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && (me.Number == 1213 || me.Number == 1205)
}

func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
