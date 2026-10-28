package agentes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-rrhh/modelos"
)

func TestMismaPersona(t *testing.T) {
	base := modelos.HojaVidaCorreo{Nombre: "Juan Pérez Gómez", Remitente: "juan@mail.com", Correo: "juan@mail.com", Cedula: "1037000111"}
	casos := []struct {
		nombre string
		otra   modelos.HojaVidaCorreo
		quiere bool
	}{
		{"misma cédula", modelos.HojaVidaCorreo{Nombre: "J Perez", Remitente: "otro@x.com", Cedula: "1037000111"}, true},
		{"cédula distinta, mismo remitente", modelos.HojaVidaCorreo{Nombre: "Juan Pérez Gómez", Remitente: "juan@mail.com", Cedula: "999"}, false},
		{"sin cédula, mismo remitente y nombre", modelos.HojaVidaCorreo{Nombre: "Juan Perez Gomez", Remitente: "juan@mail.com"}, true},
		{"formulario web: mismo remitente, otra persona", modelos.HojaVidaCorreo{Nombre: "Ana Torres Ruiz", Remitente: "juan@mail.com"}, false},
	}
	for _, c := range casos {
		if got := mismaPersona(base, c.otra); got != c.quiere {
			t.Errorf("%s: %v, quería %v", c.nombre, got, c.quiere)
		}
	}
}

func TestFusionarHojaSumaSinPerderDatos(t *testing.T) {
	a := modelos.HojaVidaCorreo{MessageID: "m1", TextoCV: "corto", Archivos: []string{"a.pdf"}}
	b := modelos.HojaVidaCorreo{MessageID: "m2", TextoCV: "un texto bastante más largo", Archivos: []string{"a.pdf", "b.docx"}, Cedula: "123", Telefono: "3001112233"}
	fusionarHoja(&a, b)
	if a.MessageID != "m1" || a.Cedula != "123" || a.Telefono != "3001112233" || a.TextoCV != b.TextoCV || len(a.Archivos) != 2 {
		t.Fatalf("fusión inesperada: %+v", a)
	}
}

func TestNombreDeAdjuntoCompletaExtension(t *testing.T) {
	for _, c := range []struct{ nombre, tipo, quiere string }{
		{"", "application/pdf", "adjunto_3.pdf"},
		{"CV Juan", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "CV Juan.docx"},
		{"CV Juan P. Perez", "application/pdf", "CV Juan P. Perez.pdf"},
		{"hv.pdf", "application/octet-stream", "hv.pdf"},
	} {
		if got := nombreDeAdjunto(c.nombre, c.tipo, 3); got != c.quiere {
			t.Errorf("nombreDeAdjunto(%q, %q) = %q, quería %q", c.nombre, c.tipo, got, c.quiere)
		}
	}
}

// El CV personal se suma al perfil de la plataforma, pero no se repite si el perfil ya lo trae (correo).
func TestTextoCandidatoSumaCVPersonal(t *testing.T) {
	cv := filepath.Join(t.TempDir(), "cv.txt")
	experiencia := "EXPERIENCIA LABORAL\nEmpresa : ALIANZA MAI\nCargo : Operador\nTiempo laborado : 4 años 6 meses\n" + strings.Repeat("detalle ", 10)
	if err := os.WriteFile(cv, []byte(experiencia), 0o644); err != nil {
		t.Fatal(err)
	}
	perfil := "Perfil de Computrabajo: " + strings.Repeat("datos personales ", 6)

	got := textoCandidato(modelos.RegistroApto{TextoPerfil: perfil, RutaCVOriginal: cv})
	if !strings.Contains(got, "datos personales") || !strings.Contains(got, "ALIANZA MAI") {
		t.Fatalf("faltan fuentes: %q", got)
	}

	conCV := modelos.RegistroApto{TextoPerfil: "Correo: a@b.c\n" + experiencia, RutaCVOriginal: cv}
	if got := textoCandidato(conCV); strings.Count(got, "ALIANZA MAI") != 1 {
		t.Fatalf("el CV del correo se repitió: %q", got)
	}
}

func TestNombreSegunContenido(t *testing.T) {
	docx := append([]byte("PK\x03\x04"), []byte("...[Content_Types].xml...word/document.xml...")...)
	for _, c := range []struct {
		nombre string
		datos  []byte
		quiere string
	}{
		{"adjunto_1", []byte("%PDF-1.7 ..."), "adjunto_1.pdf"},
		{"HV Mar\xeda", docx, "HV Mar\xeda.docx"},
		{"foto.jpg", []byte("%PDF-1.7"), "foto.jpg"},
		{"factura", []byte("hola"), "factura"},
	} {
		if got := nombreSegunContenido(c.nombre, c.datos); got != c.quiere {
			t.Errorf("nombreSegunContenido(%q) = %q, quería %q", c.nombre, got, c.quiere)
		}
	}
}

func TestTextoIlegibleYLecturaConIA(t *testing.T) {
	basura := "�\x034Q@\x1e�\x00QK mailto:paola@hotmail.com " + strings.Repeat("\x00\x14QE", 80)
	pdfCrudo := "en-US ... endstream\r\nendobj\r\n24 0 obj mailto:paola@hotmail.com " + strings.Repeat("a", 100)
	cv := "EXPERIENCIA LABORAL Empresa ALIANZA MAI Cargo Operador. Cedula 1037000111 Celular 3116324868 Perfil formacion academica referencias"
	if !textoIlegible(basura) || !textoIlegible(pdfCrudo) || textoIlegible(cv) {
		t.Fatal("mala clasificación de texto legible/ilegible")
	}
	// Antes estos pasaban por hoja de vida porque traían un correo.
	if !necesitaLecturaConIA(basura) || !necesitaLecturaConIA(pdfCrudo) {
		t.Fatal("la basura debe mandarse a leer con IA")
	}
}

func TestFusionarReemplazaTextoIlegible(t *testing.T) {
	vieja := modelos.HojaVidaCorreo{MessageID: "m", TextoCV: strings.Repeat("\x00\x14QE", 200), InteresConductor: ConductorNo, MotivoConductor: "IA: corrupto"}
	nueva := modelos.HojaVidaCorreo{MessageID: "m2", TextoCV: "Empresa : TEMSA Cargo : Conductor", InteresConductor: ConductorSi, MotivoConductor: "cargo conductor", TextoLeido: true}
	fusionarHoja(&vieja, nueva)
	if vieja.TextoCV != nueva.TextoCV || vieja.InteresConductor != ConductorSi || vieja.MessageID != "m" {
		t.Fatalf("no se reparó la hoja: %+v", vieja)
	}
}
