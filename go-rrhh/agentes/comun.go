package agentes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const userAgentChrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

var (
	reNoDigitos = regexp.MustCompile(`[^0-9]`)
	reEspacios  = regexp.MustCompile(`\s+`)
)

// soloDigitos elimina todo carácter que no sea un dígito.
func soloDigitos(texto string) string {
	return reNoDigitos.ReplaceAllString(texto, "")
}

// opcionesChrome devuelve las opciones base de Chrome compartidas por todos los scrapers.
// Copia el slice de opciones por defecto para no mutar el arreglo global de chromedp.
func opcionesChrome(headless bool, extra ...chromedp.ExecAllocatorOption) []chromedp.ExecAllocatorOption {
	opts := make([]chromedp.ExecAllocatorOption, 0, len(chromedp.DefaultExecAllocatorOptions)+10+len(extra))
	opts = append(opts, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("mute-audio", true),
		chromedp.UserAgent(userAgentChrome),
	)
	return append(opts, extra...)
}

// literalJS convierte una cadena Go en un literal JavaScript seguro (comillas y saltos de línea
// escapados). Evita romper o inyectar código al interpolar datos en scripts.
func literalJS(texto string) string {
	b, _ := json.Marshal(texto)
	return string(b)
}

// dormir espera d respetando la cancelación del contexto. Devuelve false si se canceló.
func dormir(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// errPermanente marca errores que no tiene sentido reintentar (credenciales, 4xx, etc.).
type errPermanente struct{ err error }

func (e errPermanente) Error() string { return e.err.Error() }
func (e errPermanente) Unwrap() error { return e.err }

func permanente(err error) error {
	if err == nil {
		return nil
	}
	return errPermanente{err: err}
}

// reintentar ejecuta fn hasta `intentos` veces con espera creciente (espera, 2*espera, ...).
// Se detiene ante errores permanentes o cancelación del contexto.
func reintentar(ctx context.Context, intentos int, espera time.Duration, fn func(intento int) error) error {
	var err error
	for i := 1; i <= intentos; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = fn(i); err == nil {
			return nil
		}
		var perm errPermanente
		if errors.As(err, &perm) {
			return perm.err
		}
		if i < intentos && !dormir(ctx, espera*time.Duration(i)) {
			return ctx.Err()
		}
	}
	return err
}

// esperarCondicion evalúa js (debe devolver booleano) cada `intervalo` hasta que sea verdadero
// o se agote `timeout`. Sustituye a los Sleep fijos: avanza apenas la página está lista.
// Requiere que el contexto ya haya sido inicializado con al menos un chromedp.Run.
func esperarCondicion(ctx context.Context, js string, timeout, intervalo time.Duration) bool {
	limite := time.Now().Add(timeout)
	for {
		var ok bool
		cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		err := chromedp.Run(cctx, chromedp.Evaluate(js, &ok))
		cancel()
		if err == nil && ok {
			return true
		}
		if ctx.Err() != nil || time.Now().After(limite) {
			return false
		}
		if !dormir(ctx, intervalo) {
			return false
		}
	}
}

// esperarTextoEstable espera a que el texto del selector deje de cambiar (misma longitud en dos
// lecturas consecutivas) o se agote el tiempo. Útil para SPAs que pintan la respuesta por partes.
func esperarTextoEstable(ctx context.Context, selector string, timeout time.Duration) {
	limite := time.Now().Add(timeout)
	js := fmt.Sprintf(`(document.querySelector(%s)?.innerText || '').length`, literalJS(selector))
	previo, iguales := -1, 0
	for time.Now().Before(limite) {
		var n int
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := chromedp.Run(cctx, chromedp.Evaluate(js, &n))
		cancel()
		if err != nil {
			return
		}
		if n == previo {
			iguales++
			if iguales >= 2 {
				return
			}
		} else {
			previo, iguales = n, 0
		}
		if !dormir(ctx, 300*time.Millisecond) {
			return
		}
	}
}

// esperarURL sondea la URL actual hasta que cond sea verdadera o se agote el tiempo.
func esperarURL(ctx context.Context, cond func(string) bool, timeout time.Duration) (string, bool) {
	limite := time.Now().Add(timeout)
	var u string
	for {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_ = chromedp.Run(cctx, chromedp.Location(&u))
		cancel()
		if cond(u) {
			return u, true
		}
		if ctx.Err() != nil || time.Now().After(limite) || !dormir(ctx, 300*time.Millisecond) {
			return u, false
		}
	}
}

// patronesRecursosPesados son recursos que un scraper de texto no necesita: imágenes, fuentes y
// trackers. Bloquearlos acorta mucho la carga de páginas con decenas de fotos de candidatos.
var patronesRecursosPesados = []string{
	"*://*:*/*.png", "*://*:*/*.jpg", "*://*:*/*.jpeg", "*://*:*/*.gif", "*://*:*/*.webp", "*://*:*/*.svg", "*://*:*/*.ico",
	"*://*:*/*.woff", "*://*:*/*.woff2", "*://*:*/*.ttf", "*://*:*/*.mp4",
	"*://ii.ct-stc.com:*/*", // CDN de imágenes de Computrabajo (fotos de candidatos)
	"*://*.google-analytics.com:*/*", "*://*.googletagmanager.com:*/*", "*://*.doubleclick.net:*/*",
	"*://*.facebook.net:*/*", "*://*.hotjar.com:*/*", "*://*.clarity.ms:*/*",
}

// bloquearRecursosPesados es una acción chromedp que activa el bloqueo en la pestaña actual.
// Es de mejor esfuerzo: si el navegador no lo soporta, la página simplemente carga completa.
func bloquearRecursosPesados() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := network.Enable().Do(ctx); err != nil {
			return nil
		}
		patrones := make([]*network.BlockPattern, len(patronesRecursosPesados))
		for i, p := range patronesRecursosPesados {
			patrones[i] = &network.BlockPattern{URLPattern: p, Block: true}
		}
		_ = network.SetBlockedURLs().WithURLPatterns(patrones).Do(ctx)
		return nil
	})
}

// adquirir toma un cupo del semáforo respetando la cancelación del contexto.
// Devuelve la función de liberación o nil si el contexto se canceló antes de obtener el cupo.
func adquirir(ctx context.Context, sem chan struct{}) func() {
	select {
	case sem <- struct{}{}:
		return func() { <-sem }
	case <-ctx.Done():
		return nil
	}
}

// nuevoClienteHTTP crea un cliente con pool de conexiones reutilizable (keep-alive) para
// no abrir una conexión TCP/TLS nueva en cada consulta.
func nuevoClienteHTTP(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}
