package utilidades

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNombresCoinciden(t *testing.T) {
	casos := []struct {
		a, b     string
		minimo   int
		esperado bool
	}{
		{"Daniel Ochoa Castrillón", "DANIEL OCHOA CASTRILLON", 2, true}, // acentos y mayúsculas
		{"Daniel Ochoa", "OCHOA CASTRILLON DANIEL ANDRES", 2, true},     // orden distinto, nombre parcial
		{"Juan Pérez", "JUAN PEREZ GOMEZ", 2, true},
		{"María de los Ángeles Ríos", "RIOS MARIA ANGELES", 2, true}, // conectores
		{"Carlos Perez", "PEDRO GOMEZ RUIZ", 2, false},               // otra persona
		{"Carlos Perez", "CARLOS GOMEZ RUIZ", 2, false},              // solo comparte el nombre de pila
		{"Madonna", "MADONNA CICCONE", 2, true},                      // una sola palabra: debe coincidir esa
		{"", "ALGUIEN", 2, false},
		{"ALGUIEN", "", 2, false},
	}
	for _, c := range casos {
		if got := NombresCoinciden(c.a, c.b, c.minimo); got != c.esperado {
			t.Errorf("NombresCoinciden(%q, %q) = %v; quería %v", c.a, c.b, got, c.esperado)
		}
	}
}

func escribirDocx(t *testing.T, documentoXML string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "cv.docx")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(documentoXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func TestExtraerTextoDocx(t *testing.T) {
	ruta := escribirDocx(t, `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Hoja de vida</w:t></w:r></w:p>
<w:p><w:r><w:t>C.C. </w:t></w:r><w:r><w:t>1.035.126.774</w:t></w:r></w:p>
<w:p><w:r><w:t>Tel</w:t></w:r><w:r><w:tab/></w:r><w:r><w:t>311 234 5678</w:t></w:r></w:p>
</w:body></w:document>`)
	texto, err := ExtraerTextoDocx(ruta)
	if err != nil {
		t.Fatal(err)
	}
	for _, esperado := range []string{"Hoja de vida", "C.C. 1.035.126.774", "311 234 5678"} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("falta %q en %q", esperado, texto)
		}
	}
	if !strings.Contains(texto, "\n") {
		t.Error("los párrafos deben quedar separados por saltos de línea")
	}
}

func TestExtraerTextoDocxRechazaArchivosInvalidos(t *testing.T) {
	noZip := filepath.Join(t.TempDir(), "falso.docx")
	if err := os.WriteFile(noZip, []byte("esto no es un zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtraerTextoDocx(noZip); err == nil {
		t.Error("un archivo que no es ZIP debe fallar")
	}
	if _, err := ExtraerTextoDocx(filepath.Join(t.TempDir(), "no-existe.docx")); err == nil {
		t.Error("un archivo inexistente debe fallar")
	}

	// ZIP válido pero sin word/document.xml.
	vacio := filepath.Join(t.TempDir(), "vacio.docx")
	f, _ := os.Create(vacio)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("otro.txt")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	_ = f.Close()
	if _, err := ExtraerTextoDocx(vacio); err == nil {
		t.Error("un ZIP sin document.xml debe fallar")
	}
}

// El teléfono y el correo suelen ir en el encabezado o el pie de página de Word.
func TestExtraerTextoDocxIncluyeEncabezadoYPie(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "cv.docx")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	partes := map[string]string{
		"word/document.xml": `<w:document xmlns:w="x"><w:body><w:p><w:r><w:t>Cuerpo del CV</w:t></w:r></w:p></w:body></w:document>`,
		"word/header1.xml":  `<w:hdr xmlns:w="x"><w:p><w:r><w:t>Celular 3116324868</w:t></w:r></w:p></w:hdr>`,
		"word/footer1.xml":  `<w:ftr xmlns:w="x"><w:p><w:r><w:t>juan@correo.com</w:t></w:r></w:p></w:ftr>`,
	}
	for _, nombre := range []string{"word/header1.xml", "word/document.xml", "word/footer1.xml"} {
		w, _ := zw.Create(nombre)
		w.Write([]byte(partes[nombre]))
	}
	zw.Close()
	f.Close()

	texto, err := ExtraerTextoDocx(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(texto, "Cuerpo del CV") || !strings.Contains(texto, "3116324868") || !strings.Contains(texto, "juan@correo.com") {
		t.Fatalf("texto = %q", texto)
	}
}
