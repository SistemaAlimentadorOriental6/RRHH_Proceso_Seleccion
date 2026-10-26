package servidor

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
	"go-rrhh/notificaciones"
	"go-rrhh/utilidades"
)

type notificadorFalso struct {
	enviados []notificaciones.Mensaje
	err      error
}

func (n *notificadorFalso) Enviar(_ context.Context, m notificaciones.Mensaje) error {
	n.enviados = append(n.enviados, m)
	return n.err
}

// enviosFalsos hace de base de datos para el envío: entrega un token distinto en cada llamada,
// como el real, y recuerda qué se le pidió.
type enviosFalsos struct {
	mu         sync.Mutex
	preparados []string // postulaciones para las que se preparó un envío
	vigencias  []time.Duration
	cualquiera []bool
	marcas     [][]almacen.ResultadoEnvio
	contacto   almacen.Contacto
	err        error
	pendientes []string

	estado                       almacen.EstadoSolicitudDatos
	previoCorreo, previoWhatsapp bool // canales por los que ya se había enviado
}

func (e *enviosFalsos) GuardarContacto(context.Context, string, string, string) error { return nil }

func (e *enviosFalsos) EstadoSolicitud(context.Context, string) (almacen.EstadoSolicitudDatos, error) {
	return e.estado, nil
}

func (e *enviosFalsos) EnvioPrevio(context.Context, string) (bool, bool, error) {
	return e.previoCorreo, e.previoWhatsapp, nil
}

func (e *enviosFalsos) PrepararEnvio(_ context.Context, id string, vigencia time.Duration, cualquierEtapa bool) (almacen.Envio, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return almacen.Envio{}, e.err
	}
	e.preparados = append(e.preparados, id)
	e.vigencias = append(e.vigencias, vigencia)
	e.cualquiera = append(e.cualquiera, cualquierEtapa)
	token := "tokenDePrueba" + strings.Repeat("x", 20) + string(rune('A'+len(e.preparados)))
	return almacen.Envio{
		SolicitudID: "sol-" + token, PostulacionID: id, Token: token,
		Nombre: "Daniel Ochoa Castrillón", Cargo: "Operador de buses alimentadores - Metro de Medellín",
		Contacto: e.contacto, VenceEn: time.Now().Add(vigencia),
	}, nil
}

func (e *enviosFalsos) MarcarEnvio(_ context.Context, _ string, r []almacen.ResultadoEnvio) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.marcas = append(e.marcas, r)
	return nil
}

func (e *enviosFalsos) EnviosPendientes(context.Context) ([]string, error) { return e.pendientes, nil }

const postulacionPrueba = "0190a8b0-1234-7abc-8def-0123456789ab"

func montarEnvio(t *testing.T) (*ControladorRRHH, *enviosFalsos, *notificadorFalso, *notificadorFalso, func(metodo, ruta string) *httptest.ResponseRecorder) {
	t.Helper()
	cfg, _ := montarDescargas(t)
	cfg.URLPlataformaValidacion = "https://plataforma.test/validar/"
	cfg.WhatsAppPlantilla = "plantilla_test"
	cfg.SolicitudVigenciaDias = 7
	cfg.SolicitudSoloPrueba = true
	rutas, _ := filepath.Glob(filepath.Join(cfg.DirectorioDescargas, "*", "candidatos.json"))
	if len(rutas) != 1 {
		t.Fatalf("candidatos.json esperado 1, hay %d", len(rutas))
	}
	if err := utilidades.EscribirJSONAtomico(rutas[0], []modelos.Candidato{
		// Daniel está en la base (tiene postulación): se reconoce como persona de prueba por su nombre o cédula.
		{Nombre: "Daniel Ochoa Castrillón", Cedula: "1035126774", PostulacionID: postulacionPrueba, TextoPerfil: "contacto: ConCorreo@Mail.com cel 311 234 5678"},
		{Nombre: "Otro Nombre", Cedula: "1.035.126.774", TextoPerfil: "solo experiencia"}, // de prueba por cédula, aún no está en la base
		{Nombre: "Otra Persona", Cedula: "2", PostulacionID: "0190a8b0-1234-7abc-8def-0123456789ac", TextoPerfil: "contacto: otra@mail.com"},
	}, false); err != nil {
		t.Fatal(err)
	}

	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	t.Cleanup(ctrl.Detener)
	correo, wa := &notificadorFalso{}, &notificadorFalso{}
	ctrl.notificador, ctrl.whatsapp = correo, wa
	envios := &enviosFalsos{contacto: almacen.Contacto{Correo: "concorreo@mail.com", Telefono: "3112345678"}}
	ctrl.envios = envios
	ctrl.cacheOfertas = []ofertaConConteo{{OfertaLaboral: modelos.OfertaLaboral{
		ID: idOfertaPrueba, Titulo: "Operador de buses alimentadores - Metro de Medellín",
	}}}
	h := (&ServidorWeb{controlador: ctrl}).rutas()
	llamar := func(metodo, ruta string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(metodo, "/api/computrabajo/vacantes/"+idOfertaPrueba+"/candidatos/"+ruta, nil))
		return rr
	}
	return ctrl, envios, correo, wa, llamar
}

