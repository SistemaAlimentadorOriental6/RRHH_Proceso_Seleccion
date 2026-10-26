package almacen

import (
	"strings"
	"testing"
)

func TestHashClave(t *testing.T) {
	h, err := HashClave("una-clave-de-prueba")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h, "una-clave-de-prueba") || !strings.HasPrefix(h, "pbkdf2-sha256$") {
		t.Fatalf("formato inesperado: %q", h)
	}
	if !ClaveCoincide("una-clave-de-prueba", h) {
		t.Error("la clave correcta no coincide")
	}
	for _, mala := range []string{"otra", "", "una-clave-de-prueba "} {
		if ClaveCoincide(mala, h) {
			t.Errorf("%q no debería coincidir", mala)
		}
	}
	h2, _ := HashClave("una-clave-de-prueba")
	if h == h2 {
		t.Error("dos hashes de la misma clave deben diferir (sal propia)")
	}
	for _, roto := range []string{"", "x", "pbkdf2-sha256$0$aa$bb", "pbkdf2-sha256$abc$aa$bb", "md5$1$a$b"} {
		if ClaveCoincide("x", roto) {
			t.Errorf("hash inválido %q aceptado", roto)
		}
	}
}
