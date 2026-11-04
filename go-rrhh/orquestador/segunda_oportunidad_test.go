package orquestador

import (
	"testing"

	"go-rrhh/modelos"
)

// descartadoV1 es alguien descartado con la v1 por la EPS (con la v1 la EPS descartaba).
func descartadoV1(nombre string) modelos.Candidato {
	return modelos.Candidato{Nombre: nombre, Cedula: "10203040", Genero: "Hombre", Edad: 30, PostulacionID: "id-" + nombre,
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{ReglaVersion: 1, Genero: "Hombre", MotivosDescarte: []string{"EPS 'COOSALUD' no autorizada"}}}
}

func TestSegundaOportunidadSoloAQuienPuedeCambiar(t *testing.T) {
	r := reglasV2 // acepta mujeres, edad 18 a 60, reintegros, EPS Sura y Salud Total
	casos := []struct {
		nombre  string
		cambiar func(c *modelos.Candidato)
		motivo  string // "" = se reanaliza
	}{
		{"descartado por EPS con la v1: ahora sería semillero", func(*modelos.Candidato) {}, ""},
		{"mujer descartada con la v1: la v2 acepta mujeres", func(c *modelos.Candidato) { c.Genero, c.EvaluacionIdoneidad.Genero = "Mujer", "Mujer" }, ""},
		{"55 años: la v2 sube el tope a 60", func(c *modelos.Candidato) { c.Edad = 55 }, ""},
		{"reintegro: la v2 acepta reintegros", func(c *modelos.Candidato) {
			c.ResultadoReintegro = &modelos.ResultadoReintegro{Estado: modelos.ReintegroExEmpleado, Alerta: true, Detalles: "Ya fue empleado"}
		}, ""},
		{"ya evaluado con la v2", func(c *modelos.Candidato) { c.EvaluacionIdoneidad.ReglaVersion = 2 }, SigueYaEvaluado},
		{"sin cédula", func(c *modelos.Candidato) { c.Cedula = "No especificada" }, SigueSinCedula},
		{"65 años: sigue sobre el tope", func(c *modelos.Candidato) { c.Edad = 65 }, SigueEdad},
		{"SIMIT de $3.000.000", func(c *modelos.Candidato) {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", CantidadMultas: 2, TotalDeuda: "$3.000.000"}
		}, SigueSimit},
		{"antecedentes", func(c *modelos.Candidato) {
			c.ResultadoProcuraduria = &modelos.ResultadoProcuraduria{Alerta: true, Detalles: "Sanción"}
		}, SigueIncidentes},
		{"sin evaluación", func(c *modelos.Candidato) { c.EvaluacionIdoneidad = nil }, SigueSinResultado},
		// Los del pantallazo: si la regla que los descartó sigue sin cumplirse con la v2, no se reanalizan.
		{"licencia B1: la v2 sigue sin aceptarla", func(c *modelos.Candidato) {
			c.ResultadoRunt = &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"B1"}}}}
		}, SigueRunt},
		{"licencia C1: la v2 sí la acepta", func(c *modelos.Candidato) {
			c.ResultadoRunt = &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C1"}}}}
		}, ""},
		{"Coosalud: la v2 sigue sin aceptarla", func(c *modelos.Candidato) {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "EPS_NO_AUTORIZADA", EPS: "COOSALUD EPS S.A. -CM"}
		}, SigueEPS},
		{"sin EPS en ADRES", func(c *modelos.Candidato) {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "AFILIACION_INACTIVA", EPS: "NO REPORTADA"}
		}, SigueEPS},
		{"ADRES falló: no se sabe, se reanaliza", func(c *modelos.Candidato) {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "ERROR_CONSULTA"}
		}, ""},
		{"SIMIT de $1.500.000 sin acuerdo", func(c *modelos.Candidato) {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, TotalDeuda: "$1.500.000"}
		}, SigueSimitPendiente},
		{"20 años: la v2 baja la mínima a 18", func(c *modelos.Candidato) { c.Edad = 20 }, ""},
		{"17 años: sigue bajo la mínima", func(c *modelos.Candidato) { c.Edad = 17 }, SigueEdadMinima},
	}
	for _, caso := range casos {
		c := descartadoV1("x")
		caso.cambiar(&c)
		if got := MotivoSigueDescartado(&c, &r); got != caso.motivo {
			t.Errorf("%s: %q, se esperaba %q", caso.nombre, got, caso.motivo)
		}
	}

	// Con la v1 (solo hombres, sin reintegros) la mujer y el reintegro siguen descartados.
	v1 := reglasV1
	mujer := descartadoV1("Ana")
	mujer.Genero, mujer.EvaluacionIdoneidad.Genero = "Mujer", "Mujer"
	mujer.EvaluacionIdoneidad.ReglaVersion = 0 // evaluada antes de las versiones
	if got := MotivoSigueDescartado(&mujer, &v1); got != SigueGenero {
		t.Errorf("mujer con la v1: %q", got)
	}
	ex := descartadoV1("Ex")
	ex.EvaluacionIdoneidad.ReglaVersion = 0
	ex.ResultadoReintegro = &modelos.ResultadoReintegro{Estado: modelos.ReintegroExEmpleado, Alerta: true}
	if got := MotivoSigueDescartado(&ex, &v1); got != SigueReintegro {
		t.Errorf("reintegro con la v1: %q", got)
	}

	// Henry y Jorge: descartados por reintegro antes de guardarse el resultado de Administremos (solo queda el
	// motivo). Con reintegros aceptados se reanalizan; sin aceptarlos, no.
	henry := descartadoV1("Henry")
	henry.EvaluacionIdoneidad.MotivosDescarte = []string{"Reintegro: ya fue empleado: no se admiten reintegros"}
	if got := MotivoSigueDescartado(&henry, &r); got != "" {
		t.Errorf("Henry con reintegros aceptados: %q", got)
	}
	sinReintegros := reglasV2
	sinReintegros.AceptaReintegros = false
	sinReintegros.Version = 3
	if got := MotivoSigueDescartado(&henry, &sinReintegros); got != SigueReintegro {
		t.Errorf("Henry sin aceptar reintegros: %q", got)
	}
}

func TestResumenSegundaOportunidad(t *testing.T) {
	r := reglasV2
	viejo, otroViejo, yaV2, mayor := descartadoV1("A"), descartadoV1("B"), descartadoV1("C"), descartadoV1("D")
	yaV2.EvaluacionIdoneidad.ReglaVersion = 2
	mayor.Edad = 70
	res := SegundaOportunidad([]modelos.Candidato{viejo, yaV2, mayor, otroViejo}, &r)
	if res.Total != 4 || len(res.Reanalizar) != 2 || res.Reanalizar[0].Nombre != "A" || res.Reanalizar[1].Nombre != "B" {
		t.Fatalf("resumen: %+v", res)
	}
	if res.Reanalizar[0].ReglaVersion != 1 || res.Reanalizar[0].Motivo == "" || res.Reanalizar[0].PostulacionID != "id-A" {
		t.Errorf("persona a reanalizar: %+v", res.Reanalizar[0])
	}
	if len(res.Omitidos) != 2 || res.Omitidos[0].Motivo != SigueYaEvaluado || res.Omitidos[1].Motivo != SigueEdad {
		t.Errorf("omitidos: %+v", res.Omitidos)
	}
}
