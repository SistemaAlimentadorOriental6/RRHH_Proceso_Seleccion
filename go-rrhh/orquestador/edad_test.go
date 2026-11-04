package orquestador

import (
	"testing"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

func TestMotivoEdadRespetaElRangoDeContratacion(t *testing.T) {
	a := &Analizador{clasif: agentes.NuevoAgenteClasificador(&configuracion.Configuracion{EdadMinimaContratacion: 22, EdadMaximaContratacion: 49})}
	for edad, fuera := range map[int]bool{0: false, 20: true, 21: true, 22: false, 35: false, 49: false, 50: true} {
		c := &modelos.Candidato{Edad: edad}
		if got := a.clasif.ReglasDe(c).MotivoEdad(edad) != ""; got != fuera {
			t.Errorf("edad %d: fuera de rango = %v, se esperaba %v", edad, got, fuera)
		}
	}
	// Con reglas propias manda su rango, no el de la configuración.
	c := &modelos.Candidato{Edad: 55, Reglas: &modelos.ReglasIdoneidad{EdadMinima: 18, EdadMaxima: 60}}
	if m := a.clasif.ReglasDe(c).MotivoEdad(c.Edad); m != "" {
		t.Errorf("55 años con rango 18-60 debe cumplir: %q", m)
	}
}
