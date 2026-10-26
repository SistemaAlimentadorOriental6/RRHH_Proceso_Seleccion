package almacen

import (
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"go-rrhh/modelos"
)

// layoutsFecha son los formatos que entregan los portales (RUNT: ISO con zona; ADRES: DD/MM/AAAA;
// SIMIT y licencias: AAAA-MM-DD) más el de la verificación de SARLAFT.
var layoutsFecha = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02/01/2006",
}

// parseFecha convierte el texto de un portal. Devuelve ok=false si no se reconoce el formato: en
// ese caso la columna queda en NULL y el texto original sigue en el JSON.
func parseFecha(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range layoutsFecha {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func nullFecha(s string) sql.NullTime {
	if t, ok := parseFecha(s); ok {
		return sql.NullTime{Time: t, Valid: true}
	}
	return sql.NullTime{}
}

// parseMonto convierte un monto en pesos ("$ 1.250.000", "$ 1.250.000,50", "$ 0") a un decimal con
// punto ("1250000.00"). Devuelve ok=false si el texto no trae dígitos.
//
// El último separador es decimal solo si le siguen 1 o 2 dígitos; con 3 (o con varios separadores
// iguales) es de miles. Así "1.250" son 1250 pesos y "12,50" son 12,50.
func parseMonto(s string) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	limpio := strings.Trim(b.String(), ".,")
	if limpio == "" || !strings.ContainsAny(limpio, "0123456789") {
		return "", false
	}
	entero, decimales := limpio, "00"
	if i := strings.LastIndexAny(limpio, ".,"); i >= 0 {
		resto := limpio[i+1:]
		if len(resto) == 1 || len(resto) == 2 {
			entero, decimales = limpio[:i], resto
			if len(decimales) == 1 {
				decimales += "0"
			}
		}
	}
	entero = strings.NewReplacer(".", "", ",", "").Replace(entero)
	if entero == "" {
		entero = "0"
	}
	return entero + "." + decimales, true
}

func nullMonto(s string) sql.NullString {
	if m, ok := parseMonto(s); ok {
		return sql.NullString{String: m, Valid: true}
	}
	return sql.NullString{}
}

// texto devuelve NULL si está vacío y recorta a max caracteres (no bytes) para no romper UTF-8
// ni fallar con "Data too long" en modo estricto.
func texto(s string, max int) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max])
	}
	return sql.NullString{String: s, Valid: true}
}

func nullBool(b bool) sql.NullBool { return sql.NullBool{Bool: b, Valid: true} }

// noVerificado reconoce los detalles de una regla que NO llegó a comprobarse (descarte temprano,
// consulta caída o portal en mantenimiento). Para la base esas reglas son "sin evaluar" (NULL),
// no "no cumple".
func noVerificado(detalle string) bool {
	for _, p := range []string{
		modelos.DetalleNoConsultado, "ADRES no verificado", "RUNT no verificado", "SIMIT no verificado",
		"ADRES en mantenimiento", "Error en la consulta", "Sin información de",
	} {
		if strings.HasPrefix(detalle, p) {
			return true
		}
	}
	return false
}
