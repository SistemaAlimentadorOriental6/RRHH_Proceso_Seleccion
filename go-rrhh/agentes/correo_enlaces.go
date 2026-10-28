package agentes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Los formularios web (p. ej. el de «Hojas de vida» de la página) no adjuntan el archivo al correo: lo
// listan como un enlace de descarga. Aquí se encuentran esos enlaces y se baja el PDF o Word.

// maxEnlacesPorCorreo limita cuántos archivos se bajan de un mismo correo.
const maxEnlacesPorCorreo = 3

var reEnlaceHTML = regexp.MustCompile(`(?is)<a\b[^>]*?href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)

type enlaceDocumento struct {
	url    string
	nombre string
}

// enlacesDeDocumentos devuelve los enlaces https del correo que apuntan a un PDF o Word: porque el texto
// del enlace es el nombre del archivo («CV Juan.pdf») o porque la ruta termina así.
func enlacesDeDocumentos(htmlCorreo string) []enlaceDocumento {
	var salida []enlaceDocumento
	vistos := map[string]bool{}
	for _, m := range reEnlaceHTML.FindAllStringSubmatch(htmlCorreo, -1) {
		destino := html.UnescapeString(strings.TrimSpace(m[1]))
		u, err := url.Parse(destino)
		if err != nil || u.Scheme != "https" || vistos[destino] {
			continue
		}
		texto := strings.Join(strings.Fields(html.UnescapeString(reEtiquetasHTML.ReplaceAllString(m[2], " "))), " ")
		nombre := ""
		switch {
		case esDocumentoWordOPDF(texto):
			nombre = texto
		case esDocumentoWordOPDF(path.Base(u.Path)):
			nombre = path.Base(u.Path)
		default:
			continue
		}
		vistos[destino] = true
		salida = append(salida, enlaceDocumento{url: destino, nombre: nombre})
		if len(salida) == maxEnlacesPorCorreo {
			break
		}
	}
	return salida
}

// soloIPPublica rechaza conectar a direcciones internas: el enlace viene de un correo ajeno y no debe
// servir para que el servidor consulte su propia red. Se evalúa sobre la IP ya resuelta, así que
// tampoco la burla un nombre que apunte adentro.
func soloIPPublica(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return fmt.Errorf("destino no permitido: %s", address)
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1]&0xC0 == 64 { // 100.64.0.0/10 (CGNAT)
		return fmt.Errorf("destino no permitido: %s", address)
	}
	return nil
}

var clienteEnlaces = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 10 * time.Second, Control: soloIPPublica}).DialContext},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) >= 3 {
			return errors.New("redirección no permitida")
		}
		return nil
	},
}

// descargarDocumento baja el archivo de un enlace y comprueba por su contenido que sea PDF o Word.
func descargarDocumento(ctx context.Context, enlace string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, enlace, nil)
	if err != nil {
		return nil, err
	}
	resp, err := clienteEnlaces.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respondió %d", resp.StatusCode)
	}
	datos, err := io.ReadAll(io.LimitReader(resp.Body, maxBytesAdjunto+1))
	if err != nil {
		return nil, err
	}
	if len(datos) > maxBytesAdjunto {
		return nil, errors.New("archivo demasiado grande")
	}
	if !bytes.HasPrefix(datos, []byte("%PDF-")) &&
		!(bytes.HasPrefix(datos, []byte("PK\x03\x04")) && bytes.Contains(datos, []byte("word/document.xml"))) &&
		!bytes.HasPrefix(datos, []byte{0xD0, 0xCF, 0x11, 0xE0}) {
		return nil, errors.New("no es un PDF ni un documento de Word")
	}
	return datos, nil
}
