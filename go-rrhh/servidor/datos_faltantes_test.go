package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-rrhh/almacen"
)

// solicitudesFalsas hace de base de datos para la página pública del candidato.
type solicitudesFalsas struct {
	mu           sync.Mutex
	info         almacen.InfoSolicitud
	errAbrir     error
	errCompletar error
	completadas  []almacen.DatosCandidato
	tokens       []string
	fallos       int
}

func (s *solicitudesFalsas) AbrirSolicitud(_ context.Context, token string) (almacen.InfoSolicitud, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = append(s.tokens, token)
	return s.info, s.errAbrir
}

func (s *solicitudesFalsas) CompletarSolicitud(_ context.Context, token string, d almacen.DatosCandidato) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = append(s.tokens, token)
	if s.errCompletar == nil {
		s.completadas = append(s.completadas, d)
	}
	return s.errCompletar
}

func (s *solicitudesFalsas) RegistrarFallo(context.Context, string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fallos++
	return nil
}

func (s *solicitudesFalsas) PseudonimoIP(ip string) string { return "huella-de-" + ip }

const tokenPrueba = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCde"

func montarPaginaPublica(t *testing.T) (*ControladorRRHH, *solicitudesFalsas, http.Handler) {
	t.Helper()
	cfg, _ := montarDescargas(t)
	cfg.PoliticaVersion = "politica-v1"
	cfg.EdadMinima = 18
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	t.Cleanup(ctrl.Detener)
	falsas := &solicitudesFalsas{info: almacen.InfoSolicitud{
		Nombre: "Daniel Ochoa Castrillón", Correo: "da***@gmail.com",
		Cargo: "Operador(a) de buses alimentadores - Metro de Medellín", VenceEn: time.Now().Add(72 * time.Hour),
	}}
	ctrl.solicitudes = falsas
	return ctrl, falsas, (&ServidorWeb{controlador: ctrl}).rutas()
}

func enviarJSON(h http.Handler, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, ruta, bytes.NewBufferString(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func leerVista(t *testing.T, rr *httptest.ResponseRecorder) vistaSolicitud {
	t.Helper()
	var v vistaSolicitud
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("respuesta no es JSON: %v: %s", err, rr.Body.String())
	}
	return v
}

func TestPaginaPublicaMuestraLoQueSabemosDelCandidato(t *testing.T) {
	_, falsas, h := montarPaginaPublica(t)
	rr := pedir(h, "GET", "/api/validar-datos/"+tokenPrueba, nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	v := leerVista(t, rr)
	if v.Estado != "pendiente" || v.Nombre != "Daniel Ochoa Castrillón" || v.Correo != "da***@gmail.com" ||
		!strings.HasPrefix(v.Cargo, "Operador(a)") || v.PoliticaVersion != "politica-v1" || v.EdadMinima != 18 || v.VenceEn == nil {
		t.Errorf("vista: %+v", v)
	}
	if len(falsas.tokens) != 1 || falsas.tokens[0] != tokenPrueba {
		t.Errorf("el token del enlace debe llegar tal cual a la base: %v", falsas.tokens)
	}
	if rr.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rr.Header().Get("X-Robots-Tag"), "noindex") ||
		rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("la respuesta debe ser privada: %v", rr.Header())
	}
	if strings.Contains(rr.Body.String(), "gmail.com") && !strings.Contains(rr.Body.String(), "da***@gmail.com") {
		t.Error("el correo debe mostrarse enmascarado")
	}
}

func TestPaginaPublicaSegunElEstadoDelEnlace(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		codigo int
		estado string
	}{
		{"completada", almacen.ErrYaCompletada, 200, "completada"},
		{"vencida", almacen.ErrEnlaceVencido, 410, "vencida"},
		{"inválida", almacen.ErrEnlaceInvalido, 404, "invalida"},
		{"falla de la base", errors.New("base caída: detalle interno"), 500, "invalida"},
	}
	for _, c := range casos {
		_, falsas, h := montarPaginaPublica(t)
		falsas.errAbrir = c.err
		rr := pedir(h, "GET", "/api/validar-datos/"+tokenPrueba, nil)
		if v := leerVista(t, rr); rr.Code != c.codigo || v.Estado != c.estado {
			t.Errorf("%s: %d %+v", c.nombre, rr.Code, v)
		}
		if strings.Contains(rr.Body.String(), "detalle interno") {
			t.Errorf("%s: un error interno no debe llegar al candidato", c.nombre)
		}
	}
}

