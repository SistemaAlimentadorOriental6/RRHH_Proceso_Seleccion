package orquestador

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

var (
	reglasV1 = modelos.ReglasIdoneidad{Version: 1, EdadMinima: 22, EdadMaxima: 49, EpsAceptadas: []string{"Sura"},
		GenerosAceptados: []string{"masculino"}, LicenciasAceptadas: []string{"C2", "C3"}, RuntAntiguedadMeses: 24}
	reglasV2 = modelos.ReglasIdoneidad{Version: 2, EdadMinima: 18, EdadMaxima: 60, EpsAceptadas: []string{"EPS Sura", "Salud Total"},
		GenerosAceptados: []string{"masculino", "femenino"}, LicenciasAceptadas: []string{"C1", "C2"}, RuntAntiguedadMeses: 12, AceptaReintegros: true}
)

// baseReglasFalsa es la base de datos: guarda lo evaluado y tiene dos versiones de reglas.
type baseReglasFalsa struct {
	mu       sync.Mutex
	vigente  modelos.ReglasIdoneidad
	previos  map[string]*modelos.Candidato
	guardado map[string]int // nombre -> versión de reglas con la que se guardó
}

func (b *baseReglasFalsa) ParametrosVigentes(context.Context) (modelos.ReglasIdoneidad, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.vigente, nil
}

func (b *baseReglasFalsa) ParametrosVersion(_ context.Context, v int) (modelos.ReglasIdoneidad, error) {
	switch v {
	case 1:
		return reglasV1, nil
	case 2:
		return reglasV2, nil
	}
	return modelos.ReglasIdoneidad{}, fmt.Errorf("no existe la versión %d", v)
}

func (b *baseReglasFalsa) cambiarVigente(r modelos.ReglasIdoneidad) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.vigente = r
}

func (b *baseReglasFalsa) GuardarCandidato(_ context.Context, _ modelos.OfertaLaboral, c *modelos.Candidato) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.guardado[c.Nombre] = c.EvaluacionIdoneidad.ReglaVersion
	return nil
}

func (b *baseReglasFalsa) CandidatosPrevios(context.Context, modelos.OfertaLaboral) (map[string]*modelos.Candidato, error) {
	return b.previos, nil
}

func (b *baseReglasFalsa) MarcarVigentes(context.Context, modelos.OfertaLaboral, []string, []string) error {
	return nil
}

// fuenteNombres lista a las personas en orden; todas tienen cédula y capturas.
type fuenteNombres struct{ nombres []string }

func (f fuenteNombres) ObtenerCandidatosStream(_ context.Context, _ string, _ int, alDescubrir func(*modelos.Candidato)) error {
	for i, n := range f.nombres {
		alDescubrir(&modelos.Candidato{Indice: i + 1, Nombre: n, URLDetalle: "https://x/" + n})
	}
	return nil
}

func (f fuenteNombres) ExtraerDetalleCandidato(_ context.Context, c *modelos.Candidato, _ string) error {
	c.Cedula = "10203040"
	c.Capturas = map[string]string{"perfil": "p.jpg"}
	return nil
}

type reporteroNulo struct{}

func (reporteroNulo) Emitir(string, any) {}

// validadoresQueCumplen dan EPS Sura, licencia C2 desde 2015 y SIMIT al día: con cualquiera de las dos versiones
// solo decide el género. antesDeAdres corre al empezar la consulta de ADRES de cada persona.
func validadoresQueCumplen(antesDeAdres func(c *modelos.Candidato)) Validadores {
	return Validadores{
		Adres: func(_ context.Context, c *modelos.Candidato) error {
			if antesDeAdres != nil {
				antesDeAdres(c)
			}
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "EPS SURAMERICANA S.A.", EstadoAfiliacion: "ACTIVO"}
			return nil
		},
		Runt: func(_ context.Context, c *modelos.Candidato) error {
			c.ResultadoRunt = &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}}}
			return nil
		},
		Simit: func(_ context.Context, c *modelos.Candidato) error {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true}
			return nil
		},
	}
}

func analizarCon(t *testing.T, base *baseReglasFalsa, nombres []string, opts OpcionesAnalisis, v Validadores) {
	t.Helper()
	cfg := &configuracion.Configuracion{DirectorioDescargas: t.TempDir()}
	a := NuevoAnalizadorConValidadores(cfg, fuenteNombres{nombres}, opts, reporteroNulo{}, v).ConPersistencia(base)
	if _, err := a.Analizar(context.Background(), modelos.OfertaLaboral{ID: "X", Titulo: "Conductor"}); err != nil {
		t.Fatal(err)
	}
}

