package servidor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-rrhh/almacen"
)

var _ parametrosBase = (*almacen.Almacen)(nil) // si el almacén deja de cumplirla, la pantalla respondería 503

type parametrosFalsos struct{ guardados *almacen.ParametrosAceptacion }

func (p *parametrosFalsos) ParametrosVigentes(context.Context) (almacen.ParametrosAceptacion, error) {
	return almacen.ParametrosAceptacion{Version: 2}, nil
}

func (p *parametrosFalsos) ParametrosVersion(_ context.Context, v int) (almacen.ParametrosAceptacion, error) {
	if v != 1 {
		return almacen.ParametrosAceptacion{}, fmt.Errorf("%w: %d", almacen.ErrVersionInexistente, v)
	}
	return almacen.ParametrosAceptacion{Version: 1, EpsAceptadas: []string{"Sura"}}, nil
}

func (p *parametrosFalsos) VersionesParametros(context.Context) ([]almacen.VersionReglas, error) {
	return []almacen.VersionReglas{{Version: 2}, {Version: 1}}, nil
}

func (p *parametrosFalsos) GuardarParametros(_ context.Context, g almacen.ParametrosAceptacion, _ string) (almacen.ParametrosAceptacion, error) {
	p.guardados = &g
	return g, nil
}

func TestParametrosVersionesYCamposObligatorios(t *testing.T) {
	falsos := &parametrosFalsos{}
	c := &ControladorRRHH{parametros: falsos}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/parametros-aceptacion", c.ManejarParametros)
	mux.HandleFunc("GET /api/parametros-aceptacion/versiones", c.ManejarVersionesParametros)
	mux.HandleFunc("GET /api/parametros-aceptacion/versiones/{version}", c.ManejarVersionParametros)
	pedirCon := func(metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo)))
		return rr
	}

	if rr := pedirCon("GET", "/api/parametros-aceptacion/versiones/1", ""); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"Sura"`) {
		t.Errorf("versión 1: %d %s", rr.Code, rr.Body)
	}
	if rr := pedirCon("GET", "/api/parametros-aceptacion/versiones/7", ""); rr.Code != 404 {
		t.Errorf("versión inexistente: %d", rr.Code)
	}
	if rr := pedirCon("GET", "/api/parametros-aceptacion/versiones/abc", ""); rr.Code != 400 {
		t.Errorf("versión inválida: %d", rr.Code)
	}
	if rr := pedirCon("GET", "/api/parametros-aceptacion/versiones", ""); rr.Code != 200 || !strings.HasPrefix(rr.Body.String(), "[") {
		t.Errorf("lista de versiones: %d %s", rr.Code, rr.Body)
	}

	completo := `{"edadMinima":22,"edadMaxima":49,"epsAceptadas":["Sura"],"aceptaReintegros":false,"generosAceptados":["masculino"],
		"licenciasAceptadas":["C2"],"experienciaMinimaMeses":18,"runtAntiguedadMeses":24}`
	if rr := pedirCon("PUT", "/api/parametros-aceptacion", completo); rr.Code != 200 || falsos.guardados == nil || falsos.guardados.ExperienciaMinimaMeses != 18 {
		t.Errorf("PUT completo: %d %s", rr.Code, rr.Body)
	}
	// Un número ausente o null se leería como 0: se rechaza en vez de guardarlo.
	for _, cuerpo := range []string{
		strings.Replace(completo, `"experienciaMinimaMeses":18,`, "", 1),
		strings.Replace(completo, `"runtAntiguedadMeses":24`, `"runtAntiguedadMeses":null`, 1),
		strings.Replace(completo, `}`, `,"otro":1}`, 1),
		`no es json`,
	} {
		falsos.guardados = nil
		if rr := pedirCon("PUT", "/api/parametros-aceptacion", cuerpo); rr.Code != 400 || falsos.guardados != nil {
			t.Errorf("PUT %q: %d (guardado=%v)", cuerpo, rr.Code, falsos.guardados != nil)
		}
	}
}