func TestContactoDelCandidatoSeDetectaSinEnviarNada(t *testing.T) {
	_, envios, correo, _, llamar := montarEnvio(t)
	rr := llamar("GET", "1035126774/contacto")
	if rr.Code != 200 || len(correo.enviados) != 0 || len(envios.preparados) != 0 ||
		!strings.Contains(rr.Body.String(), "concorreo@mail.com") || !strings.Contains(rr.Body.String(), "3112345678") {
		t.Fatalf("contacto: %d %s", rr.Code, rr.Body.String())
	}
}

// El enlace lleva solo el token: ni el nombre, ni el correo, ni el cargo viajan en la URL.
func TestSolicitarDatosEnviaUnEnlaceConTokenSinDatosPersonales(t *testing.T) {
	_, envios, correo, wa, llamar := montarEnvio(t)

	rr := llamar("POST", "daniel-ochoa-castrillon/solicitar-datos")
	if rr.Code != 200 {
		t.Fatalf("solicitar: %d %s", rr.Code, rr.Body.String())
	}
	if len(envios.preparados) != 1 || envios.preparados[0] != postulacionPrueba || envios.vigencias[0] != 7*24*time.Hour || !envios.cualquiera[0] {
		t.Fatalf("PrepararEnvio: %+v %+v %+v", envios.preparados, envios.vigencias, envios.cualquiera)
	}

	enlace := "https://plataforma.test/validar/validar-datos/" + "tokenDePrueba" + strings.Repeat("x", 20) + "B"
	if len(correo.enviados) != 1 || correo.enviados[0].Destino != "concorreo@mail.com" || !strings.Contains(correo.enviados[0].Cuerpo, enlace) {
		t.Fatalf("correo: %+v", correo.enviados)
	}
	if len(wa.enviados) != 1 || wa.enviados[0].Destino != "3112345678" || wa.enviados[0].Plantilla != "plantilla_test" ||
		wa.enviados[0].Variables["url"] != enlace || wa.enviados[0].Variables["nombre_minimo"] != "Daniel" {
		t.Fatalf("whatsapp: %+v", wa.enviados)
	}
	// La URL es solo la ruta y el token: sin parámetros y sin el nombre, el correo ni el cargo.
	urlWhatsapp := wa.enviados[0].Variables["url"]
	if urlWhatsapp != enlace || strings.ContainsAny(urlWhatsapp, "?&=") {
		t.Errorf("la URL debe ser solo la ruta con el token, fue %q", urlWhatsapp)
	}
	for _, dato := range []string{"Ochoa", "Daniel", "concorreo", "Metro"} {
		if strings.Contains(urlWhatsapp, dato) {
			t.Errorf("la URL no debe llevar %q: %s", dato, urlWhatsapp)
		}
	}

	if len(envios.marcas) != 1 || len(envios.marcas[0]) != 2 || !envios.marcas[0][0].OK || !envios.marcas[0][1].OK {
		t.Errorf("resultado de cada canal: %+v", envios.marcas)
	}
}

