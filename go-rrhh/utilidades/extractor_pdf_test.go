package utilidades

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pdfMinimo arma un PDF de una página con el texto dado, sin compresión.
func pdfMinimo(texto string) []byte {
	var b bytes.Buffer
	var offsets []int
	obj := func(cuerpo string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), cuerpo)
	}
	b.WriteString("%PDF-1.4\n")
	contenido := "BT /F1 12 Tf 72 700 Td (" + texto + ") Tj ET"
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	obj("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contenido), contenido))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	inicio := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, inicio)
	return b.Bytes()
}

// Sin Python/PyMuPDF (el caso del servidor) el texto debe salir igual, no basura binaria.
func TestTextoPDFNativo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "cv.pdf")
	if err := os.WriteFile(ruta, pdfMinimo("Empresa ALIANZA MAI Cargo Operador Tiempo laborado 4 anos"), 0o644); err != nil {
		t.Fatal(err)
	}
	texto, err := textoPDFNativo(ruta)
	if err != nil || !strings.Contains(texto, "ALIANZA MAI") || !strings.Contains(texto, "Operador") {
		t.Fatalf("texto = %q, err = %v", texto, err)
	}
}

func TestTextoPDFNativoNoEntraEnPanicoConBasura(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "roto.pdf")
	if err := os.WriteFile(ruta, []byte("%PDF-1.4\n\x00\x01basura sin estructura"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := textoPDFNativo(ruta); err == nil {
		t.Fatal("un PDF roto debería dar error")
	}
}