func evaluada(version int, apto, pendiente bool) *modelos.EvaluacionIdoneidad {
	return &modelos.EvaluacionIdoneidad{ReglaVersion: version, EsApto: apto, VerificacionPendiente: pendiente, DescarteTemprano: !apto && !pendiente}
}

// Las reglas nuevas solo valen para las personas nuevas: quien ya tuvo dictamen, aunque se le fuerce el
// análisis, se evalúa con las reglas que tenía; quien quedó pendiente sí entra con las vigentes.
func TestReglasNuevasSoloParaPersonasNuevas(t *testing.T) {
	base := &baseReglasFalsa{
		vigente:  reglasV2, // se guardaron parámetros nuevos que aceptan mujeres
		guardado: map[string]int{},
		previos: map[string]*modelos.Candidato{
			// Descartada por género con la versión 1.
			"https://x/Maria Antes": {Nombre: "Maria Antes", Cedula: "10203040", URLDetalle: "https://x/Maria Antes", EvaluacionIdoneidad: evaluada(1, false, false)},
			// Sin dictamen con la versión 1 (una consulta falló).
			"https://x/Laura Pendiente": {Nombre: "Laura Pendiente", Cedula: "10203040", URLDetalle: "https://x/Laura Pendiente", Capturas: map[string]string{"a": "b"},
				ErrorProceso: "falló la consulta de ADRES", EvaluacionIdoneidad: evaluada(1, false, true)},
		},
	}
	analizarCon(t, base, []string{"Maria Antes", "Laura Pendiente", "Diana Nueva", "Carlos Nuevo"}, OpcionesAnalisis{Forzar: true}, validadoresQueCumplen(nil))

	quiero := map[string]int{"Maria Antes": 1, "Laura Pendiente": 2, "Diana Nueva": 2, "Carlos Nuevo": 2}
	for nombre, v := range quiero {
		if got := base.guardado[nombre]; got != v {
			t.Errorf("%s se evaluó con reglas v%d, se esperaba v%d", nombre, got, v)
		}
	}
}

// Sin forzar, a quien ya tiene dictamen ni siquiera se le vuelve a evaluar: conserva su resultado.
func TestSinForzarLosAnterioresConservanSuResultado(t *testing.T) {
	base := &baseReglasFalsa{
		vigente:  reglasV2,
		guardado: map[string]int{},
		previos: map[string]*modelos.Candidato{
			"https://x/Maria Antes": {Nombre: "Maria Antes", Cedula: "10203040", URLDetalle: "https://x/Maria Antes", EvaluacionIdoneidad: evaluada(1, false, false)},
		},
	}
	analizarCon(t, base, []string{"Maria Antes", "Diana Nueva"}, OpcionesAnalisis{}, validadoresQueCumplen(nil))
	if _, ok := base.guardado["Maria Antes"]; ok {
		t.Error("Maria Antes no debía volver a evaluarse ni guardarse")
	}
	if base.guardado["Diana Nueva"] != 2 {
		t.Errorf("Diana Nueva debía evaluarse con v2: %v", base.guardado)
	}
}

// Si se guardan parámetros a mitad del análisis, quien ya empezó termina con los suyos y el siguiente
// recibe los nuevos.
func TestCambioDeReglasAMitadDelAnalisis(t *testing.T) {
	base := &baseReglasFalsa{vigente: reglasV1, guardado: map[string]int{}, previos: map[string]*modelos.Candidato{}}
	v := validadoresQueCumplen(func(c *modelos.Candidato) {
		if c.Nombre == "Carlos Primero" {
			base.cambiarVigente(reglasV2) // alguien guarda parámetros mientras se consulta ADRES de Carlos
		}
	})
	analizarCon(t, base, []string{"Carlos Primero", "Pedro Segundo"}, OpcionesAnalisis{Concurrencia: 1}, v)
	if base.guardado["Carlos Primero"] != 1 || base.guardado["Pedro Segundo"] != 2 {
		t.Errorf("versiones guardadas %v, se esperaba Carlos v1 y Pedro v2", base.guardado)
	}
}

