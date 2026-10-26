package notificaciones

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelefonoInternacional(t *testing.T) {
	for entrada, esperado := range map[string]string{"3002473899": "573002473899", "+57 300 247 3899": "573002473899", "573002473899": "573002473899"} {
		if got := TelefonoInternacional(entrada); got != esperado {
			t.Errorf("%q -> %q, esperaba %q", entrada, got, esperado)
		}
	}
}

func TestWhatsAppEnviaPlantilla(t *testing.T) {
	var cuerpo map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("sin token: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&cuerpo)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer srv.Close()

	err := NuevoWhatsApp(ConfigWhatsApp{URL: srv.URL, Token: "tok"}).Enviar(context.Background(), Mensaje{
		Destino: "3002473899", Plantilla: "p", Variables: map[string]string{"nombre": "Daniel\nOchoa", "url": "https://x.co/validar-datos/1/x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cuerpo)
	for _, quiero := range []string{`"to":"573002473899"`, `"name":"p"`, `"code":"es"`, `"parameter_name":"nombre"`, `"text":"Daniel Ochoa"`, `"parameter_name":"url"`, `"text":"https://x.co/validar-datos/1/x"`} {
		if !strings.Contains(string(b), quiero) {
			t.Errorf("falta %s en %s", quiero, b)
		}
	}

	if err := NuevoWhatsApp(ConfigWhatsApp{}).Enviar(context.Background(), Mensaje{Destino: "3002473899", Plantilla: "p"}); err == nil {
		t.Error("sin configuración debía fallar")
	}
}
