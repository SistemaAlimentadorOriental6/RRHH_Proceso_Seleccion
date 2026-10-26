package agentes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-rrhh/modelos"
)

func TestGuardarCapturaRegistraRutaYAlias(t *testing.T) {
	c := &modelos.Candidato{CarpetaCandidato: t.TempDir()}

	GuardarCaptura(c, "perfil", []byte{0xFF, 0xD8, 0xFF, 0xD9})
	AliasCaptura(c, "genero", "perfil")

	ruta := CapturaDe(c, "perfil")
	if ruta == "" || filepath.Base(ruta) != "perfil.jpg" || filepath.Base(filepath.Dir(ruta)) != "capturas" {
		t.Fatalf("ruta inesperada: %q", ruta)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("la captura no quedó en disco: %v", err)
	}
	if CapturaDe(c, "genero") != ruta {
		t.Errorf("el género debe reutilizar la captura del perfil")
	}
}

func TestGuardarCapturaSinCarpetaOSinBytesNoHaceNada(t *testing.T) {
	sinCarpeta := &modelos.Candidato{}
	GuardarCaptura(sinCarpeta, "adres", []byte{1})
	sinBytes := &modelos.Candidato{CarpetaCandidato: t.TempDir()}
	GuardarCaptura(sinBytes, "adres", nil)
	if len(sinCarpeta.Capturas) != 0 || len(sinBytes.Capturas) != 0 {
		t.Errorf("no debía registrar capturas: %v %v", sinCarpeta.Capturas, sinBytes.Capturas)
	}
}

func TestHTMLEvidenciaEscapaContenido(t *testing.T) {
	h := htmlEvidencia(Evidencia{
		Titulo: "<script>alert(1)</script>", Veredicto: "OK", Portal: "https://x",
		Filas: []FilaEvidencia{{"Nombre", `"><img src=x onerror=1>`}},
	})
	if strings.Contains(h, "<script>alert") || strings.Contains(h, "<img src=x") {
		t.Errorf("el contenido no se escapó:\n%s", h)
	}
}
