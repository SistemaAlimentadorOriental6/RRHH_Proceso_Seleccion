package almacen

import (
	"testing"

	"go-rrhh/modelos"
)

func TestIndicesUnicos(t *testing.T) {
	// Como en la base real: cuatro personas con el número 1 y dos con el 9.
	lista := []modelos.Candidato{
		{Nombre: "A", Indice: 1}, {Nombre: "B", Indice: 1}, {Nombre: "C", Indice: 1},
		{Nombre: "D", Indice: 3}, {Nombre: "E", Indice: 9}, {Nombre: "F", Indice: 9},
	}
	indicesUnicos(lista)
	vistos := map[int]string{}
	for _, c := range lista {
		if otro, ya := vistos[c.Indice]; ya {
			t.Fatalf("%s y %s comparten el número %d", c.Nombre, otro, c.Indice)
		}
		vistos[c.Indice] = c.Nombre
	}
	if lista[0].Indice != 1 || lista[3].Indice != 3 || lista[4].Indice != 9 {
		t.Errorf("quien ya tenía el número único (o iba primero) debe conservarlo: %+v", lista)
	}
}
