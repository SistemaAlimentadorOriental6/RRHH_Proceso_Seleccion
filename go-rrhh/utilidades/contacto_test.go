package utilidades

import "testing"

func TestTelefonosEnTexto(t *testing.T) {
	casos := []struct {
		nombre, texto string
		celular       string
	}{
		{"con espacios", "Cel 314 551 5276", "3145515276"},
		{"con barra al final", "314 551 5276 |", "3145515276"},
		{"con guiones", "314-551-5276", "3145515276"},
		{"con paréntesis y punto", "(314) 551.5276", "3145515276"},
		{"dígito por dígito", "3 1 4 5 5 1 5 2 7 6", "3145515276"},
		{"con +57", "+57 314 5515276", "3145515276"},
		{"con 57 solo", "57 3145515276", "3145515276"},
		{"con 57 y espacios", "57 314 551 5276", "3145515276"},
		{"con 0057", "0057 314 551 5276", "3145515276"},
		{"pegado a la etiqueta", "Cel:3145515276", "3145515276"},
		{"después de la cédula", "C.C. 1035126774 311 234 5678", "3112345678"},
		{"en otra línea que la cédula", "1035126774\n3112345678", "3112345678"},
		{"no mezcla líneas", "314 551\n5276", ""},
		{"no parte un número largo", "13145515276123", ""},
	}
	for _, c := range casos {
		cel, _ := TelefonosEnTexto(c.texto, "")
		got := ""
		if len(cel) > 0 {
			got = cel[0]
		}
		if got != c.celular {
			t.Errorf("%s: TelefonosEnTexto(%q) = %q; quería %q", c.nombre, c.texto, got, c.celular)
		}
	}
	if _, fijos := TelefonosEnTexto("Tel 604 444 1234", ""); len(fijos) != 1 || fijos[0] != "6044441234" {
		t.Errorf("fijo de Medellín: %v", fijos)
	}
	if c := ExtraerContacto("Tel 604 444 1234 y cel 311 234 5678", ""); c.Telefono != "3112345678" {
		t.Errorf("el celular debe ganarle al fijo: %q", c.Telefono)
	}
}

func TestExtraerContacto(t *testing.T) {
	casos := []struct {
		nombre, texto, cedula string
		quiero                ContactoCV
	}{
		{"correo y celular", "Tel: +57 311 234 5678 Mail: Juan.Perez@Gmail.com", "", ContactoCV{"juan.perez@gmail.com", "3112345678"}},
		{"sin datos", "Experiencia como conductor", "", ContactoCV{}},
		{"cedula no es celular", "CC 3091095116", "3.091.095.116", ContactoCV{}},
		{"numero largo no es celular", "ref 13112345678901", "", ContactoCV{}},
		{"correo invalido", "a@b", "", ContactoCV{}},
	}
	for _, c := range casos {
		if got := ExtraerContacto(c.texto, c.cedula); got != c.quiero {
			t.Errorf("%s: got %+v, quiero %+v", c.nombre, got, c.quiero)
		}
	}
}
