package orquestador

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

func TestNivelSimitPorDeudaYAcuerdo(t *testing.T) {
	casos := []struct {
		nombre string
		res    *modelos.ResultadoSimit
		nivel  string
	}{
		{"paz y salvo", &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true, TotalDeuda: "$ 0"}, modelos.NivelCumple},
		{"acuerdo sin deuda", &modelos.ResultadoSimit{Estado: "CON_ACUERDO_PAGO", CantidadAcuerdos: 1, TotalDeuda: "$ 0"}, modelos.NivelCumple},
		{"acuerdo de $800.000", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 2, CantidadAcuerdos: 1, TotalDeuda: "$800.000"}, modelos.NivelCumple},
		{"acuerdo de $1.000.000 justo", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, CantidadAcuerdos: 1, TotalDeuda: "$1.000.000"}, modelos.NivelCumple},
		{"acuerdo de $1.500.000", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, CantidadAcuerdos: 1, TotalDeuda: "$1.500.000"}, modelos.NivelSemillero},
		{"acuerdo de $2.000.000 justo", &modelos.ResultadoSimit{Estado: "OK", CantidadMultas: 1, CantidadAcuerdos: 1, TotalDeuda: "$2.000.000"}, modelos.NivelSemillero},
		{"acuerdo de $2.500.000", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 3, CantidadAcuerdos: 1, TotalDeuda: "$2.500.000"}, modelos.NivelDescarte},
		{"comparendo sin acuerdo", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, TotalDeuda: "$500.000"}, modelos.NivelSemillero},
		{"multas sin acuerdo de $3.000.000", &modelos.ResultadoSimit{Estado: "OK", CantidadMultas: 2, TotalDeuda: "$3.000.000"}, modelos.NivelDescarte},
		{"con centavos", &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, CantidadAcuerdos: 1, TotalDeuda: "$1.500.000,00"}, modelos.NivelSemillero},
	}
	for _, c := range casos {
		if nivel, det := agentes.NivelSimit(c.res); nivel != c.nivel {
			t.Errorf("%s: %s (%q), se esperaba %s", c.nombre, nivel, det, c.nivel)
		}
	}
	for texto, quiero := range map[string]int{"$ 0": 0, "$1.234.567": 1234567, "$ 1.234.567,00": 1234567, "$999": 999, "$2,000,000.50": 2000000, "": 0} {
		if got := agentes.Pesos(texto); got != quiero {
			t.Errorf("Pesos(%q) = %d, se esperaba %d", texto, got, quiero)
		}
	}
}

// candidataQueCumple cumple todo con las reglas v1 (hombre, 30 años, Sura, C2 desde 2015, SIMIT al día).
func candidatoQueCumple() *modelos.Candidato {
	r := reglasV1
	return &modelos.Candidato{Nombre: "Carlos", Genero: "Hombre", Edad: 30, Reglas: &r,
		ResultadoAdres: &modelos.ResultadoAdres{Estado: "OK", EPS: "EPS SURAMERICANA S.A.", EstadoAfiliacion: "ACTIVO"},
		ResultadoRunt: &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
			Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}}},
		ResultadoSimit: &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true},
	}
}