func TestVersionConservada(t *testing.T) {
	casos := map[string]struct {
		c    *modelos.Candidato
		want int
	}{
		"sin evaluación":       {&modelos.Candidato{}, 0},
		"apto con v3":          {&modelos.Candidato{EvaluacionIdoneidad: evaluada(3, true, false)}, 3},
		"descartado con v1":    {&modelos.Candidato{EvaluacionIdoneidad: evaluada(1, false, false)}, 1},
		"pendiente con v1":     {&modelos.Candidato{EvaluacionIdoneidad: evaluada(1, false, true)}, 0},
		"con error de proceso": {&modelos.Candidato{ErrorProceso: "x", EvaluacionIdoneidad: evaluada(1, false, false)}, 0},
		"en pausa":             {&modelos.Candidato{EstadoProceso: modelos.EstadoEsperandoDatos, EvaluacionIdoneidad: evaluada(1, false, false)}, 0},
	}
	for nombre, caso := range casos {
		if got := versionConservada(caso.c); got != caso.want {
			t.Errorf("%s: %d, se esperaba %d", nombre, got, caso.want)
		}
	}
}

// El clasificador aplica las reglas de la persona: lo que la v1 rechaza, la v2 lo acepta.
func TestClasificadorAplicaLasReglasDeLaPersona(t *testing.T) {
	ac := agentes.NuevoAgenteClasificador(nil)
	candidata := func(r modelos.ReglasIdoneidad) *modelos.Candidato {
		return &modelos.Candidato{Nombre: "Maria", Genero: "Mujer", Edad: 52, Reglas: &r,
			ResultadoAdres: &modelos.ResultadoAdres{Estado: "OK", EPS: "SALUD TOTAL S.A. E.P.S.", EstadoAfiliacion: "ACTIVO"},
			ResultadoRunt: &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C1"}}}},
			ResultadoSimit: &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true},
		}
	}
	ev := ac.Evaluar(candidata(reglasV1))
	if ev.EsApto || ev.CumpleGenero || ev.CumpleEPS || ev.CumpleRunt || ev.ReglaVersion != 1 {
		t.Errorf("con v1 debía rechazarse por género, EPS, licencia y edad: %+v", ev)
	}
	if !strings.Contains(strings.Join(ev.MotivosDescarte, "|"), "Edad (52 años)") {
		t.Errorf("con v1 la edad 52 debía ser motivo: %v", ev.MotivosDescarte)
	}
	if ev := ac.Evaluar(candidata(reglasV2)); !ev.EsApto || ev.ReglaVersion != 2 {
		t.Errorf("con v2 debía ser apta: %+v", ev)
	}
}

