package agentes

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

var (
	reSimitComparendos = regexp.MustCompile(`(?i)Comparendos:\s*(\d+)`)
	reSimitMultas      = regexp.MustCompile(`(?i)Multas:\s*(\d+)`)
	reSimitAcuerdos    = regexp.MustCompile(`(?i)Acuerdos de pago:\s*(\d+)`)
	reSimitTotal       = regexp.MustCompile(`(?i)Total\s*(?:\(\d+\))?:?\s*\$?\s*([\d.,]+)`)
)

// AgenteSimit valida comparendos, multas y acuerdos de pago en la Federacion Colombiana de Municipios.
type AgenteSimit struct {
	cfg         *configuracion.Configuracion
	allocCtx    context.Context
	cancelAlloc context.CancelFunc

	// Cortacircuito: si el portal no abre (caído o sin ruta de red) no se insiste con cada candidato.
	muCaido     sync.Mutex
	caidoHasta  time.Time
	caidoMotivo string
}

const (
	// esperaPortalSimitCaido es cuánto se evita volver a intentar tras detectar el portal inaccesible.
	esperaPortalSimitCaido = 90 * time.Second
	// plazoAbrirSimit acota la carga inicial del portal (antes ~47 s por intento; el portal suele estar lento si no había conexión).
	plazoAbrirSimit = 60 * time.Second
)

// NuevoAgenteSimit inicializa el agente SIMIT con soporte headless Chrome.
func NuevoAgenteSimit(cfg *configuracion.Configuracion) *AgenteSimit {
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opcionesChrome(true)...)
	return &AgenteSimit{
		cfg:         cfg,
		allocCtx:    allocCtx,
		cancelAlloc: cancel,
	}
}

func (a *AgenteSimit) Nombre() string {
	return "SIMIT"
}

// Cerrar libera los recursos del navegador.
func (a *AgenteSimit) Cerrar() {
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
}

// Validar implementa la interfaz AgenteValidador.
func (a *AgenteSimit) Validar(ctx context.Context, candidato *modelos.Candidato) error {
	cedulaLimpia := soloDigitos(candidato.Cedula)
	if cedulaLimpia == "" || candidato.Cedula == "No especificada" {
		candidato.ResultadoSimit = &modelos.ResultadoSimit{
			Estado:     "NO_APLICA",
			TotalDeuda: "$ 0",
			Detalles:   "Cédula no especificada para consulta en SIMIT.",
		}
		return nil
	}

	var captura []byte
	resultado, err := a.consultarCedula(ctx, cedulaLimpia, &captura)
	if err != nil {
		candidato.ResultadoSimit = &modelos.ResultadoSimit{
			Estado:     "ERROR_CONSULTA",
			TotalDeuda: "$ 0",
			Alerta:     true,
			Detalles:   fmt.Sprintf("Error consultando SIMIT: %v", err),
		}
		return nil
	}

	candidato.ResultadoSimit = resultado
	GuardarCaptura(candidato, "simit", captura)
	return nil
}

// scriptSimitIngresar escribe la cédula en el input Angular usando el setter nativo para que el
// framework detecte el cambio.
func scriptSimitIngresar(cedula string) string {
	return fmt.Sprintf(`
		(() => {
			const inp = document.querySelector('#txtBusqueda');
			if (!inp) return false;
			inp.focus();
			const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
			nativeSetter.call(inp, %s);
			inp.dispatchEvent(new Event('input', { bubbles: true }));
			inp.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		})()
	`, literalJS(cedula))
}

const (
	scriptSimitFormularioListo = `
		(() => {
			const inp = document.querySelector('#txtBusqueda');
			const btn = document.querySelector('#btnNumDocPlaca');
			return !!(inp && btn && !inp.disabled);
		})()
	`

	scriptSimitRespuestaLista = `
		(() => {
			const b = document.body ? document.body.innerText : '';
			return b.includes("pendientes de pago") ||
				b.includes("Paz y Salvo") ||
				b.includes("paz y salvo") ||
				b.includes("Comparendos:") ||
				b.includes("Multas:") ||
				b.includes("Acuerdos de pago:") ||
				b.includes("No se encontraron") ||
				b.includes("no posee") ||
				b.includes("Cédula:");
		})()
	`
)

