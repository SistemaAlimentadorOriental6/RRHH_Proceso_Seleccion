package utilidades

import "testing"

const cvEjemplo = `EXPERIENCIA LABORAL
Empresa : ALIANZA MAI
Cargo : Operador
Jefe Inmediato : Andrés Gaviria
Tiempo laborado : 4 años 6 meses
Teléfono : 3116324868
Empresa : TEMSA
Cargo : Conductor
Jefe Inmediato : Esteban Cardona
Tiempo laborado : 5 meses
Teléfono : 3234542451
Empresa : SEDEXCO ( Noel)
Cargo : Auxiliar logístico
Jefe Inmediato : Wilson Arango
Tiempo laborado : 2 años
Teléfono : 6046040056`

func comprobarTrabajos(t *testing.T, texto string) {
	t.Helper()
	esperado := []TrabajoCV{
		{"ALIANZA MAI", "Operador", "4 años 6 meses"},
		{"TEMSA", "Conductor", "5 meses"},
		{"SEDEXCO ( Noel)", "Auxiliar logístico", "2 años"},
	}
	got := TrabajosDeCV(texto)
	if len(got) != len(esperado) {
		t.Fatalf("trabajos = %+v", got)
	}
	for i := range esperado {
		if got[i] != esperado[i] {
			t.Errorf("trabajo %d = %+v, quería %+v", i, got[i], esperado[i])
		}
	}
}

func TestTrabajosDeCVEnLineas(t *testing.T) { comprobarTrabajos(t, cvEjemplo) }

// Los PDF a veces entregan todo en una sola línea.
func TestTrabajosDeCVEnUnaLinea(t *testing.T) {
	comprobarTrabajos(t, "Empresa : ALIANZA MAI Cargo : Operador Jefe Inmediato : Andrés Gaviria Tiempo laborado : 4 años 6 meses Teléfono : 3116324868 "+
		"Empresa : TEMSA Cargo : Conductor Jefe Inmediato : Esteban Cardona Tiempo laborado : 5 meses Teléfono : 3234542451 "+
		"Empresa : SEDEXCO ( Noel) Cargo : Auxiliar logístico Jefe Inmediato : Wilson Arango Tiempo laborado : 2 años Teléfono : 6046040056")
}

func TestAnalizarExperienciaCVDetectaConduccionSinPalabrasDeBus(t *testing.T) {
	s := AnalizarExperienciaCV(cvEjemplo)
	if s.ExperienciaPasajeros || s.ExperienciaPesada {
		t.Fatalf("no menciona buses ni pesados: %+v", s)
	}
	if !s.ExperienciaConduccion {
		t.Fatal("«Operador» y «Conductor» deberían contar como experiencia de conducción")
	}
	if AnalizarExperienciaCV("Empresa : Tienda\nCargo : Cajero\nTiempo laborado : 1 año").ExperienciaConduccion {
		t.Fatal("un cajero no conduce")
	}
}
