package agentes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

const (
	// maxBytesCV limita el tamaño de una hoja de vida descargada (evita llenar memoria/disco).
	maxBytesCV = 25 << 20
	// maxPaginasOferta evita bucles infinitos si la paginación genera enlaces circulares.
	maxPaginasOferta = 60
	// maxPasosCarga es el tope de scrolls/clics de "cargar más" por página.
	maxPasosCarga = 40
)

var (
	reCaracteresInvalidos = regexp.MustCompile(`[\\/*?:"<>|]`)
	// Admite "C.C. No. 1.037…", "Cédula de ciudadanía N° 1037…" y "Documento de identidad: 1037…".
	reDocumentoTexto = regexp.MustCompile(`(?i)(?:c\.?c\.?|c[eé]dula(?:\s+de\s+ciudadan[ií]a)?|identificaci[oó]n|documento(?:\s+de\s+identidad)?|d\.?n\.?i\.?)\s*(?:n[oº°]\.?|n[uú]mero)?\s*[:\-#]?\s*([0-9\.\,\s]{6,15})`)
)

// ErrSesionExpirada indica que Computrabajo redirigió al login: hay que autenticar de nuevo.
var ErrSesionExpirada = errors.New("la sesión de Computrabajo expiró; vuelva a autenticar")

// ErrListadoIncompleto avisa que el listado de postulados se leyó a medias (una página falló o no salió
// nadie): lo leído sirve, pero no prueba que los que faltan se hayan ido de la vacante.
var ErrListadoIncompleto = errors.New("el listado de postulados quedó incompleto")

// AgenteComputrabajo gestiona la autenticacion, extraccion de ofertas, candidatos y descarga de PDFs.
type AgenteComputrabajo struct {
	cfg *configuracion.Configuracion

	// mu protege la (re)creación de la sesión de Chrome: ExtraerDetalleCandidato corre
	// en muchas goroutines y todas llaman a sesion().
	mu          sync.Mutex
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	sesionCtx   context.Context
	cancelSes   context.CancelFunc

	http *http.Client
}

// limpiarLocksPerfil elimina candados residuales y procesos huérfanos que impiden el arranque de Chrome.
func limpiarLocksPerfil(dirPerfil string) {
	// En Windows, terminar cualquier proceso de Chrome huérfano asociado exclusivamente a este directorio de perfil
	if runtime.GOOS == "windows" {
		dirBase := filepath.Base(dirPerfil)
		scriptPowershell := fmt.Sprintf(`Get-CimInstance Win32_Process -Filter "name = 'chrome.exe'" | Where-Object { $_.CommandLine -like "*%s*" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`, dirBase)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", scriptPowershell)
		_ = cmd.Run()
		time.Sleep(300 * time.Millisecond)
	}

	archivosLock := []string{
		filepath.Join(dirPerfil, "lockfile"),
		filepath.Join(dirPerfil, "SingletonLock"),
		filepath.Join(dirPerfil, "SingletonCookie"),
		filepath.Join(dirPerfil, "SingletonSocket"),
	}
	for _, ruta := range archivosLock {
		_ = os.Remove(ruta)
	}
}

// NuevoAgenteComputrabajo inicializa el agente con el perfil persistente de Chrome.
func NuevoAgenteComputrabajo(cfg *configuracion.Configuracion) *AgenteComputrabajo {
	a := &AgenteComputrabajo{
		cfg:  cfg,
		http: nuevoClienteHTTP(45 * time.Second),
	}
	a.mu.Lock()
	a.crearSesionLocked()
	a.mu.Unlock()
	return a
}

// crearSesionLocked lanza un Chrome nuevo con el perfil persistente. Requiere a.mu tomado.
func (a *AgenteComputrabajo) crearSesionLocked() {
	// Limpiar candados y procesos huérfanos residuales de ejecuciones previas
	limpiarLocksPerfil(a.cfg.DirectorioPerfilCP)

	opts := opcionesChrome(true, chromedp.UserDataDir(a.cfg.DirectorioPerfilCP))
	a.allocCtx, a.cancelAlloc = chromedp.NewExecAllocator(context.Background(), opts...)
	a.sesionCtx, a.cancelSes = chromedp.NewContext(a.allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	// Arrancar el navegador ya: las pestañas que cuelgan de una sesión sin navegador lanzan cada una
	// su propio Chrome y chromedp cierra dos veces el mismo canal ("close of closed channel").
	if err := chromedp.Run(a.sesionCtx); err != nil {
		a.cancelSes() // sesion() verá el contexto cancelado y lo recreará en la siguiente llamada
	}
}

func (a *AgenteComputrabajo) Nombre() string {
	return "COMPUTRABAJO"
}

// sesion devuelve el contexto de Chrome vigente, recreándolo si fue cancelado o cerrado.
// Es seguro para uso concurrente.
func (a *AgenteComputrabajo) sesion() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sesionCtx == nil || a.sesionCtx.Err() != nil {
		if a.cancelAlloc != nil {
			a.cancelAlloc()
		}
		a.crearSesionLocked()
	}
	return a.sesionCtx
}

// SesionContexto expone el contexto de sesión de Chrome para operaciones avanzadas.
func (a *AgenteComputrabajo) SesionContexto() context.Context {
	return a.sesion()
}

// Cerrar libera el contexto del navegador.
func (a *AgenteComputrabajo) Cerrar() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelSes != nil {
		a.cancelSes()
	}
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
}

