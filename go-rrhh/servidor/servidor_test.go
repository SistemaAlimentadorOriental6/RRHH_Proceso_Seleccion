package servidor

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"go-rrhh/agentes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// montarDescargas crea un directorio descargas/ de prueba con una oferta, aptos y ranking.
func montarDescargas(t *testing.T) (*configuracion.Configuracion, http.Handler) {
	t.Helper()
	raiz := t.TempDir()
	desc := filepath.Join(raiz, "descargas")

	oferta := filepath.Join(desc, "Operador_de_buses_alimentadores_-_Metro_de_Medellín")
	if err := os.MkdirAll(filepath.Join(desc, "candidatos_aptos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oferta, 0o755); err != nil {
		t.Fatal(err)
	}

	escribir := func(ruta string, v any) {
		t.Helper()
		datos, _ := json.Marshal(v)
		if err := os.WriteFile(ruta, datos, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	escribir(filepath.Join(oferta, "candidatos.json"), []modelos.Candidato{
		{Indice: 1, Nombre: "Julián Andrés Castañeda Álzate", Cedula: "71799510", EsApto: true},
		{Indice: 2, Nombre: "Luis Villada", Cedula: "123456789", EsApto: false},
	})
	escribir(filepath.Join(desc, "candidatos_aptos", "candidatos_aptos.json"), []modelos.RegistroApto{
		{Nombre: "Julián Andrés Castañeda Álzate", Cedula: "71799510"},
	})
	escribir(filepath.Join(desc, "candidatos_aptos", "ranking_candidatos_ia.json"), []modelos.CandidatoCalificadoIA{
		{
			Puesto:       1,
			Candidato:    modelos.RegistroApto{Nombre: "Julián Andrés Castañeda Álzate", Cedula: "71.799.510"},
			TextoCV:      "texto largo de la hoja de vida",
			EvaluacionIA: modelos.EvaluacionIA{ScoreTotal: 90},
		},
	})

	// Un PDF dentro de descargas y un secreto fuera.
	if err := os.WriteFile(filepath.Join(oferta, "cv.pdf"), []byte("%PDF-1.4 prueba"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "secreto.pdf"), []byte("%PDF-1.4 secreto"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &configuracion.Configuracion{DirectorioDescargas: desc, DirectorioRaiz: raiz}
	srv := &ServidorWeb{
		controlador: NuevoControladorRRHH(cfg, NuevoRepositorioDisco(desc), ""),
	}
	t.Cleanup(srv.controlador.Detener)
	return cfg, srv.rutas()
}

func pedir(h http.Handler, metodo, ruta string, cabeceras map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, ruta, nil)
	for k, v := range cabeceras {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestSlugVacante(t *testing.T) {
	casos := map[string]string{
		"Operador(a) de buses alimentadores - Metro de Medellín":      "operador-buses-alimentadores-metro-medellin",
		"Operador de buses alimentadores - Metro de Medellín":         "operador-buses-alimentadores-metro-medellin",
		"Operador(a) de buses alimentadores - Metro de Medellín 2026": "operador-buses-alimentadores-metro-medellin-2026",
	}
	for entrada, esperado := range casos {
		if got := SlugVacante(entrada); got != esperado {
			t.Errorf("SlugVacante(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
	if got := SlugPersona("Julián Andrés Castañeda Álzate"); got != "julian-andres-castaneda-alzate" {
		t.Errorf("SlugPersona = %q", got)
	}
}

func TestVacantesYCandidatos(t *testing.T) {
	_, h := montarDescargas(t)
	slug := "operador-buses-alimentadores-metro-medellin"

	rr := pedir(h, "GET", "/api/vacantes", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), slug) {
		t.Fatalf("/api/vacantes: %d %s", rr.Code, rr.Body.String())
	}

	rr = pedir(h, "GET", "/api/vacantes/"+slug+"/candidatos?filtro=aptos", nil)
	var lista []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &lista); err != nil || len(lista) != 1 {
		t.Fatalf("candidatos aptos: %d %s", rr.Code, rr.Body.String())
	}
	if lista[0]["slug"] != "julian-andres-castaneda-alzate" {
		t.Errorf("slug de persona = %v", lista[0]["slug"])
	}
	if rr.Header().Get("X-Total-Count") != "1" {
		t.Errorf("X-Total-Count = %q", rr.Header().Get("X-Total-Count"))
	}

	// Por slug de persona y por cédula
	for _, id := range []string{"julian-andres-castaneda-alzate", "71799510"} {
		rr = pedir(h, "GET", "/api/vacantes/"+slug+"/candidatos/"+id, nil)
		if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"ranking"`) {
			t.Errorf("candidato %s: %d %s", id, rr.Code, rr.Body.String())
		}
	}
	if rr = pedir(h, "GET", "/api/vacantes/"+slug+"/candidatos/no-existe", nil); rr.Code != 404 {
		t.Errorf("candidato inexistente: %d", rr.Code)
	}
	if rr = pedir(h, "GET", "/api/vacantes/otra-vacante", nil); rr.Code != 404 {
		t.Errorf("vacante inexistente: %d", rr.Code)
	}

	// Paginación
	rr = pedir(h, "GET", "/api/vacantes/"+slug+"/candidatos?limite=1&offset=1", nil)
	if rr.Header().Get("X-Total-Count") != "2" {
		t.Errorf("total con paginación = %q", rr.Header().Get("X-Total-Count"))
	}
	if rr = pedir(h, "GET", "/api/vacantes/"+slug+"/candidatos?limite=abc", nil); rr.Code != 400 {
		t.Errorf("limite inválido: %d", rr.Code)
	}
}

func TestRankingSinTextoCVPorDefecto(t *testing.T) {
	_, h := montarDescargas(t)

	rr := pedir(h, "GET", "/api/ranking", nil)
	if strings.Contains(rr.Body.String(), "texto_cv") {
		t.Error("el ranking no debería incluir texto_cv por defecto")
	}
	rr = pedir(h, "GET", "/api/ranking?incluir_cv=1", nil)
	if !strings.Contains(rr.Body.String(), "texto_cv") {
		t.Error("con incluir_cv=1 debe incluir texto_cv")
	}
	rr = pedir(h, "GET", "/api/ranking?oferta=operador-buses-alimentadores-metro-medellin", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"puesto":1`) {
		t.Errorf("ranking por oferta: %d %s", rr.Code, rr.Body.String())
	}
}

func TestParametrosInvalidos(t *testing.T) {
	_, h := montarDescargas(t)

	for _, ruta := range []string{
		"/api/candidatos?oferta=../..",
		"/api/candidatos?oferta=" + url.QueryEscape(`..\..`),
		"/api/candidatos?oferta=candidatos_aptos",
	} {
		if rr := pedir(h, "GET", ruta, nil); rr.Code != 400 {
			t.Errorf("%s -> %d, esperado 400", ruta, rr.Code)
		}
	}
	if rr := pedir(h, "GET", "/api/candidatos", nil); rr.Code != 400 {
		t.Errorf("sin oferta: %d", rr.Code)
	}
	if rr := pedir(h, "GET", "/api/candidatos?oferta=Operador_de_buses_alimentadores_-_Metro_de_Medellín&filtro=xx", nil); rr.Code != 400 {
		t.Errorf("filtro inválido: %d", rr.Code)
	}
	if rr := pedir(h, "POST", "/api/ofertas", nil); rr.Code != 405 {
		t.Errorf("método no permitido: %d", rr.Code)
	}
}

func TestServirPDFSoloDentroDeDescargas(t *testing.T) {
	cfg, h := montarDescargas(t)
	dentro := filepath.Join(cfg.DirectorioDescargas, "Operador_de_buses_alimentadores_-_Metro_de_Medellín", "cv.pdf")
	fuera := filepath.Join(cfg.DirectorioRaiz, "secreto.pdf")

	if rr := pedir(h, "GET", "/api/pdf?ruta="+url.QueryEscape(dentro), nil); rr.Code != 200 || !strings.HasPrefix(rr.Body.String(), "%PDF") {
		t.Errorf("pdf permitido: %d", rr.Code)
	}
	for _, ruta := range []string{fuera, "../secreto.pdf", filepath.Join("..", "secreto.pdf"), "C:\\Windows\\win.ini"} {
		if rr := pedir(h, "GET", "/api/pdf?ruta="+url.QueryEscape(ruta), nil); rr.Code != 403 && rr.Code != 400 {
			t.Errorf("ruta %q debería rechazarse, código %d", ruta, rr.Code)
		}
	}
	if rr := pedir(h, "GET", "/api/pdf?ruta="+url.QueryEscape(fuera), nil); strings.Contains(rr.Body.String(), "secreto") {
		t.Error("se filtró el contenido de un archivo fuera de descargas")
	}
}

func TestCORSYGzip(t *testing.T) {
	_, h := montarDescargas(t)

	rr := pedir(h, "OPTIONS", "/api/candidato/preguntar", map[string]string{
		"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST",
	})
	if rr.Code != 204 || rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("preflight: %d %v", rr.Code, rr.Header())
	}

	rr = pedir(h, "GET", "/api/ofertas", map[string]string{"Origin": "http://sitio-malo.example"})
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("origen no permitido no debe recibir cabeceras CORS")
	}

	rr = pedir(h, "GET", "/api/ofertas", map[string]string{"Accept-Encoding": "gzip"})
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("esperaba gzip: %v", rr.Header())
	}
	zr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	cuerpo, _ := io.ReadAll(zr)
	if !strings.Contains(string(cuerpo), "operador-buses-alimentadores-metro-medellin") {
		t.Errorf("cuerpo gzip inesperado: %s", cuerpo)
	}
}

func TestRankingVacioNoInventaPuntajes(t *testing.T) {
	cfg, h := montarDescargas(t)
	_ = os.Remove(filepath.Join(cfg.DirectorioDescargas, "candidatos_aptos", "ranking_candidatos_ia.json"))

	rr := pedir(h, "GET", "/api/ranking", nil)
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Errorf("ranking sin evaluar debe ser []: %d %q", rr.Code, rr.Body.String())
	}
}

func TestJSONDanadoSeReportaEnLaOferta(t *testing.T) {
	cfg, h := montarDescargas(t)
	oferta := filepath.Join(cfg.DirectorioDescargas, "Operador_de_buses_alimentadores_-_Metro_de_Medellín", "candidatos.json")
	if err := os.WriteFile(oferta, []byte(`[{"es_apto": tr`), 0o644); err != nil {
		t.Fatal(err)
	}

	rr := pedir(h, "GET", "/api/ofertas", nil)
	if !strings.Contains(rr.Body.String(), `"error"`) {
		t.Errorf("la oferta con JSON dañado debe informar error: %s", rr.Body.String())
	}
}

// ofertasFalsas simula Computrabajo: primero responde "sesión expirada" y tras autenticar entrega ofertas.
type ofertasFalsas struct {
	expirada bool
	auths    int
	listados int
	cerrada  bool
}

func (o *ofertasFalsas) AsegurarAutenticacion(ctx context.Context) error {
	o.auths++
	o.expirada = false
	return nil
}

func (o *ofertasFalsas) ObtenerOfertasActivas(ctx context.Context) ([]modelos.OfertaLaboral, error) {
	o.listados++
	if o.expirada {
		return nil, agentes.ErrSesionExpirada
	}
	return []modelos.OfertaLaboral{
		{Titulo: "Operador(a) de buses alimentadores - Metro de Medellín", URLCandidatos: "https://x/Match?oi=A1", ID: "A1"},
		{Titulo: "Técnico mecánico de vehículos pesados", URLCandidatos: "https://x/Match?oi=B2", ID: "B2"},
	}, nil
}

func (o *ofertasFalsas) ContarCandidatosOferta(ctx context.Context, url string) (int, error) {
	if strings.Contains(url, "B2") {
		return 0, errors.New("falla simulada")
	}
	return 37, nil
}

func (o *ofertasFalsas) Cerrar() { o.cerrada = true }

func TestVacantesComputrabajoEnVivo(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	cfg.OfertasExcluidas = []string{"técnico mecánico"}

	falsa := &ofertasFalsas{expirada: true}
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return falsa }
	ctrl.numCuentas = 1
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	// La primera vez se lee en vivo sin conteos (rápido) y los conteos llegan en segundo plano.
	rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil)
	if rr.Code != 200 || rr.Header().Get("X-Vacantes-Fuente") != "en-vivo-actualizando" {
		t.Fatalf("código %d, fuente %q: %s", rr.Code, rr.Header().Get("X-Vacantes-Fuente"), rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"total_candidatos":37`) {
		t.Error("la primera respuesta no debe esperar los conteos")
	}
	ctrl.refrescos.Wait()
	if falsa.auths != 1 {
		t.Errorf("debía autenticar una vez tras la sesión expirada, auths=%d", falsa.auths)
	}
	rr = pedir(h, "GET", "/api/computrabajo/vacantes", nil)
	if rr.Header().Get("X-Vacantes-Fuente") != "cache-reciente" {
		t.Errorf("tras el refresco debía servirse la lista guardada: %q", rr.Header().Get("X-Vacantes-Fuente"))
	}

	var vacantes []VacanteComputrabajo
	if err := json.Unmarshal(rr.Body.Bytes(), &vacantes); err != nil || len(vacantes) != 2 {
		t.Fatalf("respuesta inesperada: %s", rr.Body.String())
	}
	a, b := vacantes[0], vacantes[1]
	if a.Slug != "operador-buses-alimentadores-metro-medellin" || !a.Procesada || a.Excluida {
		t.Errorf("primera vacante: %+v", a)
	}
	if a.TotalCandidatos == nil || *a.TotalCandidatos != 37 {
		t.Errorf("total de inscritos en Computrabajo: %v", a.TotalCandidatos)
	}
	if b.Procesada || !b.Excluida {
		t.Errorf("la segunda debe estar excluida y sin procesar: %+v", b)
	}
	// Los totales de lo analizado vienen en la lista: la pantalla no carga a los candidatos para contarlos.
	if a.Resumen == nil || a.Resumen.Postulados == 0 || a.Resumen.Aptos+a.Resumen.NoAptos+a.Resumen.Pendientes != a.Resumen.Postulados {
		t.Errorf("resumen de lo analizado: %+v", a.Resumen)
	}
	if b.Resumen != nil {
		t.Errorf("sin análisis no hay resumen: %+v", b.Resumen)
	}
	// Un fallo al contar una vacante no tumba la respuesta: queda null con su aviso.
	if b.TotalCandidatos != nil || b.ErrorConteo == "" {
		t.Errorf("conteo fallido debe ser null con error_conteo: %+v", b)
	}
	if strings.Contains(rr.Body.String(), "total_candidatos_local") {
		t.Error("ya no deben salir los totales locales")
	}

	// ?contar=0 no abre los listados
	rr = pedir(h, "GET", "/api/computrabajo/vacantes?contar=0", nil)
	if !strings.Contains(rr.Body.String(), `"total_candidatos":null`) {
		t.Errorf("con contar=0 el total debe ser null: %s", rr.Body.String())
	}

	ctrl.Detener()
	if !falsa.cerrada {
		t.Error("Detener debe cerrar el navegador de Computrabajo")
	}
}

func TestVacantesComputrabajoSinCredenciales(t *testing.T) {
	_, h := montarDescargas(t) // cfg sin usuario/clave
	if rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil); rr.Code != 503 {
		t.Errorf("sin credenciales debe responder 503, fue %d", rr.Code)
	}
}

func TestRaizEsIndiceJSONYRutasDesconocidas404(t *testing.T) {
	_, h := montarDescargas(t)
	if rr := pedir(h, "GET", "/", nil); rr.Code != 200 || !strings.Contains(rr.Body.String(), "/api/vacantes") {
		t.Errorf("índice: %d %s", rr.Code, rr.Body.String())
	}
	if rr := pedir(h, "GET", "/index.html", nil); rr.Code != 404 || !strings.Contains(rr.Header().Get("Content-Type"), "json") {
		t.Errorf("ruta desconocida: %d %v", rr.Code, rr.Header())
	}
}

// Las hojas de vida del correo salen en el listado como una vacante más, con su propio conteo,
// y las que no buscan ser conductor no cuentan.
func TestVacantesIncluyenHojasDeVidaDelCorreo(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	cfg.CorreoHost, cfg.CorreoUsuario, cfg.CorreoClave = "mail.ejemplo.com", "hv@ejemplo.com", "x"

	hojas := []modelos.HojaVidaCorreo{
		{UID: 1, MessageID: "<a>", InteresConductor: "si"},
		{UID: 2, MessageID: "<b>", InteresConductor: "dudoso"},
		{UID: 3, MessageID: "<c>", InteresConductor: "no"},
	}
	if err := utilidades.EscribirJSONAtomico(filepath.Join(cfg.DirectorioDescargas, "correo", "hojas_de_vida.json"), hojas, false); err != nil {
		t.Fatal(err)
	}

	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return &ofertasFalsas{} }
	ctrl.numCuentas = 1
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	rr := pedir(h, "GET", "/api/computrabajo/vacantes?contar=0", nil)
	var vacantes []VacanteComputrabajo
	if err := json.Unmarshal(rr.Body.Bytes(), &vacantes); err != nil || len(vacantes) != 3 {
		t.Fatalf("debían salir 2 de Computrabajo + la del correo: %s", rr.Body.String())
	}
	correo := vacantes[2]
	if correo.Origen != "correo" || correo.Titulo != "Hojas de vida por correo" || correo.ID != "CORREOHOJASDEVIDA" {
		t.Errorf("vacante del correo: %+v", correo)
	}
	if correo.TotalCandidatos == nil || *correo.TotalCandidatos != 2 {
		t.Errorf("solo 2 de 3 buscan ser conductor: %v", correo.TotalCandidatos)
	}
	if vacantes[0].Origen != "computrabajo" {
		t.Errorf("origen de las de Computrabajo: %q", vacantes[0].Origen)
	}
	if !reIDOferta.MatchString(correo.ID) {
		t.Error("el id de la vacante virtual debe pasar la validación de las rutas de análisis")
	}
}

// Con un análisis en curso y sin lista guardada la pantalla no se queda en blanco: recibe la
// vacante que se está analizando (y la del correo), no un 409.
func TestVacantesDuranteAnalisisSinListaGuardada(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.numCuentas = 1
	ctrl.fabricaOfertas = func(int) FuenteOfertas { t.Fatal("no debe abrir Computrabajo durante un análisis"); return nil }
	ctrl.jobActual = nuevoJobAnalisis("ABCDEF123456", "Operador de buses")
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Vacantes-Fuente") != "parcial-analisis-en-curso" {
		t.Errorf("fuente: %q", rr.Header().Get("X-Vacantes-Fuente"))
	}
	var vacantes []VacanteComputrabajo
	if err := json.Unmarshal(rr.Body.Bytes(), &vacantes); err != nil || len(vacantes) != 1 || vacantes[0].ID != "ABCDEF123456" {
		t.Errorf("respuesta: %s", rr.Body.String())
	}
}

// Una lista leída hace instantes se reutiliza, y la pedida con conteo exige que los traiga.
func TestVacantesReutilizaListaReciente(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	falsa := &ofertasFalsas{}
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return falsa }
	ctrl.numCuentas = 1
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	if rr := pedir(h, "GET", "/api/computrabajo/vacantes?contar=0", nil); rr.Code != 200 {
		t.Fatalf("primera: %d", rr.Code)
	}
	listados := falsa.listados
	// Sin conteos guardados, pedirlos obliga a leer de nuevo; la rápida sí se sirve de la lista.
	rr := pedir(h, "GET", "/api/computrabajo/vacantes?contar=0", nil)
	if rr.Header().Get("X-Vacantes-Fuente") != "cache-reciente" || falsa.listados != listados {
		t.Errorf("la lista reciente debía reutilizarse (fuente %q, lecturas %d→%d)", rr.Header().Get("X-Vacantes-Fuente"), listados, falsa.listados)
	}
	// Sin conteos guardados, pedirlos sirve la lista igual y los lee en segundo plano.
	if rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil); rr.Header().Get("X-Vacantes-Fuente") != "cache-actualizando" {
		t.Errorf("sin conteos en la lista guardada debía actualizarse en segundo plano: %q", rr.Header().Get("X-Vacantes-Fuente"))
	}
	ctrl.refrescos.Wait()
	if rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil); rr.Header().Get("X-Vacantes-Fuente") != "cache-reciente" {
		t.Error("con conteos guardados y recientes se reutiliza")
	}
	ctrl.Detener()
}

// Una lista vieja se responde al instante aunque el navegador esté ocupado (antes la petición esperaba
// a que terminara, p. ej., el conteo del proceso automático), y se actualiza en segundo plano.
func TestVacantesNoEsperanAlNavegador(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	falsa := &ofertasFalsas{}
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return falsa }
	ctrl.numCuentas = 1
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	ctrl.guardarOfertasEnCache([]ofertaConConteo{{OfertaLaboral: modelos.OfertaLaboral{Titulo: "Operador vieja", ID: "VIEJA0000001", Cuenta: 1}}})
	ctrl.muCache.Lock()
	ctrl.cacheOfertasEn = time.Now().Add(-time.Hour)
	ctrl.muCache.Unlock()

	ctrl.semCT <- struct{}{} // otro tiene el navegador
	listo := make(chan *httptest.ResponseRecorder, 1)
	go func() { listo <- pedir(h, "GET", "/api/computrabajo/vacantes", nil) }()
	select {
	case rr := <-listo:
		if rr.Header().Get("X-Vacantes-Fuente") != "cache-actualizando" || !strings.Contains(rr.Body.String(), "VIEJA0000001") {
			t.Errorf("fuente %q: %s", rr.Header().Get("X-Vacantes-Fuente"), rr.Body.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("la lista esperó al navegador")
	}
	if ctrl.refrescarVacantes() {
		t.Error("solo debe haber un refresco a la vez")
	}
	<-ctrl.semCT // se libera el navegador: el refresco sigue
	ctrl.refrescos.Wait()
	if lista, _ := ctrl.ofertasEnCache(); !tieneConteos(lista) {
		t.Errorf("el refresco debía dejar la lista con conteos: %+v", lista)
	}

	// La lista sobrevive a un reinicio, con la cuenta de cada vacante.
	otro := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	if lista, en := otro.ofertasEnCache(); len(lista) != 2 || en.IsZero() {
		t.Errorf("lista recuperada del disco: %+v", lista)
	}
	ctrl.guardarOfertasEnCache([]ofertaConConteo{{OfertaLaboral: modelos.OfertaLaboral{Titulo: "Operador", ID: "CUENTA000002", Cuenta: 1}}})
	if lista, _ := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "").ofertasEnCache(); len(lista) != 1 || lista[0].Cuenta != 1 {
		t.Errorf("la cuenta debe sobrevivir al reinicio: %+v", lista)
	}
	ctrl.Detener()
}

// Solo se listan (y se cuentan) las vacantes de conductor u operador: las demás no salen.
func TestVacantesSoloConductorYOperador(t *testing.T) {
	cfg, _ := montarDescargas(t)
	cfg.UsuarioComputrabajo, cfg.ClaveComputrabajo = "u", "p"
	cfg.OfertasIncluidas = []string{"conductor", "operador"}
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.fabricaOfertas = func(int) FuenteOfertas { return &ofertasFalsas{} }
	ctrl.numCuentas = 1
	t.Cleanup(ctrl.Detener) // espera el refresco de conteos en segundo plano
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	var vacantes []VacanteComputrabajo
	rr := pedir(h, "GET", "/api/computrabajo/vacantes", nil)
	if err := json.Unmarshal(rr.Body.Bytes(), &vacantes); err != nil || len(vacantes) != 1 || !strings.Contains(vacantes[0].Titulo, "Operador") {
		t.Fatalf("solo debía quedar la de operador: %s", rr.Body.String())
	}
	if !cfg.OfertaIncluida("Conductor(a) de buses alimentadores") || cfg.OfertaIncluida("Analista de desarrollo organizacional") {
		t.Error("el filtro por título no funciona")
	}
}

// Con el frontend compilado, el servidor entrega la aplicación (y sus rutas) sin tapar la API.
func TestServidorEntregaElFrontendCompilado(t *testing.T) {
	cfg, _ := montarDescargas(t)
	dist := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>app</html>"), 0o644)
	_ = os.WriteFile(filepath.Join(dist, "assets", "app-123.js"), []byte("console.log(1)"), 0o644)
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	h := (&ServidorWeb{controlador: ctrl, frontend: dist}).rutas()

	for _, ruta := range []string{"/", "/vacantes/abc/candidatos/x"} {
		if rr := pedir(h, "GET", ruta, nil); rr.Code != 200 || !strings.Contains(rr.Body.String(), "app") {
			t.Errorf("%s: %d %s", ruta, rr.Code, rr.Body.String())
		}
	}
	rr := pedir(h, "GET", "/assets/app-123.js", nil)
	if rr.Code != 200 || !strings.Contains(rr.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %v", rr.Code, rr.Header())
	}
	if rr := pedir(h, "GET", "/api/no-existe", nil); rr.Code != 404 || !strings.Contains(rr.Header().Get("Content-Type"), "json") {
		t.Errorf("api desconocida: %d", rr.Code)
	}
	if rr := pedir(h, "GET", "/api/salud", nil); rr.Code != 200 {
		t.Errorf("la API sigue respondiendo: %d", rr.Code)
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(dist), "secreto.txt"), []byte("CLAVE-PRIVADA"), 0o644)
	for _, ruta := range []string{"/../secreto.txt", "/assets/../../secreto.txt", "/%2e%2e/secreto.txt"} {
		if rr := pedir(h, "GET", ruta, nil); strings.Contains(rr.Body.String(), "CLAVE-PRIVADA") {
			t.Errorf("%s: no debe salir de la carpeta del frontend", ruta)
		}
	}
}

// Cada conteo de inscritos llega a la lista guardada en cuanto se lee, sin esperar a las demás vacantes.
func TestConteoParcialSeAnotaEnLaLista(t *testing.T) {
	cfg, _ := montarDescargas(t)
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.guardarOfertasEnCache([]ofertaConConteo{
		{OfertaLaboral: modelos.OfertaLaboral{ID: "AAA111", Titulo: "Operador"}},
		{OfertaLaboral: modelos.OfertaLaboral{ID: "BBB222", Titulo: "Conductor"}},
	})
	antes, _ := ctrl.ofertasEnCache()
	avisos, desuscribir := ctrl.cambios.suscribir()
	defer desuscribir()

	ctrl.anotarConteo("aaa111", 12)
	ctrl.anotarConteo("BBB222", 5) // dentro del intervalo: se anota pero no se vuelve a avisar

	lista, _ := ctrl.ofertasEnCache()
	if lista[0].Total == nil || *lista[0].Total != 12 || lista[1].Total == nil || *lista[1].Total != 5 {
		t.Errorf("conteos anotados: %+v", lista)
	}
	if antes[0].Total != nil {
		t.Error("la lista que ya se había leído no debe cambiar por debajo")
	}
	if n := len(avisos); n != 1 {
		t.Errorf("avisos de conteo parcial = %d; quería 1", n)
	}
}
