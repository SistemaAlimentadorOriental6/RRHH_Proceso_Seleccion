package almacen

import (
	"errors"
	"reflect"
	"testing"
)

func validos() ParametrosAceptacion {
	return ParametrosAceptacion{
		EdadMinima: 22, EdadMaxima: 49, EpsAceptadas: []string{"Nueva EPS", " Sura ", "sura"},
		GenerosAceptados: []string{"masculino"}, LicenciasAceptadas: []string{"C2"}, ExperienciaMinimaMeses: 18, RuntAntiguedadMeses: 24,
	}
}

func TestNormalizarParametros(t *testing.T) {
	p := validos()
	if err := Normalizar(&p); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Nueva EPS", "Sura"}; !reflect.DeepEqual(p.EpsAceptadas, want) {
		t.Errorf("EPS = %v, quiero %v", p.EpsAceptadas, want)
	}

	casos := map[string]func(*ParametrosAceptacion){
		"edad menor a 18":             func(p *ParametrosAceptacion) { p.EdadMinima = 17 },
		"edad final < inicial":        func(p *ParametrosAceptacion) { p.EdadMaxima = 20 },
		"edad final > 99":             func(p *ParametrosAceptacion) { p.EdadMaxima = 120 },
		"experiencia negativa":        func(p *ParametrosAceptacion) { p.ExperienciaMinimaMeses = -1 },
		"experiencia mayor a 60 años": func(p *ParametrosAceptacion) { p.ExperienciaMinimaMeses = 721 },
		"RUNT negativo":               func(p *ParametrosAceptacion) { p.RuntAntiguedadMeses = -1 },
		"sin EPS":                     func(p *ParametrosAceptacion) { p.EpsAceptadas = nil },
		"EPS vacía":                   func(p *ParametrosAceptacion) { p.EpsAceptadas = []string{"  "} },
		"sin géneros":                 func(p *ParametrosAceptacion) { p.GenerosAceptados = nil },
		"género desconocido":          func(p *ParametrosAceptacion) { p.GenerosAceptados = []string{"otro"} },
		"sin licencias":               func(p *ParametrosAceptacion) { p.LicenciasAceptadas = []string{} },
		"licencia desconocida":        func(p *ParametrosAceptacion) { p.LicenciasAceptadas = []string{"Z9"} },
	}
	for nombre, rompe := range casos {
		p := validos()
		rompe(&p)
		if err := Normalizar(&p); !errors.Is(err, ErrParametrosInvalidos) {
			t.Errorf("%s: err = %v, quiero ErrParametrosInvalidos", nombre, err)
		}
	}
}
