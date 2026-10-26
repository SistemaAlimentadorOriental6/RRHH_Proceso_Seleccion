package almacen

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTokenEsUnicoOpacoYSoloSeGuardaSuHuella(t *testing.T) {
	vistos := map[string]bool{}
	for i := 0; i < 200; i++ {
		token, huella, err := nuevoToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(token) != largoToken || !formaTokenValida(token) {
			t.Fatalf("token inválido: %q", token)
		}
		if vistos[token] {
			t.Fatal("se repitió un token")
		}
		vistos[token] = true
		if len(huella) != 64 || huella != hashToken(token) {
			t.Fatalf("huella = %q", huella)
		}
		if strings.Contains(huella, token) {
			t.Error("la huella no debe contener el token")
		}
	}
}

func TestFormaTokenValida(t *testing.T) {
	bueno, _, _ := nuevoToken()
	for _, malo := range []string{"", "abc", bueno + "x", bueno[:42], strings.Repeat("!", largoToken), strings.Repeat("a", largoToken-1) + " ", "../../etc/passwd"} {
		if formaTokenValida(malo) {
			t.Errorf("%q no debía aceptarse", malo)
		}
	}
	if !formaTokenValida(bueno) {
		t.Error("un token generado debe ser válido")
	}
}

func TestEdadEn(t *testing.T) {
	ahora := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	casos := map[string]int{
		"2008-10-21": 18, // cumple 18 hoy
		"2008-10-22": 17, // los cumple mañana
		"2008-10-20": 18,
		"1990-02-28": 36,
		"2000-12-31": 25, // aún no cumple este año
	}
	for nacimiento, esperada := range casos {
		n, _ := time.Parse("2006-01-02", nacimiento)
		if got := edadEn(n, ahora); got != esperada {
			t.Errorf("edadEn(%s) = %d; quería %d", nacimiento, got, esperada)
		}
	}
}

func TestValidarDatosCandidato(t *testing.T) {
	ahora := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	d, err := ValidarDatosCandidato("1.035.126.774", "1990-05-17", 18, ahora)
	if err != nil || d.Cedula != "1035126774" || d.Edad != 36 || d.FechaNacimiento.Format("2006-01-02") != "1990-05-17" {
		t.Fatalf("caso válido: %+v, %v", d, err)
	}
	if d, err := ValidarDatosCandidato(" 80 766 779 ", "2008-10-21", 18, ahora); err != nil || d.Edad != 18 || d.Cedula != "80766779" {
		t.Errorf("justo 18 años debe pasar: %+v, %v", d, err)
	}

	malos := []struct {
		nombre, cedula, fecha, campo string
	}{
		{"cédula corta", "12345", "1990-05-17", "cedula"},
		{"cédula larga", "1234567890123", "1990-05-17", "cedula"},
		{"cédula con letras", "10351A6774", "1990-05-17", "cedula"},
		{"cédula vacía", "", "1990-05-17", "cedula"},
		{"menor de edad", "1035126774", "2008-10-22", "fecha_nacimiento"},
		{"futura", "1035126774", "2030-01-01", "fecha_nacimiento"},
		{"formato", "1035126774", "17/05/1990", "fecha_nacimiento"},
		{"vacía", "1035126774", "", "fecha_nacimiento"},
		{"imposible", "1035126774", "1900-01-01", "fecha_nacimiento"},
	}
	for _, m := range malos {
		_, err := ValidarDatosCandidato(m.cedula, m.fecha, 18, ahora)
		var campos ErrCampos
		if !errors.As(err, &campos) || campos[m.campo] == "" {
			t.Errorf("%s: error = %v; quería un error en %q", m.nombre, err, m.campo)
		}
	}

	// Reporta todos los campos mal a la vez, no uno por intento.
	_, err = ValidarDatosCandidato("1", "x", 18, ahora)
	var campos ErrCampos
	if !errors.As(err, &campos) || len(campos) != 2 {
		t.Errorf("debe señalar ambos campos: %v", err)
	}

	// La edad mínima es configurable.
	if _, err := ValidarDatosCandidato("1035126774", "2008-10-22", 16, ahora); err != nil {
		t.Errorf("con edad mínima 16 un joven de 17 debe pasar: %v", err)
	}
}

func TestEnmascararCorreo(t *testing.T) {
	for entrada, esperado := range map[string]string{
		"daniel.ochoaa.cass@gmail.com": "da***@gmail.com",
		"ab@x.co":                      "a***@x.co",
		"a@x.co":                       "a***@x.co",
		"sin-arroba":                   "",
		"":                             "",
		"ñandú@correo.co":              "ña***@correo.co",
	} {
		if got := enmascararCorreo(entrada); got != esperado {
			t.Errorf("enmascararCorreo(%q) = %q; quería %q", entrada, got, esperado)
		}
	}
}
