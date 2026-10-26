package agentes

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// Capturas de pantalla de la evidencia: cada etapa guarda una imagen JPEG con lo que mostró el portal
// consultado. Son de mejor esfuerzo y baratas (una sola llamada CDP, JPEG con calidad media, sin
// reintentos): un fallo al capturar nunca afecta al resultado de la validación.
const (
	anchoMaxCaptura = 1400
	altoMinCaptura  = 320
	altoMaxCaptura  = 1800
	calidadCaptura  = 60
	plazoCaptura    = 5 * time.Second
)

// muCapturas protege el mapa Capturas del candidato: varias etapas pueden guardar a la vez.
var muCapturas sync.Mutex

// capturarPagina toma un JPEG de la pestaña asociada a ctx (contenido visible, hasta altoMaxCaptura).
// Devuelve nil si no se pudo: la captura es opcional.
func capturarPagina(ctx context.Context) []byte {
	cctx, cancel := context.WithTimeout(ctx, plazoCaptura)
	defer cancel()

	var buf []byte
	_ = chromedp.Run(cctx, chromedp.ActionFunc(func(c context.Context) error {
		buf = capturarEnAccion(c)
		return nil
	}))
	return buf
}

// capturarEnAccion es capturarPagina para quien ya está dentro de una acción chromedp (c es el contexto
// de la acción, con el ejecutor de la pestaña).
func capturarEnAccion(c context.Context) []byte {
	params := page.CaptureScreenshot().
		WithFormat(page.CaptureScreenshotFormatJpeg).
		WithQuality(calidadCaptura).
		WithOptimizeForSpeed(true)

	if _, _, _, _, _, contenido, err := page.GetLayoutMetrics().Do(c); err == nil && contenido != nil {
		ancho := min(max(contenido.Width, 800), anchoMaxCaptura)
		alto := min(max(contenido.Height, altoMinCaptura), altoMaxCaptura)
		params = params.
			WithCaptureBeyondViewport(true).
			WithClip(&page.Viewport{X: 0, Y: 0, Width: ancho, Height: alto, Scale: 1})
	}

	buf, err := params.Do(c)
	if err != nil || len(buf) == 0 {
		return nil
	}
	return buf
}

// GuardarCaptura escribe la imagen en <carpeta del candidato>/capturas/<etapa>.jpg y la registra.
// No hace nada si faltan los bytes o la carpeta del candidato.
func GuardarCaptura(c *modelos.Candidato, etapa string, jpg []byte) {
	if c == nil || len(jpg) == 0 || c.CarpetaCandidato == "" {
		return
	}
	carpeta := filepath.Join(c.CarpetaCandidato, "capturas")
	if err := os.MkdirAll(carpeta, 0o755); err != nil {
		return
	}
	ruta := filepath.Join(carpeta, etapa+".jpg")
	if err := utilidades.EscribirArchivoAtomico(ruta, jpg, 0o644); err != nil {
		return
	}
	muCapturas.Lock()
	defer muCapturas.Unlock()
	if c.Capturas == nil {
		c.Capturas = make(map[string]string)
	}
	c.Capturas[etapa] = ruta
}

// CapturaDe devuelve la ruta de la captura de una etapa ("" si no hay).
func CapturaDe(c *modelos.Candidato, etapa string) string {
	muCapturas.Lock()
	defer muCapturas.Unlock()
	return c.Capturas[etapa]
}

// AliasCaptura reutiliza la captura de origen como evidencia de destino (p. ej. el género sale del perfil).
func AliasCaptura(c *modelos.Candidato, destino, origen string) {
	muCapturas.Lock()
	defer muCapturas.Unlock()
	if ruta := c.Capturas[origen]; ruta != "" {
		c.Capturas[destino] = ruta
	}
}

// ---- Evidencia generada (consultas sin página: RUNT por API, lista ONU en memoria) -----------------

// FilaEvidencia es un par etiqueta/valor de la tarjeta de evidencia.
type FilaEvidencia struct {
	Etiqueta string
	Valor    string
}

// Evidencia describe la tarjeta que se dibuja cuando la fuente no tiene página que capturar.
type Evidencia struct {
	Fuente    string
	Portal    string
	Titulo    string
	Veredicto string
	Alerta    bool
	Filas     []FilaEvidencia
	Nota      string
}

// renderizador mantiene un Chrome headless con UNA pestaña ya abierta para dibujar las tarjetas: abrir y
// cerrar una pestaña cuesta ~2 s, mientras que dibujar y capturar en la pestaña abierta cuesta ~0,15 s.
// La pestaña se prepara en segundo plano (PrecalentarRenderizador) y el dibujo se serializa con uso.
var renderizador struct {
	mu          sync.Mutex // protege el Chrome y la pestaña
	uso         sync.Mutex // una tarjeta a la vez en la pestaña
	alloc       context.Context
	cancelAlloc context.CancelFunc
	tab         context.Context
	cancelTab   context.CancelFunc
}

