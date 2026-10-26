package agentes

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	reProcNoAlfanum = regexp.MustCompile(`[^a-zA-Z0-9\s\+\-\*xX]`)
	reProcOperacion = regexp.MustCompile(`(\d+)\s*([\+\-\*xX])\s*(\d+)`)
)

// ResolvedorPreguntasProcuraduria resuelve retos de seguridad del portal oficial.
type ResolvedorPreguntasProcuraduria struct {
	capitales         map[string]string
	preguntasDirectas map[string]string
}

// NuevoResolvedorPreguntas inicializa las tablas de conocimiento para los retos.
func NuevoResolvedorPreguntas() *ResolvedorPreguntasProcuraduria {
	caps := map[string]string{
		"amazonas": "leticia", "antioquia": "medellin", "arauca": "arauca",
		"atlantico": "barranquilla", "bolivar": "cartagena", "boyaca": "tunja",
		"caldas": "manizales", "caqueta": "florencia", "casanare": "yopal",
		"cauca": "popayan", "cesar": "valledupar", "choco": "quibdo",
		"cordoba": "monteria", "cundinamarca": "bogota", "guainia": "inirida",
		"guaviare": "san jose del guaviare", "huila": "neiva", "la guajira": "riohacha",
		"magdalena": "santa marta", "meta": "villavicencio", "narino": "pasto",
		"norte de santander": "cucuta", "putumayo": "mocoa", "quindio": "armenia",
		"risaralda": "pereira", "san andres": "san andres", "santander": "bucaramanga",
		"sucre": "sincelejo", "tolima": "ibague", "valle": "cali",
		"valle del cauca": "cali", "vallle del cauca": "cali", "vaupes": "mitu",
		"vichada": "puerto carreno", "colombia": "bogota",
	}

	directas := map[string]string{
		"cuanto es 2 x 3": "6",
		"cuanto es 3 x 3": "9",
		"cuanto es 6 + 2": "8",
		"cuanto es 4 + 3": "7",
		"cuanto es 5 + 3": "8",
		"cuanto es 9 - 2": "7",
		"cuanto es 3 - 2": "1",
		"cuanto es 5 - 2": "3",
		"cuanto es 3 + 2": "5",
	}

	return &ResolvedorPreguntasProcuraduria{
		capitales:         caps,
		preguntasDirectas: directas,
	}
}

func (r *ResolvedorPreguntasProcuraduria) normalizar(texto string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	res, _, _ := transform.String(t, texto)
	limpio := reProcNoAlfanum.ReplaceAllString(res, " ")
	return strings.ToLower(strings.TrimSpace(reEspacios.ReplaceAllString(limpio, " ")))
}

