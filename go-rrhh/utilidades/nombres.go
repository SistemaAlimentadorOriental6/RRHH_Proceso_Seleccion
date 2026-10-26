package utilidades

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var quitarAcentos = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// particulas son conectores de los nombres y apellidos que no ayudan a identificar a nadie.
var particulas = map[string]struct{}{"de": {}, "del": {}, "la": {}, "las": {}, "los": {}, "y": {}, "e": {}}

// TokensNombre separa un nombre en palabras en minúscula y sin acentos, sin conectores.
func TokensNombre(nombre string) []string {
	sinAcentos, _, _ := transform.String(quitarAcentos, nombre)
	palabras := strings.FieldsFunc(strings.ToLower(sinAcentos), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	salida := palabras[:0]
	for _, p := range palabras {
		if _, conector := particulas[p]; !conector && len(p) > 1 {
			salida = append(salida, p)
		}
	}
	return salida
}

// NombresCoinciden dice si dos nombres parecen ser de la misma persona: comparten al menos
// minimo palabras, sin importar el orden (unos portales ponen primero los apellidos). Si uno de
// los dos tiene menos palabras que minimo, deben coincidir todas las suyas.
func NombresCoinciden(a, b string, minimo int) bool {
	ta, tb := TokensNombre(a), TokensNombre(b)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	en := make(map[string]struct{}, len(tb))
	for _, p := range tb {
		en[p] = struct{}{}
	}
	comunes := 0
	vistas := map[string]struct{}{}
	for _, p := range ta {
		if _, repetida := vistas[p]; repetida {
			continue
		}
		vistas[p] = struct{}{}
		if _, ok := en[p]; ok {
			comunes++
		}
	}
	necesarias := minimo
	if menor := min(len(vistas), len(en)); menor < necesarias {
		necesarias = menor
	}
	return comunes >= necesarias && necesarias > 0
}
