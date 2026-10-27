package servidor

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
	"go-rrhh/utilidades"
)

// fuenteAnalisisFalsa simula Computrabajo: tres postulados, uno por cada camino del análisis.
type fuenteAnalisisFalsa struct {
	ofertasFalsas
	fichas atomic.Int32
}

const idOfertaPrueba = "ABCDEF123456"

func (f *fuenteAnalisisFalsa) ObtenerOfertasActivas(ctx context.Context) ([]modelos.OfertaLaboral, error) {
	return []modelos.OfertaLaboral{
		{Titulo: "Conductor de pruebas SSE", URLCandidatos: "https://x/Match?oi=" + idOfertaPrueba, ID: idOfertaPrueba},
	}, nil
}

func (f *fuenteAnalisisFalsa) ObtenerCandidatosStream(ctx context.Context, url string, limite int, alDescubrir func(*modelos.Candidato)) error {
	for i, n := range []string{"Carlos Perez", "Maria Lopez", "Juan Gomez"} {
		if limite > 0 && i >= limite {
			break
		}
		alDescubrir(&modelos.Candidato{Indice: i + 1, Nombre: n, URLDetalle: "https://x/Detail/" + n})
	}
	return nil
}

func (f *fuenteAnalisisFalsa) ExtraerDetalleCandidato(ctx context.Context, c *modelos.Candidato, carpeta string) error {
	f.fichas.Add(1)
	switch {
	case strings.HasPrefix(c.Nombre, "Carlos"):
		c.Cedula = "1111111"
	case strings.HasPrefix(c.Nombre, "Maria"):
		c.Cedula = "2222222"
	default:
		c.Cedula = "3333333"
	}
	c.Capturas = map[string]string{"perfil": "perfil.jpg"} // como un análisis con capturas
	return nil
}

func montarControladorAnalisis(t *testing.T, fuente FuenteOfertas, validadores orquestador.Validadores) (*ControladorRRHH, *ServidorWeb) {
	t.Helper()
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	cfg.EPSAutorizadas = []string{"SURA"}
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return fuente }
	ctrl.numCuentas = 1
	ctrl.fabricaAnalizador = func(f orquestador.FuenteCandidatos, o orquestador.OpcionesAnalisis, rep orquestador.Reportero) *orquestador.Analizador {
		return orquestador.NuevoAnalizadorConValidadores(cfg, f, o, rep, validadores)
	}
	t.Cleanup(ctrl.Detener)
	return ctrl, &ServidorWeb{controlador: ctrl}
}