func TestPaginaPublicaSinBaseDeDatos(t *testing.T) {
	ctrl, _, h := montarPaginaPublica(t)
	ctrl.solicitudes = nil
	if rr := pedir(h, "GET", "/api/validar-datos/"+tokenPrueba, nil); rr.Code != 503 {
		t.Errorf("GET sin base: %d", rr.Code)
	}
	if rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba, `{}`); rr.Code != 503 {
		t.Errorf("POST sin base: %d", rr.Code)
	}
}

func TestEnviarDatosGuardaYAvisaAlTrabajador(t *testing.T) {
	ctrl, falsas, h := montarPaginaPublica(t)
	rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba,
		`{"cedula":"1.035.126.774","fecha_nacimiento":"1990-05-17","autoriza":true,"politica_version":"politica-v1"}`)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	var r respuestaEnvioDatos
	_ = json.Unmarshal(rr.Body.Bytes(), &r)
	if !r.OK || r.Estado != "completada" {
		t.Errorf("respuesta: %+v", r)
	}

	if len(falsas.completadas) != 1 {
		t.Fatalf("se guardaron %d solicitudes; quería 1", len(falsas.completadas))
	}
	d := falsas.completadas[0]
	if d.Cedula != "1035126774" || d.FechaNacimiento.Format("2006-01-02") != "1990-05-17" || d.Edad < 35 ||
		d.PoliticaVersion != "politica-v1" || d.IPHuella == "" || strings.Contains(d.IPHuella, "1035126774") {
		t.Errorf("datos guardados: %+v", d)
	}
	select {
	case <-ctrl.despertar:
	default:
		t.Error("al guardar debe despertarse al trabajador para reanudar el análisis")
	}
}

func TestEnviarDatosRechazaLoInvalido(t *testing.T) {
	casos := []struct {
		nombre, cuerpo, campo string
	}{
		{"cédula corta", `{"cedula":"123","fecha_nacimiento":"1990-05-17","autoriza":true,"politica_version":"politica-v1"}`, "cedula"},
		{"menor de edad", `{"cedula":"1035126774","fecha_nacimiento":"2015-01-01","autoriza":true,"politica_version":"politica-v1"}`, "fecha_nacimiento"},
		{"fecha futura", `{"cedula":"1035126774","fecha_nacimiento":"2999-01-01","autoriza":true,"politica_version":"politica-v1"}`, "fecha_nacimiento"},
		{"sin autorización", `{"cedula":"1035126774","fecha_nacimiento":"1990-05-17","autoriza":false,"politica_version":"politica-v1"}`, "autoriza"},
		{"política distinta", `{"cedula":"1035126774","fecha_nacimiento":"1990-05-17","autoriza":true,"politica_version":"vieja"}`, "autoriza"},
	}
	for _, c := range casos {
		_, falsas, h := montarPaginaPublica(t)
		rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba, c.cuerpo)
		var r respuestaEnvioDatos
		_ = json.Unmarshal(rr.Body.Bytes(), &r)
		if rr.Code != http.StatusUnprocessableEntity || r.Campos[c.campo] == "" {
			t.Errorf("%s: %d %+v", c.nombre, rr.Code, r)
		}
		if len(falsas.completadas) != 0 {
			t.Errorf("%s: no debía guardarse nada", c.nombre)
		}
		if falsas.fallos != 1 {
			t.Errorf("%s: el intento rechazado debe contarse contra el enlace (fallos=%d)", c.nombre, falsas.fallos)
		}
	}
}

func TestEnviarDatosMenorDeEdadNoSeGuarda(t *testing.T) {
	_, falsas, h := montarPaginaPublica(t)
	hoy := time.Now().UTC()
	diecisiete := hoy.AddDate(-17, 0, 0).Format("2006-01-02")
	rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba,
		`{"cedula":"1035126774","fecha_nacimiento":"`+diecisiete+`","autoriza":true,"politica_version":"politica-v1"}`)
	if rr.Code != http.StatusUnprocessableEntity || len(falsas.completadas) != 0 {
		t.Fatalf("un menor de 18 no debe pasar ni guardarse: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "18") {
		t.Errorf("el mensaje debe decir la edad mínima: %s", rr.Body.String())
	}
	diesiocho := hoy.AddDate(-18, 0, 0).Format("2006-01-02")
	rr = enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba,
		`{"cedula":"1035126774","fecha_nacimiento":"`+diesiocho+`","autoriza":true,"politica_version":"politica-v1"}`)
	if rr.Code != 200 || len(falsas.completadas) != 1 {
		t.Errorf("quien cumple 18 hoy sí pasa: %d %s", rr.Code, rr.Body.String())
	}
}