// ConsultarCedula ejecuta la consulta en la aplicación Angular del SIMIT, con hasta 3 intentos.
func (a *AgenteSimit) ConsultarCedula(ctx context.Context, cedula string) (*modelos.ResultadoSimit, error) {
	return a.consultarCedula(ctx, cedula, nil)
}

// consultarCedula es ConsultarCedula con salida opcional de la captura de pantalla del resultado.
func (a *AgenteSimit) consultarCedula(ctx context.Context, cedula string, captura *[]byte) (*modelos.ResultadoSimit, error) {
	cedulaLimpia := soloDigitos(cedula)
	if cedulaLimpia == "" {
		return &modelos.ResultadoSimit{
			Estado:   "SIN_CEDULA",
			Detalles: "Número de cédula inválido.",
		}, nil
	}

	if motivo, restante := a.portalCaido(); restante > 0 {
		return nil, fmt.Errorf("portal SIMIT no disponible (%s); se reintentará en %s", motivo, restante.Round(time.Second))
	}

	var resultado *modelos.ResultadoSimit
	err := reintentar(ctx, 3, 2*time.Second, func(int) error {
		if captura != nil {
			*captura = nil
		}
		texto, err := a.leerPortal(ctx, cedulaLimpia, captura)
		if err != nil {
			if esErrorConexion(err) {
				a.marcarPortalCaido(err)
				return permanente(err) // sin ruta al portal: reintentar solo gasta tiempo
			}
			return err
		}
		res, ok := interpretarRespuestaSimit(texto)
		if !ok {
			return fmt.Errorf("respuesta de SIMIT no reconocida")
		}
		resultado = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resultado, nil
}

func (a *AgenteSimit) portalCaido() (string, time.Duration) {
	a.muCaido.Lock()
	defer a.muCaido.Unlock()
	return a.caidoMotivo, time.Until(a.caidoHasta)
}

func (a *AgenteSimit) marcarPortalCaido(err error) {
	a.muCaido.Lock()
	defer a.muCaido.Unlock()
	a.caidoHasta = time.Now().Add(esperaPortalSimitCaido)
	a.caidoMotivo = err.Error()
}

// esErrorConexion detecta fallos de red al abrir el portal (no responde, DNS, sin internet).
func esErrorConexion(err error) bool {
	t := err.Error()
	return strings.Contains(t, "net::ERR_CONNECTION") || strings.Contains(t, "net::ERR_NAME") ||
		strings.Contains(t, "net::ERR_INTERNET") || strings.Contains(t, "net::ERR_TIMED_OUT") ||
		strings.Contains(t, "net::ERR_ADDRESS") || strings.Contains(t, "net::ERR_NETWORK") ||
		strings.Contains(t, "net::ERR_PROXY") || strings.Contains(t, "net::ERR_SSL")
}

// leerPortal carga SIMIT, lanza la consulta y devuelve el texto de la página cuando la respuesta
// terminó de renderizar. Devuelve error si el portal no respondió (antes se tomaba como "paz y salvo").
func (a *AgenteSimit) leerPortal(ctx context.Context, cedula string, captura *[]byte) (string, error) {
	taskCtx, cancelTask := chromedp.NewContext(a.allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	defer cancelTask()
	detener := context.AfterFunc(ctx, cancelTask)
	defer detener()

	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, 150*time.Second)
	defer cancelTimeout()

	// La pestaña se inicializa con el contexto de la tarea, no con uno con plazo: al cancelarse este último
	// la pestaña queda inutilizable y todo lo que sigue (formulario, consulta, captura) se cuelga.
	if err := chromedp.Run(taskCtx); err != nil {
		return "", fmt.Errorf("error al abrir el portal SIMIT: %w", err)
	}

	abrirCtx, cancelAbrir := context.WithTimeout(taskCtx, plazoAbrirSimit)
	err := chromedp.Run(abrirCtx,
		chromedp.Navigate(a.cfg.URLSimit),
		chromedp.WaitVisible("#txtBusqueda", chromedp.ByID),
	)
	cancelAbrir()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return "", fmt.Errorf("error al abrir el portal SIMIT: net::ERR_TIMED_OUT (no cargó en %s)", plazoAbrirSimit)
		}
		return "", fmt.Errorf("error al abrir el portal SIMIT: %w", err)
	}

	// La SPA Angular tarda en hidratar: esperar a que formulario y botón estén operativos.
	if !esperarCondicion(taskCtx, scriptSimitFormularioListo, 20*time.Second, 200*time.Millisecond) {
		return "", fmt.Errorf("el formulario de SIMIT no terminó de cargar")
	}

	var ingresado bool
	if err := chromedp.Run(taskCtx, chromedp.Evaluate(scriptSimitIngresar(cedula), &ingresado)); err != nil {
		return "", fmt.Errorf("error ingresando la cédula en SIMIT: %w", err)
	}
	if !ingresado {
		return "", fmt.Errorf("no se encontró el campo de búsqueda de SIMIT")
	}

	// El clic se pierde si la SPA aún está cargando sus scripts (captcha, widgets): se repite hasta que
	// el portal responda en vez de esperar 30 s a una consulta que nunca salió.
	respondio := false
	for intento := 0; intento < 4 && !respondio; intento++ {
		if err := chromedp.Run(taskCtx, chromedp.Click("#btnNumDocPlaca", chromedp.ByID)); err != nil {
			return "", fmt.Errorf("error al interactuar con portal SIMIT: %w", err)
		}
		respondio = esperarCondicion(taskCtx, scriptSimitRespuestaLista, 15*time.Second, 400*time.Millisecond)
	}
	if !respondio {
		return "", fmt.Errorf("SIMIT no respondió a la consulta a tiempo")
	}
	esperarTextoEstable(taskCtx, "body", 4*time.Second)

	var texto string
	if err := chromedp.Run(taskCtx, chromedp.Text("body", &texto, chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("error leyendo respuesta de SIMIT: %w", err)
	}
	if captura != nil {
		if _, ok := interpretarRespuestaSimit(texto); ok {
			*captura = capturarPagina(taskCtx)
		}
	}
	return texto, nil
}

// interpretarRespuestaSimit convierte el texto del portal en un ResultadoSimit. El segundo valor es
// false cuando el texto no contiene ninguna señal reconocible (ni paz y salvo ni conteos).
func interpretarRespuestaSimit(texto string) (*modelos.ResultadoSimit, bool) {
	pazYSalvo := strings.Contains(texto, "no posee a la fecha pendientes de pago") ||
		strings.Contains(texto, "A la fecha se encuentra a paz y salvo") ||
		strings.Contains(texto, "se encuentra a Paz y Salvo")

	contar := func(re *regexp.Regexp) (int, bool) {
		if m := re.FindStringSubmatch(texto); len(m) > 1 {
			n, _ := strconv.Atoi(m[1])
			return n, true
		}
		return 0, false
	}

	cantidadComparendos, okComp := contar(reSimitComparendos)
	cantidadMultas, okMult := contar(reSimitMultas)
	cantidadAcuerdos, okAcue := contar(reSimitAcuerdos)

	if !pazYSalvo && !okComp && !okMult && !okAcue {
		return nil, false
	}

	totalDeuda := "$ 0"
	if m := reSimitTotal.FindStringSubmatch(texto); len(m) > 1 {
		totalDeuda = "$" + strings.TrimSpace(m[1])
	}

	tienePendientes := cantidadComparendos > 0 || cantidadMultas > 0 ||
		(totalDeuda != "$ 0" && totalDeuda != "$0" && totalDeuda != "$")

	var estado string
	switch {
	case cantidadMultas > 0:
		estado = "CON_MULTAS"
	case cantidadComparendos > 0:
		estado = "CON_COMPARENDOS"
	case cantidadAcuerdos > 0:
		estado = "CON_ACUERDO_PAGO"
	default:
		estado = "PAZ_Y_SALVO"
	}

	return &modelos.ResultadoSimit{
		Estado:              estado,
		PazYSalvo:           pazYSalvo && !tienePendientes,
		CantidadComparendos: cantidadComparendos,
		CantidadMultas:      cantidadMultas,
		CantidadAcuerdos:    cantidadAcuerdos,
		TotalDeuda:          totalDeuda,
		Alerta:              tienePendientes,
		Detalles: fmt.Sprintf("SIMIT: %s | Comparendos: %d | Multas: %d | Acuerdos: %d | Deuda: %s",
			estado, cantidadComparendos, cantidadMultas, cantidadAcuerdos, totalDeuda),
	}, true
}
