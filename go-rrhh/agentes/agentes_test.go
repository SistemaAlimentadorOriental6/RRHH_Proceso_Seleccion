package agentes

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-rrhh/modelos"
)

func TestResolvedorProcuraduria(t *testing.T) {
	r := NuevoResolvedorPreguntas()
	casos := []struct {
		pregunta, cedula, nombre, esperado string
	}{
		{"¿Cuánto es 7 x 4?", "", "", "28"},
		{"¿Cuánto es 9 - 2?", "", "", "7"},
		{"¿Cuál es la capital de Antioquia?", "", "", "medellin"},
		{"¿Cuál es la capital del Valle del Cauca?", "", "", "cali"},     // gana el nombre más largo
		{"¿Cuál es la capital de Norte de Santander?", "", "", "cucuta"}, // no "bucaramanga"
		{"¿Cuáles son los tres primeros dígitos de su documento?", "71799510", "", "717"},
		{"¿Cuáles son los dos últimos dígitos del documento?", "71799510", "", "10"},
		{"¿Cuál es el primer nombre de la persona?", "", "Julián Andrés", "julian"},
	}
	for _, c := range casos {
		got, ok := r.Resolver(c.pregunta, c.cedula, c.nombre)
		if !ok || got != c.esperado {
			t.Errorf("Resolver(%q) = %q,%v; esperado %q", c.pregunta, got, ok, c.esperado)
		}
	}
	if _, ok := r.Resolver("¿Color favorito del presidente?", "1", "x"); ok {
		t.Error("una pregunta desconocida no debe resolverse")
	}
	if _, ok := r.Resolver("", "1", "x"); ok {
		t.Error("una pregunta vacía no debe resolverse")
	}
}

func TestInterpretarRespuestaSimit(t *testing.T) {
	limpio, ok := interpretarRespuestaSimit("La persona no posee a la fecha pendientes de pago. Comparendos: 0 Multas: 0")
	if !ok || limpio.Estado != "PAZ_Y_SALVO" || !limpio.PazYSalvo || limpio.Alerta {
		t.Errorf("paz y salvo mal interpretado: %+v ok=%v", limpio, ok)
	}

	deuda, ok := interpretarRespuestaSimit("Comparendos: 2 Multas: 1 Acuerdos de pago: 0 Total (3): $2.480.000")
	if !ok || deuda.Estado != "CON_MULTAS" || deuda.CantidadComparendos != 2 || deuda.TotalDeuda != "$2.480.000" || deuda.PazYSalvo {
		t.Errorf("deuda mal interpretada: %+v", deuda)
	}

	// Antes un texto irreconocible se tomaba como "PAZ_Y_SALVO": ahora debe reportarse como no reconocido.
	if _, ok := interpretarRespuestaSimit("Cargando... por favor espere"); ok {
		t.Error("un texto sin señales no debe interpretarse como resultado")
	}
}

func TestClasificadorNoApruebaSimitNoVerificado(t *testing.T) {
	ac := NuevoAgenteClasificador(nil)
	for _, estado := range []string{"ERROR_CONSULTA", "NO_APLICA", "SIN_CEDULA"} {
		if ok, _ := ac.EvaluarSimit(&modelos.ResultadoSimit{Estado: estado}); ok {
			t.Errorf("SIMIT con estado %s no debería aprobar", estado)
		}
	}
	if ok, _ := ac.EvaluarSimit(&modelos.ResultadoSimit{Estado: "PAZ_Y_SALVO", PazYSalvo: true}); !ok {
		t.Error("paz y salvo debe aprobar")
	}
}

func TestParsearLicenciasRunt(t *testing.T) {
	raw := []map[string]any{{
		"numeroLicencia":  "71799510",
		"estadoLicencia":  "ACTIVA",
		"fechaExpedicion": "2023-12-13T00:00:00",
		"detalleLicencia": []any{
			map[string]any{"categoria": "C2", "fechaVencimiento": "2026-12-13T00:00:00"},
			map[string]any{"categoria": "A2", "fechaVencimientoLimite": "2033-12-13"},
		},
	}}
	lics, activas := parsearLicencias(raw)
	if len(lics) != 1 || lics[0].FechaExpedicion != "2023-12-13" || len(lics[0].Categorias) != 2 {
		t.Fatalf("licencias mal parseadas: %+v", lics)
	}
	if len(activas) != 2 || activas[0] != "C2 (2026-12-13)" {
		t.Errorf("categorías activas: %v", activas)
	}
}

