package notificaciones

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// DatosSolicitudDatos alimenta el correo que pide al candidato validar sus datos en la plataforma.
type DatosSolicitudDatos struct {
	Nombre, Correo, Vacante, URL string
	// Rellenados por SolicitudDatos.
	Inicial, Fecha string
}

//go:embed plantilla_solicitud_datos.html
var htmlSolicitudDatos string

var plantillaSolicitudDatos = template.Must(template.New("solicitud").Parse(htmlSolicitudDatos))

// SolicitudDatos devuelve el correo (texto plano y HTML) que invita al candidato a validar sus datos.
func SolicitudDatos(d DatosSolicitudDatos) (texto, html string, err error) {
	d.Inicial = strings.ToUpper(string([]rune(d.Correo + "?")[:1]))
	d.Fecha = FechaEnvio(time.Now())
	var b bytes.Buffer
	if err := plantillaSolicitudDatos.Execute(&b, d); err != nil {
		return "", "", err
	}
	texto = fmt.Sprintf("Hola %s,\n\nGracias por postularte a «%s». Para poder continuar con tu proceso de selección "+
		"necesitamos que valides unos datos que nos faltan.\n\nPor favor ingresa a la siguiente plataforma:\n%s\n\nSaludos,\nEquipo de Selección",
		d.Nombre, d.Vacante, d.URL)
	return texto, b.String(), nil
}

// FechaEnvio da la fecha y hora en español para los mensajes ("20 de octubre a las 10:43 AM").
func FechaEnvio(t time.Time) string {
	meses := [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("%d de %s a las %s", t.Day(), meses[t.Month()-1], t.Format("3:04 PM"))
}
