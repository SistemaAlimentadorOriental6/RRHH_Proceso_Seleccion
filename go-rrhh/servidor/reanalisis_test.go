package servidor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
)

var _ reanalisisBase = (*almacen.Almacen)(nil)

type reanalisisFalso struct{ encolados []string }

func (f *reanalisisFalso) ParametrosVigentes(context.Context) (almacen.ParametrosAceptacion, error) {
	return almacen.ParametrosAceptacion{Version: 2, EdadMinima: 18, EdadMaxima: 60, GenerosAceptados: []string{"masculino", "femenino"}}, nil
}

func (f *reanalisisFalso) DescartadosVigentes(context.Context) ([]modelos.Candidato, error) {
	ev := func(v int) *modelos.EvaluacionIdoneidad {
		return &modelos.EvaluacionIdoneidad{ReglaVersion: v, MotivosDescarte: []string{"motivo"}}
	}
	return []modelos.Candidato{
		{Nombre: "Ana", Cedula: "10203040", Genero: "Mujer", Edad: 30, PostulacionID: "p-ana", EvaluacionIdoneidad: ev(1)},
		{Nombre: "Beto", Cedula: "10203041", Genero: "Hombre", Edad: 70, PostulacionID: "p-beto", EvaluacionIdoneidad: ev(1)},
		{Nombre: "Caro", Cedula: "10203042", Genero: "Mujer", Edad: 30, PostulacionID: "p-caro", EvaluacionIdoneidad: ev(2)},
	}, nil
}

func (f *reanalisisFalso) EncolarReanalisis(_ context.Context, ids []string) (int, error) {
	f.encolados = append(f.encolados, ids...)
	return len(ids), nil
}

func (f *reanalisisFalso) ReanalisisEnCola(context.Context) ([]string, error) { return f.encolados, nil }

// El trabajador vuelve a comprobar a cada descartado antes de reanalizarlo: quien con los parámetros vigentes
// sigue sin cumplir (aunque se haya encolado antes) no gasta ninguna consulta.
func TestElTrabajadorNoReanalizaAQuienSigueSinCumplir(t *testing.T) {
	descartado := func(motivo string) *modelos.Candidato {
		c := candidatoQueEntregoSusDatos()
		c.EstadoProceso, c.Edad = "", 30
		c.EvaluacionIdoneidad = &modelos.EvaluacionIdoneidad{ReglaVersion: 1, Genero: "Hombre", MotivosDescarte: []string{motivo},
			Clasificacion: modelos.ClasificacionDescartado}
		return c
	}
	casos := []struct {
		nombre    string
		candidato *modelos.Candidato
		consulta  bool
	}{
		// Con la v2 de reanalisisFalso no se aceptan reintegros: Henry sigue descartado.
		{"reintegro sin aceptarse", descartado("Reintegro: ya fue empleado: no se admiten reintegros"), false},
		{"ya no está descartado", func() *modelos.Candidato {
			c := descartado("x")
			c.EsApto, c.EvaluacionIdoneidad.EsApto, c.EvaluacionIdoneidad.Clasificacion = true, true, modelos.ClasificacionReclutadoP1
			return c
		}(), false},
		{"ahora podría cumplir", descartado("Edad (55 años) mayor al tope de contratación (49 años)"), true},
	}
	for _, caso := range casos {
		consultas := nuevasConsultas("CARLOS", "PEREZ")
		ctrl, trabajos, _ := montarTrabajador(t, consultas)
		ctrl.reanalisis = &reanalisisFalso{}
		trabajos.pendiente.Origen = almacen.OrigenReanalisis
		trabajos.candidato = caso.candidato
		if !ctrl.reanudarUna("prueba") {
			t.Fatalf("%s: debía procesar el trabajo", caso.nombre)
		}
		if len(trabajos.terminados) != 1 || trabajos.terminados[0] != nil {
			t.Errorf("%s: el trabajo debía cerrarse sin error: %v", caso.nombre, trabajos.terminados)
		}
		if got := consultas.adres.Load() > 0; got != caso.consulta {
			t.Errorf("%s: consultó ADRES = %v, se esperaba %v", caso.nombre, got, caso.consulta)
		}
	}
}

func TestReanalizarDescartados(t *testing.T) {
	falso := &reanalisisFalso{}
	c := &ControladorRRHH{reanalisis: falso, despertar: make(chan struct{}, 1)}
	pedirCon := func(ruta string) respuestaReanalisis {
		rr := httptest.NewRecorder()
		c.ManejarReanalizarDescartados(rr, httptest.NewRequest(http.MethodPost, ruta, nil))
		if rr.Code != 200 {
			t.Fatalf("%s: código %d %s", ruta, rr.Code, rr.Body)
		}
		var res respuestaReanalisis
		if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	// Simular no encola nada: solo Ana (Beto sigue sobre la edad tope; Caro ya se evaluó con la v2).
	res := pedirCon("/api/descartados/reanalizar?simular=1")
	if !res.Simulado || res.Encolados != 0 || len(falso.encolados) != 0 || len(res.Reanalizar) != 1 || res.Reanalizar[0].Nombre != "Ana" {
		t.Fatalf("simulación: %+v (encolados %v)", res, falso.encolados)
	}
	if len(res.Omitidos) != 2 {
		t.Errorf("omitidos: %+v", res.Omitidos)
	}

	res = pedirCon("/api/descartados/reanalizar")
	if res.Simulado || res.Encolados != 1 || len(falso.encolados) != 1 || falso.encolados[0] != "p-ana" {
		t.Fatalf("reanálisis: %+v (encolados %v)", res, falso.encolados)
	}
	select {
	case <-c.despertar:
	default:
		t.Error("debía despertar al trabajador de segundo plano")
	}

	// Sin base de datos no hay nada que reanalizar.
	rr := httptest.NewRecorder()
	(&ControladorRRHH{}).ManejarReanalizarDescartados(rr, httptest.NewRequest(http.MethodPost, "/api/descartados/reanalizar", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("sin base: %d", rr.Code)
	}
}