// pestanaRenderizador devuelve la pestaña lista (la crea si hace falta) o nil si Chrome no arrancó.
func pestanaRenderizador() context.Context {
	renderizador.mu.Lock()
	defer renderizador.mu.Unlock()
	if renderizador.tab != nil && renderizador.tab.Err() == nil {
		return renderizador.tab
	}
	if renderizador.alloc == nil || renderizador.alloc.Err() != nil {
		renderizador.alloc, renderizador.cancelAlloc = chromedp.NewExecAllocator(context.Background(), opcionesChrome(true)...)
	}
	tab, cancelTab := chromedp.NewContext(renderizador.alloc, chromedp.WithErrorf(func(string, ...interface{}) {}))
	// La primera Run debe usar el contexto de la pestaña tal cual: si recibe uno derivado con plazo,
	// cancelarlo después cerraría el navegador.
	if err := chromedp.Run(tab); err != nil {
		cancelTab()
		return nil
	}
	arranque, cancelArranque := context.WithTimeout(tab, 20*time.Second)
	defer cancelArranque()
	if err := chromedp.Run(arranque, chromedp.EmulateViewport(960, 640), chromedp.Navigate("about:blank")); err != nil {
		cancelTab()
		return nil
	}
	renderizador.tab, renderizador.cancelTab = tab, cancelTab
	return tab
}

// descartarPestanaRenderizador fuerza una pestaña nueva en el siguiente uso (tras un fallo).
func descartarPestanaRenderizador() {
	renderizador.mu.Lock()
	defer renderizador.mu.Unlock()
	if renderizador.cancelTab != nil {
		renderizador.cancelTab()
	}
	renderizador.tab, renderizador.cancelTab = nil, nil
}

// PrecalentarRenderizador abre el Chrome y su pestaña en segundo plano para no retrasar la primera tarjeta.
func PrecalentarRenderizador() {
	go func() {
		renderizador.uso.Lock()
		defer renderizador.uso.Unlock()
		pestanaRenderizador()
	}()
}

// CerrarRenderizador libera el Chrome de las tarjetas.
func CerrarRenderizador() {
	renderizador.uso.Lock()
	defer renderizador.uso.Unlock()
	renderizador.mu.Lock()
	defer renderizador.mu.Unlock()
	if renderizador.cancelTab != nil {
		renderizador.cancelTab()
	}
	if renderizador.cancelAlloc != nil {
		renderizador.cancelAlloc()
	}
	renderizador.alloc, renderizador.cancelAlloc, renderizador.tab, renderizador.cancelTab = nil, nil, nil, nil
}

// CapturarEvidencia dibuja la tarjeta y la guarda como captura de la etapa. Mejor esfuerzo.
func CapturarEvidencia(ctx context.Context, c *modelos.Candidato, etapa string, ev Evidencia) {
	if c == nil || c.CarpetaCandidato == "" || ctx.Err() != nil {
		return
	}
	renderizador.uso.Lock()
	defer renderizador.uso.Unlock()
	tab := pestanaRenderizador()
	if tab == nil {
		return
	}
	cctx, cancel := context.WithTimeout(tab, 10*time.Second)
	defer cancel()

	if err := chromedp.Run(cctx, chromedp.ActionFunc(func(c context.Context) error {
		arbol, err := page.GetFrameTree().Do(c)
		if err != nil {
			return err
		}
		return page.SetDocumentContent(arbol.Frame.ID, htmlEvidencia(ev)).Do(c)
	})); err != nil {
		descartarPestanaRenderizador()
		return
	}
	GuardarCaptura(c, etapa, capturarPagina(cctx))
}

