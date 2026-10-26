package almacen

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"go-rrhh/modelos"
)

// Esta prueba corre contra un servidor real y SOLO si se define RRHH_TEST_MYSQL_DSN, por ejemplo:
//
//	usuario:clave@tcp(servidor:3306)/rrhh_prueba
//
// Debe apuntar a una base VACÍA y exclusiva para pruebas: la prueba crea el esquema, lo usa y borra
// todas sus tablas al terminar. Si la base tiene tablas, se niega a correr.
func baseDePrueba(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("RRHH_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("RRHH_TEST_MYSQL_DSN no está definida: se omite la prueba de integración")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("RRHH_TEST_MYSQL_DSN inválido: %v", err)
	}
	cfg.MultiStatements = true
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Collation = "utf8mb4_unicode_ci"
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	var tablas int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tablas); err != nil {
		t.Fatal(err)
	}
	if tablas != 0 {
		t.Fatalf("la base de pruebas tiene %d tablas: use una base vacía y exclusiva para pruebas", tablas)
	}
	for _, archivo := range []string{"001_inicial.sql", "002_datos_para_leer.sql", "003_lectura.sql", "004_solicitud_datos.sql"} {
		migracion, err := os.ReadFile("migraciones/" + archivo)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(migracion)); err != nil {
			t.Fatalf("aplicando %s: %v", archivo, err)
		}
	}
	t.Cleanup(func() { borrarTablas(db) })
	return db
}

