package servidor

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"go-rrhh/modelos"
	"go-rrhh/orquestador"
)

// ---- Dobles ----

type registroPausa struct {
	candidato modelos.Candidato
	pausa     modelos.PausaDatos
}

// persistenciaConPausas es la base de datos falsa con la parte de datos faltantes.
type persistenciaConPausas struct {
	*persistenciaFalsa
	pausas         []registroPausa
	cedulaConocida map[string]string // correo -> cédula de una postulación anterior
	reanudaciones  []string
}

func nuevaPersistenciaConPausas() *persistenciaConPausas {
	return &persistenciaConPausas{persistenciaFalsa: &persistenciaFalsa{}, cedulaConocida: map[string]string{}}
}

func (p *persistenciaConPausas) PausarPorDatos(_ context.Context, _ modelos.OfertaLaboral, c *modelos.Candidato, d modelos.PausaDatos) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pausas = append(p.pausas, registroPausa{candidato: *c, pausa: d})
	return nil
}

func (p *persistenciaConPausas) CedulaConocida(_ context.Context, _, correo string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cedulaConocida[correo], nil
}

func (p *persistenciaConPausas) RegistrarReanudacion(_ context.Context, _ *modelos.Candidato, resultado string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reanudaciones = append(p.reanudaciones, resultado)
	return nil
}

func (p *persistenciaConPausas) guardado(nombre string) *modelos.Candidato {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.guardados {
		if p.guardados[i].Nombre == nombre {
			return &p.guardados[i]
		}
	}
	return nil
}

type perfilPrueba struct {
	nombre, cedula, texto string
	intentos              []modelos.IntentoCedula
}

// fuentePerfiles simula Computrabajo con los perfiles que se le den.
type fuentePerfiles struct {
	ofertasFalsas
	perfiles []perfilPrueba
	fichas   atomic.Int32
}

func (f *fuentePerfiles) ObtenerOfertasActivas(context.Context) ([]modelos.OfertaLaboral, error) {
	return []modelos.OfertaLaboral{{Titulo: "Conductor de pruebas SSE", URLCandidatos: "https://x/Match?oi=" + idOfertaPrueba, ID: idOfertaPrueba}}, nil
}

func (f *fuentePerfiles) ObtenerCandidatosStream(_ context.Context, _ string, _ int, alDescubrir func(*modelos.Candidato)) error {
	for i, p := range f.perfiles {
		alDescubrir(&modelos.Candidato{Indice: i + 1, Nombre: p.nombre, URLDetalle: "https://x/Detail/" + p.nombre})
	}
	return nil
}

func (f *fuentePerfiles) ExtraerDetalleCandidato(_ context.Context, c *modelos.Candidato, _ string) error {
	f.fichas.Add(1)
	for _, p := range f.perfiles {
		if p.nombre == c.Nombre {
			c.Cedula, c.TextoPerfil, c.IntentosCedula = p.cedula, p.texto, p.intentos
		}
	}
	c.Capturas = map[string]string{"perfil": "perfil.jpg"}
	return nil
}

// consultasContadas son consultas que cumplen todo y cuentan cuántas veces se llamó a cada una.
type consultasContadas struct {
	validadores        orquestador.Validadores
	adres, runt, simit atomic.Int32
}

// nuevasConsultas arma las consultas. El titular que devuelve ADRES se indica con nombres y apellidos.
func nuevasConsultas(nombresAdres, apellidosAdres string) *consultasContadas {
	c := &consultasContadas{}
	c.validadores = orquestador.Validadores{
		Adres: func(_ context.Context, cand *modelos.Candidato) error {
			c.adres.Add(1)
			cand.ResultadoAdres = &modelos.ResultadoAdres{
				Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO", CumpleEPS: true,
				Nombres: nombresAdres, Apellidos: apellidosAdres,
			}
			return nil
		},
		Runt: func(_ context.Context, cand *modelos.Candidato) error {
			c.runt.Add(1)
			cand.ResultadoRunt = &modelos.ResultadoRunt{
				Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}},
			}
			return nil
		},
		Simit: func(_ context.Context, cand *modelos.Candidato) error {
			c.simit.Add(1)
			cand.ResultadoSimit = &modelos.ResultadoSimit{Estado: "PAZ_Y_SALVO", PazYSalvo: true, TotalDeuda: "$ 0"}
			return nil
		},
	}
	return c
}