// nombresADRES son, por cada EPS del catálogo de la pantalla (y los de la versión 1), las formas en que ADRES
// muestra la entidad: las vistas en consultas reales y las razones sociales.
var nombresADRES = map[string][]string{
	"Nueva EPS":                {"NUEVA EPS S.A.", "NUEVA EPS S.A. -CM", "NUEVA E.P.S."},
	"Salud Total":              {"SALUD TOTAL S.A. E.P.S.", "SALUD TOTAL ENTIDAD PROMOTORA DE SALUD DEL REGIMEN CONTRIBUTIVO Y DEL REGIMEN SUBSIDIADO S.A."},
	"EPS Sura":                 {"EPS SURAMERICANA S.A.", "EPS SURAMERICANA S.A. -CM", "EPS Y MEDICINA PREPAGADA SURAMERICANA S.A.", "EPS SURA"},
	"Savia Salud":              {"SAVIA SALUD EPS", "ALIANZA MEDELLIN ANTIOQUIA EPS S.A.S.", "ALIANZA MEDELLÍN ANTIOQUIA EPS S.A.S. - SAVIA SALUD EPS"},
	"EPS Sanitas":              {"ENTIDAD PROMOTORA DE SALUD SANITAS S.A.S.", "EPS SANITAS S.A.S.", "E.P.S. SANITAS S.A.S."},
	"Coosalud":                 {"COOSALUD EPS S.A.", "COOSALUD EPS S.A. -CM", "COOSALUD ENTIDAD PROMOTORA DE SALUD S.A."},
	"EPS SOS":                  {"EPS SERVICIO OCCIDENTAL DE SALUD S.A.", "SERVICIO OCCIDENTAL DE SALUD S.A. S.O.S.", "EPS S.O.S."},
	"Famisanar":                {"EPS FAMISANAR S.A.S.", "ENTIDAD PROMOTORA DE SALUD FAMISANAR S.A.S."},
	"Compensar":                {"COMPENSAR E.P.S.", "COMPENSAR ENTIDAD PROMOTORA DE SALUD", "CAJA DE COMPENSACION FAMILIAR COMPENSAR"},
	"Aliansalud":               {"ALIANSALUD EPS S.A.", "ALIANSALUD ENTIDAD PROMOTORA DE SALUD S.A."},
	"Capital Salud":            {"CAPITAL SALUD EPS-S S.A.S.", "CAPITAL SALUD ENTIDAD PROMOTORA DE SALUD DEL REGIMEN SUBSIDIADO S.A.S."},
	"Mutual Ser":               {"ASOCIACION MUTUAL SER EMPRESA SOLIDARIA DE SALUD EPS-S", "MUTUAL SER EPS"},
	"Asmet Salud":              {"ASMET SALUD EPS S.A.S.", "ASOCIACION MUTUAL LA ESPERANZA ASMET SALUD"},
	"Emssanar":                 {"EMSSANAR S.A.S.", "ASOCIACION MUTUAL EMPRESA SOLIDARIA DE SALUD DE NARIÑO E.S.S. EMSSANAR E.S.S."},
	"Cajacopi Atlántico":       {"CAJA DE COMPENSACION FAMILIAR CAJACOPI ATLANTICO", "CAJACOPI EPS S.A.S."},
	"Comfenalco Valle":         {"COMFENALCO VALLE E.P.S.", "CAJA DE COMPENSACION FAMILIAR DEL VALLE DEL CAUCA COMFENALCO VALLE DE LA GENTE"},
	"Fundación Salud Mía":      {"FUNDACION SALUD MIA EPS", "FUNDACIÓN SALUD MÍA"},
	"EPS Familiar de Colombia": {"EPS FAMILIAR DE COLOMBIA S.A.S."},
	"Capresoca":                {"CAPRESOCA EPS", "CAJA DE PREVISION SOCIAL Y SEGURIDAD DEL CASANARE CAPRESOCA EPS"},
	"Comfachocó":               {"COMFACHOCO", "CAJA DE COMPENSACION FAMILIAR DEL CHOCO"},
	"Comfaoriente":             {"COMFAORIENTE EPS-S", "CAJA DE COMPENSACION FAMILIAR DEL ORIENTE COLOMBIANO COMFAORIENTE"},
	"Comfamiliar Huila":        {"COMFAMILIAR HUILA", "CAJA DE COMPENSACION FAMILIAR DEL HUILA - COMFAMILIAR"},
	"Comfamiliar Nariño":       {"COMFAMILIAR NARIÑO", "CAJA DE COMPENSACION FAMILIAR DE NARIÑO"},
}

// mismaEPS agrupa los nombres de la versión 1 con su equivalente del catálogo.
var mismaEPS = map[string]string{"Sura": "EPS Sura", "Sanitas": "EPS Sanitas"}

func TestEPSAceptadaConTodoElCatalogo(t *testing.T) {
	aceptadas := []string{}
	for nombre := range nombresADRES {
		aceptadas = append(aceptadas, nombre)
	}
	aceptadas = append(aceptadas, "Sura", "Sanitas") // los de la versión 1
	for _, aceptada := range aceptadas {
		propia := aceptada
		if v, ok := mismaEPS[aceptada]; ok {
			propia = v
		}
		for eps, nombres := range nombresADRES {
			for _, adres := range nombres {
				want := eps == propia
				if got := agentes.EPSAceptada(adres, []string{aceptada}); got != want {
					t.Errorf("con %q aceptada, ADRES %q (de %s) = %v; se esperaba %v", aceptada, adres, eps, got, want)
				}
			}
		}
	}
}

func TestEPSAceptadaCasosLimite(t *testing.T) {
	casos := []struct {
		adres     string
		aceptadas []string
		want      bool
	}{
		{"NUEVA EPS S.A.", []string{"EPS"}, false},                    // "EPS" sola no identifica a ninguna
		{"NUEVA EPS S.A.", []string{"  "}, false},                     // un nombre vacío no acepta todo
		{"", []string{"EPS Sura"}, false},                             // sin EPS
		{"NO REPORTADA", []string{"Nueva EPS"}, false},                // sin afiliación
		{"nueva eps s.a.", []string{"NUEVA EPS"}, true},               // mayúsculas/minúsculas
		{"EPS SURAMERICANA S.A.", []string{"Coosalud", "Sura"}, true}, // basta una de la lista
		{"COOSALUD EPS S.A.", []string{"Salud Total", "Savia Salud", "Capital Salud", "Asmet Salud"}, false},
		{"EPS SERVICIO OCCIDENTAL DE SALUD S.A.", []string{"Mutual Ser"}, false}, // SER no es SERVICIO
		{"COMFAORIENTE EPS-S", []string{"EPS Familiar de Colombia"}, false},
		// La configuración sin base (variables de entorno) usa estas formas cortas.
		{"EPS SURAMERICANA S.A.", []string{"SURAMERICANA"}, true},
		{"ALIANZA MEDELLIN ANTIOQUIA EPS S.A.S.", []string{"SAVIA", "SAVIA SALUD"}, true},
	}
	for _, c := range casos {
		if got := agentes.EPSAceptada(c.adres, c.aceptadas); got != c.want {
			t.Errorf("EPSAceptada(%q, %v) = %v, se esperaba %v", c.adres, c.aceptadas, got, c.want)
		}
	}
}