func TestClasificacionDelEvaluador(t *testing.T) {
	ac := agentes.NuevoAgenteClasificador(nil)
	casos := []struct {
		nombre        string
		cambiar       func(c *modelos.Candidato)
		clasificacion string
		semillero     []string
	}{
		{"cumple todo", func(*modelos.Candidato) {}, modelos.ClasificacionReclutadoP1, nil},
		{"menor a la edad mínima", func(c *modelos.Candidato) { c.Edad = 20 }, modelos.ClasificacionSemillero, []string{"edad"}},
		{"mayor a la edad tope", func(c *modelos.Candidato) { c.Edad = 50 }, modelos.ClasificacionDescartado, nil},
		{"EPS no aceptada", func(c *modelos.Candidato) { c.ResultadoAdres.EPS = "COOSALUD EPS S.A." }, modelos.ClasificacionSemillero, []string{"adres"}},
		{"sin licencia aceptada", func(c *modelos.Candidato) { c.ResultadoRunt.Licencias[0].Categorias = []string{"B1"} }, modelos.ClasificacionSemillero, []string{"runt"}},
		{"poco tiempo en el RUNT", func(c *modelos.Candidato) { c.ResultadoRunt.FechaInscripcion = "2099-01-01" }, modelos.ClasificacionSemillero, []string{"runt"}},
		{"SIMIT de $1.500.000 con acuerdo", func(c *modelos.Candidato) {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, CantidadAcuerdos: 1, TotalDeuda: "$1.500.000"}
		}, modelos.ClasificacionSemillero, []string{"simit"}},
		{"SIMIT de $2.500.000 con acuerdo", func(c *modelos.Candidato) {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 1, CantidadAcuerdos: 1, TotalDeuda: "$2.500.000"}
		}, modelos.ClasificacionDescartado, nil},
		{"reintegro sin aceptar reintegros", func(c *modelos.Candidato) {
			c.ResultadoReintegro = &modelos.ResultadoReintegro{Estado: modelos.ReintegroExEmpleado, Alerta: true, Detalles: "Ya fue empleado"}
		}, modelos.ClasificacionDescartado, nil},
		{"reintegro aceptando reintegros", func(c *modelos.Candidato) {
			c.Reglas.AceptaReintegros = true
			c.ResultadoReintegro = &modelos.ResultadoReintegro{Estado: modelos.ReintegroExEmpleado, Alerta: true, Detalles: "Ya fue empleado"}
		}, modelos.ClasificacionReclutadoP1, nil},
		{"antecedentes en la Procuraduría", func(c *modelos.Candidato) {
			c.ResultadoProcuraduria = &modelos.ResultadoProcuraduria{Estado: "CON_ANTECEDENTES", Alerta: true, Detalles: "Sanción vigente"}
		}, modelos.ClasificacionDescartado, nil},
		{"coincidencia en OFAC", func(c *modelos.Candidato) {
			c.ResultadoOfac = &modelos.ResultadoOfac{Estado: "COINCIDENCIA", Alerta: true}
		}, modelos.ClasificacionDescartado, nil},
		{"semillero y además incidente", func(c *modelos.Candidato) {
			c.ResultadoAdres.EPS = "COOSALUD EPS S.A."
			c.ResultadoSarlaft = &modelos.ResultadoSarlaft{Alerta: true, Detalles: "Coincidencia"}
		}, modelos.ClasificacionDescartado, []string{"adres"}},
		{"mujer cuando solo se aceptan hombres", func(c *modelos.Candidato) { c.Genero = "Mujer" }, modelos.ClasificacionDescartado, nil},
	}
	for _, caso := range casos {
		c := candidatoQueCumple()
		caso.cambiar(c)
		ev := ac.Evaluar(c)
		if ev.Clasificacion != caso.clasificacion || !reflect.DeepEqual(ev.Semillero, caso.semillero) {
			t.Errorf("%s: %s %v, se esperaba %s %v (motivos %v)", caso.nombre, ev.Clasificacion, ev.Semillero, caso.clasificacion, caso.semillero, ev.MotivosDescarte)
		}
		if ev.EsApto != (caso.clasificacion == modelos.ClasificacionReclutadoP1) {
			t.Errorf("%s: EsApto = %v con clasificación %s", caso.nombre, ev.EsApto, ev.Clasificacion)
		}
	}
}

// fuentesContadas responde las consultas como pida el caso y cuenta cuáles se hicieron.
type fuentesContadas struct {
	consultadas []string
	runt        *modelos.ResultadoRunt
	simit       *modelos.ResultadoSimit
	procu       *modelos.ResultadoProcuraduria
}

func (f *fuentesContadas) validadores() Validadores {
	marcar := func(nombre string, poner func(c *modelos.Candidato)) FuncionValidar {
		return func(_ context.Context, c *modelos.Candidato) error {
			f.consultadas = append(f.consultadas, nombre)
			poner(c)
			return nil
		}
	}
	return Validadores{
		Adres: marcar("adres", func(c *modelos.Candidato) {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "EPS SURAMERICANA S.A.", EstadoAfiliacion: "ACTIVO"}
		}),
		Runt:         marcar("runt", func(c *modelos.Candidato) { c.ResultadoRunt = f.runt }),
		Simit:        marcar("simit", func(c *modelos.Candidato) { c.ResultadoSimit = f.simit }),
		Procuraduria: marcar("procuraduria", func(c *modelos.Candidato) { c.ResultadoProcuraduria = f.procu }),
	}
}

