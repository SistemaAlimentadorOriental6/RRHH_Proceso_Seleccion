package agentes

import "strings"

var (
	sinTildes = strings.NewReplacer("Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N")
	// palabrasEPS no distinguen una EPS de otra: "EPS Sura" debe casar con "EPS SURAMERICANA S.A.".
	palabrasEPS = map[string]bool{"EPS": true, "EPSS": true, "SA": true, "SAS": true, "DE": true, "DEL": true, "LA": true,
		"Y": true, "ENTIDAD": true, "PROMOTORA": true, "ESS": true, "CM": true}

	// otrosNombresEPS son razones sociales con que ADRES muestra algunas EPS y que no contienen el nombre
	// comercial de la pantalla. La clave son las palabras propias del nombre comercial.
	otrosNombresEPS = map[string][]string{
		"SURA":               {"SURAMERICANA"},
		"SAVIA SALUD":        {"ALIANZA MEDELLIN ANTIOQUIA"},
		"SOS":                {"SERVICIO OCCIDENTAL DE SALUD"},
		"CAJACOPI ATLANTICO": {"CAJACOPI"},
		"COMFENALCO VALLE":   {"COMPENSACION FAMILIAR DEL VALLE DEL CAUCA"},
		"COMFACHOCO":         {"COMPENSACION FAMILIAR DEL CHOCO"},
		"COMFAORIENTE":       {"COMPENSACION FAMILIAR DEL ORIENTE COLOMBIANO"},
		"COMFAMILIAR HUILA":  {"COMPENSACION FAMILIAR DEL HUILA"},
		"COMFAMILIAR NARINO": {"COMPENSACION FAMILIAR DE NARINO"},
		"CAPRESOCA":          {"PREVISION SOCIAL Y SEGURIDAD DEL CASANARE"},
	}
)

// palabrasDe pasa el texto a mayúsculas sin tildes y lo parte en palabras de letras y números. Las letras
// sueltas seguidas se juntan también como sigla: "S.O.S." da S, O, S y SOS.
func palabrasDe(s string) []string {
	s = sinTildes.Replace(strings.ToUpper(s))
	palabras := strings.FieldsFunc(s, func(r rune) bool { return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') })
	var sigla strings.Builder
	cerrar := func() {
		if sigla.Len() > 1 {
			palabras = append(palabras, sigla.String())
		}
		sigla.Reset()
	}
	for _, p := range palabras[:len(palabras):len(palabras)] {
		if len(p) == 1 {
			sigla.WriteString(p)
		} else {
			cerrar()
		}
	}
	cerrar()
	return palabras
}

// propias son las palabras que identifican a la EPS (sin "EPS", "S.A.", "DE"... ni letras sueltas).
func propias(nombre string) []string {
	var l []string
	for _, p := range palabrasDe(nombre) {
		if len(p) > 1 && !palabrasEPS[p] {
			l = append(l, p)
		}
	}
	return l
}

// coincide dice si cada palabra propia del patrón está, completa, entre las palabras de ADRES. Por partes
// no: "COLOMBIA" (EPS Familiar de Colombia) casaría con "COLOMBIANO" (Comfaoriente) y "SER" con "SERVICIO".
func coincide(patron []string, adres []string) bool {
	if len(patron) == 0 {
		return false
	}
	for _, p := range patron {
		hallada := false
		for _, w := range adres {
			if w == p {
				hallada = true
				break
			}
		}
		if !hallada {
			return false
		}
	}
	return true
}

// EPSAceptada dice si la EPS que devolvió ADRES es una de las aceptadas. Los nombres de los parámetros
// ("EPS Sura", "Savia Salud") no son los oficiales ("EPS SURAMERICANA S.A.", "ALIANZA MEDELLIN ANTIOQUIA EPS
// S.A.S."): se comparan palabra por palabra y con las razones sociales conocidas.
func EPSAceptada(epsAdres string, aceptadas []string) bool {
	adres := palabrasDe(epsAdres)
	if len(adres) == 0 {
		return false
	}
	for _, nombre := range aceptadas {
		propio := propias(nombre)
		if coincide(propio, adres) {
			return true
		}
		for _, otro := range otrosNombresEPS[strings.Join(propio, " ")] {
			if coincide(propias(otro), adres) {
				return true
			}
		}
	}
	return false
}