// Resolver deduce la respuesta de seguridad a partir de la pregunta del portal.
func (r *ResolvedorPreguntasProcuraduria) Resolver(preguntaRaw string, cedula string, nombreCompleto string) (string, bool) {
	pregunta := r.normalizar(preguntaRaw)
	if pregunta == "" {
		return "", false
	}
	cedulaLimpia := soloDigitos(cedula)
	primerNombre := ""
	if campos := strings.Fields(nombreCompleto); len(campos) > 0 {
		primerNombre = r.normalizar(campos[0])
	}

	// 1. Preguntas aritméticas directas
	for p, resp := range r.preguntasDirectas {
		if strings.Contains(pregunta, p) {
			return resp, true
		}
	}

	// 2. Operaciones aritméticas generales
	if m := reProcOperacion.FindStringSubmatch(pregunta); len(m) > 3 {
		n1, _ := strconv.Atoi(m[1])
		op := strings.ToLower(m[2])
		n2, _ := strconv.Atoi(m[3])
		switch op {
		case "+":
			return strconv.Itoa(n1 + n2), true
		case "-":
			return strconv.Itoa(n1 - n2), true
		case "*", "x":
			return strconv.Itoa(n1 * n2), true
		}
	}

	// 3. Capitales de Colombia. Se prefiere el nombre de departamento más largo que coincida
	// ("valle del cauca" antes que "valle", "norte de santander" antes que "santander").
	if strings.Contains(pregunta, "capital") {
		mejor, mejorLen := "", 0
		for dpto, cap := range r.capitales {
			if strings.Contains(pregunta, dpto) && len(dpto) > mejorLen {
				mejor, mejorLen = cap, len(dpto)
			}
		}
		if mejor != "" {
			return mejor, true
		}
	}

	// 4. Preguntas sobre los dígitos del documento
	if cedulaLimpia != "" {
		if strings.Contains(pregunta, "tres primeros d") || strings.Contains(pregunta, "3 primeros d") {
			if len(cedulaLimpia) >= 3 {
				return cedulaLimpia[:3], true
			}
		}
		if strings.Contains(pregunta, "dos ultimos d") || strings.Contains(pregunta, "2 ultimos d") {
			if len(cedulaLimpia) >= 2 {
				return cedulaLimpia[len(cedulaLimpia)-2:], true
			}
		}
		if strings.Contains(pregunta, "dos primeros d") || strings.Contains(pregunta, "2 primeros d") {
			if len(cedulaLimpia) >= 2 {
				return cedulaLimpia[:2], true
			}
		}
		if strings.Contains(pregunta, "tres ultimos d") || strings.Contains(pregunta, "3 ultimos d") {
			if len(cedulaLimpia) >= 3 {
				return cedulaLimpia[len(cedulaLimpia)-3:], true
			}
		}
		if strings.Contains(pregunta, "primer d") {
			return string(cedulaLimpia[0]), true
		}
		if strings.Contains(pregunta, "ultimo d") {
			return string(cedulaLimpia[len(cedulaLimpia)-1]), true
		}
	}

	// 5. Preguntas sobre el primer nombre del candidato
	if primerNombre != "" {
		if strings.Contains(pregunta, "dos primeras letras del primer nombre") {
			if len(primerNombre) >= 2 {
				return primerNombre[:2], true
			}
		}
		if strings.Contains(pregunta, "cantidad de letras del primer nombre") || strings.Contains(pregunta, "cuantas letras tiene el primer nombre") {
			return strconv.Itoa(len(primerNombre)), true
		}
		if strings.Contains(pregunta, "primer nombre de la persona") {
			return primerNombre, true
		}
	}

	return "", false
}

// AgenteProcuraduria consulta antecedentes disciplinarios en la Procuraduría General de la Nación.
type AgenteProcuraduria struct {
	cfg         *configuracion.Configuracion
	resolvedor  *ResolvedorPreguntasProcuraduria
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
}

// NuevoAgenteProcuraduria inicializa el agente con su solucionador de retos.
func NuevoAgenteProcuraduria(cfg *configuracion.Configuracion) *AgenteProcuraduria {
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opcionesChrome(true)...)
	return &AgenteProcuraduria{
		cfg:         cfg,
		resolvedor:  NuevoResolvedorPreguntas(),
		allocCtx:    allocCtx,
		cancelAlloc: cancel,
	}
}

func (a *AgenteProcuraduria) Nombre() string {
	return "PROCURADURIA"
}

// Cerrar libera recursos del navegador.
func (a *AgenteProcuraduria) Cerrar() {
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
}

// Validar implementa la interfaz AgenteValidador.
func (a *AgenteProcuraduria) Validar(ctx context.Context, candidato *modelos.Candidato) error {
	cedulaLimpia := soloDigitos(candidato.Cedula)
	if cedulaLimpia == "" || candidato.Cedula == "No especificada" {
		candidato.ResultadoProcuraduria = &modelos.ResultadoProcuraduria{
			Estado:       "NO_APLICA",
			Alerta:       false,
			Coincidencia: false,
			Detalles:     "Cédula no especificada en el perfil del candidato.",
		}
		return nil
	}

	var captura []byte
	resultado, err := a.consultarCedula(ctx, cedulaLimpia, candidato.Nombre, &captura)
	if err != nil {
		candidato.ResultadoProcuraduria = &modelos.ResultadoProcuraduria{
			Estado:       "ERROR_CONSULTA",
			Alerta:       false,
			Coincidencia: false,
			Detalles:     fmt.Sprintf("Error consultando Procuraduría: %v", err),
		}
		return nil
	}

	candidato.ResultadoProcuraduria = resultado
	GuardarCaptura(candidato, "procuraduria", captura)
	return nil
}

