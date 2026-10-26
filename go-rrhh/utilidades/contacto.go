package utilidades

import (
	"net/mail"
	"regexp"
	"strings"
)

var (
	reCorreo = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,}`)
	reTramo  = regexp.MustCompile(`\d+`)
)

// ContactoCV son los datos de contacto hallados en el texto de una hoja de vida.
type ContactoCV struct {
	Correo   string // vacío si no hay uno válido
	Telefono string // solo dígitos, sin prefijo de país; vacío si no hay
}

// ExtraerContacto toma el primer correo válido y el primer teléfono del texto (un celular tiene
// prioridad sobre un fijo). cedula (opcional) evita confundir el documento de identidad —que también
// son 10 dígitos— con un celular.
// shortcut: solo teléfonos colombianos (celular 3XX y fijo 60X), otros formatos se ignoran; ampliar al soportar otros países.
func ExtraerContacto(texto, cedula string) ContactoCV {
	var c ContactoCV
	for _, m := range reCorreo.FindAllString(texto, -1) {
		if dir, err := mail.ParseAddress(m); err == nil {
			c.Correo = strings.ToLower(dir.Address)
			break
		}
	}

	celulares, fijos := TelefonosEnTexto(texto, cedula)
	switch {
	case len(celulares) > 0:
		c.Telefono = celulares[0]
	case len(fijos) > 0:
		c.Telefono = fijos[0]
	}
	return c
}

// TelefonosEnTexto devuelve, sin repetir y en orden de aparición, los celulares y los fijos
// colombianos del texto, como 10 dígitos sin prefijo de país. Entiende los formatos con que la gente
// escribe un número: "314 551 5276", "314-551-5276", "(314) 551.5276", "3 1 4 5 5 1 5 2 7 6",
// "+57 314 5515276", "57 3145515276", "0057 314…", "314 551 5276 |" o pegado a una etiqueta
// ("Cel:3145515276"). Un número largo (más de 10 dígitos seguidos) no se parte, y cedula se excluye.
func TelefonosEnTexto(texto, cedula string) (celulares, fijos []string) {
	ced := soloDigitos(cedula)
	vistos := map[string]bool{}
	registrar := func(d string) bool {
		tipo, num := clasificarTelefono(d)
		if tipo == 0 || num == ced || vistos[num] {
			return false
		}
		vistos[num] = true
		if tipo == 'c' {
			celulares = append(celulares, num)
		} else {
			fijos = append(fijos, num)
		}
		return true
	}

	// Los grupos de dígitos separados solo por espacios o signos (nunca por letras ni saltos de
	// línea) forman un mismo número; se prueba cada ventana de grupos contiguos.
	locs := reTramo.FindAllStringIndex(texto, -1)
	for i := 0; i < len(locs); {
		fin := i + 1
		for fin < len(locs) && separadorDeNumero(texto[locs[fin-1][1]:locs[fin][0]]) {
			fin++
		}
		grupo := locs[i:fin]
		for a := 0; a < len(grupo); a++ {
			acum := ""
			for b := a; b < len(grupo); b++ {
				acum += texto[grupo[b][0]:grupo[b][1]]
				if len(acum) > 14 {
					break
				}
				if registrar(acum) {
					a = b // los grupos usados ya no inician otro número
					break
				}
			}
		}
		i = fin
	}
	return celulares, fijos
}

// separadorDeNumero dice si lo que hay entre dos grupos de dígitos puede ser parte de un mismo número.
func separadorDeNumero(gap string) bool {
	if len(gap) > 4 {
		return false
	}
	for _, r := range gap {
		if !strings.ContainsRune(" \t .-–()/|·_,", r) {
			return false
		}
	}
	return true
}

// clasificarTelefono devuelve 'c' (celular), 'f' (fijo) o 0, y el número de 10 dígitos sin prefijo.
func clasificarTelefono(d string) (byte, string) {
	switch {
	case len(d) == 14 && strings.HasPrefix(d, "0057"):
		d = d[4:]
	case len(d) == 12 && strings.HasPrefix(d, "57"):
		d = d[2:]
	}
	if len(d) != 10 {
		return 0, ""
	}
	if d[0] == '3' {
		return 'c', d
	}
	if strings.HasPrefix(d, "60") {
		return 'f', d
	}
	return 0, ""
}

func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
