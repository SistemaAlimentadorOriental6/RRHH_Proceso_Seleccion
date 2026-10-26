package almacen

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-rrhh/modelos"
)

const (
	claveA = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	claveB = "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"
)

func TestClavesCifradoYPseudonimo(t *testing.T) {
	k, err := nuevasClaves(claveA, claveB)
	if err != nil {
		t.Fatal(err)
	}

	cifrado, err := k.Cifrar("1035126774")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cifrado), "1035126774") {
		t.Error("la cédula aparece en claro dentro del dato cifrado")
	}
	otro, _ := k.Cifrar("1035126774")
	if string(cifrado) == string(otro) {
		t.Error("dos cifrados del mismo valor no deben ser iguales (nonce aleatorio)")
	}
	claro, err := k.Descifrar(cifrado)
	if err != nil || claro != "1035126774" {
		t.Errorf("Descifrar = %q, %v", claro, err)
	}
	cifrado[len(cifrado)-1] ^= 1
	if _, err := k.Descifrar(cifrado); err == nil {
		t.Error("un dato alterado debe fallar al descifrar")
	}

	if k.Pseudonimo("cc:123") != k.Pseudonimo("cc:123") {
		t.Error("el pseudónimo debe ser determinista")
	}
	if len(k.Pseudonimo("cc:123")) != 64 {
		t.Error("el pseudónimo debe tener 64 caracteres hexadecimales")
	}
	k2, _ := nuevasClaves(claveB, claveA)
	if k.Pseudonimo("cc:123") == k2.Pseudonimo("cc:123") {
		t.Error("con otra clave el pseudónimo debe cambiar")
	}
}

func TestClavesInvalidas(t *testing.T) {
	for nombre, par := range map[string][2]string{
		"vacía":       {"", claveB},
		"corta":       {"abcd", claveB},
		"no hex":      {strings.Repeat("zz", 32), claveB},
		"iguales":     {claveA, claveA},
		"cifrado mal": {claveA, "12"},
	} {
		if _, err := nuevasClaves(par[0], par[1]); err == nil {
			t.Errorf("%s: se esperaba error", nombre)
		}
	}
}

func TestParseMonto(t *testing.T) {
	for entrada, esperado := range map[string]string{
		"$ 0":             "0.00",
		"$ 1.250.000":     "1250000.00",
		"$ 1.250.000,50":  "1250000.50",
		"$ 1,250,000.50":  "1250000.50",
		"1.250":           "1250.00",
		"12,5":            "12.50",
		"  $ 980.000  ":   "980000.00",
		"$1.000.000":      "1000000.00",
		"deuda: 45.300 $": "45300.00",
	} {
		got, ok := parseMonto(entrada)
		if !ok || got != esperado {
			t.Errorf("parseMonto(%q) = %q, %v; quería %q", entrada, got, ok, esperado)
		}
	}
	for _, vacia := range []string{"", "sin deuda", "$", "..,"} {
		if got, ok := parseMonto(vacia); ok {
			t.Errorf("parseMonto(%q) = %q; no debía reconocerse", vacia, got)
		}
	}
}

func TestParseFecha(t *testing.T) {
	casos := map[string]time.Time{
		"2019-03-04T10:20:30.000-05:00": time.Date(2019, 3, 4, 15, 20, 30, 0, time.UTC),
		"2020-01-31":                    time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC),
		"31/01/2020":                    time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC),
		"2024-05-06 07:08:09":           time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
	}
	for entrada, esperado := range casos {
		got, ok := parseFecha(entrada)
		if !ok || !got.Equal(esperado) {
			t.Errorf("parseFecha(%q) = %v, %v; quería %v", entrada, got, ok, esperado)
		}
	}
	for _, mala := range []string{"", "  ", "ayer", "31-31-2020"} {
		if _, ok := parseFecha(mala); ok {
			t.Errorf("parseFecha(%q) no debía reconocerse", mala)
		}
	}
}

func TestTextoRecortaPorCaracteres(t *testing.T) {
	if texto("   ", 10).Valid {
		t.Error("un texto vacío debe ser NULL")
	}
	// "ñ" ocupa 2 bytes: recortar por bytes cortaría el carácter a la mitad.
	got := texto("ñññññ", 3)
	if got.String != "ñññ" || !got.Valid {
		t.Errorf("texto = %q", got.String)
	}
}

func TestNoVerificado(t *testing.T) {
	for _, d := range []string{
		modelos.DetalleNoConsultado,
		"ADRES no verificado (ERROR_CONSULTA): sin detalle",
		"RUNT no verificado (SIN_APELLIDO): x",
		"SIMIT no verificado (ERROR_CONSULTA)",
		"ADRES en mantenimiento oficial (Pendiente validación EPS)",
		"Sin información de RUNT",
	} {
		if !noVerificado(d) {
			t.Errorf("%q debía contarse como no verificado", d)
		}
	}
	for _, d := range []string{"RUNT OK: 5.0 años de registro | Licencia C2/C3 ACTIVA", "No posee licencia activa en categoría C2 o C3", ""} {
		if noVerificado(d) {
			t.Errorf("%q no debía contarse como no verificado", d)
		}
	}
}

