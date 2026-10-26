// Package notificaciones envía mensajes a candidatos por un canal intercambiable (correo hoy,
// WhatsApp mañana): cada canal implementa Notificador y el resto del sistema no cambia.
package notificaciones

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Mensaje es el contenido a entregar; Destino es una dirección de correo, un teléfono, etc. según el canal.
type Mensaje struct {
	Destino string
	Asunto  string
	Cuerpo  string
	// HTML es opcional: si viene, el correo sale multipart con Cuerpo como alternativa en texto plano.
	HTML string

	// Solo canales de plantilla (WhatsApp): nombre de la plantilla aprobada y variables nombradas del cuerpo.
	Plantilla string
	Variables map[string]string
}

// Notificador entrega un mensaje por un canal concreto.
type Notificador interface {
	Enviar(ctx context.Context, m Mensaje) error
}

// ErrNoConfigurado indica que el canal no tiene las credenciales necesarias.
var ErrNoConfigurado = errors.New("canal de notificación no configurado")

// ConfigSMTP reúne los datos del servidor de correo saliente.
type ConfigSMTP struct {
	Host, Puerto, Usuario, Clave, Remitente string
}

// SMTP envía correos con net/smtp (STARTTLS automático si el servidor lo ofrece, p. ej. puerto 587).
// shortcut: no soporta TLS implícito (puerto 465) ni reintentos/cola; migrar a un proveedor transaccional
// con cola si el volumen crece.
type SMTP struct {
	cfg ConfigSMTP
}

func NuevoSMTP(cfg ConfigSMTP) *SMTP {
	if cfg.Puerto == "" {
		cfg.Puerto = "587"
	}
	if cfg.Remitente == "" {
		cfg.Remitente = cfg.Usuario
	}
	return &SMTP{cfg: cfg}
}

// escribirBase64 vuelca el texto en base64 con líneas de 76 caracteres (RFC 2045).
func escribirBase64(w *strings.Builder, texto string) {
	b := base64.StdEncoding.EncodeToString([]byte(texto))
	for len(b) > 76 {
		w.WriteString(b[:76] + "\r\n")
		b = b[76:]
	}
	w.WriteString(b + "\r\n")
}

func (s *SMTP) Enviar(ctx context.Context, m Mensaje) error {
	if s.cfg.Host == "" || s.cfg.Remitente == "" {
		return fmt.Errorf("%w: faltan SMTP_HOST / SMTP_REMITENTE", ErrNoConfigurado)
	}
	// ParseAddress rechaza saltos de línea: evita inyección de cabeceras con un destino manipulado.
	destino, err := mail.ParseAddress(m.Destino)
	if err != nil {
		return fmt.Errorf("correo destino inválido: %w", err)
	}
	remitente, err := mail.ParseAddress(s.cfg.Remitente)
	if err != nil {
		return fmt.Errorf("%w: SMTP_REMITENTE inválido", ErrNoConfigurado)
	}

	var msg strings.Builder
	msg.WriteString("From: " + remitente.String() + "\r\n")
	msg.WriteString("To: " + destino.String() + "\r\n")
	msg.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", strings.NewReplacer("\r", " ", "\n", " ").Replace(m.Asunto)) + "\r\n")
	// Message-ID propio: permite rastrear el correo en los logs del servidor SMTP.
	aleatorio := make([]byte, 8)
	_, _ = rand.Read(aleatorio)
	idMensaje := fmt.Sprintf("<%d.%x%s>", time.Now().UnixNano(), aleatorio, remitente.Address[strings.LastIndex(remitente.Address, "@"):])
	msg.WriteString("Message-ID: " + idMensaje + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	if m.HTML == "" {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
		escribirBase64(&msg, m.Cuerpo)
	} else {
		const limite = "=_rrhh_alt_boundary"
		msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + limite + "\"\r\n\r\n")
		for _, p := range []struct{ tipo, contenido string }{{"text/plain", m.Cuerpo}, {"text/html", m.HTML}} {
			msg.WriteString("--" + limite + "\r\nContent-Type: " + p.tipo + "; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
			escribirBase64(&msg, p.contenido)
		}
		msg.WriteString("--" + limite + "--\r\n")
	}

	var auth smtp.Auth
	if s.cfg.Usuario != "" {
		auth = smtp.PlainAuth("", s.cfg.Usuario, s.cfg.Clave, s.cfg.Host)
	}
	// smtp.SendMail no acepta contexto: se corre aparte para respetar el plazo del llamador.
	log.Printf("[correo] enviando a %s (de %s vía %s:%s, id %s)", destino.Address, remitente.Address, s.cfg.Host, s.cfg.Puerto, idMensaje)
	inicio := time.Now()
	hecho := make(chan error, 1)
	go func() {
		hecho <- smtp.SendMail(net.JoinHostPort(s.cfg.Host, s.cfg.Puerto), auth, remitente.Address, []string{destino.Address}, []byte(msg.String()))
	}()
	select {
	case err := <-hecho:
		if err != nil {
			log.Printf("[correo] FALLÓ el envío a %s tras %s: %v", destino.Address, time.Since(inicio).Round(time.Millisecond), err)
		} else {
			log.Printf("[correo] el servidor SMTP aceptó el correo para %s en %s (id %s); si no llega, revisar spam/remitente", destino.Address, time.Since(inicio).Round(time.Millisecond), idMensaje)
		}
		return err
	case <-ctx.Done():
		log.Printf("[correo] tiempo agotado enviando a %s: %v", destino.Address, ctx.Err())
		return ctx.Err()
	}
}