func analizarPerfiles(t *testing.T, perfiles []perfilPrueba, c *consultasContadas, p orquestador.Persistencia) (*fuentePerfiles, []eventoLeido) {
	t.Helper()
	fuente := &fuentePerfiles{perfiles: perfiles}
	return fuente, analizarCon(t, fuente, c.validadores, p)
}

// ---- Pausar en vez de descartar ----

func TestSinCedulaSePausaEnVezDeDescartar(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	consultas := nuevasConsultas("JUAN", "GOMEZ")
	perfiles := []perfilPrueba{
		{"Carlos Perez", "No especificada", "Contacto: carlos@mail.com cel 311 234 5678",
			[]modelos.IntentoCedula{{Metodo: "ficha_html"}, {Metodo: "texto_pdf_cv"}}},
		{"Pedro Ruiz", "No especificada", "sin ningún dato de contacto", nil},
		{"Maria Lopez", "No especificada", "maria@mail.com", nil},
		{"Juan Gomez", "3333333", "perfil completo", nil},
	}
	_, eventos := analizarPerfiles(t, perfiles, consultas, p)

	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.completado" {
		t.Fatalf("último evento = %s", ultimo)
	}

	carlos := p.guardado("Carlos Perez")
	if carlos == nil || carlos.EstadoProceso != modelos.EstadoEsperandoDatos {
		t.Fatalf("Carlos debe quedar esperando datos: %+v", carlos)
	}
	ev := carlos.EvaluacionIdoneidad
	if ev == nil || !ev.VerificacionPendiente || ev.DescarteTemprano || ev.EsApto || carlos.EsApto {
		t.Errorf("Carlos debe quedar pendiente, no rechazado ni descartado: %+v", ev)
	}
	if carlos.ErrorProceso != "" {
		t.Errorf("estar en pausa no es un error (un error lo haría reprocesar): %q", carlos.ErrorProceso)
	}
	if len(ev.MotivosDescarte) != 1 || !strings.HasPrefix(ev.MotivosDescarte[0], "Verificación pendiente") {
		t.Errorf("motivo: %v", ev.MotivosDescarte)
	}

	if pedro := p.guardado("Pedro Ruiz"); pedro == nil || pedro.EstadoProceso != modelos.EstadoSinContacto {
		t.Errorf("sin correo ni celular debe quedar sin_contacto: %+v", pedro)
	}

	// A quien las reglas ya excluyen por género no se le pide nada.
	maria := p.guardado("Maria Lopez")
	if maria == nil || maria.EstadoProceso != "" || !maria.EvaluacionIdoneidad.DescarteTemprano {
		t.Errorf("Maria se descarta por género, sin pedirle datos: %+v", maria)
	}

	// Quien tiene cédula sigue el camino de siempre.
	if juan := p.guardado("Juan Gomez"); juan == nil || juan.EstadoProceso != "" || juan.EvaluacionIdoneidad == nil || juan.EvaluacionIdoneidad.VerificacionPendiente {
		t.Errorf("Juan debe seguir el flujo normal: %+v", juan)
	}
	if consultas.adres.Load() != 1 || consultas.runt.Load() != 1 || consultas.simit.Load() != 1 {
		t.Errorf("solo Juan debía consultarse: adres=%d runt=%d simit=%d", consultas.adres.Load(), consultas.runt.Load(), consultas.simit.Load())
	}

	// Se dejó constancia de la pausa de Carlos y de Pedro (y de nadie más), con lo que se probó.
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pausas) != 2 {
		t.Fatalf("pausas registradas = %d; quería 2", len(p.pausas))
	}
	c := p.pausas[0]
	if c.candidato.Nombre != "Carlos Perez" || c.pausa.Correo != "carlos@mail.com" || c.pausa.Telefono != "3112345678" ||
		c.pausa.Motivo != "cedula_no_encontrada" || c.pausa.EtapaPausada != "adres" {
		t.Errorf("pausa de Carlos: %+v", c)
	}
	// Los dos métodos que probó el agente más el último intento antes de pedírsela: la postulación anterior.
	intentos := c.candidato.IntentosCedula
	if len(intentos) != 3 || intentos[0].Metodo != "ficha_html" || intentos[1].Metodo != "texto_pdf_cv" ||
		intentos[2].Metodo != "postulacion_previa" || intentos[2].Hallada {
		t.Errorf("deben quedar registrados todos los métodos probados, en orden: %+v", intentos)
	}
	if o := p.pausas[1]; o.candidato.Nombre != "Pedro Ruiz" || o.pausa.Correo != "" || o.pausa.Telefono != "" {
		t.Errorf("pausa de Pedro: %+v", o)
	}
}