// AsegurarAutenticacion valida si la sesión existe o realiza el inicio de sesión oficial.
func (a *AgenteComputrabajo) AsegurarAutenticacion(ctx context.Context) error {
	if !a.cfg.ValidarCredenciales() {
		return fmt.Errorf("credenciales de Computrabajo no configuradas en variables de entorno")
	}

	ses := a.sesion()
	sesionActiva := func(u string) bool {
		return u != "" && !strings.Contains(u, "Login") && !strings.Contains(u, "Account/Used")
	}

	var urlActual string
	err := chromedp.Run(ses,
		chromedp.Navigate(a.cfg.URLOfertas),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err == nil {
		urlActual, _ = esperarURL(ses, func(u string) bool { return u != "" && u != "about:blank" }, 5*time.Second)
		if sesionActiva(urlActual) {
			fmt.Println("[+] Sesión activa en Computrabajo detectada.")
			return nil
		}
	} else if strings.Contains(err.Error(), "ERR_TOO_MANY_REDIRECTS") {
		// Sesión dañada (cookies viejas): se borran para poder iniciar sesión de cero.
		_ = chromedp.Run(ses, network.ClearBrowserCookies())
	}

	fmt.Println("[*] Autenticando en Computrabajo...")
	// Las credenciales se pasan como literales JS escapados: una contraseña con comillas,
	// barras o saltos de línea ya no rompe el script ni lo inyecta.
	scriptPassword := `
		(() => {
			const p = document.querySelector('#fiesta') || document.querySelector('input[name="Password"]');
			if (p) {
				p.value = ` + literalJS(a.cfg.ClaveComputrabajo) + `;
				p.dispatchEvent(new Event('input', { bubbles: true }));
				p.dispatchEvent(new Event('change', { bubbles: true }));
			}
			const btn = document.querySelector('input[type="submit"]') || document.querySelector('button[type="submit"]');
			if (btn) btn.click();
		})()
	`

	enviar := func() error {
		return chromedp.Run(ses,
			chromedp.Navigate(a.cfg.URLLogin),
			chromedp.WaitVisible("#UserName", chromedp.ByID),
			chromedp.SetValue("#UserName", a.cfg.UsuarioComputrabajo, chromedp.ByID),
			chromedp.Evaluate(scriptPassword, nil),
		)
	}
	err = enviar()
	if err != nil && strings.Contains(err.Error(), "ERR_TOO_MANY_REDIRECTS") {
		_ = chromedp.Run(ses, network.ClearBrowserCookies())
		err = enviar()
	}
	if err != nil {
		return fmt.Errorf("error enviando credenciales en Computrabajo: %w", err)
	}

	// Esperar a que la navegación posterior al envío salga de /Login (máx. 20 s) en vez de dormir fijo.
	urlActual, _ = esperarURL(ses, func(u string) bool { return u != "" && !strings.Contains(u, "Login") }, 20*time.Second)

	if strings.Contains(urlActual, "Account/Used") {
		fmt.Println("[!] Aviso: Cuenta en uso por otra sesión. Esperando liberación (10s)...")
		if !dormir(ctx, 10*time.Second) {
			return ctx.Err()
		}
		_ = chromedp.Run(ses, chromedp.Navigate(a.cfg.URLOfertas), chromedp.WaitReady("body", chromedp.ByQuery))
		urlActual, _ = esperarURL(ses, func(u string) bool { return u != "" && u != "about:blank" }, 8*time.Second)
	}

	if !sesionActiva(urlActual) {
		return fmt.Errorf("falló la autenticación en Computrabajo. Verifique usuario y contraseña")
	}

	fmt.Println("[+] Sesión iniciada exitosamente en Computrabajo.")
	return nil
}

// ObtenerOfertasActivas extrae la lista de ofertas vigentes en la plataforma.
func (a *AgenteComputrabajo) ObtenerOfertasActivas(ctx context.Context) ([]modelos.OfertaLaboral, error) {
	scriptExtraerOfertas := `
		(() => {
			const articulos = Array.from(document.querySelectorAll('article.listacheckbox'));
			const ofertas = [];
			for (const art of articulos) {
				const status = art.querySelector('.fc_status_activa');
				if (!status || !status.innerText.toLowerCase().includes('activa')) continue;

				const titEl = art.querySelector('.test_offername') || art.querySelector('span[data-offer-title] a');
				const titulo = titEl ? titEl.innerText.trim() : 'Oferta';

				// Seleccionar el enlace directo de la oferta que lleva a los postulados (Match)
				let linkCandidatos = art.querySelector('a[href*="Match"]');
				if (!linkCandidatos) {
					linkCandidatos = art.querySelector('a[href*="oi="]');
				}
				if (!linkCandidatos) continue;

				let href = linkCandidatos.getAttribute('href') || '';
				if (!href.startsWith('http')) {
					href = 'https://empresa.co.computrabajo.com' + href;
				}

				const mId = href.match(/[?&]oi=([^&#]+)/i);
				ofertas.push({
					titulo: titulo,
					url_candidatos: href,
					id: mId ? mId[1] : ''
				});
			}
			return ofertas;
		})()
	`

	ses := a.sesion()
	var ofertas []modelos.OfertaLaboral
	err := reintentar(ctx, 2, 2*time.Second, func(int) error {
		if err := chromedp.Run(ses,
			chromedp.Navigate(a.cfg.URLOfertas),
			chromedp.WaitReady("body", chromedp.ByQuery),
		); err != nil {
			return err
		}
		// Esperar el listado; si la cuenta no tiene ofertas el selector nunca aparece y no es un error.
		esperarCondicion(ses, `document.querySelectorAll('article.listacheckbox').length > 0`, 15*time.Second, 400*time.Millisecond)

		var u string
		_ = chromedp.Run(ses, chromedp.Location(&u))
		if strings.Contains(u, "Login") {
			return permanente(ErrSesionExpirada)
		}
		return chromedp.Run(ses, chromedp.Evaluate(scriptExtraerOfertas, &ofertas))
	})
	if err != nil {
		// Un bucle de redirecciones es una sesión dañada (cookies viejas): se borran y se pide iniciar
		// sesión de nuevo en vez de fallar para siempre con el mismo perfil.
		if strings.Contains(err.Error(), "ERR_TOO_MANY_REDIRECTS") {
			_ = chromedp.Run(ses, network.ClearBrowserCookies())
			return nil, fmt.Errorf("error al obtener ofertas activas (%v): %w", err, ErrSesionExpirada)
		}
		return nil, fmt.Errorf("error al obtener ofertas activas: %w", err)
	}

	fmt.Printf("[+] Total ofertas activas detectadas: %d\n", len(ofertas))
	return ofertas, nil
}

// ContarCandidatosOferta devuelve cuántas personas se inscribieron a la oferta en Computrabajo,
// recorriendo el listado completo (scroll y paginación) sin abrir la ficha de cada una.
// No cuenta a quienes abandonaron el proceso, igual que el motor.
// Es seguro llamarla en paralelo: cada conteo usa su propia pestaña, sin imágenes ni trackers.
func (a *AgenteComputrabajo) ContarCandidatosOferta(ctx context.Context, urlCandidatos string) (int, error) {
	tabCtx, cancelTab := chromedp.NewContext(a.sesion())
	defer cancelTab()
	detener := context.AfterFunc(ctx, cancelTab)
	defer detener()

	ctxTimeout, cancelTimeout := context.WithTimeout(tabCtx, 3*time.Minute)
	defer cancelTimeout()

	if err := chromedp.Run(ctxTimeout, bloquearRecursosPesados()); err != nil {
		return 0, err
	}
	candidatos, _, err := a.obtenerCandidatos(ctx, ctxTimeout, urlCandidatos, 0, false, nil)
	if err != nil {
		return 0, err
	}
	return len(candidatos), nil
}

// ObtenerCandidatosStream recorre el listado de la oferta y entrega cada candidato a alDescubrir
// apenas aparece en pantalla (durante el scroll), sin esperar a que el listado termine de cargar.
// Los candidatos llegan en orden de descubrimiento con su Indice definitivo (1, 2, 3…); respeta
// limite (0 = sin límite). Usa su propia pestaña, así que puede correr junto a las fichas.
// alDescubrir se invoca desde esta goroutine: si bloquea, el recorrido espera.
func (a *AgenteComputrabajo) ObtenerCandidatosStream(ctx context.Context, urlCandidatos string, limite int, alDescubrir func(*modelos.Candidato)) error {
	tabCtx, cancelTab := chromedp.NewContext(a.sesion())
	defer cancelTab()
	detener := context.AfterFunc(ctx, cancelTab)
	defer detener()

	ctxTimeout, cancelTimeout := context.WithTimeout(tabCtx, 20*time.Minute)
	defer cancelTimeout()

	if err := chromedp.Run(ctxTimeout, bloquearRecursosPesados()); err != nil {
		return err
	}
	_, incompleto, err := a.obtenerCandidatos(ctx, ctxTimeout, urlCandidatos, limite, false, alDescubrir)
	if err == nil && incompleto {
		return ErrListadoIncompleto
	}
	return err
}

type candidatoListado struct {
	Nombre     string `json:"nombre"`
	URLDetalle string `json:"url_detalle"`
	FotoURL    string `json:"foto_url"`
}

type paginaCandidatos struct {
	Candidatos   []candidatoListado `json:"candidatos"`
	OtrasPaginas []string           `json:"otras_paginas"`
}

const scriptExtraerCandidatos = `
	(() => {
		const rows = Array.from(document.querySelectorAll('article.rowuser'));
		const candidatos = [];
		for (const r of rows) {
			// Ignorar postulaciones de usuarios que abandonaron el proceso
			if (r.querySelector('.fila_not_available') || (r.innerText || '').includes('abandonado el proceso')) {
				continue;
			}

			const link = r.querySelector('a[href*="MatchDetail"], a.js-o-link');
			if (!link) continue;

			let href = link.getAttribute('href') || '';
			if (!href.startsWith('http')) {
				href = 'https://empresa.co.computrabajo.com' + href;
			}

			const nombre = link.innerText.trim().split('\n')[0].trim();
			const img = r.querySelector('img[src*="candidates"]');
			const foto = img ? img.getAttribute('src') : '';

			candidatos.push({
				nombre: nombre,
				url_detalle: href,
				foto_url: foto
			});
		}

		// Solo páginas del MISMO listado: misma ruta y misma oferta (oi). Antes cualquier enlace con
		// "p=" o "page=" (filtros, otras ofertas) se visitaba y alargaba el recorrido.
		// Los enlaces con "cf=" son las otras categorías de la oferta (Recibidos, nuevas…): también
		// guardan postulados, así que se recorren igual (los candidatos repetidos se descartan por URL).
		const actual = new URL(window.location.href);
		const oiActual = actual.searchParams.get('oi');
		// Sin fragmento ni orden (by/o): ordenar una categoría por nombre, fecha, etc. muestra los mismos
		// candidatos, y visitar cada variante multiplicaba las páginas por ocho.
		const sinHash = (u) => {
			const c = new URL(u.href);
			c.hash = '';
			c.searchParams.delete('by');
			c.searchParams.delete('o');
			return c.href;
		};
		const paginadores = Array.from(document.querySelectorAll('.pagination a, .pages a, a[href*="p="], a[href*="page="], a[href*="cf="]'));
		const otrasPaginas = [];
		for (const p of paginadores) {
			const href = (p.getAttribute('href') || '').trim();
			if (!href || href.startsWith('javascript') || href.startsWith('#')) continue;
			let u;
			try { u = new URL(href, window.location.href); } catch (e) { continue; }
			if (u.pathname !== actual.pathname) continue;
			const oi = u.searchParams.get('oi');
			if (oiActual && oi && oi !== oiActual) continue;
			const clave = sinHash(u);
			if (clave !== sinHash(actual) && !otrasPaginas.includes(clave)) {
				otrasPaginas.push(clave);
			}
		}

		return { candidatos: candidatos, otras_paginas: otrasPaginas };
	})()
`

// scriptCargarMas hace scroll al final y pulsa el botón "cargar más" si existe. Solo hace clic en
// botones o enlaces sin destino real: un enlace "ver más" con href sacaría la página del listado.
const scriptCargarMas = `
	(() => {
		window.scrollTo(0, document.body.scrollHeight);
		const elementos = Array.from(document.querySelectorAll('button, a'));
		for (const b of elementos) {
			if (b.offsetParent === null) continue;
			const t = (b.innerText || '').toLowerCase();
			const esBoton = b.classList.contains('js_more_candidates') ||
				t.includes('cargar más') || t.includes('ver más') || t.includes('más candidatos');
			if (!esBoton) continue;
			const href = (b.getAttribute('href') || '').trim();
			const sinDestino = b.tagName === 'BUTTON' || href === '' || href.startsWith('#') || href.startsWith('javascript');
			if (sinDestino || b.classList.contains('js_more_candidates')) {
				b.click();
				return true;
			}
		}
		return false;
	})()
`

// normalizarURLPagina quita el fragmento para detectar páginas repetidas.
func normalizarURLPagina(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	return u.String()
}

// ObtenerCandidatosDeOferta extrae TODOS los candidatos inscritos en una oferta, recorriendo scroll y paginación.
// El orden y el índice de los candidatos son deterministas (orden de descubrimiento en la plataforma).
func (a *AgenteComputrabajo) ObtenerCandidatosDeOferta(ctx context.Context, urlCandidatos string, limite int) ([]*modelos.Candidato, error) {
	lista, _, err := a.obtenerCandidatos(ctx, a.sesion(), urlCandidatos, limite, true, nil)
	return lista, err
}

// obtenerCandidatos recorre el listado de una oferta en la pestaña ses. verbose imprime avance por consola.
// Si alDescubrir no es nil, cada candidato nuevo se entrega en cuanto aparece (también mientras se
// hace scroll) y se respeta limite al entregarlos; el resultado devuelto sigue siendo la lista completa.
// incompleto es verdadero si alguna página no se pudo leer o no apareció nadie.
func (a *AgenteComputrabajo) obtenerCandidatos(ctx context.Context, ses context.Context, urlCandidatos string, limite int, verbose bool, alDescubrir func(*modelos.Candidato)) (_ []*modelos.Candidato, incompleto bool, _ error) {
	if verbose {
		fmt.Printf("[*] Navegando a la oferta para extraer todos los candidatos inscritos: %s\n", urlCandidatos)
	}

	var (
		orden     []string
		porURL    = make(map[string]candidatoListado)
		visitadas = make(map[string]bool)
		cola      = []string{urlCandidatos}
		paginas   int
	)

	agregar := func(lista []candidatoListado) {
		for _, c := range lista {
			if c.URLDetalle == "" {
				continue
			}
			if _, ya := porURL[c.URLDetalle]; ya {
				continue
			}
			if alDescubrir != nil && limite > 0 && len(orden) >= limite {
				return
			}
			porURL[c.URLDetalle] = c
			orden = append(orden, c.URLDetalle)
			if alDescubrir != nil {
				alDescubrir(&modelos.Candidato{
					Indice:     len(orden),
					Nombre:     c.Nombre,
					URLDetalle: c.URLDetalle,
					URLFoto:    c.FotoURL,
				})
			}
		}
	}

	// extraerVisibles entrega los candidatos que ya están en el DOM (solo en modo streaming).
	extraerVisibles := func() {
		if alDescubrir == nil {
			return
		}
		var visibles paginaCandidatos
		if err := chromedp.Run(ses, chromedp.Evaluate(scriptExtraerCandidatos, &visibles)); err == nil {
			agregar(visibles.Candidatos)
		}
	}

	for len(cola) > 0 && paginas < maxPaginasOferta {
		if ctx.Err() != nil {
			return nil, true, ctx.Err()
		}
		if limite > 0 && len(orden) >= limite {
			break
		}

		paginaURL := cola[0]
		cola = cola[1:]
		clave := normalizarURLPagina(paginaURL)
		if visitadas[clave] {
			continue
		}
		visitadas[clave] = true
		paginas++

		var pagina paginaCandidatos
		err := reintentar(ctx, 2, 2*time.Second, func(int) error {
			if err := chromedp.Run(ses,
				chromedp.Navigate(paginaURL),
				chromedp.WaitReady("body", chromedp.ByQuery),
			); err != nil {
				return fmt.Errorf("error al navegar a la oferta: %w", err)
			}

			// Esperar la primera fila; una oferta sin postulados simplemente devuelve lista vacía.
			esperarCondicion(ses, `document.querySelectorAll('article.rowuser').length > 0`, 12*time.Second, 120*time.Millisecond)

			a.cargarTodasLasFilas(ctx, ses, limite, extraerVisibles)

			if err := chromedp.Run(ses, chromedp.Evaluate(scriptExtraerCandidatos, &pagina)); err != nil {
				return fmt.Errorf("error extrayendo candidatos del DOM: %w", err)
			}
			return nil
		})
		if err != nil {
			if paginas == 1 {
				return nil, true, err // la página principal es obligatoria
			}
			fmt.Printf("[!] No se pudo leer la página %s: %v\n", paginaURL, err)
			incompleto = true
			continue
		}

		agregar(pagina.Candidatos)
		for _, p := range pagina.OtrasPaginas {
			if !visitadas[normalizarURLPagina(p)] {
				cola = append(cola, p)
			}
		}
	}

	resultado := make([]*modelos.Candidato, 0, len(orden))
	for i, urlDetalle := range orden {
		if limite > 0 && len(resultado) >= limite {
			break
		}
		datos := porURL[urlDetalle]
		resultado = append(resultado, &modelos.Candidato{
			Indice:     i + 1,
			Nombre:     datos.Nombre,
			URLDetalle: urlDetalle,
			URLFoto:    datos.FotoURL,
		})
	}

	if verbose {
		fmt.Printf("[+] Total candidatos extraídos de la oferta: %d (%d páginas)\n", len(resultado), paginas)
	}
	return resultado, incompleto || len(orden) == 0, nil
}

// cargarTodasLasFilas hace scroll y pulsa "cargar más" hasta que el número de filas deja de crecer
// o se alcanza el límite.
func (a *AgenteComputrabajo) cargarTodasLasFilas(ctx context.Context, ses context.Context, limite int, alCrecer func()) {
	contar := func() int {
		var n int
		_ = chromedp.Run(ses, chromedp.Evaluate(`document.querySelectorAll('article.rowuser').length`, &n))
		return n
	}

	previo := contar()
	if alCrecer != nil {
		alCrecer()
	}
	for paso := 0; paso < maxPasosCarga; paso++ {
		if ctx.Err() != nil || (limite > 0 && previo >= limite) {
			return
		}

		var clic bool
		_ = chromedp.Run(ses, chromedp.Evaluate(scriptCargarMas, &clic))

		// Se espera a que lleguen filas nuevas y se continúa en cuanto aparecen (sondeo cada 100 ms)
		// en vez de dormir un tiempo fijo. Si no crecen, se da por terminado el listado: tras un clic
		// en "cargar más" se concede más tiempo porque hay una petición de red de por medio.
		espera := 900 * time.Millisecond
		if clic {
			espera = 3 * time.Second
		}
		crecio := esperarCondicion(ses,
			fmt.Sprintf(`document.querySelectorAll('article.rowuser').length > %d`, previo),
			espera, 100*time.Millisecond)
		if !crecio {
			return
		}
		previo = contar()
		if alCrecer != nil {
			alCrecer()
		}
	}
}

// datosFicha agrupa lo extraído de la ficha HTML del candidato.
type datosFicha struct {
	Doc    string `json:"doc"`
	Edad   int    `json:"edad"` // años que declara la ficha; 0 si no la trae
	Gen    string `json:"gen"`
	URLCV  string `json:"url_cv"`
	Perfil string `json:"perfil"`
}

const scriptDatosFicha = `
	(() => {
		let doc = '';
		let gen = '';
		let edad = 0;

		// 1. Datos de contacto de la ficha: cada <li> trae un icono con title ("Identificación", "Edad",
		// "E-mail"…) y el valor en span.w100. Se lee por la etiqueta del icono, nunca por la forma del
		// número: un celular de 10 dígitos sin separadores se confundiría con una cédula.
		const norm = (s) => (s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
		for (const li of document.querySelectorAll('li')) {
			const icono = li.querySelector('span.icon[title], i.icon[title]');
			const valor = li.querySelector('span.w100');
			if (!icono || !valor) continue;
			const etiqueta = norm(icono.getAttribute('title'));
			const txt = valor.innerText.trim();
			if (!doc && /identificacion|cedula|documento/.test(etiqueta)) {
				// Se aceptan puntos, comas y espacios entre miles ("1.020.486.502"); letras no (pasaportes).
				if (/^[0-9][0-9.,\s]{4,16}$/.test(txt)) doc = txt.replace(/[^0-9]/g, '');
			} else if (!edad && etiqueta === 'edad') {
				const m = txt.match(/^(\d{1,3})\s*a[nñ]os?$/i);
				if (m) edad = parseInt(m[1], 10);
			}
		}

		// 2. Cédula: respaldo con etiqueta explícita en el texto de la ficha (la etiqueta debe ser una
		// palabra completa: "dirección" contiene "cc" y arrastraba números de la dirección).
		if (!doc) {
			const cabecera = document.querySelector('.b_detail, .box_user, .user_header, main')?.innerText || document.body.innerText;
			const matchGeneral = cabecera.match(/(?:\bc\.?c\.?|\bc[eé]dula|\bidentificaci[oó]n|\bdocumento|\bd\.?n\.?i\.?)\s*[:\-#]?\s*([0-9][0-9\.\,\s]{5,14})/i);
			if (matchGeneral) {
				doc = matchGeneral[1].replace(/[^0-9]/g, '');
			}
		}

		// 5. Detección de género
		const selectoresGen = [
			'span[title*="Género"]',
			'span[title*="Genero"]',
			'span[title*="Sexo"]',
			'i[title*="Género"]'
		];
		for (const sel of selectoresGen) {
			const el = document.querySelector(sel);
			if (el) {
				const contenedor = el.closest('li, div, p');
				if (contenedor) {
					const spanW = contenedor.querySelector('span.w100, strong, b, span');
					if (spanW) {
						gen = spanW.innerText.trim();
						break;
					}
				}
			}
		}
		if (!gen) {
			const textoGeneral = document.querySelector('.b_detail, .box_user')?.innerText || '';
			const mGen = textoGeneral.match(/(?:g[eé]nero|sexo)\s*[:\-]?\s*(hombre|mujer|masculino|femenino)/i);
			if (mGen) {
				gen = mGen[1];
			}
		}

		// Búsqueda del enlace de descarga de la hoja de vida original (PDF o DOCX)
		let urlCV = '';
		const btnDescarga = document.querySelector('a.js_download_file, a[href*="CvDownloader"], a[href*="CvDetail/Download"]');
		if (btnDescarga) {
			let h = btnDescarga.getAttribute('href') || btnDescarga.href || '';
			if (h && !h.startsWith('http')) {
				h = 'https://empresa.co.computrabajo.com' + h;
			}
			urlCV = h;
		}
		if (!urlCV) {
			const enlaces = Array.from(document.querySelectorAll('a'));
			for (const a of enlaces) {
				const h = a.getAttribute('href') || '';
				if (h.includes('CvDownloader') || h.includes('CvDetail/Download')) {
					urlCV = h.startsWith('http') ? h : ('https://empresa.co.computrabajo.com' + h);
					break;
				}
			}
		}

		// 6. Extraer texto completo del perfil (experiencia, educación, habilidades)
		let perfil = '';
		const secciones = document.querySelectorAll('#candidato, .box_detail, .b_detail, .box_m, article, section, .box');
		const textos = new Set();
		secciones.forEach(sec => {
			const t = sec.innerText.trim();
			if (t.length > 20 && !textos.has(t)) {
				textos.add(t);
			}
		});
		if (textos.size > 0) {
			perfil = Array.from(textos).join('\n\n');
		} else {
			const main = document.querySelector('main, #candidato, body');
			if (main) perfil = main.innerText;
		}
		// Limitar a 5000 chars para no sobrecargar
		if (perfil.length > 5000) perfil = perfil.substring(0, 5000);

		return { doc: doc, edad: edad, gen: gen, url_cv: urlCV, perfil: perfil };
	})()
`

// fichaCandidato es el resultado completo de visitar la ficha de un postulante.
type fichaCandidato struct {
	Datos   datosFicha
	PDF     []byte
	Captura []byte // JPEG de la ficha tal como se ve en pantalla
	RutaCV  string
}

// cargarFicha abre una pestaña propia para el candidato, extrae datos, genera el PDF del perfil y
// descarga la hoja de vida original. La pestaña se cierra si ctx se cancela.
func (a *AgenteComputrabajo) cargarFicha(ctx context.Context, urlDetalle, carpeta, nombreLimpio string) (*fichaCandidato, error) {
	// Una pestaña independiente por candidato evita ERR_ABORTED por navegaciones simultáneas.
	tabCtx, cancelTab := chromedp.NewContext(a.sesion())
	defer cancelTab()
	detener := context.AfterFunc(ctx, cancelTab) // propagar la cancelación del llamador
	defer detener()

	ctxTimeout, cancelTimeout := context.WithTimeout(tabCtx, 90*time.Second)
	defer cancelTimeout()

	ficha := &fichaCandidato{}
	var cookies []*network.Cookie

	err := chromedp.Run(ctxTimeout,
		chromedp.Navigate(urlDetalle),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("error navegando a la ficha: %w", err)
	}

	// Esperar a que la ficha renderice contenido real (la página es dinámica) en vez de dormir fijo.
	esperarCondicion(ctxTimeout,
		`document.readyState === 'complete' && (document.body.innerText || '').length > 300`,
		10*time.Second, 250*time.Millisecond)

	err = chromedp.Run(ctxTimeout,
		chromedp.Evaluate(scriptDatosFicha, &ficha.Datos),
		chromedp.ActionFunc(func(c context.Context) error {
			// Cookies de la sesión autenticada para descargar el archivo adjunto
			if cks, err := network.GetCookies().Do(c); err == nil {
				cookies = cks
			}

			// Captura rápida de la ficha (evidencia visual del perfil y del género)
			ficha.Captura = capturarEnAccion(c)

			// Generar PDF nativo del perfil de Computrabajo
			buf, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(c)
			if err == nil && len(buf) > 0 {
				ficha.PDF = buf
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error extrayendo detalle de candidato: %w", err)
	}

	// Una ficha sin contenido indica que la página no cargó bien: se reintenta con pestaña nueva.
	if len(strings.TrimSpace(ficha.Datos.Perfil)) < 50 && len(ficha.PDF) == 0 {
		return nil, fmt.Errorf("la ficha del candidato llegó vacía")
	}

	// Descargar hoja de vida original (HTTP directo con cookies; fallback vía fetch en el navegador)
	if ficha.Datos.URLCV != "" {
		ruta, errDesc := a.descargarArchivoOriginal(ctx, ficha.Datos.URLCV, cookies, carpeta, nombreLimpio)
		if errDesc != nil {
			// Se conservan los dos errores: el log debe mostrar por qué falló cada vía, no solo la última.
			errDirecto := errDesc
			ruta, errDesc = a.descargarCVViaBrowser(ctxTimeout, ficha.Datos.URLCV, carpeta, nombreLimpio)
			if errDesc != nil {
				errDesc = fmt.Errorf("HTTP directo: %v; navegador: %w", errDirecto, errDesc)
			}
		}
		if errDesc == nil {
			ficha.RutaCV = ruta
		} else {
			fmt.Printf("[!] No se pudo descargar la hoja de vida de %s: %v\n", nombreLimpio, errDesc)
		}
	}

	return ficha, nil
}

// ExtraerDetalleCandidato navega a la ficha del postulante, extrae cédula, género, descarga el CV original (PDF o DOCX) y genera el perfil impreso.
func (a *AgenteComputrabajo) ExtraerDetalleCandidato(ctx context.Context, c *modelos.Candidato, carpetaCandidato string) error {
	nombreLimpio := LimpiarNombre(c.Nombre)

	var ficha *fichaCandidato
	err := reintentar(ctx, 2, 3*time.Second, func(int) error {
		f, err := a.cargarFicha(ctx, c.URLDetalle, carpetaCandidato, nombreLimpio)
		if err != nil {
			return err
		}
		ficha = f
		return nil
	})
	if err != nil {
		return fmt.Errorf("error extrayendo detalle de candidato: %w", err)
	}

	// La cédula se busca por varias vías, de la más confiable a la menos; cada intento queda
	// registrado para saber, si no aparece, qué se probó antes de pedírsela al candidato.
	c.IntentosCedula = nil
	c.Cedula = "No especificada"
	intento := func(metodo, cedula string) {
		c.IntentosCedula = append(c.IntentosCedula, modelos.IntentoCedula{Metodo: metodo, Hallada: cedula != ""})
		if cedula != "" && c.Cedula == "No especificada" {
			c.Cedula = cedula
		}
	}
	doc := soloDigitos(ficha.Datos.Doc)
	if len(doc) < 6 || len(doc) > 12 {
		doc = ""
	}
	intento("ficha_html", doc)

	// La edad solo se toma tal cual la declara la ficha; fuera de un rango laboral plausible se descarta.
	c.Edad = 0
	if e := ficha.Datos.Edad; e >= 16 && e <= 85 {
		c.Edad = e
	}

	c.CarpetaCandidato = carpetaCandidato
	GuardarCaptura(c, "perfil", ficha.Captura)
	AliasCaptura(c, "genero", "perfil") // el género se lee de esta misma ficha

	c.TextoPerfil = ficha.Datos.Perfil
	if g :=strings.TrimSpace(ficha.Datos.Gen); g != "" && c.Genero == "" {
		c.Genero = g
	}

	// 1. Guardar perfil nativo de Computrabajo en PDF
	rutaPerfilPDF := filepath.Join(carpetaCandidato, fmt.Sprintf("perfil_computrabajo_%s.pdf", nombreLimpio))
	if len(ficha.PDF) > 0 {
		if err := utilidades.EscribirArchivoAtomico(rutaPerfilPDF, ficha.PDF, 0o644); err == nil {
			c.RutaPerfilPDF = rutaPerfilPDF
			c.RutaPDF = rutaPerfilPDF
		}
	}

	// 2. Hoja de vida original (PDF o DOCX)
	if ficha.Datos.URLCV != "" {
		c.URLDescargaCV = ficha.Datos.URLCV
	}
	if ficha.RutaCV != "" {
		c.RutaCVOriginal = ficha.RutaCV
		if strings.HasSuffix(strings.ToLower(ficha.RutaCV), ".pdf") {
			c.RutaPDF = ficha.RutaCV
		}
	}

	// 3. Si la ficha no la trae, buscarla en los textos: hoja de vida (PDF o Word), perfil impreso y
	// el texto de la ficha. Solo se prueba la siguiente vía si la anterior no la encontró.
	sinCedula := func() bool { return c.Cedula == "No especificada" }
	textoDe := func(ruta string, leer func(string) (string, error)) string {
		if ruta == "" {
			return ""
		}
		texto, err := leer(ruta)
		if err != nil {
			return ""
		}
		return texto
	}
	if sinCedula() && strings.HasSuffix(strings.ToLower(c.RutaCVOriginal), ".pdf") {
		intento("texto_pdf_cv", cedulaEnTexto(textoDe(c.RutaCVOriginal, utilidades.ExtraerTextoPDF)))
	}
	if sinCedula() && strings.HasSuffix(strings.ToLower(c.RutaCVOriginal), ".docx") {
		intento("texto_docx", cedulaEnTexto(textoDe(c.RutaCVOriginal, utilidades.ExtraerTextoDocx)))
	}
	if sinCedula() && c.RutaPerfilPDF != "" {
		intento("texto_pdf_perfil", cedulaEnTexto(textoDe(c.RutaPerfilPDF, utilidades.ExtraerTextoPDF)))
	}
	if sinCedula() {
		intento("texto_perfil", cedulaEnTexto(c.TextoPerfil))
	}

	return nil
}

// cedulaEnTexto busca en un texto libre el primer número de documento plausible (6 a 12 dígitos)
// que siga a una etiqueta como "C.C.", "cédula" o "documento". Toma el primer grupo de la captura
// para no pegarle a la cédula el número que venga después ("C.C. 1035126774 311 234 5678").
func cedulaEnTexto(texto string) string {
	for _, m := range reDocumentoTexto.FindAllStringSubmatch(texto, -1) {
		if len(m) < 2 {
			continue
		}
		captura := strings.TrimSpace(m[1])
		candidatas := []string{captura}
		if grupos := strings.Fields(captura); len(grupos) > 1 {
			candidatas = []string{grupos[0], captura}
		}
		for _, candidata := range candidatas {
			if n := soloDigitos(candidata); len(n) >= 6 && len(n) <= 12 {
				return n
			}
		}
	}
	return ""
}

// detectarExtension decide .pdf/.docx/.doc por cabeceras HTTP y, si no son concluyentes, por los
// bytes mágicos del archivo.
func detectarExtension(cuerpo []byte, disposicion, tipoContenido string) string {
	disp := strings.ToLower(disposicion)
	ct := strings.ToLower(tipoContenido)

	switch {
	case strings.Contains(disp, ".docx") || strings.Contains(ct, "wordprocessingml"):
		return ".docx"
	case strings.Contains(disp, ".doc") || strings.Contains(ct, "msword"):
		return ".doc"
	}

	if len(cuerpo) >= 4 {
		switch {
		case cuerpo[0] == 0x25 && cuerpo[1] == 0x50 && cuerpo[2] == 0x44 && cuerpo[3] == 0x46: // %PDF
			return ".pdf"
		case cuerpo[0] == 0x50 && cuerpo[1] == 0x4B && cuerpo[2] == 0x03 && cuerpo[3] == 0x04: // PK.. (ZIP/DOCX)
			return ".docx"
		case cuerpo[0] == 0xD0 && cuerpo[1] == 0xCF && cuerpo[2] == 0x11 && cuerpo[3] == 0xE0: // OLE2 (DOC)
			return ".doc"
		}
	}
	return ".pdf"
}

// pareceHTML detecta cuando el servidor devolvió una página (p. ej. login) en lugar del archivo.
func pareceHTML(cuerpo []byte, tipoContenido string) bool {
	if strings.Contains(strings.ToLower(tipoContenido), "text/html") {
		return true
	}
	inicio := strings.ToLower(strings.TrimSpace(string(cuerpo[:min(len(cuerpo), 64)])))
	return strings.HasPrefix(inicio, "<!doctype html") || strings.HasPrefix(inicio, "<html")
}

// guardarCV valida el contenido descargado y lo persiste de forma atómica.
func guardarCV(cuerpo []byte, disposicion, tipoContenido, carpeta, nombreCandidato string) (string, error) {
	if len(cuerpo) < 10 {
		return "", fmt.Errorf("el archivo descargado está vacío o corrupto")
	}
	if pareceHTML(cuerpo, tipoContenido) {
		return "", fmt.Errorf("el servidor devolvió una página HTML en lugar del archivo (¿sesión expirada?)")
	}
	extension := detectarExtension(cuerpo, disposicion, tipoContenido)
	ruta := filepath.Join(carpeta, fmt.Sprintf("hoja_de_vida_original_%s%s", nombreCandidato, extension))
	if err := utilidades.EscribirArchivoAtomico(ruta, cuerpo, 0o644); err != nil {
		return "", fmt.Errorf("error guardando hoja de vida original: %w", err)
	}
	return ruta, nil
}

// descargarCVViaBrowser usa fetch dentro del navegador (con las cookies de sesión) y recibe el
// archivo como base64 mediante FileReader: mucho más liviano que serializar un array de bytes.
func (a *AgenteComputrabajo) descargarCVViaBrowser(
	ctx context.Context,
	urlDescarga string,
	carpetaDestino string,
	nombreCandidato string,
) (string, error) {
	if urlDescarga == "" {
		return "", fmt.Errorf("URL de descarga vacía")
	}

	scriptFetch := fmt.Sprintf(`
		(async () => {
			try {
				const resp = await fetch(%s, { credentials: 'include' });
				if (!resp.ok) return { error: 'HTTP ' + resp.status };
				const blob = await resp.blob();
				const dataUrl = await new Promise((ok, fail) => {
					const r = new FileReader();
					r.onload = () => ok(r.result);
					r.onerror = () => fail(r.error);
					r.readAsDataURL(blob);
				});
				return {
					base64: String(dataUrl).split(',')[1] || '',
					contentType: resp.headers.get('content-type') || '',
					disposition: resp.headers.get('content-disposition') || ''
				};
			} catch (e) {
				return { error: String(e && e.message || e) };
			}
		})()
	`, literalJS(urlDescarga))

	var resultado struct {
		Error       string `json:"error"`
		Base64      string `json:"base64"`
		ContentType string `json:"contentType"`
		Disposition string `json:"disposition"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(scriptFetch, &resultado, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return "", fmt.Errorf("error ejecutando fetch en browser: %w", err)
	}
	if resultado.Error != "" {
		return "", fmt.Errorf("error en fetch: %s", resultado.Error)
	}

	cuerpo, err := base64.StdEncoding.DecodeString(resultado.Base64)
	if err != nil {
		return "", fmt.Errorf("respuesta fetch inválida: %w", err)
	}
	if len(cuerpo) > maxBytesCV {
		return "", fmt.Errorf("archivo demasiado grande (%d bytes)", len(cuerpo))
	}
	return guardarCV(cuerpo, resultado.Disposition, resultado.ContentType, carpetaDestino, nombreCandidato)
}

// descargarArchivoOriginal descarga el archivo adjunto (PDF o DOCX) usando las cookies de la sesión
// activa de Chrome, con reintentos ante errores transitorios (5xx, 429, cortes de red).
func (a *AgenteComputrabajo) descargarArchivoOriginal(
	ctx context.Context,
	urlDescarga string,
	cookies []*network.Cookie,
	carpetaDestino string,
	nombreCandidato string,
) (string, error) {
	if urlDescarga == "" {
		return "", fmt.Errorf("URL de descarga vacía")
	}

	var rutaFinal string
	err := reintentar(ctx, 3, time.Second, func(int) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlDescarga, nil)
		if err != nil {
			return permanente(err)
		}
		req.Header.Set("User-Agent", userAgentChrome)
		req.Header.Set("Referer", "https://empresa.co.computrabajo.com/")

		for _, ck := range cookies {
			// Sanitizar valor para asegurar compatibilidad estricta con HTTP
			valLimpio := strings.Map(func(r rune) rune {
				if r < 32 || r >= 127 {
					return -1
				}
				return r
			}, ck.Value)
			if valLimpio != "" {
				req.AddCookie(&http.Cookie{Name: ck.Name, Value: valLimpio})
			}
		}

		resp, err := a.http.Do(req)
		if err != nil {
			return fmt.Errorf("error al descargar archivo adjunto: %w", err)
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return fmt.Errorf("código HTTP transitorio al descargar archivo: %d", resp.StatusCode)
		case resp.StatusCode != http.StatusOK:
			return permanente(fmt.Errorf("código HTTP no exitoso al descargar archivo: %d", resp.StatusCode))
		}

		cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxBytesCV+1))
		if err != nil {
			return fmt.Errorf("error leyendo bytes del archivo descargado: %w", err)
		}
		if len(cuerpo) > maxBytesCV {
			return permanente(fmt.Errorf("archivo demasiado grande (> %d MB)", maxBytesCV>>20))
		}

		ruta, err := guardarCV(cuerpo, resp.Header.Get("Content-Disposition"), resp.Header.Get("Content-Type"), carpetaDestino, nombreCandidato)
		if err != nil {
			return permanente(err)
		}
		rutaFinal = ruta
		return nil
	})
	if err != nil {
		return "", err
	}
	return rutaFinal, nil
}

// LimpiarNombre sanitiza cadenas para su uso como nombres de archivo y carpeta.
func LimpiarNombre(texto string) string {
	limpio := reCaracteresInvalidos.ReplaceAllString(strings.TrimSpace(texto), "_")
	limpio = reEspacios.ReplaceAllString(limpio, "_")
	// Windows no admite nombres terminados en punto o espacio.
	return strings.TrimRight(limpio, ". ")
}