// Frases con las que el portal responde cuando la consulta ya se resolvió.
const scriptProcuraduriaResuelta = `
	(() => {
		const t = (document.body ? document.body.innerText : '').toLowerCase();
		const sigueFormulario = !!document.querySelector('#btnConsultar');
		return !sigueFormulario ||
			t.includes('no presenta antecedentes') ||
			t.includes('no registra antecedentes') ||
			t.includes('no registra sanciones') ||
			t.includes('no registra inhabilidades') ||
			t.includes('no se encuentra registrado en el sistema') ||
			t.includes('cancelada por muerte') ||
			t.includes('vigencia del documento');
	})()
`

// ConsultarCedula realiza la automatización web completa resolviendo la pregunta de seguridad.
// Reintenta la consulta completa ante fallos transitorios (portal lento, respuesta rechazada).
func (a *AgenteProcuraduria) ConsultarCedula(ctx context.Context, cedula string, nombreCompleto string) (*modelos.ResultadoProcuraduria, error) {
	return a.consultarCedula(ctx, cedula, nombreCompleto, nil)
}

// consultarCedula es ConsultarCedula con salida opcional de la captura de pantalla del resultado.
func (a *AgenteProcuraduria) consultarCedula(ctx context.Context, cedula string, nombreCompleto string, captura *[]byte) (*modelos.ResultadoProcuraduria, error) {
	cedulaLimpia := soloDigitos(cedula)
	if cedulaLimpia == "" {
		return &modelos.ResultadoProcuraduria{
			Estado:   "SIN_CEDULA",
			Detalles: "Número de cédula inválido o vacío.",
		}, nil
	}

	var resultado *modelos.ResultadoProcuraduria
	err := reintentar(ctx, 3, 2*time.Second, func(intento int) error {
		if captura != nil {
			*captura = nil
		}
		res, err := a.consultarUnaVez(ctx, cedulaLimpia, nombreCompleto, intento == 3, captura)
		if err != nil {
			return err
		}
		resultado = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resultado, nil
}

// consultarUnaVez ejecuta un ciclo completo en una pestaña nueva. En el último intento
// (ultimo=true) interpreta cualquier página resultante en lugar de pedir otro reintento.
func (a *AgenteProcuraduria) consultarUnaVez(ctx context.Context, cedula, nombreCompleto string, ultimo bool, captura *[]byte) (*modelos.ResultadoProcuraduria, error) {
	taskCtx, cancelTask := chromedp.NewContext(a.allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	defer cancelTask()
	detener := context.AfterFunc(ctx, cancelTask)
	defer detener()

	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, 60*time.Second)
	defer cancelTimeout()

	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(a.cfg.URLProcuraduria),
		chromedp.WaitVisible("#ddlTipoID", chromedp.ByID),
		chromedp.SetValue("#ddlTipoID", "1", chromedp.ByID),
		chromedp.SetValue("#txtNumID", cedula, chromedp.ByID),
	); err != nil {
		return nil, fmt.Errorf("error al conectar con formulario de Procuraduría: %w", err)
	}

	// Resolver la pregunta; si no se conoce, refrescar el reto hasta 15 veces
	// esperando el cambio de texto (no un Sleep fijo).
	respuesta, encontrada := "", false
	for i := 0; i < 15 && !encontrada; i++ {
		var pregunta string
		_ = chromedp.Run(taskCtx, chromedp.Text("#lblPregunta", &pregunta, chromedp.ByID))
		if resp, ok := a.resolvedor.Resolver(pregunta, cedula, nombreCompleto); ok {
			respuesta, encontrada = resp, true
			break
		}
		if err := chromedp.Run(taskCtx, chromedp.Click("#ImageButton1", chromedp.ByID)); err != nil {
			return nil, fmt.Errorf("error refrescando la pregunta de seguridad: %w", err)
		}
		esperarCondicion(taskCtx,
			`(document.querySelector('#lblPregunta')?.innerText || '') !== `+literalJS(pregunta),
			5*time.Second, 200*time.Millisecond)
	}

	if !encontrada {
		return &modelos.ResultadoProcuraduria{
			Estado:   "ERROR_PREGUNTA",
			Alerta:   false,
			Detalles: "No fue posible resolver la pregunta de seguridad tras 15 intentos.",
		}, nil
	}

	// Enviar respuesta y esperar el resultado real (hasta 15 s) en vez de dormir 2 s fijos.
	var texto string
	if err := chromedp.Run(taskCtx,
		chromedp.SetValue("#txtRespuestaPregunta", respuesta, chromedp.ByID),
		chromedp.Click("#btnConsultar", chromedp.ByID),
	); err != nil {
		return nil, fmt.Errorf("error al ejecutar consulta en Procuraduría: %w", err)
	}
	resuelta := esperarCondicion(taskCtx, scriptProcuraduriaResuelta, 15*time.Second, 300*time.Millisecond)

	if err := chromedp.Run(taskCtx, chromedp.Text("body", &texto, chromedp.ByQuery)); err != nil {
		return nil, fmt.Errorf("error leyendo respuesta de Procuraduría: %w", err)
	}

	// Sin respuesta reconocible y todavía en el formulario: el portal rechazó la respuesta o no
	// avanzó. Se pide reintento salvo que sea el último intento.
	if !resuelta && !ultimo {
		return nil, fmt.Errorf("la consulta en Procuraduría no avanzó (¿respuesta de seguridad rechazada?)")
	}

	if captura != nil && resuelta {
		*captura = capturarPagina(taskCtx)
	}
	return a.interpretarRespuesta(texto, resuelta), nil
}

// interpretarRespuesta traduce el texto de la página de resultado a un ResultadoProcuraduria.
// paginaAvanzo indica si el portal salió del formulario; si no, no se asume un hallazgo.
func (a *AgenteProcuraduria) interpretarRespuesta(texto string, paginaAvanzo bool) *modelos.ResultadoProcuraduria {
	textoLower := strings.ToLower(texto)
	esLimpio := strings.Contains(textoLower, "no presenta antecedentes") ||
		strings.Contains(textoLower, "no registra antecedentes") ||
		strings.Contains(textoLower, "no registra sanciones") ||
		strings.Contains(textoLower, "no registra inhabilidades")

	noRegistrado := strings.Contains(textoLower, "no se encuentra registrado en el sistema")
	cancelada := strings.Contains(textoLower, "cancelada por muerte") || strings.Contains(textoLower, "vigencia del documento")

	switch {
	case esLimpio:
		return &modelos.ResultadoProcuraduria{
			Estado:   "SIN_ANTECEDENTES",
			Detalles: "El ciudadano NO registra antecedentes ni inhabilidades vigentes en Procuraduría.",
		}
	case noRegistrado:
		return &modelos.ResultadoProcuraduria{
			Estado:   "NO_REGISTRADO",
			Detalles: "El documento no se encuentra registrado en el sistema SIRI de la Procuraduría.",
		}
	case cancelada:
		return &modelos.ResultadoProcuraduria{
			Estado:       "CANCELADA_REGISTRADURIA",
			Alerta:       true,
			Coincidencia: true,
			Detalles:     "Cédula cancelada o con novedad de vigencia en Registraduría Nacional.",
		}
	case !paginaAvanzo:
		// Seguimos en el formulario: no hay evidencia de hallazgo, solo de que la consulta falló.
		return &modelos.ResultadoProcuraduria{
			Estado:   "ERROR_RESPUESTA",
			Detalles: "El portal de la Procuraduría no devolvió un resultado tras enviar la consulta.",
		}
	default:
		return &modelos.ResultadoProcuraduria{
			Estado:       "CON_ANTECEDENTES",
			Alerta:       true,
			Coincidencia: true,
			Detalles:     "El ciudadano presenta antecedentes, sanciones o inhabilidades registradas en Procuraduría.",
		}
	}
}
