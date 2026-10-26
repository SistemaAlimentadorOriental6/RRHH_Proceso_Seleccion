package utilidades

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// limiteDocx evita que un archivo comprimido malicioso ocupe toda la memoria al descomprimirse.
const limiteDocx = 20 << 20

// ExtraerTextoDocx devuelve el texto de un documento de Word (.docx). Un .docx es un ZIP cuyo
// contenido está en word/document.xml. Los .doc antiguos (binarios) no se soportan.
func ExtraerTextoDocx(ruta string) (string, error) {
	zr, err := zip.OpenReader(ruta)
	if err != nil {
		return "", fmt.Errorf("no es un .docx válido: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		datos, err := io.ReadAll(io.LimitReader(rc, limiteDocx))
		if err != nil {
			return "", err
		}
		return textoDeDocumentoXML(datos)
	}
	return "", fmt.Errorf("el .docx no tiene word/document.xml")
}

// textoDeDocumentoXML junta el contenido de los <w:t> separando párrafos con saltos de línea.
func textoDeDocumentoXML(datos []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(datos))
	var b strings.Builder
	enTexto := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				enTexto = true
			case "tab":
				b.WriteByte('\t')
			case "br":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				enTexto = false
			case "p":
				b.WriteByte('\n')
			}
		case xml.CharData:
			if enTexto {
				b.Write(t)
			}
		}
	}
	return b.String(), nil
}