func borrarTablas(db *sql.DB) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return
	}
	defer conn.Close()
	filas, err := conn.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`)
	if err != nil {
		return
	}
	var nombres []string
	for filas.Next() {
		var n string
		if filas.Scan(&n) == nil {
			nombres = append(nombres, n)
		}
	}
	filas.Close()
	_, _ = conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0`)
	for _, n := range nombres {
		_, _ = conn.ExecContext(ctx, "DROP TABLE IF EXISTS `"+n+"`")
	}
	_, _ = conn.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1`)
}

func slugDePrueba(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "-")) }

func contar(t *testing.T, db *sql.DB, consulta string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(consulta, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", consulta, err)
	}
	return n
}

func candidatoCompleto(t *testing.T) *modelos.Candidato {
	carpeta := t.TempDir()
	pdf := filepath.Join(carpeta, "cv.pdf")
	captura := filepath.Join(carpeta, "perfil.jpg")
	for _, f := range []string{pdf, captura} {
		if err := os.WriteFile(f, []byte("contenido de "+filepath.Base(f)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &modelos.Candidato{
		Indice: 1, Nombre: "Carlos Prueba", Cedula: "1111111", URLDetalle: "https://x/Detail/1",
		Genero: "Hombre", TextoPerfil: "Perfil de prueba con texto suficiente",
		CarpetaCandidato: carpeta, RutaPDF: pdf, Capturas: map[string]string{"perfil": captura},
		ResultadoAdres: &modelos.ResultadoAdres{
			Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO", FechaAfiliacion: "15/03/2018", CumpleEPS: true,
		},
		ResultadoRunt: &modelos.ResultadoRunt{
			Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01T00:00:00.000-05:00",
			CategoriasActivas: []string{"C2"},
			Licencias: []modelos.LicenciaRunt{{
				NumeroLicencia: "L1", Estado: "ACTIVA", FechaExpedicion: "2016-02-03", Categorias: []string{"C2", "B1"},
				DetallesCategorias: []modelos.DetalleCategoria{{Categoria: "C2", FechaVencimiento: "2030-02-03"}},
			}},
		},
		ResultadoSimit:        &modelos.ResultadoSimit{Estado: "PAZ_Y_SALVO", PazYSalvo: true, TotalDeuda: "$ 0"},
		ResultadoProcuraduria: &modelos.ResultadoProcuraduria{Estado: "SIN_ANTECEDENTES", Detalles: "Sin antecedentes"},
		ResultadoSarlaft:      &modelos.ResultadoSarlaft{Detalles: "Sin coincidencias"},
		ResultadoOfac:         &modelos.ResultadoOfac{Estado: "SIN_COINCIDENCIAS", Registros: []modelos.RegistroOfac{}},
		EsApto:                true,
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{
			EsApto: true, Genero: "Hombre", CumpleGenero: true,
			CumpleEPS: true, DetalleEPS: "EPS SURA (ACTIVO)",
			CumpleRunt: true, DetalleRunt: "RUNT OK: 11.0 años de registro | Licencia C2/C3 ACTIVA",
			CumpleSimit: true, DetalleSimit: "SIMIT OK: 0 multas y Paz y Salvo",
		},
	}
}

func TestIntegracionGuardarCandidato(t *testing.T) {
	db := baseDePrueba(t)
	ctx := context.Background()
	k, err := nuevasClaves(claveA, claveB)
	if err != nil {
		t.Fatal(err)
	}
	a, err := nuevo(ctx, db, k, t.TempDir(), slugDePrueba, slugDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	oferta := modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}
	c := candidatoCompleto(t)

	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}

	for tabla, esperado := range map[string]int{
		"ofertas": 1, "personas": 1, "postulaciones": 1, "perfiles": 1,
		"validaciones": 6, "val_adres": 1, "val_runt": 1, "val_simit": 1, "val_procuraduria": 1, "val_ofac": 1, "val_sarlaft": 1,
		"runt_licencias": 1, "licencia_categorias": 2, // C2 (con vencimiento) y B1
		"evaluaciones_idoneidad": 1, "archivos": 3, // cv.pdf, la captura del perfil y perfil.txt
	} {
		if n := contar(t, db, "SELECT COUNT(*) FROM "+tabla); n != esperado {
			t.Errorf("%s: %d filas; quería %d", tabla, n, esperado)
		}
	}

	// La cédula no está en claro y se recupera con la clave.
	var cifrada []byte
	var hmacCedula string
	if err := db.QueryRow(`SELECT cedula_cifrada, cedula_hmac FROM personas`).Scan(&cifrada, &hmacCedula); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cifrada), "1111111") || strings.Contains(hmacCedula, "1111111") {
		t.Error("la cédula no debe estar en claro en la base")
	}
	if claro, err := k.Descifrar(cifrada); err != nil || claro != "1111111" {
		t.Errorf("descifrado = %q, %v", claro, err)
	}

	var dictamen string
	if err := db.QueryRow(`SELECT dictamen FROM postulaciones`).Scan(&dictamen); err != nil || dictamen != "apto" {
		t.Errorf("dictamen = %q, %v", dictamen, err)
	}

	// Lo necesario para reconstruir al candidato sin los JSON.
	var nombre, slug, detalleRunt string
	if err := db.QueryRow(`SELECT p.nombre, p.slug_persona, e.detalle_runt
	                         FROM perfiles p JOIN evaluaciones_idoneidad e ON e.postulacion_id = p.postulacion_id`).Scan(&nombre, &slug, &detalleRunt); err != nil {
		t.Fatal(err)
	}
	if nombre != "Carlos Prueba" || slug != "carlos-prueba" || !strings.HasPrefix(detalleRunt, "RUNT OK") {
		t.Errorf("perfil/evaluación = %q, %q, %q", nombre, slug, detalleRunt)
	}

	// Guardar de nuevo: misma persona y postulación; el historial crece y queda una sola evaluación vigente.
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM personas"); n != 1 {
		t.Errorf("personas = %d tras reguardar", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM postulaciones"); n != 1 {
		t.Errorf("postulaciones = %d tras reguardar", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM validaciones"); n != 12 {
		t.Errorf("validaciones = %d; el historial debe crecer a 12", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM evaluaciones_idoneidad"); n != 2 {
		t.Errorf("evaluaciones = %d; quería 2 en el historial", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM evaluaciones_idoneidad WHERE vigente = 1"); n != 1 {
		t.Errorf("evaluaciones vigentes = %d; debe haber una sola", n)
	}

	// Importar a alguien que ya está no hace nada.
	creado, err := a.ImportarCandidato(ctx, oferta, c, time.Now())
	if err != nil || creado {
		t.Errorf("ImportarCandidato de uno existente = %v, %v", creado, err)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM validaciones"); n != 12 {
		t.Errorf("importar no debe agregar historial: %d", n)
	}
}

func TestIntegracionCasosEspeciales(t *testing.T) {
	db := baseDePrueba(t)
	ctx := context.Background()
	k, _ := nuevasClaves(claveA, claveB)
	a, err := nuevo(ctx, db, k, t.TempDir(), slugDePrueba, slugDePrueba)
	if err != nil {
		t.Fatal(err)
	}

	// Descarte temprano por género: sin cédula ni consultas; las reglas no evaluadas quedan en NULL.
	sinCedula := &modelos.Candidato{
		Indice: 2, Nombre: "Maria Prueba", Cedula: "No especificada", URLDetalle: "https://x/Detail/2", Genero: "Mujer",
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{
			Genero: "Mujer", CumpleGenero: false,
			DetalleEPS: modelos.DetalleNoConsultado, DetalleRunt: modelos.DetalleNoConsultado, DetalleSimit: modelos.DetalleNoConsultado,
			MotivosDescarte:  []string{"Género (Mujer) no cumple requisito (requerido: Hombre)"},
			DescarteTemprano: true,
		},
	}
	if err := a.GuardarCandidato(ctx, modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}, sinCedula); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM evaluaciones_idoneidad WHERE cumple_eps IS NULL AND cumple_runt IS NULL AND cumple_simit IS NULL AND cumple_genero = 0"); n != 1 {
		t.Errorf("las reglas no evaluadas deben quedar en NULL (filas que cumplen: %d)", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM postulacion_motivos WHERE tipo = 'genero'"); n != 1 {
		t.Errorf("motivo de género = %d", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM validaciones"); n != 0 {
		t.Errorf("sin consultas no debe haber validaciones: %d", n)
	}

	// La oferta importada con id provisional se reemplaza por el id real al analizarla en vivo.
	if _, err := a.ImportarCandidato(ctx, modelos.OfertaLaboral{ID: IDCarpeta("Vacante_X"), Titulo: "Vacante X"},
		&modelos.Candidato{Nombre: "Pedro", Cedula: "9999999", URLDetalle: "https://x/Detail/9"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := a.GuardarCandidato(ctx, modelos.OfertaLaboral{ID: "REAL456", Titulo: "Vacante X"},
		&modelos.Candidato{Nombre: "Pedro", Cedula: "9999999", URLDetalle: "https://x/Detail/9"}); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM ofertas WHERE slug = 'vacante-x'"); n != 1 {
		t.Errorf("la oferta importada debe adoptarse y no duplicarse: %d", n)
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM ofertas WHERE id_computrabajo = 'REAL456'"); n != 1 {
		t.Error("la oferta debe quedar con el id real de Computrabajo")
	}
	if n := contar(t, db, "SELECT COUNT(*) FROM postulaciones po JOIN ofertas o ON o.id = po.oferta_id WHERE o.slug = 'vacante-x'"); n != 1 {
		t.Errorf("la postulación debe seguir siendo una sola: %d", n)
	}
}
