package orquestador

import (
	"testing"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

func TestMotivoEdadRespetaElRangoDeContratacion(t *testing.T) {
	a := &Analizador{cfg: &configuracion.Configuracion{EdadMinimaContratacion: 22, EdadMaximaContratacion: 49}}
	for edad, fuera := range map[int]bool{0: false, 20: true, 21: true, 22: false, 35: false, 49: false, 50: true} {
		if got := a.motivoEdad(&modelos.Candidato{Edad: edad}) != ""; got != fuera {
			t.Errorf("edad %d: fuera de rango = %v, se esperaba %v", edad, got, fuera)
		}
	}
}