func TestTriEstado(t *testing.T) {
	if got := triEstado(true, "SIMIT OK: 0 multas y Paz y Salvo"); got != (sql.NullBool{Bool: true, Valid: true}) {
		t.Errorf("cumple = %+v", got)
	}
	if got := triEstado(false, "SIMIT Rechazado: 2 multas"); got != (sql.NullBool{Bool: false, Valid: true}) {
		t.Errorf("no cumple = %+v", got)
	}
	if got := triEstado(false, modelos.DetalleNoConsultado); got.Valid {
		t.Errorf("sin evaluar debe ser NULL, fue %+v", got)
	}
	// Portal en mantenimiento: el clasificador lo da por cumplido, pero la base no debe afirmarlo.
	if got := triEstado(true, "ADRES en mantenimiento oficial (Pendiente validación EPS)"); got.Valid {
		t.Errorf("en mantenimiento debe ser NULL, fue %+v", got)
	}
}

func TestTipoMotivo(t *testing.T) {
	ev := &modelos.EvaluacionIdoneidad{
		DetalleEPS:   "EPS 'X' no autorizada (Solo Sura, Nueva EPS, Savia Salud, Sanitas)",
		DetalleRunt:  "No posee licencia activa en categoría C2 o C3",
		DetalleSimit: "SIMIT Rechazado: 2 multas de tránsito",
	}
	casos := map[string]string{
		"Género (Mujer) no cumple requisito (requerido: Hombre)":                        "genero",
		"Cédula no encontrada en el perfil: no se puede consultar RUNT, SIMIT ni ADRES": "cedula",
		ev.DetalleEPS:   "eps",
		ev.DetalleRunt:  "runt",
		ev.DetalleSimit: "simit",
		"Verificación pendiente: RUNT no verificado (ERROR_CONSULTA): x": "pendiente",
	}
	for motivo, esperado := range casos {
		if got := tipoMotivo(motivo, ev); got != esperado {
			t.Errorf("tipoMotivo(%q) = %q; quería %q", motivo, got, esperado)
		}
	}
	// Texto desconocido: se atribuye a la primera regla que no se cumple.
	ev.CumpleGenero, ev.CumpleEPS = true, true
	if got := tipoMotivo("texto raro", ev); got != "runt" {
		t.Errorf("fallback = %q; quería runt", got)
	}
}

func TestEstadoPostulacion(t *testing.T) {
	casos := []struct {
		nombre          string
		c               modelos.Candidato
		etapa, dictamen string
		pendiente       bool
	}{
		{"sin evaluación", modelos.Candidato{}, "perfil", "pendiente", true},
		{"apto", modelos.Candidato{EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: true}}, "completado", "apto", false},
		{"no apto", modelos.Candidato{EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{}}, "completado", "no_apto", false},
		{"descarte temprano", modelos.Candidato{EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{DescarteTemprano: true}}, "descartado_temprano", "no_apto", false},
		{"pendiente", modelos.Candidato{EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{VerificacionPendiente: true}}, "pendiente", "pendiente", true},
	}
	for _, tc := range casos {
		etapa, dictamen, pendiente := estadoPostulacion(&tc.c)
		if etapa != tc.etapa || dictamen != tc.dictamen || pendiente != tc.pendiente {
			t.Errorf("%s: %s/%s/%v; quería %s/%s/%v", tc.nombre, etapa, dictamen, pendiente, tc.etapa, tc.dictamen, tc.pendiente)
		}
	}
}

func TestIdentidad(t *testing.T) {
	if k, ced := identidad(&modelos.Candidato{Cedula: "1.035.126.774", URLDetalle: "https://x"}); k != "cc:1035126774" || ced != "1035126774" {
		t.Errorf("con cédula: %q, %q", k, ced)
	}
	if k, ced := identidad(&modelos.Candidato{Cedula: "No especificada", URLDetalle: "https://x/Detail/1"}); k != "url:https://x/Detail/1" || ced != "" {
		t.Errorf("sin cédula: %q, %q", k, ced)
	}
	if k, _ := identidad(&modelos.Candidato{Cedula: "123"}); k != "" {
		t.Errorf("sin cédula válida ni URL no se puede identificar, fue %q", k)
	}
}

