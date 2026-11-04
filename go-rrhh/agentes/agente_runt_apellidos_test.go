package agentes

import (
	"slices"
	"testing"
)

func TestDeducirApellidosCompuestos(t *testing.T) {
	a := &AgenteRunt{}
	casos := []struct {
		nombre, ref, primero string
	}{
		{"Joel David De la Hoz Guzman", "", "DE LA HOZ"},
		{"Joel David De la Hoz Guzman", "DE LA HOZ GUZMAN", "DE LA HOZ"},
		{"Ana Del Rio Perez", "", "DEL RIO"},
	}
	for _, c := range casos {
		lista := a.DeducirApellidos(c.nombre, c.ref)
		if len(lista) == 0 || lista[0] != c.primero {
			t.Errorf("%q (ref %q): primer candidato = %v, quería %q", c.nombre, c.ref, lista, c.primero)
		}
	}
	// Sin conectores todo sigue igual: paterno primero.
	if lista := a.DeducirApellidos("Juan Perez Gomez", ""); !slices.Equal(lista[:2], []string{"PEREZ", "GOMEZ"}) {
		t.Errorf("sin compuestos cambió el orden: %v", lista)
	}
}
