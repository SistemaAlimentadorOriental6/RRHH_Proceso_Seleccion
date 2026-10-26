package servidor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-rrhh/almacen"
)

type sesionesFalsas struct{}

func (sesionesFalsas) IniciarSesion(_ context.Context, correo, clave string) (almacen.Usuario, string, error) {
	if correo == "a@b.co" && clave == "bien" {
		return almacen.Usuario{ID: "1", Correo: correo, Nombre: "A", Cargo: "C"}, "tok", nil
	}
	return almacen.Usuario{}, "", almacen.ErrCredenciales
}

func (sesionesFalsas) UsuarioDeSesion(_ context.Context, token string) (almacen.Usuario, bool, error) {
	return almacen.Usuario{ID: "1"}, token == "tok", nil
}

func (sesionesFalsas) CerrarSesion(context.Context, string) error { return nil }

func TestConSesionProtegeLaAPI(t *testing.T) {
	c := &ControladorRRHH{sesiones: sesionesFalsas{}}
	h := c.conSesion(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	pedir := func(ruta, token string) int {
		r := httptest.NewRequest(http.MethodGet, ruta, nil)
		if token != "" {
			r.AddCookie(&http.Cookie{Name: cookieSesion, Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	casos := []struct {
		ruta, token string
		esperado    int
	}{
		{"/api/vacantes", "", 401},
		{"/api/vacantes", "mala", 401},
		{"/api/vacantes", "tok", 200},
		{"/api/salud", "", 200},
		{"/api/validar-datos/abc", "", 200}, // página pública del candidato
		{"/", "", 200},
	}
	for _, k := range casos {
		if got := pedir(k.ruta, k.token); got != k.esperado {
			t.Errorf("%s con token %q: %d, se esperaba %d", k.ruta, k.token, got, k.esperado)
		}
	}

	// Sin base de datos la API sigue abierta, como antes.
	abierta := (&ControladorRRHH{}).conSesion(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	w := httptest.NewRecorder()
	abierta.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/vacantes", nil))
	if w.Code != 200 {
		t.Errorf("sin sesiones configuradas se esperaba 200, fue %d", w.Code)
	}
}