// Sin base de datos no se rompe: queda pendiente, sin solicitud.
func TestSinCedulaYSinPausasQuedaPendiente(t *testing.T) {
	p := &persistenciaFalsa{} // implementa la persistencia pero no las pausas
	consultas := nuevasConsultas("JUAN", "GOMEZ")
	_, eventos := analizarPerfiles(t, []perfilPrueba{{"Carlos Perez", "No especificada", "carlos@mail.com", nil}}, consultas, p)
	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.completado" {
		t.Fatalf("último evento = %s", ultimo)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.guardados) != 1 || p.guardados[0].EstadoProceso != modelos.EstadoEsperandoDatos {
		t.Errorf("guardados: %+v", p.guardados)
	}
}

func TestUnCandidatoEnPausaNoSeVuelveALeerEnCadaAnalisis(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	pausado := previoValido("Carlos Perez", "No especificada")
	pausado.EstadoProceso = modelos.EstadoEsperandoDatos
	pausado.EsApto = false
	pausado.EvaluacionIdoneidad.EsApto = false
	pausado.EvaluacionIdoneidad.VerificacionPendiente = true
	p.previos = map[string]*modelos.Candidato{pausado.URLDetalle: pausado}

	perfiles := []perfilPrueba{
		{"Carlos Perez", "No especificada", "carlos@mail.com", nil},
		{"Juan Gomez", "3333333", "perfil", nil},
	}
	fuente, _ := analizarPerfiles(t, perfiles, nuevasConsultas("JUAN", "GOMEZ"), p)
	if n := fuente.fichas.Load(); n != 1 {
		t.Errorf("se leyeron %d fichas; quería 1 (la de Juan: Carlos ya está en manos del proceso de solicitud)", n)
	}
	if len(p.pausas) != 0 {
		t.Errorf("no debe repetirse la pausa de quien ya está en pausa: %d", len(p.pausas))
	}
}

// ---- Cédula de una postulación anterior ----