func TestEnviarDatosEstadosDelEnlace(t *testing.T) {
	cuerpo := `{"cedula":"1035126774","fecha_nacimiento":"1990-05-17","autoriza":true,"politica_version":"politica-v1"}`
	casos := []struct {
		err    error
		codigo int
		estado string
	}{
		{almacen.ErrYaCompletada, 200, "completada"}, // doble clic: no es un error para el candidato
		{almacen.ErrEnlaceVencido, 410, "vencida"},
		{almacen.ErrEnlaceInvalido, 404, "invalida"},
		{errors.New("detalle interno"), 500, ""},
	}
	for _, c := range casos {
		_, falsas, h := montarPaginaPublica(t)
		falsas.errCompletar = c.err
		rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba, cuerpo)
		var r respuestaEnvioDatos
		_ = json.Unmarshal(rr.Body.Bytes(), &r)
		if rr.Code != c.codigo || r.Estado != c.estado {
			t.Errorf("%v: %d %+v", c.err, rr.Code, r)
		}
		if strings.Contains(rr.Body.String(), "detalle interno") {
			t.Error("un error interno no debe llegar al candidato")
		}
	}
}

func TestEnviarDatosCuerpoMalformadoOExcesivo(t *testing.T) {
	_, falsas, h := montarPaginaPublica(t)
	for nombre, cuerpo := range map[string]string{
		"no es JSON":        `esto no es json`,
		"campo desconocido": `{"cedula":"1035126774","fecha_nacimiento":"1990-05-17","autoriza":true,"politica_version":"politica-v1","admin":true}`,
		"demasiado grande":  `{"cedula":"` + strings.Repeat("1", 10000) + `"}`,
		"vacío":             ``,
	} {
		if rr := enviarJSON(h, "POST", "/api/validar-datos/"+tokenPrueba, cuerpo); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: código %d; quería 400", nombre, rr.Code)
		}
	}
	if len(falsas.completadas) != 0 {
		t.Error("ninguna de esas peticiones debía guardar nada")
	}
}

func TestRutasPublicasLimitanLasPeticionesPorDireccion(t *testing.T) {
	ctrl, _, h := montarPaginaPublica(t)
	ctrl.limite = nuevoLimitadorIP(3, time.Minute)
	codigos := []int{}
	for i := 0; i < 5; i++ {
		codigos = append(codigos, pedir(h, "GET", "/api/validar-datos/"+tokenPrueba, nil).Code)
	}
	if codigos[0] != 200 || codigos[2] != 200 || codigos[3] != 429 || codigos[4] != 429 {
		t.Errorf("códigos: %v; quería 200,200,200,429,429", codigos)
	}
}

func TestLimitadorLiberaLaVentana(t *testing.T) {
	l := nuevoLimitadorIP(2, 20*time.Millisecond)
	if !l.permitir("a") || !l.permitir("a") || l.permitir("a") {
		t.Fatal("debe permitir 2 y bloquear la tercera dentro de la ventana")
	}
	if !l.permitir("b") {
		t.Error("otra dirección tiene su propio cupo")
	}
	time.Sleep(30 * time.Millisecond)
	if !l.permitir("a") {
		t.Error("pasada la ventana debe volver a permitir")
	}
}

// El token es una credencial: no puede quedar en el log de peticiones.
func TestElLogDePeticionesNoLlevaElToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/validar-datos/"+tokenPrueba+"?x=1", nil)
	if got := rutaParaLog(r); strings.Contains(got, tokenPrueba) || got != "/api/validar-datos/<token>" {
		t.Errorf("rutaParaLog = %q", got)
	}
	otra := httptest.NewRequest("GET", "/api/vacantes?filtro=aptos", nil)
	if got := rutaParaLog(otra); got != "/api/vacantes?filtro=aptos" {
		t.Errorf("las demás rutas se registran completas, fue %q", got)
	}
}

func TestFormularioPublicoPuedeLlamarDesdeOtroOrigen(t *testing.T) {
	_, _, h := montarPaginaPublica(t)
	req := httptest.NewRequest("OPTIONS", "/api/validar-datos/"+tokenPrueba, nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		!strings.Contains(rr.Header().Get("Access-Control-Allow-Methods"), "POST") ||
		!strings.Contains(rr.Header().Get("Access-Control-Allow-Headers"), "Content-Type") {
		t.Errorf("preflight: %d %v", rr.Code, rr.Header())
	}
}
