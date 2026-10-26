package agentes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-rrhh/modelos"
)

var (
	reAnosDuracion  = regexp.MustCompile(`(\d+)\s*a[ñn]os?`)
	reMesesDuracion = regexp.MustCompile(`(\d+)\s*mes(?:es)?`)
)

// mesesDeDuracion convierte "1 año y 11 meses", "5 años, 8 meses" o "9 meses" en meses.
func mesesDeDuracion(duracion string) (int, bool) {
	d := strings.ToLower(duracion)
	a, m := reAnosDuracion.FindStringSubmatch(d), reMesesDuracion.FindStringSubmatch(d)
	if a == nil && m == nil {
		return 0, false // "2 semanas", "actualmente" o vacío: no se puede medir
	}
	total := 0
	if a != nil {
		n, _ := strconv.Atoi(a[1])
		total += n * 12
	}
	if m != nil {
		n, _ := strconv.Atoi(m[1])
		total += n
	}
	return total, true
}

// mesesEntre cuenta los meses entre dos fechas AAAA-MM ("actual" en fin es hoy). ok es falso si falta
// una fecha, no se entiende o el fin es anterior al inicio.
func mesesEntre(inicio, fin string, hoy time.Time) (int, bool) {
	ini, ok1 := mesAbsoluto(inicio, hoy)
	fi, ok2 := mesAbsoluto(fin, hoy)
	if !ok1 || !ok2 || fi < ini {
		return 0, false
	}
	return fi - ini, true
}

func mesAbsoluto(f string, hoy time.Time) (int, bool) {
	f = strings.ToLower(strings.TrimSpace(f))
	if f == "actual" {
		return hoy.Year()*12 + int(hoy.Month()) - 1, true
	}
	t, err := time.Parse("2006-01", f)
	if err != nil {
		return 0, false
	}
	return t.Year()*12 + int(t.Month()) - 1, true
}

// textoDuracion escribe meses como "1 año y 11 meses".
func textoDuracion(meses int) string {
	anos, resto := meses/12, meses%12
	pl := func(n int, uno, varios string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s", uno)
		}
		return fmt.Sprintf("%d %s", n, varios)
	}
	switch {
	case anos == 0:
		return pl(resto, "mes", "meses")
	case resto == 0:
		return pl(anos, "año", "años")
	}
	return pl(anos, "año", "años") + " y " + pl(resto, "mes", "meses")
}

// estabilidadPorPermanencia puntúa la estabilidad laboral (0 a 20) con las permanencias de los 3
// empleos más recientes que duraron 3 meses o más. La IA es poco consistente con este criterio (un
// mismo caso salió entre 10 y 15 de 20), así que se calcula aquí con los datos que ella extrajo:
//
//	los 3 recientes de 1,5 años o más → 17; 18 si alguno pasa de 3 años; 19 con dos; 20 con los tres
//	hay entre 1 y 1,5 años (ninguno por debajo de 1 año)                  → 14
//	una permanencia menor de 1 año                                          → 13
//	dos                                                                     → 9
//	tres (rotación alta y reciente)                                         → 6
//
// Los empleos de menos de 3 meses no cuentan (ni a favor ni en contra), y un empleo vigente cuenta con
// el tiempo que lleve. Solo puntúa si hay al menos 2 empleos medibles; si no, devuelve false y se
// conserva la nota de la IA. experiencia debe venir del más reciente al más antiguo.
func estabilidadPorPermanencia(experiencia []modelos.ExperienciaLaboral, hoy time.Time) (puntos int, detalle string, ok bool) {
	type permanencia struct {
		empresa  string
		duracion string
		meses    int
	}
	var recientes []permanencia
	for i := range experiencia {
		e := &experiencia[i]
		// Las fechas mandan: la IA se equivoca al restarlas (dio 11 meses a un empleo vigente de 23).
		m, medible := mesesEntre(e.Inicio, e.Fin, hoy)
		if medible {
			e.Duracion = textoDuracion(m)
		} else {
			m, medible = mesesDeDuracion(e.Duracion)
		}
		if medible && m >= 3 {
			recientes = append(recientes, permanencia{strings.TrimSpace(e.Empresa), strings.TrimSpace(e.Duracion), m})
			if len(recientes) == 3 {
				break
			}
		}
	}
	if len(recientes) < 2 {
		return 0, "", false
	}

	cortos, medios, largos := 0, 0, 0
	partes := make([]string, len(recientes))
	for i, p := range recientes {
		switch {
		case p.meses < 12:
			cortos++
		case p.meses < 18:
			medios++
		}
		if p.meses >= 36 {
			largos++
		}
		partes[i] = fmt.Sprintf("%s (%s)", p.empresa, p.duracion)
	}
	switch {
	case cortos >= 3 || (cortos == 2 && len(recientes) == 2):
		puntos = 6
	case cortos == 2:
		puntos = 9
	case cortos == 1:
		puntos = 13
	case medios > 0:
		puntos = 14
	default:
		puntos = min(17+largos, 20)
	}
	return puntos, "Permanencia en los empleos más recientes: " + strings.Join(partes, "; ") + ".", true
}
