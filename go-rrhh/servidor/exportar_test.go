package servidor

import (
	"testing"

	"go-rrhh/modelos"
)

func TestEsAptoParaExportar(t *testing.T) {
	casos := []struct {
		nombre string
		c      modelos.Candidato
		quiero bool
	}{
		{"apto", modelos.Candidato{EsApto: true}, true},
		{"no apto", modelos.Candidato{}, false},
		{"apto con error", modelos.Candidato{EsApto: true, ErrorProceso: "x"}, false},
		{"verificación pendiente", modelos.Candidato{EsApto: true, EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{VerificacionPendiente: true}}, false},
	}
	for _, k := range casos {
		if got := esAptoParaExportar(&k.c); got != k.quiero {
			t.Errorf("%s: got %v, quiero %v", k.nombre, got, k.quiero)
		}
	}
}

func TestNombreSeguro(t *testing.T) {
	if got := nombreSeguro(`JOSÉ "Pérez"/..\x`); got != "JOSÉ Pérez..x" {
		t.Errorf("got %q", got)
	}
	if got := nombreSeguro("///"); got != "sin-nombre" {
		t.Errorf("got %q", got)
	}
}

func TestHayPendientesTecnicos(t *testing.T) {
	casos := []struct {
		nombre string
		lista  []modelos.Candidato
		quiero bool
	}{
		{"vacía", nil, false},
		{"consulta fallida", []modelos.Candidato{{ErrorProceso: "falló la consulta de RUNT"}}, true},
		{"espera sus datos: no se resuelve reintentando", []modelos.Candidato{{ErrorProceso: "x", EstadoProceso: modelos.EstadoEsperandoDatos}}, false},
		{"todo en orden", []modelos.Candidato{{EsApto: true}}, false},
	}
	for _, c := range casos {
		if got := hayPendientesTecnicos(c.lista); got != c.quiero {
			t.Errorf("%s: got %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}