// Quien no figura en la BDUA se rechaza con un motivo claro; una respuesta ilegible queda pendiente.
func TestEPSNoAfiliadoYErrorDeLectura(t *testing.T) {
	ac := agentes.NuevoAgenteClasificador(nil)
	if ok, det := ac.EvaluarEPS(&modelos.ResultadoAdres{Estado: agentes.EstadoAdresNoAfiliado}, &reglasV2); ok || !strings.Contains(det, "No figura afiliado") {
		t.Errorf("no afiliado: %v %q", ok, det)
	}
	a := NuevoAnalizadorConValidadores(nil, nil, OpcionesAnalisis{}, reporteroNulo{}, Validadores{})
	adres := a.etapasNucleo()[0]
	c := &modelos.Candidato{Reglas: &reglasV2, ResultadoAdres: &modelos.ResultadoAdres{Estado: "ERROR_LECTURA", Detalles: "sin tabla"}}
	if estado, _, _ := adres.evaluar(c); estado != estadoError {
		t.Errorf("una respuesta ilegible no es un rechazo: %s", estado)
	}
	c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "AFILIACION_INACTIVA", EPS: "ALIANZA MEDELLIN ANTIOQUIA EPS S.A.S.", EstadoAfiliacion: "RETIRADO"}
	if estado, cumple, det := adres.evaluar(c); estado != estadoSemillero || cumple {
		t.Errorf("Savia Salud no está en la v2: queda para semillero: %s %q", estado, det)
	}
	c.Reglas = &modelos.ReglasIdoneidad{EpsAceptadas: []string{"Savia Salud"}}
	if estado, cumple, det := adres.evaluar(c); estado != estadoOK || !cumple {
		t.Errorf("afiliación retirada a una EPS aceptada sí cumple: %s %q", estado, det)
	}
}

func TestRuntUsaMesesYLicenciasDeLasReglas(t *testing.T) {
	ac := agentes.NuevoAgenteClasificador(nil)
	runt := &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01T00:00:00.000",
		Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C1"}}}}
	if ok, det := ac.EvaluarRunt(runt, &reglasV1); ok || !strings.Contains(det, "C2 o C3") {
		t.Errorf("v1 no acepta C1: %v %q", ok, det)
	}
	if ok, det := ac.EvaluarRunt(runt, &reglasV2); !ok {
		t.Errorf("v2 acepta C1: %q", det)
	}
	exigente := reglasV2
	exigente.RuntAntiguedadMeses = 720
	if ok, det := ac.EvaluarRunt(runt, &exigente); ok || !strings.Contains(det, "mínimo 60 años") {
		t.Errorf("60 años de RUNT no se cumplen: %v %q", ok, det)
	}
}

func TestReintegroSegunLasReglas(t *testing.T) {
	ex := &modelos.ResultadoReintegro{Estado: modelos.ReintegroExEmpleado, Alerta: true, Detalles: "Ya fue empleado"}
	if estado, _, _ := evaluarEtapaReintegro(&modelos.Candidato{ResultadoReintegro: ex, Reglas: &reglasV1}); estado != estadoRechazado {
		t.Errorf("v1 no acepta reintegros: %s", estado)
	}
	if estado, cumple, _ := evaluarEtapaReintegro(&modelos.Candidato{ResultadoReintegro: ex, Reglas: &reglasV2}); estado != estadoOK || !cumple {
		t.Errorf("v2 acepta reintegros: %s", estado)
	}
}

func TestTextoMeses(t *testing.T) {
	for meses, want := range map[int]string{0: "0 meses", 1: "1 mes", 12: "1 año", 18: "1 año y 6 meses", 25: "2 años y 1 mes"} {
		if got := modelos.TextoMeses(meses); got != want {
			t.Errorf("TextoMeses(%d) = %q, se esperaba %q", meses, got, want)
		}
	}
}