func TestFlujoSigueHastaCompletarConSemillero(t *testing.T) {
	runtOK := &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
		Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}}}
	runtSinLicencia := &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
		Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"B1"}}}}
	simitOK := &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true}
	simitAlto := &modelos.ResultadoSimit{Estado: "OK", CantidadComparendos: 4, CantidadAcuerdos: 1, TotalDeuda: "$3.000.000"}
	sinAntecedentes := &modelos.ResultadoProcuraduria{Estado: "SIN_ANTECEDENTES"}
	conAntecedentes := &modelos.ResultadoProcuraduria{Estado: "CON_ANTECEDENTES", Alerta: true, Detalles: "Sanción"}

	casos := []struct {
		nombre        string
		edad          int
		f             fuentesContadas
		clasificacion string
		semillero     []string
		consultadas   string
	}{
		{"cumple todo", 30, fuentesContadas{runt: runtOK, simit: simitOK, procu: sinAntecedentes},
			modelos.ClasificacionReclutadoP1, nil, "adres runt simit procuraduria"},
		// RUNT no cumple: queda para semillero, pero se sigue con SIMIT y las informativas.
		{"RUNT para semillero", 30, fuentesContadas{runt: runtSinLicencia, simit: simitOK, procu: sinAntecedentes},
			modelos.ClasificacionSemillero, []string{"runt"}, "adres runt simit procuraduria"},
		{"menor a la edad mínima", 20, fuentesContadas{runt: runtOK, simit: simitOK, procu: sinAntecedentes},
			modelos.ClasificacionSemillero, []string{"edad"}, "adres runt simit procuraduria"},
		// Semillero en RUNT y luego SIMIT por encima del tope: se descarta ahí, conservando lo de semillero.
		{"SIMIT descarta", 30, fuentesContadas{runt: runtSinLicencia, simit: simitAlto, procu: sinAntecedentes},
			modelos.ClasificacionDescartado, []string{"runt"}, "adres runt simit"},
		// Mayor a la edad tope: se descarta sin gastar consultas.
		{"mayor a la edad tope", 55, fuentesContadas{runt: runtOK, simit: simitOK, procu: sinAntecedentes},
			modelos.ClasificacionDescartado, nil, ""},
		{"incidentes", 30, fuentesContadas{runt: runtSinLicencia, simit: simitOK, procu: conAntecedentes},
			modelos.ClasificacionDescartado, []string{"runt"}, "adres runt simit procuraduria"},
	}
	for _, caso := range casos {
		f := caso.f
		cfg := &configuracion.Configuracion{DirectorioDescargas: t.TempDir()}
		a := NuevoAnalizadorConValidadores(cfg, nil, OpcionesAnalisis{Antecedentes: true}, reporteroNulo{}, f.validadores())
		r := reglasV1
		c := &modelos.Candidato{Nombre: "Carlos", Cedula: "10203040", Genero: "Hombre", Edad: caso.edad, Reglas: &r}
		a.continuarDesdeCedula(context.Background(), c, t.TempDir(), &acumuladorResultados{}, false)

		ev := c.EvaluacionIdoneidad
		if ev == nil {
			t.Fatalf("%s: sin evaluación", caso.nombre)
		}
		if ev.Clasificacion != caso.clasificacion || !reflect.DeepEqual(ev.Semillero, caso.semillero) || ev.VerificacionPendiente {
			t.Errorf("%s: %s %v (pendiente=%v), se esperaba %s %v; motivos %v", caso.nombre, ev.Clasificacion, ev.Semillero,
				ev.VerificacionPendiente, caso.clasificacion, caso.semillero, ev.MotivosDescarte)
		}
		if got := strings.Join(f.consultadas, " "); got != caso.consultadas {
			t.Errorf("%s: consultó %q, se esperaba %q", caso.nombre, got, caso.consultadas)
		}
		if c.EsApto != (caso.clasificacion == modelos.ClasificacionReclutadoP1) {
			t.Errorf("%s: EsApto = %v", caso.nombre, c.EsApto)
		}
	}
}

// Si una consulta falla no hay clasificación: queda pendiente para reintentarla (aunque algo sea semillero).
func TestConsultaFallidaQuedaPendienteSinClasificar(t *testing.T) {
	f := fuentesContadas{runt: &modelos.ResultadoRunt{Estado: "ERROR_CONSULTA"}, simit: &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true}}
	cfg := &configuracion.Configuracion{DirectorioDescargas: t.TempDir()}
	a := NuevoAnalizadorConValidadores(cfg, nil, OpcionesAnalisis{}, reporteroNulo{}, f.validadores())
	r := reglasV1
	c := &modelos.Candidato{Nombre: "Carlos", Cedula: "10203040", Genero: "Hombre", Edad: 20, Reglas: &r}
	a.continuarDesdeCedula(context.Background(), c, t.TempDir(), &acumuladorResultados{}, false)
	if ev := c.EvaluacionIdoneidad; ev == nil || !ev.VerificacionPendiente || ev.Clasificacion != "" {
		t.Errorf("debía quedar pendiente sin clasificación: %+v", ev)
	}
}
