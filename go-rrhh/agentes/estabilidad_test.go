package agentes

import (
	"testing"
	"time"

	"go-rrhh/modelos"
)

var hoyPrueba = time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)

func exp(duraciones ...string) []modelos.ExperienciaLaboral {
	var l []modelos.ExperienciaLaboral
	for i, d := range duraciones {
		l = append(l, modelos.ExperienciaLaboral{Empresa: string(rune('A' + i)), Duracion: d})
	}
	return l
}

func TestMesesDeDuracion(t *testing.T) {
	for d, quiero := range map[string]int{"1 año y 11 meses": 23, "5 años, 8 meses": 68, "9 meses": 9, "1 mes": 1, "3 años": 36, "2 AÑOS Y 3 MESES": 27} {
		if got, ok := mesesDeDuracion(d); !ok || got != quiero {
			t.Errorf("%q = %d (%v); quería %d", d, got, ok, quiero)
		}
	}
	for _, d := range []string{"", "2 semanas", "actualmente"} {
		if _, ok := mesesDeDuracion(d); ok {
			t.Errorf("%q no es medible", d)
		}
	}
}

func TestEstabilidadPorPermanencia(t *testing.T) {
	casos := []struct {
		nombre string
		exp    []modelos.ExperienciaLaboral
		quiero int // 0 = no puntúa
	}{
		// El caso reportado: 1a11m (vigente), 3a8m y 1a10m recientes; el resto, oficios cortos y viejos.
		{"reportado", exp("1 año y 11 meses", "3 años y 8 meses", "1 año y 10 meses", "1 mes", "9 meses", "6 meses", "2 años y 3 meses"), 18},
		{"dos largos", exp("4 años", "3 años", "2 años"), 19},
		{"todo largo", exp("4 años", "5 años", "3 años"), 20},
		{"estable sin picos", exp("2 años", "20 meses", "2 años y 2 meses"), 17},
		{"entre 1 y 1,5 años", exp("14 meses", "2 años", "13 meses"), 14},
		{"una salida corta", exp("10 meses", "3 años", "2 años"), 13},
		{"dos cortas", exp("8 meses", "7 meses", "2 años"), 9},
		{"rotación alta", exp("6 meses", "5 meses", "4 meses", "8 meses"), 6},
		{"los de menos de 3 meses no cuentan", exp("2 meses", "1 mes", "2 años", "2 años"), 17},
		{"un solo empleo medible: se conserva la IA", exp("3 años"), 0},
		{"sin duraciones", exp("", "actualmente"), 0},
	}
	for _, c := range casos {
		got, detalle, ok := estabilidadPorPermanencia(c.exp, hoyPrueba)
		if c.quiero == 0 {
			if ok {
				t.Errorf("%s: no debía puntuar y dio %d", c.nombre, got)
			}
			continue
		}
		if !ok || got != c.quiero || detalle == "" {
			t.Errorf("%s: got %d (ok=%v), quería %d", c.nombre, got, ok, c.quiero)
		}
	}
}

func TestAjustarEvaluacionUsaLaEstabilidadCalculada(t *testing.T) {
	ev := &modelos.EvaluacionIA{ScoreEstabilidad: 10, JustEstabilidad: "la IA dijo otra cosa",
		Experiencia: exp("1 año y 11 meses", "3 años y 8 meses", "1 año y 10 meses")}
	ajustarEvaluacion(ev, modelos.RegistroApto{}, "conductor de buses")
	if ev.ScoreEstabilidad != 18 || ev.JustEstabilidad == "la IA dijo otra cosa" {
		t.Errorf("estabilidad %d / %q", ev.ScoreEstabilidad, ev.JustEstabilidad)
	}
	// Sin experiencia medible se respeta lo que dio la IA.
	ev = &modelos.EvaluacionIA{ScoreEstabilidad: 12, JustEstabilidad: "sin fechas"}
	ajustarEvaluacion(ev, modelos.RegistroApto{}, "conductor de buses")
	if ev.ScoreEstabilidad != 12 {
		t.Errorf("debía conservar 12, dio %d", ev.ScoreEstabilidad)
	}
}

func TestEstabilidadUsaLasFechasNoLaCuentaDeLaIA(t *testing.T) {
	// La IA escribió "11 meses" para un empleo vigente desde 2024-11; las fechas dicen 23.
	e := []modelos.ExperienciaLaboral{
		{Empresa: "Impresos", Inicio: "2024-11", Fin: "actual", Duracion: "11 meses"},
		{Empresa: "Coordinadora", Inicio: "2020-06", Fin: "2024-02", Duracion: "3 años, 8 meses"},
		{Empresa: "Maxioferta", Inicio: "2018-08", Fin: "2020-06", Duracion: "1 año, 10 meses"},
	}
	puntos, detalle, ok := estabilidadPorPermanencia(e, hoyPrueba)
	if !ok || puntos != 18 {
		t.Fatalf("puntos %d (ok=%v): %s", puntos, ok, detalle)
	}
	if e[0].Duracion != "1 año y 11 meses" {
		t.Errorf("la duración debe quedar calculada con las fechas: %q", e[0].Duracion)
	}
	if m, ok := mesesEntre("2024-11", "2024-10", hoyPrueba); ok || m != 0 {
		t.Error("un fin anterior al inicio no es válido")
	}
	for meses, quiero := range map[int]string{1: "1 mes", 8: "8 meses", 12: "1 año", 24: "2 años", 23: "1 año y 11 meses", 38: "3 años y 2 meses"} {
		if got := textoDuracion(meses); got != quiero {
			t.Errorf("textoDuracion(%d) = %q; quería %q", meses, got, quiero)
		}
	}
}
