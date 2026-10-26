package orquestador

import (
	"context"
	"errors"
	"testing"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

func validadorReintegro(estado string, alerta bool, err error) FuncionValidar {
	return func(_ context.Context, c *modelos.Candidato) error {
		c.ResultadoReintegro = &modelos.ResultadoReintegro{Estado: estado, Alerta: alerta, Detalles: "detalle de prueba"}
		return err
	}
}

func TestEtapaReintegroRechazaAQuienYaFueEmpleado(t *testing.T) {
	casos := []struct {
		nombre, quieroEtapa, quieroEstado string
		validar                           FuncionValidar
	}{
		{"ya fue empleado", "reintegro:rechazado", estadoRechazado, validadorReintegro(modelos.ReintegroExEmpleado, true, nil)},
		{"empleado activo", "reintegro:rechazado", estadoRechazado, validadorReintegro(modelos.ReintegroEmpleadoActivo, true, nil)},
		{"sin registro", "reintegro:ok", estadoOK, validadorReintegro(modelos.ReintegroSinRegistro, false, nil)},
		{"servicio caído", "reintegro:error", estadoError, validadorReintegro(modelos.ReintegroErrorConsulta, false, errors.New("sin respuesta"))},
	}
	for _, caso := range casos {
		rep := &reporteroEtapas{}
		a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, rep, Validadores{Reintegro: caso.validar})
		r, continuar := a.consultarReintegro(context.Background(), &modelos.Candidato{Nombre: "A", Cedula: "123456"})
		if !continuar || r.estado != caso.quieroEstado {
			t.Errorf("%s: estado %q (continuar=%v), se esperaba %q", caso.nombre, r.estado, continuar, caso.quieroEstado)
		}
		if got := rep.etapas; len(got) == 0 || got[len(got)-1] != caso.quieroEtapa {
			t.Errorf("%s: etapas emitidas %v, la última debe ser %s", caso.nombre, got, caso.quieroEtapa)
		}
	}

	// Sin credenciales de Administremos la etapa no existe y nadie queda detenido por ella.
	rep := &reporteroEtapas{}
	a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, rep, Validadores{})
	if r, continuar := a.consultarReintegro(context.Background(), &modelos.Candidato{}); !continuar || r.estado != estadoOK || len(rep.etapas) != 0 {
		t.Errorf("sin validador no debe emitirse nada: %v %+v", rep.etapas, r)
	}
}

// Quien ya fue empleado termina ahí: no se gasta ninguna consulta de EPS, RUNT ni SIMIT.
func TestReintegroTerminaElProcesoAntesDeADRES(t *testing.T) {
	consultadas := []string{}
	marcar := func(nombre string) FuncionValidar {
		return func(context.Context, *modelos.Candidato) error {
			consultadas = append(consultadas, nombre)
			return nil
		}
	}
	rep := &reporteroEtapas{}
	cfg := &configuracion.Configuracion{DirectorioDescargas: t.TempDir(), EdadMinimaContratacion: 22, EdadMaximaContratacion: 49}
	a := NuevoAnalizadorConValidadores(cfg, nil, OpcionesAnalisis{}, rep, Validadores{
		Reintegro: validadorReintegro(modelos.ReintegroExEmpleado, true, nil),
		Adres:     marcar("adres"), Runt: marcar("runt"), Simit: marcar("simit"),
	})
	c := &modelos.Candidato{Nombre: "Ex Empleado", Cedula: "123456789", Genero: "Hombre"}
	a.continuarDesdeCedula(context.Background(), c, t.TempDir(), &acumuladorResultados{}, false)

	if len(consultadas) != 0 {
		t.Errorf("no debía consultarse nada más: %v", consultadas)
	}
	ev := c.EvaluacionIdoneidad
	if c.EsApto || ev == nil || ev.VerificacionPendiente || len(ev.MotivosDescarte) != 1 {
		t.Fatalf("debe quedar descartado (no apto, sin pendiente) con su motivo: %+v", ev)
	}
	if ev.MotivosDescarte[0] == "" || ev.MotivosDescarte[0][:10] != "Reintegro:" {
		t.Errorf("el motivo debe decir que es un reintegro: %q", ev.MotivosDescarte[0])
	}
}
