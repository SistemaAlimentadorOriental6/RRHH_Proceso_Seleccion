package orquestador

import (
	"context"
	"errors"
	"testing"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

type iaGeneroFalsa struct {
	respuesta string
	err       error
	llamadas  int
}

func (f *iaGeneroFalsa) Calificar(context.Context, modelos.RegistroApto, string, string) (*modelos.EvaluacionIA, error) {
	return nil, errors.New("no se usa")
}
func (f *iaGeneroFalsa) DeterminarGenero(context.Context, string) (string, error) {
	f.llamadas++
	return f.respuesta, f.err
}

func analizadorConIA(ia EvaluadorIA) *Analizador {
	a := NuevoAnalizadorConValidadores(&configuracion.Configuracion{}, nil, OpcionesAnalisis{}, nil, Validadores{})
	a.ia = ia
	return a
}

func TestGeneroPorIACuandoElDiccionarioNoLoConoce(t *testing.T) {
	ia := &iaGeneroFalsa{respuesta: "Hombre"}
	a := analizadorConIA(ia)

	// «Anthony» no termina en «o» ni está en el diccionario: antes salía «No especificado».
	g, origen := a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Anthony romario Velez granados"})
	if g != "Hombre" || origen != " (IA)" {
		t.Fatalf("género = %q%s", g, origen)
	}
	// El mismo nombre no se vuelve a preguntar.
	a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "ANTHONY ROMARIO  VELEZ GRANADOS"})
	if ia.llamadas != 1 {
		t.Errorf("la IA se consultó %d veces; debía ser 1", ia.llamadas)
	}

	// Lo que consta en el perfil o está en el diccionario no pasa por la IA.
	ia.llamadas = 0
	if g, origen := a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Juan Pérez"}); g != "Hombre" || origen != "" {
		t.Errorf("Juan: %q%s", g, origen)
	}
	if g, _ := a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Xyz Qwe", TextoPerfil: "Género: Mujer"}); g != "Mujer" {
		t.Errorf("el perfil manda: %q", g)
	}
	if ia.llamadas != 0 {
		t.Errorf("no debía consultar a la IA: %d", ia.llamadas)
	}
}

func TestGeneroSiLaIAFallaOEsAmbigua(t *testing.T) {
	// IA caída: se usa el método de respaldo (terminación del nombre) en vez de dejar al candidato sin género.
	a := analizadorConIA(&iaGeneroFalsa{err: errors.New("sin red")})
	if g, origen := a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Anthony Velez"}); g != "No especificado" || origen != "" {
		t.Errorf("respaldo: %q%s", g, origen)
	}
	// Ambiguo: tampoco se inventa; queda lo que diga el respaldo.
	a = analizadorConIA(&iaGeneroFalsa{respuesta: "Indeterminado"})
	if g, _ := a.determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Alexis Ruiz"}); g == "Mujer" {
		t.Errorf("no debe afirmar Mujer: %q", g)
	}
	// Sin IA configurada todo sigue como antes.
	if g, _ := analizadorConIA(nil).determinarGenero(context.Background(), &modelos.Candidato{Nombre: "Carlos Ruiz"}); g != "Hombre" {
		t.Errorf("sin IA: %q", g)
	}
}
