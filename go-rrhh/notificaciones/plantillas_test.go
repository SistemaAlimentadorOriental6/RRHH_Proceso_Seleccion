package notificaciones

import (
	"strings"
	"testing"
)

func TestSolicitudDatosEscapaHTML(t *testing.T) {
	_, html, err := SolicitudDatos(DatosSolicitudDatos{Nombre: "<script>x</script>", Correo: "a@b.co", Vacante: "V", URL: "https://p.test/validar"})
	if err != nil || strings.Contains(html, "<script>") || !strings.Contains(html, `href="https://p.test/validar"`) {
		t.Fatalf("html inseguro o sin enlace: %v", err)
	}
}
