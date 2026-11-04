package modelos

import (
	"fmt"
	"strings"
)

// ReglasIdoneidad son los criterios con los que se evalúa a un candidato: una versión de la tabla
// reglas_idoneidad, que se edita desde Configuración > Parámetros. Cada persona se evalúa con la versión
// vigente cuando empezó su análisis y la conserva aunque después se guarden reglas nuevas.
type ReglasIdoneidad struct {
	EdadMinima         int      `json:"edadMinima"`
	EdadMaxima         int      `json:"edadMaxima"`
	EpsAceptadas       []string `json:"epsAceptadas"`
	AceptaReintegros   bool     `json:"aceptaReintegros"`
	GenerosAceptados   []string `json:"generosAceptados"` // "masculino", "femenino"
	LicenciasAceptadas []string `json:"licenciasAceptadas"`
	// ExperienciaMinimaMeses es la experiencia mínima como conductor, en meses (18 = 1 año y 6 meses).
	// shortcut: se guarda pero no se aplica (no hay de dónde leer la experiencia); aplicarla cuando la IA la extraiga.
	ExperienciaMinimaMeses int `json:"experienciaMinimaMeses"`
	// RuntAntiguedadMeses es el tiempo mínimo inscrito en el RUNT, en meses.
	RuntAntiguedadMeses int `json:"runtAntiguedadMeses"`
	// Version es el número de la versión de reglas; 0 si no salen de la base (reglas por defecto).
	Version int `json:"version"`
}

// generoDe traduce el género del candidato ("Hombre"/"Mujer") al de los parámetros.
var generoDe = map[string]string{"Hombre": "masculino", "Mujer": "femenino"}

// AceptaGenero dice si el género del candidato cumple. Si se aceptan ambos, el género no importa (ni
// siquiera hace falta saberlo).
func (r *ReglasIdoneidad) AceptaGenero(genero string) bool {
	if r.AceptaCualquierGenero() {
		return true
	}
	g, ok := generoDe[genero]
	return ok && contiene(r.GenerosAceptados, g)
}

// AceptaCualquierGenero indica que se aceptan hombres y mujeres.
func (r *ReglasIdoneidad) AceptaCualquierGenero() bool {
	return contiene(r.GenerosAceptados, "masculino") && contiene(r.GenerosAceptados, "femenino")
}

// GeneroRequerido es el texto del requisito de género para los motivos de descarte ("Hombre", "Mujer").
func (r *ReglasIdoneidad) GeneroRequerido() string {
	var nombres []string
	for _, g := range []string{"Hombre", "Mujer"} {
		if contiene(r.GenerosAceptados, generoDe[g]) {
			nombres = append(nombres, g)
		}
	}
	if len(nombres) == 0 {
		return "ninguno"
	}
	return strings.Join(nombres, " o ")
}

// NivelEdad clasifica la edad: mayor al tope descarta; menor a la mínima deja para semillero (todavía puede
// llegar a la edad). Una edad desconocida cumple: no se inventa.
func (r *ReglasIdoneidad) NivelEdad(edad int) (nivel, motivo string) {
	switch {
	case edad <= 0 || r.EdadMaxima <= 0:
		return NivelCumple, ""
	case edad > r.EdadMaxima:
		return NivelDescarte, fmt.Sprintf("Edad (%d años) mayor al tope de contratación (%d años)", edad, r.EdadMaxima)
	case edad < r.EdadMinima:
		return NivelSemillero, fmt.Sprintf("Edad (%d años) menor a la mínima de contratación (%d años)", edad, r.EdadMinima)
	}
	return NivelCumple, ""
}

// MotivoEdad devuelve por qué la edad queda fuera del rango, o "" si cumple o no se conoce.
func (r *ReglasIdoneidad) MotivoEdad(edad int) string {
	_, motivo := r.NivelEdad(edad)
	return motivo
}

// TextoMeses escribe una duración en meses como "1 año y 6 meses".
func TextoMeses(meses int) string {
	plural := func(n int, uno, varios string) string {
		if n == 1 {
			return "1 " + uno
		}
		return fmt.Sprintf("%d %s", n, varios)
	}
	a, m := meses/12, meses%12
	switch {
	case a == 0:
		return plural(m, "mes", "meses")
	case m == 0:
		return plural(a, "año", "años")
	}
	return plural(a, "año", "años") + " y " + plural(m, "mes", "meses")
}

func contiene(l []string, v string) bool {
	for _, x := range l {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}