func TestCedulaDePostulacionPreviaSeVerificaConAdres(t *testing.T) {
	texto := "Contacto: carlos@mail.com"
	casos := []struct {
		nombre                  string
		nombresAdres, apellidos string
		estadoEsperado          string // EstadoProceso final
		corrieRunt              bool
		motivo                  string
	}{
		{"el titular coincide", "CARLOS ANDRES", "PEREZ GOMEZ", "", true, ""},
		{"otro titular", "PEDRO", "RUIZ", modelos.EstadoIdentidadEnRevision, false, "no coincide"},
		{"ADRES no encuentra a nadie", "", "", modelos.EstadoIdentidadEnRevision, false, "no encontró"},
	}
	for _, c := range casos {
		p := nuevaPersistenciaConPausas()
		p.cedulaConocida["carlos@mail.com"] = "9999999"
		consultas := nuevasConsultas(c.nombresAdres, c.apellidos)
		_, _ = analizarPerfiles(t, []perfilPrueba{{"Carlos Perez", "No especificada", texto, nil}}, consultas, p)

		carlos := p.guardado("Carlos Perez")
		if carlos == nil {
			t.Fatalf("%s: no se guardó", c.nombre)
		}
		if carlos.Cedula != "9999999" {
			t.Errorf("%s: la cédula de la postulación anterior debe usarse, fue %q", c.nombre, carlos.Cedula)
		}
		if carlos.EstadoProceso != c.estadoEsperado {
			t.Errorf("%s: estado = %q; quería %q", c.nombre, carlos.EstadoProceso, c.estadoEsperado)
		}
		if (consultas.runt.Load() == 1) != c.corrieRunt {
			t.Errorf("%s: RUNT corrió %d veces; debía correr=%v", c.nombre, consultas.runt.Load(), c.corrieRunt)
		}
		if c.motivo != "" {
			ev := carlos.EvaluacionIdoneidad
			if ev == nil || !ev.VerificacionPendiente || ev.EsApto || len(ev.MotivosDescarte) != 1 || !strings.Contains(ev.MotivosDescarte[0], c.motivo) {
				t.Errorf("%s: debe quedar pendiente de revisión (no rechazado): %+v", c.nombre, ev)
			}
		}
		if len(p.pausas) != 0 {
			t.Errorf("%s: con cédula conocida no se pausa ni se le pide nada", c.nombre)
		}
		encontrada := false
		for _, i := range carlos.IntentosCedula {
			if i.Metodo == "postulacion_previa" && i.Hallada {
				encontrada = true
			}
		}
		if !encontrada {
			t.Errorf("%s: el intento por postulación anterior debe quedar registrado: %+v", c.nombre, carlos.IntentosCedula)
		}
	}
}

func TestSinCorreoNoSeBuscaEnPostulacionesAnteriores(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	p.cedulaConocida[""] = "9999999" // aunque la base respondiera, sin correo no se pregunta
	_, _ = analizarPerfiles(t, []perfilPrueba{{"Carlos Perez", "No especificada", "sin correo", nil}}, nuevasConsultas("X", "Y"), p)
	carlos := p.guardado("Carlos Perez")
	if carlos == nil || carlos.Cedula == "9999999" || carlos.EstadoProceso != modelos.EstadoSinContacto {
		t.Errorf("sin correo no hay forma de reconocerlo: %+v", carlos)
	}
}

// ---- Reanudar ----

func nuevoAnalizadorDePrueba(t *testing.T, consultas *consultasContadas, p orquestador.Persistencia) *orquestador.Analizador {
	t.Helper()
	cfg, _ := montarDescargas(t)
	cfg.EPSAutorizadas = []string{"SURA"}
	a := orquestador.NuevoAnalizadorConValidadores(cfg, nil, orquestador.OpcionesAnalisis{Concurrencia: 1}, reporteroNulo{}, consultas.validadores)
	return a.ConPersistencia(p)
}

func candidatoQueEntregoSusDatos() *modelos.Candidato {
	return &modelos.Candidato{
		Indice: 1, Nombre: "Carlos Perez", Cedula: "1111111", URLDetalle: "https://x/Detail/Carlos Perez",
		PostulacionID: postulacionPrueba, EstadoProceso: modelos.EstadoDatosRecibidos, Genero: "Hombre",
		TextoPerfil: "perfil de Carlos",
	}
}

func TestReanudarContinuaDesdeLaCedula(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	consultas := nuevasConsultas("CARLOS", "PEREZ")
	a := nuevoAnalizadorDePrueba(t, consultas, p)

	if err := a.Reanudar(context.Background(), modelos.OfertaLaboral{ID: idOfertaPrueba, Titulo: "Conductor de pruebas SSE"}, candidatoQueEntregoSusDatos()); err != nil {
		t.Fatal(err)
	}
	if consultas.adres.Load() != 1 || consultas.runt.Load() != 1 || consultas.simit.Load() != 1 {
		t.Errorf("debían consultarse las tres fuentes: adres=%d runt=%d simit=%d", consultas.adres.Load(), consultas.runt.Load(), consultas.simit.Load())
	}
	guardado := p.guardado("Carlos Perez")
	if guardado == nil || guardado.EstadoProceso != "" || guardado.EvaluacionIdoneidad == nil || !guardado.EsApto {
		t.Fatalf("tras reanudar debe quedar evaluado y fuera de la pausa: %+v", guardado)
	}
	if guardado.ResultadoAdres == nil || guardado.ResultadoRunt == nil || guardado.ResultadoSimit == nil {
		t.Errorf("deben guardarse las validaciones: %+v", guardado)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.reanudaciones) != 1 || p.reanudaciones[0] != "apto" {
		t.Errorf("la línea de tiempo debe registrar el resultado: %v", p.reanudaciones)
	}
}

