package agentes

import (
	"testing"
)

func TestEnlacesDeDocumentos(t *testing.T) {
	correo := `<p>attachment(s)</p>
<a href="https://archivos.ejemplo.com/f/abc?x=1&amp;y=2">YOHAN SEBASTIAN VARGAS ALVAREZ-4.pdf</a> 158 KB
<a href="https://archivos.ejemplo.com/files/CV_Juan.docx">descargar</a>
<a href="http://inseguro.com/a.pdf">a.pdf</a>
<a href="https://ejemplo.com/politica">Política de privacidad</a>
<a href="https://ejemplo.com/logo.png">logo.png</a>`
	got := enlacesDeDocumentos(correo)
	if len(got) != 2 {
		t.Fatalf("enlaces = %+v", got)
	}
	if got[0].url != "https://archivos.ejemplo.com/f/abc?x=1&y=2" || got[0].nombre != "YOHAN SEBASTIAN VARGAS ALVAREZ-4.pdf" {
		t.Errorf("primero = %+v", got[0])
	}
	if got[1].nombre != "CV_Juan.docx" {
		t.Errorf("segundo = %+v", got[1])
	}
}

func TestSoloIPPublica(t *testing.T) {
	for ip, quiere := range map[string]bool{
		"8.8.8.8:443": true, "127.0.0.1:443": false, "10.0.0.5:443": false, "192.168.1.10:443": false,
		"169.254.169.254:80": false, "100.64.0.1:443": false, "[::1]:443": false, "[2001:4860:4860::8888]:443": true,
	} {
		if err := soloIPPublica("tcp", ip, nil); (err == nil) != quiere {
			t.Errorf("soloIPPublica(%s): err=%v, permitido esperado %v", ip, err, quiere)
		}
	}
}
