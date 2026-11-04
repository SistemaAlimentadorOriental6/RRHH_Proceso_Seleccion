package orquestador

import (
	"context"
	"errors"
	"testing"

	"go-rrhh/modelos"
)

type iaFalsa struct {
	ev     *modelos.EvaluacionIA
	err    error
	vistos int
}

func (f *iaFalsa) Calificar(context.Context, modelos.RegistroApto, string, string) (*modelos.EvaluacionIA, error) {
	f.vistos++
	return f.ev, f.err
}

type reporteroEtapas struct{ etapas []string }

func (r *reporteroEtapas) Emitir(tipo string, datos any) {
	if m, ok := datos.(map[string]any); ok && tipo == "candidato.etapa" {
		r.etapas = append(r.etapas, m["etapa"].(string)+":"+m["estado"].(string))
	}
}

func TestEvaluarConIASoloCalificaAptosSinErrores(t *testing.T) {
	ia := &iaFalsa{ev: &modelos.EvaluacionIA{ScoreTotal: 72, Dictamen: "Apto Recomendado"}}
	rep := &reporteroEtapas{}
	a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, rep, Validadores{}).ConEvaluadorIA(ia)

	apto := &modelos.Candidato{Nombre: "A", EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: true}}
	if !a.evaluarConIA(context.Background(), apto) || apto.EvaluacionIA == nil || apto.EvaluacionIA.ScoreTotal != 72 {
		t.Fatalf("el apto debe quedar calificado: %+v", apto.EvaluacionIA)
	}
	if got := rep.etapas; len(got) != 2 || got[0] != "ia:iniciada" || got[1] != "ia:ok" {
		t.Fatalf("etapas emitidas: %v", got)
	}

	for nombre, c := range map[string]*modelos.Candidato{
		"no apto":       {EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: false}},
		"con error":     {ErrorProceso: "falló", EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: true}},
		"ya calificado": {EvaluacionIA: &modelos.EvaluacionIA{}, EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: true}},
		"sin dictamen":  {},
	} {
		antes := ia.vistos
		if a.evaluarConIA(context.Background(), c) || ia.vistos != antes {
			t.Errorf("%s: no debe llamar a la IA", nombre)
		}
	}
}

func TestEvaluarConIAConFalloDejaAlAptoSinPuntaje(t *testing.T) {
	rep := &reporteroEtapas{}
	a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, rep, Validadores{}).ConEvaluadorIA(&iaFalsa{err: errors.New("cuota")})
	c := &modelos.Candidato{EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{EsApto: true}}
	if a.evaluarConIA(context.Background(), c) || c.EvaluacionIA != nil {
		t.Fatal("si la IA falla no debe quedar calificación")
	}
	if rep.etapas[len(rep.etapas)-1] != "ia:error" {
		t.Fatalf("debe informar el error: %v", rep.etapas)
	}
}

// La IA también califica a los semilleros (siguen en el proceso); no a descartados ni a pendientes.
func TestEvaluarConIACalificaSemilleros(t *testing.T) {
	for _, caso := range []struct {
		nombre string
		ev     *modelos.EvaluacionIdoneidad
		quiero bool
	}{
		{"semillero", &modelos.EvaluacionIdoneidad{Clasificacion: modelos.ClasificacionSemillero}, true},
		{"reclutado P1", &modelos.EvaluacionIdoneidad{EsApto: true, Clasificacion: modelos.ClasificacionReclutadoP1}, true},
		{"descartado", &modelos.EvaluacionIdoneidad{Clasificacion: modelos.ClasificacionDescartado}, false},
		{"pendiente", &modelos.EvaluacionIdoneidad{VerificacionPendiente: true}, false},
	} {
		ia := &iaFalsa{ev: &modelos.EvaluacionIA{ScoreTotal: 60, Dictamen: "Apto"}}
		a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, &reporteroEtapas{}, Validadores{}).ConEvaluadorIA(ia)
		c := &modelos.Candidato{Nombre: "A", EvaluacionIdoneidad: caso.ev}
		if got := a.evaluarConIA(context.Background(), c); got != caso.quiero {
			t.Errorf("%s: calificó = %v, se esperaba %v", caso.nombre, got, caso.quiero)
		}
	}
}