func TestReanudarConOtroTitularNoRechazaSinoRevisa(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	consultas := nuevasConsultas("PEDRO", "RUIZ")
	a := nuevoAnalizadorDePrueba(t, consultas, p)

	if err := a.Reanudar(context.Background(), modelos.OfertaLaboral{ID: idOfertaPrueba, Titulo: "Conductor de pruebas SSE"}, candidatoQueEntregoSusDatos()); err != nil {
		t.Fatal(err)
	}
	if consultas.runt.Load() != 0 || consultas.simit.Load() != 0 {
		t.Errorf("con una cédula que no cuadra no se siguen consultando datos de otra persona: runt=%d simit=%d", consultas.runt.Load(), consultas.simit.Load())
	}
	guardado := p.guardado("Carlos Perez")
	if guardado == nil || guardado.EstadoProceso != modelos.EstadoIdentidadEnRevision || guardado.EsApto || !guardado.EvaluacionIdoneidad.VerificacionPendiente {
		t.Errorf("debe quedar en revisión, pendiente: %+v", guardado)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.reanudaciones) != 1 || p.reanudaciones[0] != "identidad_en_revision" {
		t.Errorf("resultado registrado: %v", p.reanudaciones)
	}
}

func TestReanudarExigeCedula(t *testing.T) {
	a := nuevoAnalizadorDePrueba(t, nuevasConsultas("X", "Y"), nuevaPersistenciaConPausas())
	c := candidatoQueEntregoSusDatos()
	c.Cedula = "No especificada"
	if err := a.Reanudar(context.Background(), modelos.OfertaLaboral{ID: idOfertaPrueba, Titulo: "Conductor"}, c); err == nil {
		t.Error("sin cédula no hay nada que reanudar")
	}
}

// La persona de prueba tiene cédula, pero se la trata como si no la tuviera: antes de consultar nada
// pasa por la rama de contacto y el formulario. Una persona normal con cédula sigue de largo.
func TestPersonaDePruebaPasaPrimeroPorElFormularioAunqueTengaCedula(t *testing.T) {
	p := nuevaPersistenciaConPausas()
	consultas := nuevasConsultas("DANIEL", "OCHOA CASTRILLON")
	fuente := &fuentePerfiles{perfiles: []perfilPrueba{
		{"Daniel Ochoa Castrillón", "1035126774", "Contacto: daniel@mail.com cel 311 234 5678", nil},
		{"Juan Gomez", "3333333", "perfil completo", nil},
	}}
	ctrl, srv := montarControladorAnalisis(t, fuente, consultas.validadores)
	ctrl.fabricaAnalizador = func(f orquestador.FuenteCandidatos, o orquestador.OpcionesAnalisis, rep orquestador.Reportero) *orquestador.Analizador {
		o.SimularSinCedula = esPrueba
		return orquestador.NuevoAnalizadorConValidadores(ctrl.cfg, f, o, rep, consultas.validadores)
	}
	ctrl.ConPersistencia(p)
	rr := pedir(srv.rutas(), "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?antecedentes=0&sarlaft=0&ofac=0", nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}

	daniel := p.guardado("Daniel Ochoa Castrillón")
	if daniel == nil || daniel.EstadoProceso != modelos.EstadoEsperandoDatos {
		t.Fatalf("Daniel debe quedar esperando datos: %+v", daniel)
	}
	if consultas.adres.Load() != 1 {
		t.Errorf("solo Juan debía consultarse (adres=%d)", consultas.adres.Load())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pausas) != 1 || p.pausas[0].pausa.Correo != "daniel@mail.com" {
		t.Errorf("pausas: %+v", p.pausas)
	}
}