// Abrir el flujo otra vez no repite el mensaje; solo un reenvío pedido a propósito genera un enlace nuevo.
func TestNoSeVuelveAEnviarLoYaEnviado(t *testing.T) {
	_, envios, correo, wa, llamar := montarEnvio(t)
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 200 {
		t.Fatalf("primer envío: %d %s", rr.Code, rr.Body.String())
	}
	envios.previoCorreo, envios.previoWhatsapp = true, true // la base ya registra el envío

	rr := llamar("POST", "1035126774/solicitar-datos") // el usuario cierra y vuelve a entrar al flujo
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "ya_enviado") || !strings.Contains(rr.Body.String(), "whatsapp_ya_enviado") {
		t.Fatalf("segunda llamada: %d %s", rr.Code, rr.Body.String())
	}
	if len(correo.enviados) != 1 || len(wa.enviados) != 1 || len(envios.preparados) != 1 {
		t.Fatalf("no debía repetirse: %d correos, %d whatsapp, %d preparados", len(correo.enviados), len(wa.enviados), len(envios.preparados))
	}

	if rr := llamar("POST", "1035126774/solicitar-datos?reenviar=1"); rr.Code != 200 {
		t.Fatalf("reenvío: %d %s", rr.Code, rr.Body.String())
	}
	if len(correo.enviados) != 2 || correo.enviados[0].Cuerpo == correo.enviados[1].Cuerpo {
		t.Fatalf("el reenvío debe salir con un enlace nuevo: %d correos", len(correo.enviados))
	}
}

func TestSolicitarDatosErrores(t *testing.T) {
	ctrl, envios, correo, wa, llamar := montarEnvio(t)

	// Solo las personas de prueba, mientras el modo de prueba esté activo.
	if rr := llamar("POST", "otra-persona/solicitar-datos"); rr.Code != 403 || len(envios.preparados) != 0 {
		t.Errorf("persona no de prueba: %d %s", rr.Code, rr.Body.String())
	}
	// Una persona de prueba que todavía no está guardada en la base.
	if rr := llamar("POST", "otro-nombre/solicitar-datos"); rr.Code != 409 || len(envios.preparados) != 0 {
		t.Errorf("sin postulación en la base: %d %s", rr.Code, rr.Body.String())
	}

	envios.err = almacen.ErrSinContacto
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 422 || len(correo.enviados) != 0 {
		t.Errorf("sin contacto: %d %s", rr.Code, rr.Body.String())
	}
	envios.err = almacen.ErrEstadoNoPermite
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 409 {
		t.Errorf("fuera de pausa: %d %s", rr.Code, rr.Body.String())
	}
	envios.err = nil

	// Ningún canal funciona: error, pero el intento queda registrado.
	correo.err, wa.err = notificaciones.ErrNoConfigurado, notificaciones.ErrNoConfigurado
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 503 {
		t.Errorf("canales sin configurar: %d %s", rr.Code, rr.Body.String())
	}
	correo.err, wa.err = nil, nil

	// Sin la URL de la plataforma no se genera ningún token.
	antes := len(envios.preparados)
	ctrl.cfg.URLPlataformaValidacion = ""
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 500 && rr.Code != 503 || len(envios.preparados) != antes {
		t.Errorf("sin URL de plataforma: %d %s (preparados %d, antes %d)", rr.Code, rr.Body.String(), len(envios.preparados), antes)
	}
	ctrl.cfg.URLPlataformaValidacion = "https://plataforma.test/validar/"

	// Sin base de datos el envío no está disponible.
	ctrl.envios = nil
	if rr := llamar("POST", "1035126774/solicitar-datos"); rr.Code != 503 {
		t.Errorf("sin base de datos: %d %s", rr.Code, rr.Body.String())
	}
}

// Fuera del modo de prueba, cualquier candidato en pausa puede recibir la solicitud.
func TestSolicitarDatosFueraDelModoPrueba(t *testing.T) {
	ctrl, envios, correo, _, llamar := montarEnvio(t)
	ctrl.cfg.SolicitudSoloPrueba = false
	if rr := llamar("POST", "otra-persona/solicitar-datos"); rr.Code != 200 || len(correo.enviados) != 1 {
		t.Fatalf("persona cualquiera: %d %s", rr.Code, rr.Body.String())
	}
	if envios.cualquiera[0] {
		t.Error("fuera del modo de prueba solo se debe pedir a quien está en pausa (cualquierEtapa=false)")
	}
}