func htmlEvidencia(ev Evidencia) string {
	color, fondo := "#1f8f3a", "#e8f8ea"
	if ev.Alerta {
		color, fondo = "#c0392b", "#fdecea"
	}
	var filas strings.Builder
	for _, f := range ev.Filas {
		fmt.Fprintf(&filas, `<tr><th>%s</th><td>%s</td></tr>`, html.EscapeString(f.Etiqueta), html.EscapeString(f.Valor))
	}
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><style>
*{box-sizing:border-box}body{margin:0;padding:28px;background:#f4f7f6;font-family:Segoe UI,Arial,sans-serif;color:#2c3e50}
.tarjeta{background:#fff;border:1px solid #dfe6e4;border-radius:16px;overflow:hidden}
.barra{background:#eef2f1;padding:10px 16px;font-size:12px;color:#5d6d6b;border-bottom:1px solid #dfe6e4}
.cuerpo{padding:22px 26px}.fuente{font-size:12px;letter-spacing:.06em;text-transform:uppercase;color:#7b8b88}
h1{margin:6px 0 16px;font-size:20px}.veredicto{display:inline-block;padding:8px 14px;border-radius:10px;font-weight:700;font-size:14px;color:%s;background:%s}
table{width:100%%;border-collapse:collapse;margin-top:18px;font-size:14px}th,td{text-align:left;padding:8px 10px;border-bottom:1px solid #edf1f0;vertical-align:top}
th{width:34%%;color:#7b8b88;font-weight:600}.nota{margin-top:16px;font-size:11.5px;color:#8a9694;line-height:1.5}
</style></head><body><div class="tarjeta"><div class="barra">%s</div><div class="cuerpo">
<div class="fuente">%s</div><h1>%s</h1><span class="veredicto">%s</span><table>%s</table><p class="nota">%s</p></div></div></body></html>`,
		color, fondo,
		html.EscapeString(ev.Portal), html.EscapeString(ev.Fuente), html.EscapeString(ev.Titulo),
		html.EscapeString(ev.Veredicto), filas.String(), html.EscapeString(ev.Nota))
}

// ---- Tarjetas de cada fuente ------------------------------------------------------------------------

func capturarEvidenciaRunt(ctx context.Context, c *modelos.Candidato) {
	r := c.ResultadoRunt
	if r == nil || (r.Estado != "INSCRITO_ACTIVO" && r.Estado != "NO_INSCRITO") {
		return // sin información consultada no hay nada que mostrar
	}
	ev := Evidencia{
		Fuente: "Registro Único Nacional de Tránsito (RUNT)",
		Portal: "https://portalpublico.runt.gov.co",
		Titulo: "Consulta de ciudadano y licencias de conducción",
		Filas: []FilaEvidencia{
			{"Documento", "CC " + soloDigitos(c.Cedula)},
			{"Estado en RUNT", textoONada(r.EstadoPersona, r.Estado)},
		},
		Nota: "Imagen generada por el sistema con la respuesta del RUNT (el RUNT se consulta por API, no hay una página que capturar). " +
			"Fecha de consulta: " + time.Now().Format("2006-01-02 15:04:05"),
	}
	if r.Estado == "NO_INSCRITO" {
		ev.Veredicto, ev.Alerta = "NO INSCRITO EN EL RUNT", true
	} else {
		ev.Veredicto = "INSCRITO · conductor " + textoONada(strings.ToLower(r.EstadoConductor), "sin dato")
		ev.Alerta = r.Alerta
		ev.Filas = append(ev.Filas,
			FilaEvidencia{"Nombre", strings.TrimSpace(r.Nombres + " " + r.Apellidos)},
			FilaEvidencia{"Fecha de inscripción", textoONada(soloFecha(r.FechaInscripcion), "sin dato")},
		)
		for _, l := range r.Licencias {
			cats := strings.Join(l.Categorias, ", ")
			ev.Filas = append(ev.Filas, FilaEvidencia{
				"Licencia " + textoONada(l.NumeroLicencia, ""),
				fmt.Sprintf("%s · %s · expedida %s", textoONada(cats, "sin categorías"), textoONada(l.Estado, "sin estado"), textoONada(l.FechaExpedicion, "sin fecha")),
			})
		}
		if len(r.Licencias) == 0 {
			ev.Filas = append(ev.Filas, FilaEvidencia{"Licencias", "Sin licencias registradas"})
		}
		multas := "Sin multas"
		if r.TieneMultas {
			multas = "Registra multas"
		}
		ev.Filas = append(ev.Filas, FilaEvidencia{"Multas (RUNT)", multas})
	}
	CapturarEvidencia(ctx, c, "runt", ev)
}

func capturarEvidenciaSarlaft(ctx context.Context, c *modelos.Candidato) {
	r := c.ResultadoSarlaft
	if r == nil || r.TipoCoincidencia == "LISTA_NO_DISPONIBLE" {
		return
	}
	ev := Evidencia{
		Fuente: "ONU — Consejo de Seguridad · Lista consolidada de sanciones",
		Portal: "https://scsanctions.un.org/consolidated-sp/",
		Titulo: "Cruce contra la lista consolidada de sanciones",
		Alerta: r.Alerta,
		Filas: []FilaEvidencia{
			{"Persona consultada", c.Nombre},
			{"Documento", "CC " + soloDigitos(c.Cedula)},
			{"Resultado", r.Detalles},
		},
		Nota: "Imagen generada por el sistema: la lista ONU se descarga y se cruza en memoria, no hay una página que capturar. " +
			"Fecha de consulta: " + textoONada(r.FechaVerificacion, time.Now().Format("2006-01-02 15:04:05")),
	}
	if r.Alerta {
		ev.Veredicto = "COINCIDENCIA EN LISTA ONU"
		ev.Filas = append(ev.Filas,
			FilaEvidencia{"Tipo de coincidencia", r.TipoCoincidencia},
			FilaEvidencia{"Registro ONU", r.IDONU + " · " + r.NombreSancionado},
		)
	} else {
		ev.Veredicto = "SIN COINCIDENCIAS EN LISTA ONU"
	}
	CapturarEvidencia(ctx, c, "sarlaft", ev)
}

func textoONada(texto, alternativa string) string {
	if strings.TrimSpace(texto) != "" {
		return texto
	}
	return alternativa
}