func TestAnalisisSSEDescarteTemprano(t *testing.T) {
	fuente := &fuenteAnalisisFalsa{}
	var llamadasRunt, llamadasSimit, llamadasAdres atomic.Int32
	validadores := orquestador.Validadores{
		Runt: func(ctx context.Context, c *modelos.Candidato) error {
			llamadasRunt.Add(1)
			r := &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01"}
			if c.Cedula == "1111111" {
				r.Licencias = []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}}
			}
			c.ResultadoRunt = r
			return nil
		},
		Simit: func(ctx context.Context, c *modelos.Candidato) error {
			llamadasSimit.Add(1)
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true, TotalDeuda: "$ 0"}
			return nil
		},
		Adres: func(ctx context.Context, c *modelos.Candidato) error {
			llamadasAdres.Add(1)
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO"}
			return nil
		},
	}
	_, srv := montarControladorAnalisis(t, fuente, validadores)
	h := srv.rutas()

	ruta := "/api/computrabajo/vacantes/" + idOfertaPrueba + "/analizar?antecedentes=0&sarlaft=0&ofac=0"
	rr := pedir(h, "GET", ruta, nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rr.Header().Get("Content-Encoding") != "" {
		t.Error("el stream SSE no debe comprimirse")
	}
	idJob := rr.Header().Get("X-Job-ID")

	eventos := leerSSE(t, rr.Body.String())
	tipos := map[string]int{}
	completos := map[string]modelos.Candidato{}
	for _, ev := range eventos {
		tipos[ev.tipo]++
		if ev.tipo == "candidato.completo" {
			var c modelos.Candidato
			if err := json.Unmarshal([]byte(ev.datos), &c); err != nil {
				t.Fatal(err)
			}
			completos[c.Nombre] = c
		}
	}
	if eventos[0].tipo != "job.iniciado" || eventos[len(eventos)-1].tipo != "job.completado" {
		t.Errorf("primer/último evento: %s / %s", eventos[0].tipo, eventos[len(eventos)-1].tipo)
	}
	if tipos["candidato.descubierto"] != 3 || tipos["candidato.completo"] != 3 {
		t.Errorf("eventos por candidato: %v", tipos)
	}

	// Persona por persona: las etapas de un candidato no se mezclan con las de otro.
	actual := 0
	for _, ev := range eventos {
		if ev.tipo != "candidato.etapa" && ev.tipo != "candidato.completo" {
			continue
		}
		var cuerpo struct {
			Indice int `json:"indice"`
		}
		_ = json.Unmarshal([]byte(ev.datos), &cuerpo)
		if cuerpo.Indice < actual {
			t.Fatalf("evento %s del candidato %d llegó después de empezar el %d", ev.tipo, cuerpo.Indice, actual)
		}
		actual = cuerpo.Indice
	}

	carlos, maria, juan := completos["Carlos Perez"], completos["Maria Lopez"], completos["Juan Gomez"]
	if !carlos.EsApto {
		t.Errorf("Carlos debía ser apto: %+v", carlos.EvaluacionIdoneidad)
	}
	if maria.EsApto || maria.EvaluacionIdoneidad == nil || !maria.EvaluacionIdoneidad.DescarteTemprano ||
		!strings.Contains(maria.EvaluacionIdoneidad.MotivosDescarte[0], "Género") {
		t.Errorf("María debía descartarse por género sin consultas: %+v", maria.EvaluacionIdoneidad)
	}
	if juan.EsApto || juan.EvaluacionIdoneidad == nil || !strings.Contains(juan.EvaluacionIdoneidad.MotivosDescarte[0], "C2 o C3") {
		t.Errorf("Juan debía descartarse por licencia: %+v", juan.EvaluacionIdoneidad)
	}
	// Descarte temprano: la EPS (ADRES) va primero y se consulta solo para los hombres (2); RUNT solo para
	// quien pasó la EPS (2) y SIMIT solo para quien pasó RUNT (1).
	if llamadasAdres.Load() != 2 || llamadasRunt.Load() != 2 || llamadasSimit.Load() != 1 {
		t.Errorf("consultas adres=%d runt=%d simit=%d (esperado 2/2/1)", llamadasAdres.Load(), llamadasRunt.Load(), llamadasSimit.Load())
	}

	// Reconectar con el último id: ya recibió todo, 204 para que EventSource no reintente.
	ultimo := eventos[len(eventos)-1].id
	rr = pedir(h, "GET", "/api/jobs/"+idJob+"/eventos", map[string]string{"Last-Event-ID": ultimo})
	if rr.Code != 204 {
		t.Errorf("reconexión al final: código %d", rr.Code)
	}

	// Segundo análisis: todo sale de la caché y no vuelve a abrir fichas ni consultas.
	fichas := fuente.fichas.Load()
	rr = pedir(h, "GET", ruta, nil)
	if rr.Code != 200 || fuente.fichas.Load() != fichas {
		t.Fatalf("el reanálisis debía usar la caché (fichas %d -> %d, código %d)", fichas, fuente.fichas.Load(), rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"cache":true`) {
		t.Error("los candidatos del reanálisis deben marcarse cache:true")
	}

	// Candidatos guardados de la vacante, buscados por su id (no por el slug, que puede repetirse).
	rr = pedir(h, "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/candidatos", nil)
	var guardados []CandidatoVacante
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &guardados) != nil || len(guardados) != 3 {
		t.Fatalf("candidatos guardados: código %d, %s", rr.Code, rr.Body.String())
	}
	if guardados[0].Slug == "" {
		t.Error("cada candidato guardado debe traer su slug")
	}
}

// El inicio y el final de cada análisis llegan por /api/cambios: la pantalla no necesita sondear /api/jobs.
func TestAnalisisAvisaInicioYFinPorCambios(t *testing.T) {
	ctrl, srv := montarControladorAnalisis(t, &fuenteAnalisisFalsa{}, orquestador.Validadores{})
	avisos, desuscribir := ctrl.cambios.suscribir()
	defer desuscribir()

	pedir(srv.rutas(), "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?limite=1", nil)
	var estados []string
	limite := time.After(3 * time.Second) // el aviso final sale justo después de cerrar el stream
	for len(estados) < 2 {
		var av aviso
		select {
		case av = <-avisos:
		case <-limite:
			t.Fatalf("faltan avisos de job: %v", estados)
		}
		if av.evento != "job" {
			continue
		}
		var d map[string]string
		_ = json.Unmarshal([]byte(av.datos), &d)
		if d["oferta_id"] != idOfertaPrueba || d["id"] == "" {
			t.Errorf("aviso sin la vacante o el id: %s", av.datos)
		}
		estados = append(estados, d["estado"])
	}
	if len(estados) != 2 || estados[0] != jobEnCurso || estados[1] == jobEnCurso {
		t.Errorf("avisos de job: %v", estados)
	}
}

func TestAnalisisValidaciones(t *testing.T) {
	_, srv := montarControladorAnalisis(t, &fuenteAnalisisFalsa{}, orquestador.Validadores{})
	h := srv.rutas()

	if rr := pedir(h, "GET", "/api/computrabajo/vacantes/x!/analizar", nil); rr.Code != 400 {
		t.Errorf("id inválido: código %d", rr.Code)
	}
	if rr := pedir(h, "GET", "/api/computrabajo/vacantes/NOEXISTE1234/analizar", nil); rr.Code != 404 {
		t.Errorf("vacante inexistente: código %d", rr.Code)
	}
	if rr := pedir(h, "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?limite=-1", nil); rr.Code != 400 {
		t.Errorf("limite inválido: código %d", rr.Code)
	}
	if rr := pedir(h, "GET", "/api/jobs/inexistente/eventos", nil); rr.Code != 404 {
		t.Errorf("job inexistente: código %d", rr.Code)
	}
}

func TestJobSoloUnoALaVez(t *testing.T) {
	ctrl, srv := montarControladorAnalisis(t, &fuenteAnalisisFalsa{}, orquestador.Validadores{})
	h := srv.rutas()

	job := nuevoJobAnalisis("AAAAAAAA1111", "Vacante A")
	cancelado := false
	job.cancel = func() { cancelado = true }
	ctrl.jobActual = job
	ctrl.jobs = []*JobAnalisis{job}

	rr := pedir(h, "GET", "/api/computrabajo/vacantes/BBBBBBBB2222/analizar", nil)
	if rr.Code != 409 || !strings.Contains(rr.Body.String(), job.ID) {
		t.Errorf("otra vacante con un job activo debe dar 409 con el job: %d %s", rr.Code, rr.Body.String())
	}

	// La lista de vacantes no abre Chrome mientras corre un análisis. Sin caché igual responde (con la
	// vacante en análisis) en vez de dejar la pantalla sin nada.
	if rr = pedir(h, "GET", "/api/computrabajo/vacantes", nil); rr.Code != 200 || rr.Header().Get("X-Vacantes-Fuente") != "parcial-analisis-en-curso" {
		t.Errorf("vacantes durante un análisis sin caché: código %d", rr.Code)
	}
	ctrl.guardarOfertasEnCache([]ofertaConConteo{{OfertaLaboral: modelos.OfertaLaboral{Titulo: "X", ID: "AAAAAAAA1111"}}})
	rr = pedir(h, "GET", "/api/computrabajo/vacantes", nil)
	if rr.Code != 200 || rr.Header().Get("X-Vacantes-Fuente") == "" {
		t.Errorf("vacantes con caché durante un análisis: código %d", rr.Code)
	}

	rr = pedir(h, "DELETE", "/api/jobs/"+job.ID, nil)
	if rr.Code != 202 || !cancelado {
		t.Errorf("cancelar: código %d, cancelado=%v", rr.Code, cancelado)
	}
}

type eventoLeido struct{ id, tipo, datos string }

// leerSSE interpreta un cuerpo text/event-stream.
func leerSSE(t *testing.T, cuerpo string) []eventoLeido {
	t.Helper()
	var eventos []eventoLeido
	var actual eventoLeido
	sc := bufio.NewScanner(strings.NewReader(cuerpo))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		linea := sc.Text()
		switch {
		case linea == "":
			if actual.tipo != "" {
				eventos = append(eventos, actual)
			}
			actual = eventoLeido{}
		case strings.HasPrefix(linea, "id: "):
			actual.id = strings.TrimPrefix(linea, "id: ")
		case strings.HasPrefix(linea, "event: "):
			actual.tipo = strings.TrimPrefix(linea, "event: ")
		case strings.HasPrefix(linea, "data: "):
			actual.datos = strings.TrimPrefix(linea, "data: ")
		}
	}
	if len(eventos) == 0 {
		t.Fatalf("el stream no trajo eventos: %q", cuerpo)
	}
	return eventos
}

// Un portal caído (SIMIT) no debe rechazar al candidato: queda pendiente, se siguen las demás reglas
// y el siguiente análisis lo reintenta en vez de tomarlo de la caché.
func TestAnalisisConsultaCaidaQuedaPendiente(t *testing.T) {
	fuente := &fuenteAnalisisFalsa{}
	var llamadasSimit, llamadasAdres atomic.Int32
	validadores := orquestador.Validadores{
		Runt: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoRunt = &modelos.ResultadoRunt{
				Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}},
			}
			return nil
		},
		Simit: func(ctx context.Context, c *modelos.Candidato) error {
			llamadasSimit.Add(1)
			c.ResultadoSimit = &modelos.ResultadoSimit{
				Estado:   "ERROR_CONSULTA",
				Detalles: "Error consultando SIMIT: error al abrir el portal SIMIT: net::ERR_CONNECTION_TIMED_OUT",
			}
			return nil
		},
		Adres: func(ctx context.Context, c *modelos.Candidato) error {
			llamadasAdres.Add(1)
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO"}
			return nil
		},
	}
	_, srv := montarControladorAnalisis(t, fuente, validadores)
	h := srv.rutas()

	ruta := "/api/computrabajo/vacantes/" + idOfertaPrueba + "/analizar?antecedentes=0&sarlaft=0&ofac=0&limite=1"
	rr := pedir(h, "GET", ruta, nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}

	var candidato modelos.Candidato
	causaVisible := false
	for _, ev := range leerSSE(t, rr.Body.String()) {
		switch ev.tipo {
		case "candidato.completo":
			_ = json.Unmarshal([]byte(ev.datos), &candidato)
		case "candidato.etapa":
			if strings.Contains(ev.datos, `"etapa":"simit"`) && strings.Contains(ev.datos, "ERR_CONNECTION_TIMED_OUT") {
				causaVisible = true
			}
		}
	}
	ev := candidato.EvaluacionIdoneidad
	if ev == nil || !ev.VerificacionPendiente || ev.EsApto || ev.DescarteTemprano {
		t.Fatalf("debía quedar pendiente (ni apto ni descartado): %+v", ev)
	}
	if !causaVisible {
		t.Error("el evento de la etapa SIMIT debe traer la causa real del fallo")
	}
	if llamadasSimit.Load() != 2 {
		t.Errorf("la consulta fallida debía reintentarse una vez (llamadas=%d)", llamadasSimit.Load())
	}
	if llamadasAdres.Load() != 1 {
		t.Errorf("ADRES debía consultarse igual aunque SIMIT falló (llamadas=%d)", llamadasAdres.Load())
	}
	if candidato.ErrorProceso == "" {
		t.Error("ErrorProceso debe marcar al candidato para reintentarlo")
	}

	// Reanálisis: al no ser un dictamen firme, no sale de la caché.
	antes := llamadasAdres.Load()
	_ = pedir(h, "GET", ruta, nil)
	if llamadasAdres.Load() == antes {
		t.Error("el candidato pendiente debía reprocesarse, no tomarse de la caché")
	}
}

// fuenteLenta tarda en leer la ficha de los nuevos, como Computrabajo con captchas.
type fuenteLenta struct {
	fuenteAnalisisFalsa
	espera atomic.Int64 // milisegundos
}

func (f *fuenteLenta) ExtraerDetalleCandidato(ctx context.Context, c *modelos.Candidato, carpeta string) error {
	time.Sleep(time.Duration(f.espera.Load()) * time.Millisecond)
	return f.fuenteAnalisisFalsa.ExtraerDetalleCandidato(ctx, c, carpeta)
}

// Un candidato ya analizado se confirma enseguida aunque delante haya uno nuevo cuyo análisis tarda.
func TestAnalisisReutilizadosNoEsperanALosNuevos(t *testing.T) {
	fuente := &fuenteLenta{}
	ctrl, srv := montarControladorAnalisis(t, fuente, orquestador.Validadores{
		Runt: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoRunt = &modelos.ResultadoRunt{Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01"}
			return nil
		},
		Simit: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "OK", PazYSalvo: true, TotalDeuda: "$ 0"}
			return nil
		},
		Adres: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO"}
			return nil
		},
	})
	h := srv.rutas()
	ruta := "/api/computrabajo/vacantes/" + idOfertaPrueba + "/analizar?antecedentes=0&sarlaft=0&ofac=0"
	if rr := pedir(h, "GET", ruta, nil); rr.Code != 200 {
		t.Fatalf("primer análisis: %d", rr.Code)
	}

	// Solo Juan (el último del listado) queda guardado; Carlos y María vuelven a ser nuevos y lentos.
	archivo := filepath.Join(ctrl.cfg.DirectorioDescargas, agentes.LimpiarNombre("Conductor de pruebas SSE"), "candidatos.json")
	var lista []modelos.Candidato
	if err := utilidades.LeerJSON(archivo, &lista); err != nil {
		t.Fatal(err)
	}
	var soloJuan []modelos.Candidato
	for _, c := range lista {
		if strings.HasPrefix(c.Nombre, "Juan") {
			soloJuan = append(soloJuan, c)
		}
	}
	if len(soloJuan) != 1 {
		t.Fatalf("candidatos guardados: %+v", lista)
	}
	if err := utilidades.EscribirJSONAtomico(archivo, soloJuan, false); err != nil {
		t.Fatal(err)
	}

	fuente.espera.Store(300)
	var completos []string
	for _, ev := range leerSSE(t, pedir(h, "GET", ruta, nil).Body.String()) {
		if ev.tipo == "candidato.completo" {
			var c modelos.Candidato
			_ = json.Unmarshal([]byte(ev.datos), &c)
			completos = append(completos, c.Nombre)
		}
	}
	if len(completos) != 3 || completos[0] != "Juan Gomez" {
		t.Errorf("Juan (guardado) debía confirmarse primero: %v", completos)
	}
}

// Al unirse a un análisis largo no se repiten las etapas intermedias: llega lo justo para reconstruirlo.
func TestRepeticionCompactada(t *testing.T) {
	job := nuevoJobAnalisis(idOfertaPrueba, "Vacante")
	job.Emitir("job.iniciado", map[string]any{"job_id": job.ID})
	for i := 1; i <= 20; i++ {
		job.Emitir("candidato.descubierto", map[string]any{"indice": i, "nombre": "P"})
		for _, etapa := range []string{"perfil", "adres", "runt"} {
			job.Emitir("candidato.etapa", map[string]any{"indice": i, "etapa": etapa, "estado": "iniciada"})
			job.Emitir("candidato.etapa", map[string]any{"indice": i, "etapa": etapa, "estado": "ok"})
		}
		if i < 20 { // el 20 sigue en análisis
			job.Emitir("candidato.completo", map[string]any{"indice": i, "nombre": "P"})
			job.Emitir("resumen", map[string]any{"procesados": i})
		}
	}
	total := job.estadoJob().Eventos

	pendientes, _ := job.desde(0)
	tipos := map[string]int{}
	for _, ev := range pendientes {
		tipos[ev.tipo]++
		if ev.tipo == "candidato.etapa" && ev.indice != 20 {
			t.Errorf("etapa de alguien que ya terminó: %s", ev.datos)
		}
	}
	if tipos["candidato.descubierto"] != 20 || tipos["candidato.completo"] != 19 || tipos["resumen"] != 1 || tipos["candidato.etapa"] != 3 {
		t.Errorf("eventos compactados: %v (de %d)", tipos, total)
	}
	if !strings.Contains(string(pendientes[len(pendientes)-1].datos), `"estado":"ok"`) || pendientes[len(pendientes)-1].id != total {
		t.Errorf("el último evento debe conservarse con su id: %+v", pendientes[len(pendientes)-1])
	}
	// En vivo (pocos pendientes) se envía todo tal cual.
	if p, _ := job.desde(total - 3); len(p) != 3 {
		t.Errorf("en vivo no se compacta: %d", len(p))
	}
}

// Un refresco sin cambios responde 304 sin cuerpo; si algo cambia, el contenido nuevo con otro ETag.
func TestCandidatosGuardadosConETag(t *testing.T) {
	_, srv := montarControladorAnalisis(t, &fuenteAnalisisFalsa{}, validadoresQueCumplen())
	h := srv.rutas()
	pedir(h, "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?limite=1&antecedentes=0&sarlaft=0&ofac=0", nil)
	ruta := "/api/computrabajo/vacantes/" + idOfertaPrueba + "/candidatos"

	rr := pedir(h, "GET", ruta, nil)
	etag := rr.Header().Get("ETag")
	if rr.Code != 200 || etag == "" {
		t.Fatalf("primera lectura: %d, etag %q", rr.Code, etag)
	}
	rr = pedir(h, "GET", ruta, map[string]string{"If-None-Match": etag})
	if rr.Code != http.StatusNotModified || rr.Body.Len() != 0 {
		t.Errorf("sin cambios debía ser 304 vacío: %d %q", rr.Code, rr.Body.String())
	}
	pedir(h, "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?antecedentes=0&sarlaft=0&ofac=0", nil)
	if rr = pedir(h, "GET", ruta, map[string]string{"If-None-Match": etag}); rr.Code != 200 || rr.Header().Get("ETag") == etag {
		t.Errorf("con candidatos nuevos debía responder 200 con otro ETag: %d", rr.Code)
	}
}