func TestDetectarExtensionYHTML(t *testing.T) {
	if e := detectarExtension([]byte("%PDF-1.7"), "", ""); e != ".pdf" {
		t.Errorf("pdf: %s", e)
	}
	if e := detectarExtension([]byte{0x50, 0x4B, 0x03, 0x04}, "", ""); e != ".docx" {
		t.Errorf("docx: %s", e)
	}
	if e := detectarExtension([]byte("x"), `attachment; filename="cv.doc"`, ""); e != ".doc" {
		t.Errorf("doc: %s", e)
	}
	if !pareceHTML([]byte("<!DOCTYPE html><html>login"), "") {
		t.Error("una página de login no debe aceptarse como hoja de vida")
	}
	if _, err := guardarCV([]byte("<html>login</html>"), "", "text/html", t.TempDir(), "x"); err == nil {
		t.Error("guardarCV debe rechazar HTML")
	}
}

func TestLimpiarNombreYLiteralJS(t *testing.T) {
	if got := LimpiarNombre(`Ana: "la" <Gómez>. `); got != "Ana___la___Gómez_" {
		t.Errorf("LimpiarNombre = %q", got)
	}
	// Una contraseña con comillas y saltos de línea queda como literal JS seguro.
	if got := literalJS("a\"b\\c\nd"); got != `"a\"b\\c\nd"` {
		t.Errorf("literalJS = %s", got)
	}
}

func TestAdresRespuestaValida(t *testing.T) {
	if adresRespuestaValida("") || adresRespuestaValida("   cargando   ") {
		t.Error("una respuesta vacía no es válida")
	}
	if !adresRespuestaValida("ESTADO ENTIDAD REGIMEN\nACTIVO EPS SURAMERICANA CONTRIBUTIVO") {
		t.Error("una respuesta con datos es válida")
	}
	if !adresRespuestaValida("La cédula no se encuentra en la base de datos") {
		t.Error("un aviso explícito de no encontrado es una respuesta válida")
	}
}

func TestReintentar(t *testing.T) {
	ctx := context.Background()
	intentos := 0
	err := reintentar(ctx, 3, time.Millisecond, func(n int) error {
		intentos++
		if n < 3 {
			return errors.New("transitorio")
		}
		return nil
	})
	if err != nil || intentos != 3 {
		t.Errorf("err=%v intentos=%d", err, intentos)
	}

	intentos = 0
	err = reintentar(ctx, 5, time.Millisecond, func(int) error {
		intentos++
		return permanente(errors.New("fatal"))
	})
	if err == nil || intentos != 1 || err.Error() != "fatal" {
		t.Errorf("un error permanente no se reintenta: err=%v intentos=%d", err, intentos)
	}

	cancelado, cancel := context.WithCancel(ctx)
	cancel()
	if err := reintentar(cancelado, 3, time.Millisecond, func(int) error { return errors.New("x") }); !errors.Is(err, context.Canceled) {
		t.Errorf("debe respetar la cancelación: %v", err)
	}
}

func TestSarlaftSinListaNoDiceSinCoincidencias(t *testing.T) {
	ag := &AgenteSarlaft{cfg: nil}
	ag.errCarga = errors.New("sin lista")
	ag.ultimoIntento = time.Now()

	c := &modelos.Candidato{Nombre: "Persona Cualquiera", Cedula: "12345678"}
	_ = ag.Validar(context.Background(), c)
	if c.ResultadoSarlaft == nil || c.ResultadoSarlaft.TipoCoincidencia != "LISTA_NO_DISPONIBLE" || !c.ResultadoSarlaft.Alerta {
		t.Errorf("sin lista debe dejar constancia: %+v", c.ResultadoSarlaft)
	}
}
