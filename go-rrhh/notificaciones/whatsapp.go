package notificaciones

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ConfigWhatsApp reúne los datos de la API de nube de WhatsApp (Meta): URL del número emisor
// (…/v23.0/<id>/messages), token de acceso e idioma de las plantillas.
type ConfigWhatsApp struct {
	URL, Token, Idioma string
}

// WhatsApp envía mensajes de plantilla aprobada (Mensaje.Plantilla/Variables) por la API de Meta.
// shortcut: sin cola ni reintentos ni seguimiento de entrega (webhooks); agregar si el volumen crece.
type WhatsApp struct {
	cfg     ConfigWhatsApp
	cliente *http.Client
}

func NuevoWhatsApp(cfg ConfigWhatsApp) *WhatsApp {
	if cfg.Idioma == "" {
		cfg.Idioma = "es"
	}
	return &WhatsApp{cfg: cfg, cliente: &http.Client{Timeout: 30 * time.Second}}
}

var reNoDigito = regexp.MustCompile(`\D`)

// TelefonoInternacional devuelve el número con indicativo para WhatsApp: un celular colombiano de
// 10 dígitos (3XXXXXXXXX) recibe el 57; un número que ya trae indicativo se deja igual.
func TelefonoInternacional(tel string) string {
	d := reNoDigito.ReplaceAllString(tel, "")
	if len(d) == 10 && d[0] == '3' {
		return "57" + d
	}
	return d
}

// limpiarVariable evita lo que Meta rechaza en una variable: saltos de línea, tabulaciones y 4+ espacios seguidos.
func limpiarVariable(s string) string { return strings.Join(strings.Fields(s), " ") }

func (w *WhatsApp) Enviar(ctx context.Context, m Mensaje) error {
	if w.cfg.URL == "" || w.cfg.Token == "" || m.Plantilla == "" {
		return fmt.Errorf("%w: faltan WHATSAPP_API_URL / ACCESS_TOKEN_WHATSAPP", ErrNoConfigurado)
	}
	destino := TelefonoInternacional(m.Destino)
	if len(destino) < 10 {
		return fmt.Errorf("teléfono destino inválido")
	}

	type param map[string]string
	cuerpo := []param{}
	for nombre, valor := range m.Variables {
		cuerpo = append(cuerpo, param{"type": "text", "parameter_name": nombre, "text": limpiarVariable(valor)})
	}
	componentes := []any{map[string]any{"type": "body", "parameters": cuerpo}}
	carga, err := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                destino,
		"type":              "template",
		"template":          map[string]any{"name": m.Plantilla, "language": map[string]string{"code": w.cfg.Idioma}, "components": componentes},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.URL, bytes.NewReader(carga))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	// El enlace lleva el token del candidato (una credencial): no se escribe en el log.
	log.Printf("[whatsapp] enviando plantilla %q a %s", m.Plantilla, destino)
	inicio := time.Now()
	resp, err := w.cliente.Do(req)
	if err != nil {
		log.Printf("[whatsapp] FALLÓ el envío a %s tras %s: %v", destino, time.Since(inicio).Round(time.Millisecond), err)
		return err
	}
	defer resp.Body.Close()
	cuerpoResp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode/100 != 2 {
		log.Printf("[whatsapp] FALLÓ el envío a %s (%d): %s", destino, resp.StatusCode, cuerpoResp)
		return fmt.Errorf("whatsapp respondió %d", resp.StatusCode)
	}
	log.Printf("[whatsapp] Meta aceptó el mensaje para %s en %s: %s", destino, time.Since(inicio).Round(time.Millisecond), cuerpoResp)
	return nil
}