func TestResultadosPorFuente(t *testing.T) {
	// ADRES en mantenimiento: error, sin afirmar que cumple, con la bandera de mantenimiento.
	res, cumple, mant := resultadoAdres(&modelos.ResultadoAdres{Estado: "PORTAL_EN_MANTENIMIENTO"}, nil)
	if res != resError || cumple.Valid || !mant {
		t.Errorf("mantenimiento: %s %+v %v", res, cumple, mant)
	}
	if res, _, _ := resultadoAdres(&modelos.ResultadoAdres{Estado: "OK", CumpleEPS: true}, nil); res != resOK {
		t.Errorf("EPS que cumple: %s", res)
	}
	if res, _, _ := resultadoAdres(&modelos.ResultadoAdres{Estado: "OK"}, nil); res != resRechazado {
		t.Errorf("EPS que no cumple: %s", res)
	}

	// RUNT: los estados técnicos no son un rechazo.
	for _, estado := range []string{"ERROR_INTENTOS", "SIN_APELLIDO", "APELLIDO_NO_COINCIDE", "NO_CONSULTADO_SIN_CEDULA"} {
		if res, _ := resultadoRunt(&modelos.ResultadoRunt{Estado: estado}, nil); res != resError {
			t.Errorf("RUNT %s: %s", estado, res)
		}
	}
	if res, _ := resultadoRunt(&modelos.ResultadoRunt{Estado: "NO_INSCRITO"}, nil); res != resRechazado {
		t.Errorf("RUNT no inscrito: %s", res)
	}
	// Manda el dictamen del clasificador cuando evaluó la regla.
	ev := &modelos.EvaluacionIdoneidad{CumpleRunt: false, DetalleRunt: "Antigüedad en RUNT insuficiente: 1.0 años (Mínimo 2 años requeridos)"}
	if res, _ := resultadoRunt(&modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true}, ev); res != resRechazado {
		t.Errorf("RUNT con antigüedad insuficiente: %s", res)
	}

	// SIMIT
	if res, _ := resultadoSimit(&modelos.ResultadoSimit{Estado: "ERROR_CONSULTA"}, nil); res != resError {
		t.Errorf("SIMIT caído: %s", res)
	}
	if res, c := resultadoSimit(&modelos.ResultadoSimit{Estado: "PAZ_Y_SALVO", PazYSalvo: true}, nil); res != resOK || !c.Bool {
		t.Errorf("SIMIT paz y salvo: %s %+v", res, c)
	}
	if res, _ := resultadoSimit(&modelos.ResultadoSimit{Estado: "CON_MULTAS", CantidadMultas: 2}, nil); res != resRechazado {
		t.Errorf("SIMIT con multas: %s", res)
	}

	// Informativas
	if resultadoInformativo(true, true) != resError || resultadoInformativo(false, true) != resAlerta || resultadoInformativo(false, false) != resOK {
		t.Error("resultadoInformativo")
	}
}

func TestRutaRelativa(t *testing.T) {
	descargas := t.TempDir()
	dentro := filepath.Join(descargas, "Oferta", "1_Juan", "cv.pdf")
	if got := rutaRelativa(descargas, dentro); got != "Oferta/1_Juan/cv.pdf" {
		t.Errorf("dentro de descargas: %q", got)
	}
	fuera := filepath.Join(t.TempDir(), "otro.pdf")
	got := rutaRelativa(descargas, fuera)
	if strings.Contains(got, `\`) || !filepath.IsAbs(filepath.FromSlash(got)) {
		t.Errorf("fuera de descargas debe quedar absoluta y con barras normales: %q", got)
	}
}

func TestClaveOfertaSiempreCabeEnLaColumna(t *testing.T) {
	larga := "Operador(a)_de_buses_alimentadores_-_Metro_de_Medellín"
	casos := map[string]string{
		"id normal":          claveOferta("ABCDEF123456", "x"),
		"sin id":             claveOferta("", "operador-buses-alimentadores-metro-medellin"),
		"id demasiado largo": claveOferta(strings.Repeat("A", 80), "x"),
		"carpeta":            IDCarpeta(larga),
	}
	for nombre, clave := range casos {
		if len(clave) == 0 || len(clave) > maxIDOferta {
			t.Errorf("%s: %q mide %d; el máximo es %d", nombre, clave, len(clave), maxIDOferta)
		}
	}
	if claveOferta("ABCDEF123456", "x") != "ABCDEF123456" {
		t.Error("un id de Computrabajo que cabe debe conservarse tal cual")
	}
	if IDCarpeta(larga) != IDCarpeta(larga) || IDCarpeta(larga) == IDCarpeta(larga+"2") {
		t.Error("el id de carpeta debe ser estable y distinto por carpeta")
	}
	if !strings.HasPrefix(IDCarpeta(larga), prefijoCarpeta) {
		t.Error("el id de carpeta debe empezar por el prefijo que reconoce la adopción")
	}
}

func TestIDsV7OrdenadosPorTiempo(t *testing.T) {
	a, err := nuevoID()
	if err != nil || len(a) != 16 {
		t.Fatalf("nuevoID = %x, %v", a, err)
	}
	if a[6]>>4 != 7 {
		t.Errorf("la versión del UUID debe ser 7, fue %d", a[6]>>4)
	}
	time.Sleep(3 * time.Millisecond)
	b, _ := nuevoID()
	if string(a) >= string(b) {
		t.Error("un UUID v7 posterior debe ordenar después")
	}
}
