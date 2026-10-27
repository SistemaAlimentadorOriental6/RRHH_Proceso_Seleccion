package agentes

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

var reAdresColumnas = regexp.MustCompile(`\t+|\s{2,}`)

// AgenteAdres valida afiliaciones en salud y EPS autorizadas en la BDUA de ADRES.
type AgenteAdres struct {
	cfg         *configuracion.Configuracion
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
}

// NuevoAgenteAdres inicializa el agente de validación de EPS.
func NuevoAgenteAdres(cfg *configuracion.Configuracion) *AgenteAdres {
	// Chrome VISIBLE para ADRES: reCAPTCHA Enterprise invisible requiere browser real
	// Sin servidor gráfico (Linux sin DISPLAY) Chrome visible no arranca: se usa headless.
	// shortcut: reCAPTCHA puede rechazar headless; si pasa, correr con `xvfb-run` y DISPLAY queda definido.
	sinPantalla := runtime.GOOS == "linux" && os.Getenv("DISPLAY") == ""
	// ADRES sirve la cadena de certificados incompleta; el Linux del servidor no la completa como Windows.
	// Solo afecta a este agente (consulta pública, sin credenciales).
	opts := opcionesChrome(sinPantalla,
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("window-position", "-2000,-2000"),
		chromedp.Flag("window-size", "800,600"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &AgenteAdres{
		cfg:         cfg,
		allocCtx:    allocCtx,
		cancelAlloc: cancel,
	}
}

func (a *AgenteAdres) Nombre() string {
	return "ADRES"
}

// Cerrar libera los recursos de asignación del navegador.
func (a *AgenteAdres) Cerrar() {
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
}

// Validar implementa el contrato de AgenteValidador.
func (a *AgenteAdres) Validar(ctx context.Context, candidato *modelos.Candidato) error {
	cedulaLimpia := soloDigitos(candidato.Cedula)
	if cedulaLimpia == "" || candidato.Cedula == "No especificada" {
		candidato.ResultadoAdres = &modelos.ResultadoAdres{
			Estado:           "NO_APLICA",
			CumpleEPS:        false,
			Alerta:           false,
			EPS:              "NO REPORTADA",
			Regimen:          "NO REGISTRADO",
			EstadoAfiliacion: "NO REGISTRADO",
			Detalles:         "Cédula no especificada en el perfil del candidato.",
		}
		return nil
	}

	var captura []byte
	resultado, err := a.consultarCedula(ctx, cedulaLimpia, &captura)
	if err != nil {
		fmt.Printf("    [!] ADRES ERROR (%s): %v\n", candidato.Nombre, err)
		candidato.ResultadoAdres = &modelos.ResultadoAdres{
			Estado:           "ERROR_CONSULTA",
			Documento:        cedulaLimpia,
			CumpleEPS:        false,
			Alerta:           true,
			EPS:              "ERROR",
			Regimen:          "ERROR",
			EstadoAfiliacion: "ERROR",
			Detalles:         fmt.Sprintf("Error consultando portal ADRES: %v", err),
		}
		return nil
	}

	candidato.ResultadoAdres = resultado
	GuardarCaptura(candidato, "adres", captura)
	return nil
}

// scriptAdresListo es verdadero cuando el formulario de consulta está disponible o el portal
// mostró su cortina de mantenimiento.
const scriptAdresListo = `
	(() => {
		if (document.querySelector('#tipoDoc') && document.querySelector('#txtNumDoc')) return true;
		const t = (document.body ? document.body.innerText : '');
		return location.href.includes('CortinaWeb') ||
			t.includes('en proceso de actualización') ||
			t.includes('presentamos inconvenientes');
	})()
`

// ConsultarCedula consulta la Base de Datos Única de Afiliados (BDUA) de ADRES. Reintenta hasta
// 3 veces cuando el portal falla (reCAPTCHA sin token, pestaña de resultados vacía, etc.).
func (a *AgenteAdres) ConsultarCedula(ctx context.Context, cedula string) (*modelos.ResultadoAdres, error) {
	return a.consultarCedula(ctx, cedula, nil)
}

// consultarCedula es ConsultarCedula con salida opcional de la captura de pantalla del resultado.
func (a *AgenteAdres) consultarCedula(ctx context.Context, cedula string, captura *[]byte) (*modelos.ResultadoAdres, error) {
	cedulaLimpia := soloDigitos(cedula)
	if cedulaLimpia == "" {
		return &modelos.ResultadoAdres{
			Estado:   "SIN_CEDULA",
			Detalles: "Número de documento inválido.",
		}, nil
	}

	var resultado *modelos.ResultadoAdres
	err := reintentar(ctx, 3, 3*time.Second, func(int) error {
		res, err := a.consultarUnaVez(ctx, cedulaLimpia, captura)
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

func (a *AgenteAdres) consultarUnaVez(ctx context.Context, cedulaLimpia string, captura *[]byte) (*modelos.ResultadoAdres, error) {
	if captura != nil {
		*captura = nil // solo vale la captura del intento que termina bien
	}
	taskCtx, cancelTask := chromedp.NewContext(a.allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	defer cancelTask()
	detener := context.AfterFunc(ctx, cancelTask)
	defer detener()

	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, 75*time.Second)
	defer cancelTimeout()

	var textoResultado string
	var urlActual, textoInicial string

	// Script para evadir detección de WebDriver por reCAPTCHA invisible
	scriptAntiDeteccion := `
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
		window.chrome = { runtime: {} };
		Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3] });
		Object.defineProperty(navigator, 'languages', { get: () => ['es-CO', 'es', 'en'] });
	`

	err := chromedp.Run(taskCtx,
		chromedp.ActionFunc(func(c context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(scriptAntiDeteccion).Do(c)
			return err
		}),
		chromedp.Navigate(a.cfg.URLAdres),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con portal ADRES: %w", err)
	}

	// Esperar formulario o cortina de mantenimiento (en vez de dormir 3 s fijos).
	esperarCondicion(taskCtx, scriptAdresListo, 15*time.Second, 250*time.Millisecond)
	if err := chromedp.Run(taskCtx,
		chromedp.Location(&urlActual),
		chromedp.Text("body", &textoInicial, chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("error leyendo portal ADRES: %w", err)
	}

	if strings.Contains(urlActual, "CortinaWeb") || strings.Contains(textoInicial, "en proceso de actualización") || strings.Contains(textoInicial, "presentamos inconvenientes") {
		return &modelos.ResultadoAdres{
			Estado:           "PORTAL_EN_MANTENIMIENTO",
			Documento:        cedulaLimpia,
			CumpleEPS:        false,
			Alerta:           false,
			EPS:              "NO DISPONIBLE (MANTENIMIENTO)",
			Regimen:          "N/A",
			EstadoAfiliacion: "N/A",
			Detalles:         "Portal ADRES en proceso de mantenimiento oficial (CortinaWeb).",
		}, nil
	}

	// Obtener lista de targets antes del clic para detectar la nueva pestaña
	var targetsPre []*target.Info
	err = chromedp.Run(taskCtx,
		chromedp.WaitVisible("#tipoDoc", chromedp.ByID),
		chromedp.SetValue("#tipoDoc", "CC", chromedp.ByID),
		chromedp.SetValue("#txtNumDoc", cedulaLimpia, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			infos, err := target.GetTargets().Do(c)
			if err == nil {
				targetsPre = infos
			}
			return nil
		}),
		chromedp.Click("#btnConsultar", chromedp.ByID),
	)
	if err != nil {
		return nil, fmt.Errorf("error al interactuar con formulario ADRES: %w", err)
	}

	// Esperar la nueva pestaña (cualquier URL; puede iniciar en about:blank) o resultado en la misma pestaña
	preIDs := make(map[target.ID]bool)
	for _, t := range targetsPre {
		preIDs[t.TargetID] = true
	}

	var newTargetID target.ID
	var origURL, origTexto string
	var tokenLen int
	diagnosticoImpreso := false
	envioForzado := false
	for i := 0; i < 40 && newTargetID == ""; i++ {
		if !dormir(taskCtx, 500*time.Millisecond) {
			return nil, fmt.Errorf("consulta ADRES cancelada o sin respuesta a tiempo")
		}
		var targetsPost []*target.Info
		_ = chromedp.Run(taskCtx,
			chromedp.ActionFunc(func(c context.Context) error {
				infos, err := target.GetTargets().Do(c)
				if err == nil {
					targetsPost = infos
				}
				return nil
			}),
		)
		for _, t := range targetsPost {
			if !preIDs[t.TargetID] && t.Type == "page" {
				newTargetID = t.TargetID
				break
			}
		}
		if newTargetID != "" {
			break
		}
		// Revisar si la misma pestaña navegó al resultado
		if i%4 == 3 {
			cctx, cc := context.WithTimeout(taskCtx, 3*time.Second)
			_ = chromedp.Run(cctx,
				chromedp.Location(&origURL),
				chromedp.Evaluate(`(function(){var t=document.querySelector('textarea[name="g-recaptcha-response"]');return t?t.value.length:-1})()`, &tokenLen),
			)
			cc()
			if strings.Contains(origURL, "RespuestaConsulta") {
				break
			}
			// Con token de reCAPTCHA ya generado pero sin envío: enviar el formulario en la misma pestaña
			if !envioForzado && i >= 7 && tokenLen > 0 {
				envioForzado = true
				_ = chromedp.Run(taskCtx,
					chromedp.Evaluate(`(function(){
						var f=document.getElementById('afiliado'); if(!f) return false;
						f.target='_self';
						if(!f.querySelector('input[type=hidden][name=btnConsultar]')){
							var h=document.createElement('input'); h.type='hidden'; h.name='btnConsultar'; h.value='Consultar'; f.appendChild(h);
						}
						HTMLFormElement.prototype.submit.call(f); return true;
					})()`, nil),
				)
				if !dormir(taskCtx, 2*time.Second) {
					return nil, fmt.Errorf("consulta ADRES cancelada")
				}
				continue
			}
			if !diagnosticoImpreso && i >= 11 {
				diagnosticoImpreso = true
				var urls []string
				for _, t := range targetsPost {
					urls = append(urls, fmt.Sprintf("%s:%s", t.Type, t.URL))
				}
				fmt.Printf("    [ADRES-DIAG] CC %s sin nueva pestaña tras %ds | url=%s | token_recaptcha_len=%d | targets=%v\n",
					cedulaLimpia, (i+1)/2, origURL, tokenLen, urls)
			}
		}
	}

	if newTargetID != "" {
		newTabCtx, cancelNewTab := chromedp.NewContext(taskCtx, chromedp.WithTargetID(newTargetID))
		defer cancelNewTab()
		// Inicializa la conexión con la pestaña usando su propio contexto (no uno con plazo): si no, las
		// llamadas siguientes —incluida la captura de pantalla— se cuelgan hasta agotar el plazo.
		_ = chromedp.Run(newTabCtx)
		// Esperar a que la pestaña cargue el resultado
		for j := 0; j < 30; j++ {
			var u string
			cctx, cc := context.WithTimeout(newTabCtx, 4*time.Second)
			_ = chromedp.Run(cctx, chromedp.Location(&u), chromedp.Text("body", &textoResultado, chromedp.ByQuery))
			cc()
			if strings.Contains(u, "RespuestaConsulta") && len(strings.TrimSpace(textoResultado)) > 100 {
				break
			}
			if !dormir(taskCtx, 500*time.Millisecond) {
				break
			}
		}
		if captura != nil && adresRespuestaValida(textoResultado) {
			*captura = capturarPagina(newTabCtx)
		}
		_ = chromedp.Run(newTabCtx,
			chromedp.ActionFunc(func(c context.Context) error {
				_ = target.CloseTarget(newTargetID).Do(c)
				return nil
			}),
		)
	} else {
		err = chromedp.Run(taskCtx,
			chromedp.Location(&origURL),
			chromedp.Text("body", &origTexto, chromedp.ByQuery),
		)
		if err != nil {
			return nil, fmt.Errorf("error leyendo resultado ADRES: %w", err)
		}
		textoResultado = origTexto
		if captura != nil && strings.Contains(origURL, "RespuestaConsulta") && adresRespuestaValida(textoResultado) {
			*captura = capturarPagina(taskCtx)
		}
		if !strings.Contains(origURL, "RespuestaConsulta") {
			snippet := strings.Join(strings.Fields(origTexto), " ")
			if len(snippet) > 300 {
				snippet = snippet[:300]
			}
			fmt.Printf("    [ADRES-DIAG] CC %s sin resultado (url=%s, token_len=%d). Texto: %s\n", cedulaLimpia, origURL, tokenLen, snippet)
		}
	}

	// Una respuesta sin datos ni mensaje de "no encontrado" es una falla del portal, no un resultado:
	// se reintenta en vez de clasificar a la persona como "afiliación inactiva".
	if !adresRespuestaValida(textoResultado) {
		return nil, fmt.Errorf("ADRES devolvió una respuesta vacía o incompleta (%d caracteres)", len(strings.TrimSpace(textoResultado)))
	}

	return a.parsearResultado(textoResultado, cedulaLimpia), nil
}

// adresRespuestaValida indica si el texto contiene datos de afiliación o un aviso explícito de que
// la persona no figura en la BDUA.
func adresRespuestaValida(texto string) bool {
	mayus := strings.ToUpper(texto)
	if strings.Contains(mayus, "ENTIDAD") && (strings.Contains(mayus, "ACTIVO") || strings.Contains(mayus, "ESTADO")) {
		return true
	}
	bajas := strings.ToLower(texto)
	for _, frase := range []string{"no se encuentra", "no existe", "no registra", "no se encontr", "sin información"} {
		if strings.Contains(bajas, frase) {
			return true
		}
	}
	return len(strings.TrimSpace(texto)) >= 200
}

func (a *AgenteAdres) parsearResultado(texto string, cedula string) *modelos.ResultadoAdres {
	nombres := ""
	apellidos := ""
	departamento := ""
	municipio := ""
	estadoAfiliacion := "NO REPORTADO"
	entidadEPS := "NO REPORTADA"
	regimen := "NO REPORTADO"
	fechaAfiliacion := ""
	tipoAfiliado := ""

	lineas := strings.Split(texto, "\n")
	var lineasLimpias []string
	for _, l := range lineas {
		lTrim := strings.TrimSpace(l)
		if lTrim != "" {
			lineasLimpias = append(lineasLimpias, lTrim)
		}
	}

	for i, linea := range lineasLimpias {
		partesCol := reAdresColumnas.Split(linea, -1)
		if len(partesCol) > 0 {
			clave := strings.ToUpper(partesCol[0])
			if strings.Contains(clave, "NOMBRES") {
				if len(partesCol) > 1 {
					nombres = strings.TrimSpace(partesCol[1])
				} else if i+1 < len(lineasLimpias) {
					nombres = lineasLimpias[i+1]
				}
			} else if strings.Contains(clave, "APELLIDOS") {
				if len(partesCol) > 1 {
					apellidos = strings.TrimSpace(partesCol[1])
				} else if i+1 < len(lineasLimpias) {
					apellidos = lineasLimpias[i+1]
				}
			} else if strings.Contains(clave, "DEPARTAMENTO") {
				if len(partesCol) > 1 {
					departamento = strings.TrimSpace(partesCol[1])
				} else if i+1 < len(lineasLimpias) {
					departamento = lineasLimpias[i+1]
				}
			} else if strings.Contains(clave, "MUNICIPIO") {
				if len(partesCol) > 1 {
					municipio = strings.TrimSpace(partesCol[1])
				} else if i+1 < len(lineasLimpias) {
					municipio = lineasLimpias[i+1]
				}
			}
		}

		// Buscar fila de datos de afiliación
		lineaUpper := strings.ToUpper(linea)
		if strings.Contains(lineaUpper, "ESTADO") && strings.Contains(lineaUpper, "ENTIDAD") && i+1 < len(lineasLimpias) {
			filaDatos := lineasLimpias[i+1]
			partes := reAdresColumnas.Split(filaDatos, -1)
			if len(partes) >= 3 {
				estadoAfiliacion = strings.TrimSpace(partes[0])
				entidadEPS = strings.TrimSpace(partes[1])
				regimen = strings.TrimSpace(partes[2])
				if len(partes) >= 4 {
					fechaAfiliacion = strings.TrimSpace(partes[3])
				}
				if len(partes) >= 6 {
					tipoAfiliado = strings.TrimSpace(partes[5])
				}
			}
		}
	}

	epsUpper := strings.ToUpper(entidadEPS)
	esAutorizada := false
	for _, aut := range a.cfg.EPSAutorizadas {
		if strings.Contains(epsUpper, strings.ToUpper(aut)) {
			esAutorizada = true
			break
		}
	}
	esActivo := strings.Contains(strings.ToUpper(estadoAfiliacion), "ACTIVO")

	var estado string
	var cumpleEPS, alerta bool
	var detalles string

	if esAutorizada && esActivo {
		estado = "AFILIADO_VALIDO"
		cumpleEPS = true
		alerta = false
		detalles = fmt.Sprintf("Cumple con EPS autorizada: %s (Estado: %s, Régimen: %s)", entidadEPS, estadoAfiliacion, regimen)
	} else if !esAutorizada && esActivo {
		estado = "EPS_NO_AUTORIZADA"
		cumpleEPS = false
		alerta = true
		detalles = fmt.Sprintf("EPS no autorizada (%s). Solo se admiten: Sura, Nueva EPS, Savia Salud o Sanitas.", entidadEPS)
	} else if !esActivo {
		estado = "AFILIACION_INACTIVA"
		cumpleEPS = false
		alerta = true
		detalles = fmt.Sprintf("Afiliación en estado inactivo (%s) en EPS %s.", estadoAfiliacion, entidadEPS)
	} else {
		estado = "NO_VALIDO"
		cumpleEPS = false
		alerta = true
		detalles = fmt.Sprintf("No cumple requisitos de afiliación (EPS: %s, Estado: %s).", entidadEPS, estadoAfiliacion)
	}

	return &modelos.ResultadoAdres{
		Estado:           estado,
		Documento:        cedula,
		Nombres:          nombres,
		Apellidos:        apellidos,
		Departamento:     departamento,
		Municipio:        municipio,
		EPS:              entidadEPS,
		Regimen:          regimen,
		EstadoAfiliacion: estadoAfiliacion,
		TipoAfiliado:     tipoAfiliado,
		FechaAfiliacion:  fechaAfiliacion,
		CumpleEPS:        cumpleEPS,
		Alerta:           alerta,
		Detalles:         detalles,
	}
}
