package agentes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// paginaAdresFalsa imita el formulario de ADRES: el botón envía el formulario a una pestaña nueva
// (target=_blank) y la página anota si el clic fue confiable y tuvo gesto de usuario.
const paginaAdresFalsa = `<!doctype html><html><body style="margin:0">
<div style="height:900px"></div>
<form id="afiliado" action="/RespuestaConsulta" method="post" target="_blank">
  <select id="tipoDoc"><option value="">-</option><option value="CC">CC</option></select>
  <input id="txtNumDoc" name="doc">
  <input type="submit" id="btnConsultar" value="Consultar">
</form>
<script>
  window.__clic = null;
  document.getElementById('btnConsultar').addEventListener('click', function (e) {
    window.__clic = { confiable: e.isTrusted, gesto: navigator.userActivation.isActive };
  });
</script></body></html>`

func chromeParaPruebas(t *testing.T) context.Context {
	t.Helper()
	if _, err := exec.LookPath("chrome"); err != nil {
		if _, err := os.Stat(`C:\Program Files\Google\Chrome\Application\chrome.exe`); err != nil {
			if _, err := exec.LookPath("google-chrome"); err != nil {
				t.Skip("no hay Chrome instalado")
			}
		}
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opcionesChrome(true)...)
	t.Cleanup(cancelAlloc)
	ctx, cancel := chromedp.NewContext(alloc)
	t.Cleanup(cancel)
	ctx, cancelPlazo := context.WithTimeout(ctx, 60*time.Second)
	t.Cleanup(cancelPlazo)
	return ctx
}

// Regresión: el envío de la consulta a ADRES debe ser un clic real del ratón. Con btn.click() de
// JavaScript (isTrusted=false, sin gesto de usuario) Chrome bloquea la pestaña de resultados y
// reCAPTCHA no genera el token.
func TestEnvioAdresEsUnClicConfiableYAbrePestanaDeResultados(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/RespuestaConsulta" {
			_, _ = w.Write([]byte("<html><body>resultado</body></html>"))
			return
		}
		_, _ = w.Write([]byte(paginaAdresFalsa))
	}))
	defer servidor.Close()
	ctx := chromeParaPruebas(t)

	if err := chromedp.Run(ctx, chromedp.Navigate(servidor.URL), chromedp.WaitReady("body")); err != nil {
		t.Fatal(err)
	}
	var antes []*target.Info
	_ = chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) (err error) {
		antes, err = target.GetTargets().Do(c)
		return err
	}))

	if err := enviarConsultaAdres(ctx, "1140845928"); err != nil {
		t.Fatal(err)
	}

	var clic struct {
		Confiable bool `json:"confiable"`
		Gesto     bool `json:"gesto"`
	}
	var valores struct {
		Tipo   string `json:"tipo"`
		Numero string `json:"numero"`
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`window.__clic || {}`, &clic),
		chromedp.Evaluate(`({ tipo: document.getElementById('tipoDoc').value, numero: document.getElementById('txtNumDoc').value })`, &valores),
	); err != nil {
		t.Fatal(err)
	}
	if !clic.Confiable || !clic.Gesto {
		t.Errorf("el clic debe ser confiable y con gesto de usuario: %+v", clic)
	}
	if valores.Tipo != "CC" || valores.Numero != "1140845928" {
		t.Errorf("formulario sin llenar: %+v", valores)
	}

	// La pestaña de resultados (target=_blank) debe abrirse.
	nueva := false
	conocidas := map[target.ID]bool{}
	for _, t := range antes {
		conocidas[t.TargetID] = true
	}
	for i := 0; i < 20 && !nueva; i++ {
		time.Sleep(250 * time.Millisecond)
		var ahora []*target.Info
		_ = chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) (err error) {
			ahora, err = target.GetTargets().Do(c)
			return err
		}))
		for _, info := range ahora {
			if !conocidas[info.TargetID] && info.Type == "page" {
				nueva = true
			}
		}
	}
	if !nueva {
		t.Error("no se abrió la pestaña de resultados (target=_blank): Chrome la bloqueó")
	}
}

// Documenta la causa: un btn.click() de JavaScript NO es confiable ni cuenta como gesto de usuario.
func TestClicSinteticoDeJavaScriptNoEsConfiable(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(paginaAdresFalsa))
	}))
	defer servidor.Close()
	ctx := chromeParaPruebas(t)

	var clic struct {
		Confiable bool `json:"confiable"`
		Gesto     bool `json:"gesto"`
	}
	if err := chromedp.Run(ctx,
		chromedp.Navigate(servidor.URL), chromedp.WaitReady("body"),
		chromedp.Evaluate(`document.getElementById('btnConsultar').click()`, nil),
		chromedp.Evaluate(`window.__clic || {}`, &clic),
	); err != nil {
		t.Fatal(err)
	}
	if clic.Confiable || clic.Gesto {
		t.Errorf("un clic de JavaScript no debería ser confiable (si esto falla, Chrome cambió y la causa es otra): %+v", clic)
	}
}
