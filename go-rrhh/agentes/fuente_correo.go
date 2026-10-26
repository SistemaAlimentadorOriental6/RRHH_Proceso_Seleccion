package agentes

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go-rrhh/modelos"
)

// Las hojas de vida del correo entran al análisis como una vacante más: AgenteCorreo cumple la
// misma interfaz que Computrabajo (orquestador.FuenteCandidatos), así que reciben el mismo flujo —
// cédula, ADRES, RUNT, SIMIT, listas, IA, pausa por datos faltantes y guardado en la base.

// IDOfertaCorreo y TituloOfertaCorreo identifican la vacante virtual de las hojas de vida recibidas
// por correo. El id cumple lo que exigen las rutas (8 a 64 letras y números).
const (
	IDOfertaCorreo     = "CORREOHOJASDEVIDA"
	TituloOfertaCorreo = "Hojas de vida por correo"
)

func acotarTexto(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// URLFichaCorreo es el identificador estable de una hoja de vida (hace de «URL de la ficha»).
func URLFichaCorreo(h modelos.HojaVidaCorreo) string {
	return fmt.Sprintf("correo:%x", sha256.Sum256([]byte(h.MessageID)))[:7+16]
}

// LeerHojas devuelve las hojas de vida guardadas, de la más antigua a la más nueva (así un correo
// nuevo se suma al final y los números de orden de las anteriores no cambian).
func (a *AgenteCorreo) LeerHojas() ([]modelos.HojaVidaCorreo, error) {
	var hojas []modelos.HojaVidaCorreo
	if err := leerJSON(a.rutaResultados(), &hojas); err != nil {
		return nil, err
	}
	sort.SliceStable(hojas, func(i, j int) bool {
		if !hojas[i].Fecha.Equal(hojas[j].Fecha) {
			return hojas[i].Fecha.Before(hojas[j].Fecha)
		}
		return hojas[i].UID < hojas[j].UID
	})
	return hojas, nil
}

// HojasParaAnalizar son las que entran al flujo: las de quien busca ser conductor u operador (o
// podría serlo). Las que dicen «no» se conservan en el archivo pero no gastan consultas.
func (a *AgenteCorreo) HojasParaAnalizar() ([]modelos.HojaVidaCorreo, error) {
	todas, err := a.LeerHojas()
	if err != nil {
		return nil, err
	}
	var salida []modelos.HojaVidaCorreo
	for _, h := range todas {
		if h.InteresConductor != ConductorNo {
			salida = append(salida, h)
		}
	}
	return salida, nil
}

// nombreDe elige cómo llamar a la persona en el flujo.
func nombreDe(h modelos.HojaVidaCorreo) string {
	for _, n := range []string{h.Nombre, h.NombreRemitente} {
		if strings.TrimSpace(n) != "" {
			return strings.TrimSpace(n)
		}
	}
	if i := strings.IndexByte(h.Remitente, '@'); i > 0 {
		return h.Remitente[:i]
	}
	return "Sin nombre"
}

// ObtenerCandidatosStream entrega cada hoja de vida a analizar. No usa la URL: la vacante virtual
// no tiene página.
func (a *AgenteCorreo) ObtenerCandidatosStream(ctx context.Context, _ string, limite int, alDescubrir func(*modelos.Candidato)) error {
	hojas, err := a.HojasParaAnalizar()
	if err != nil {
		return fmt.Errorf("no se pudieron leer las hojas de vida del correo: %w", err)
	}
	for i, h := range hojas {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if limite > 0 && i >= limite {
			break
		}
		alDescubrir(&modelos.Candidato{Indice: i + 1, Nombre: nombreDe(h), URLDetalle: URLFichaCorreo(h)})
	}
	return nil
}

// ExtraerDetalleCandidato completa al candidato con lo ya leído del correo: cédula (si la trajo) y un
// texto de perfil encabezado por el correo y el celular, de donde el flujo toma el contacto cuando
// toca pedirle los datos que faltan y de donde se deduce el género.
func (a *AgenteCorreo) ExtraerDetalleCandidato(_ context.Context, c *modelos.Candidato, _ string) error {
	hojas, err := a.LeerHojas()
	if err != nil {
		return err
	}
	for _, h := range hojas {
		if URLFichaCorreo(h) != c.URLDetalle {
			continue
		}
		c.Cedula = h.Cedula
		c.TextoPerfil = fmt.Sprintf("Correo: %s\nTeléfono: %s\n%s", h.Correo, h.Telefono, h.TextoCV)
		for _, ruta := range h.Archivos {
			if ext := strings.ToLower(filepath.Ext(ruta)); ext == ".pdf" || ext == ".docx" {
				c.RutaCVOriginal = ruta
				break
			}
		}
		return nil
	}
	return fmt.Errorf("la hoja de vida %s ya no está en el archivo del correo", c.URLDetalle)
}
