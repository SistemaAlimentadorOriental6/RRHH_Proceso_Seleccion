package orquestador

import (
	"testing"

	"go-rrhh/modelos"
)

func TestMotivoSinCache(t *testing.T) {
	conCapturas := map[string]string{"adres": "a.jpg"}
	evCompleta := &modelos.EvaluacionIdoneidad{}
	descarte := &modelos.EvaluacionIdoneidad{DescarteTemprano: true}

	casos := []struct {
		nombre string
		opts   OpcionesAnalisis
		c      modelos.Candidato
		rehace bool
	}{
		{"completo con cédula y capturas", OpcionesAnalisis{}, modelos.Candidato{Cedula: "1037000111", Capturas: conCapturas, EvaluacionIdoneidad: evCompleta}, false},
		{"sin evaluación", OpcionesAnalisis{}, modelos.Candidato{Cedula: "1037000111", Capturas: conCapturas}, true},
		{"con error pendiente", OpcionesAnalisis{}, modelos.Candidato{Cedula: "1037000111", Capturas: conCapturas, EvaluacionIdoneidad: evCompleta, ErrorProceso: "falló la consulta de RUNT"}, true},
		{"completo pero sin cédula", OpcionesAnalisis{}, modelos.Candidato{Cedula: "No especificada", Capturas: conCapturas, EvaluacionIdoneidad: evCompleta}, true},
		{"completo pero sin capturas", OpcionesAnalisis{}, modelos.Candidato{Cedula: "1037000111", EvaluacionIdoneidad: evCompleta}, true},
		// Estos eran los que se rehacían en cada reinicio: descartados por una regla firme, sin consultas.
		{"descartado por género sin cédula ni capturas", OpcionesAnalisis{}, modelos.Candidato{Cedula: "No especificada", EvaluacionIdoneidad: descarte}, false},
		{"descartado temprano si se pide completo", OpcionesAnalisis{Completo: true}, modelos.Candidato{Cedula: "No especificada", EvaluacionIdoneidad: descarte}, true},
		{"en pausa por datos", OpcionesAnalisis{}, modelos.Candidato{EstadoProceso: modelos.EstadoEsperandoDatos, Cedula: "No especificada", EvaluacionIdoneidad: evCompleta}, false},
		{"forzar", OpcionesAnalisis{Forzar: true}, modelos.Candidato{Cedula: "1037000111", Capturas: conCapturas, EvaluacionIdoneidad: evCompleta}, true},
	}
	for _, c := range casos {
		a := &Analizador{opts: c.opts}
		if motivo := a.motivoSinCache(&c.c); (motivo != "") != c.rehace {
			t.Errorf("%s: motivo %q, se esperaba rehacer=%v", c.nombre, motivo, c.rehace)
		}
	}
}

func TestYaIncluido(t *testing.T) {
	cv := "EXPERIENCIA LABORAL\nEmpresa : ALIANZA MAI\nCargo : Operador"
	if !yaIncluido("Correo: a@b.c\n"+cv, "EXPERIENCIA  LABORAL Empresa : ALIANZA MAI Cargo : Operador") {
		t.Error("el mismo texto, con otros espacios, ya está incluido")
	}
	if yaIncluido("Perfil de la plataforma sin la experiencia", cv) {
		t.Error("un texto distinto no está incluido")
	}
}

func TestAusentesConfirmados(t *testing.T) {
	previos := map[string]*modelos.Candidato{"a": {}, "b": {}, "c": {}}
	// Primer listado corto: falta "c", pero todavía no se retira.
	if got := ausentesConfirmados("prueba-ausentes", previos, map[string]bool{"a": true, "b": true}); len(got) != 0 {
		t.Fatalf("a la primera ausencia no se retira a nadie: %v", got)
	}
	// "c" reaparece y ahora falta "b": tampoco se retira (no faltó dos veces seguidas).
	if got := ausentesConfirmados("prueba-ausentes", previos, map[string]bool{"a": true, "c": true}); len(got) != 0 {
		t.Fatalf("una ausencia aislada no retira: %v", got)
	}
	// "b" falta otra vez: se retira.
	if got := ausentesConfirmados("prueba-ausentes", previos, map[string]bool{"a": true, "c": true}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("retirados = %v; quería [b]", got)
	}
}
