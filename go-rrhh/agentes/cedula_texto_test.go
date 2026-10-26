package agentes

import "testing"

func TestCedulaEnTexto(t *testing.T) {
	casos := []struct {
		nombre, texto, esperada string
	}{
		{"con puntos", "Nombre: Juan\nC.C. 1.035.126.774\nBogotá", "1035126774"},
		{"cédula con acento", "Cédula: 80766779", "80766779"},
		{"con la etiqueta documento", "Documento 1035126774 expedido en Medellín", "1035126774"},
		{"no pega el celular que sigue", "C.C. 1035126774 311 234 5678", "1035126774"},
		{"espacios entre miles", "Identificación: 1 035 126 774", "1035126774"},
		{"toma la siguiente etiqueta si la primera no sirve", "Documento de identidad: n/a. C.C. 80766779", "80766779"},
		{"un celular suelto no es cédula", "Teléfono: 3112345678", ""},
		{"número demasiado corto", "Documento: 12345", ""},
		{"número demasiado largo", "Documento 12345678901234567", ""},
		{"sin etiqueta", "1035126774", ""},
		{"vacío", "", ""},
	}
	for _, c := range casos {
		if got := cedulaEnTexto(c.texto); got != c.esperada {
			t.Errorf("%s: cedulaEnTexto(%q) = %q; quería %q", c.nombre, c.texto, got, c.esperada)
		}
	}
}
