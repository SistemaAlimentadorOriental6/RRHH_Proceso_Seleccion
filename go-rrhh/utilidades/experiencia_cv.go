package utilidades

import (
	"regexp"
	"strings"
)

// TrabajoCV es un empleo leído de un bloque «Empresa / Cargo / Tiempo laborado» de la hoja de vida.
type TrabajoCV struct {
	Empresa  string
	Cargo    string
	Duracion string // tal como figura: «4 años 6 meses»
}

// reEtiquetaTrabajo marca los campos de un bloque de experiencia. «Jefe inmediato» y «Teléfono» solo sirven
// para saber dónde termina el valor anterior (cuando el texto viene en una sola línea).
var reEtiquetaTrabajo = regexp.MustCompile(`(?i)\b(empresa|cargo|jefe\s+inmediato|tiempo\s+laborado|tel[eé]fono)\s*:`)

// reCargoConduccion: cargos que llevan al candidato al volante. «Operador» es ambiguo (también hay de
// maquinaria o de call center): por eso solo se anota y la IA decide de qué vehículo se trata.
var reCargoConduccion = regexp.MustCompile(`(?i)\b(conductor|conductora|chofer|operador|operadora|piloto)\b`)

// TrabajosDeCV lee los bloques de experiencia con la forma
//
//	Empresa : ALIANZA MAI
//	Cargo : Operador
//	Tiempo laborado : 4 años 6 meses
//
// aunque vengan en una sola línea. Cada «Empresa» abre un trabajo nuevo. Los formatos que no sigan esa
// forma no se leen aquí: los resuelve la IA con el texto completo.
func TrabajosDeCV(texto string) []TrabajoCV {
	marcas := reEtiquetaTrabajo.FindAllStringSubmatchIndex(texto, -1)
	var trabajos []TrabajoCV
	for i, m := range marcas {
		fin := len(texto)
		if i+1 < len(marcas) {
			fin = marcas[i+1][0]
		}
		valor := texto[m[1]:fin]
		if salto := strings.IndexByte(valor, '\n'); salto >= 0 {
			valor = valor[:salto]
		}
		valor = strings.TrimSpace(valor)
		switch strings.Fields(strings.ToLower(texto[m[2]:m[3]]))[0] {
		case "empresa":
			trabajos = append(trabajos, TrabajoCV{Empresa: valor})
		case "cargo":
			if n := len(trabajos); n > 0 && trabajos[n-1].Cargo == "" {
				trabajos[n-1].Cargo = valor
			}
		case "tiempo":
			if n := len(trabajos); n > 0 && trabajos[n-1].Duracion == "" {
				trabajos[n-1].Duracion = valor
			}
		}
	}
	return trabajos
}

// EsCargoDeConduccion dice si el cargo es de conductor, chofer u operador.
func EsCargoDeConduccion(cargo string) bool {
	return reCargoConduccion.MatchString(cargo)
}
