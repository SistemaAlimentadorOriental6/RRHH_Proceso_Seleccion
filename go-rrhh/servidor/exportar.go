package servidor

import (
	"archive/zip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

var reNoArchivo = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

// nombreSeguro deja un texto usable como nombre de archivo o carpeta dentro del zip.
func nombreSeguro(s string) string {
	s = strings.TrimSpace(reNoArchivo.ReplaceAllString(s, ""))
	if s == "" {
		return "sin-nombre"
	}
	return s
}

// idsDeVacantes lee ?ids=a,b,c; si falta o es inválido ya respondió al cliente y devuelve ok=false.
func idsDeVacantes(w http.ResponseWriter, r *http.Request) (ids []string, ok bool) {
	for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !reIDOferta.MatchString(id) {
			ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "identificador de vacante inválido"})
			return nil, false
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "Parámetro 'ids' requerido"})
		return nil, false
	}
	return ids, true
}

// candidatosGuardados devuelve el título de la vacante y sus candidatos ya analizados. Una vacante sin
// análisis previo (o desconocida) devuelve lista vacía sin error.
func (c *ControladorRRHH) candidatosGuardados(r *http.Request, id string) (string, []CandidatoVacante, error) {
	titulo := agentes.TituloOfertaCorreo
	if !strings.EqualFold(id, agentes.IDOfertaCorreo) {
		var err error
		if titulo, err = c.tituloDeOferta(r.Context(), id); err != nil {
			if errors.Is(err, ErrNoEncontrado) {
				return titulo, nil, nil
			}
			return "", nil, err
		}
	}
	lista, err := c.candidatosDeVacante(id, titulo, true)
	if errors.Is(err, ErrNoEncontrado) {
		return titulo, nil, nil
	}
	return titulo, conSlug(lista), err
}

// responderErrorExportar traduce un error al leer vacantes (código 0 = el cliente ya se fue).
func responderErrorExportar(w http.ResponseWriter, err error) {
	if codigo, mensaje := traducirErrorComputrabajo(err); codigo != 0 {
		ResponderJSON(w, codigo, map[string]string{"error": mensaje})
		return
	}
	responderError(w, err)
}

// ManejarExportarAptos entrega un .zip con los candidatos aptos de las vacantes indicadas (?ids=a,b,c):
// un aptos.csv con nombre, cédula, teléfono y correo, y una carpeta por persona con sus hojas de vida.
// Solo entran aptos con el análisis completo; un candidato repetido en dos vacantes sale una sola vez.
// El teléfono y el correo se leen de la hoja de vida (puede quedar vacío si no los trae).
func (c *ControladorRRHH) ManejarExportarAptos(w http.ResponseWriter, r *http.Request) {
	ids, ok := idsDeVacantes(w, r)
	if !ok {
		return
	}

	type apto struct {
		cv       *CandidatoVacante
		vacante  string
		contacto utilidades.ContactoCV
	}
	var aptos []apto
	vistos := map[string]bool{}
	for _, id := range ids {
		titulo, lista, err := c.candidatosGuardados(r, id)
		if err != nil {
			responderErrorExportar(w, err)
			return
		}
		for i := range lista {
			cv := &lista[i]
			if !esAptoParaExportar(&cv.Candidato) {
				continue
			}
			clave := soloDigitos(cv.Cedula)
			if clave == "" || vistos[clave] {
				continue
			}
			vistos[clave] = true
			aptos = append(aptos, apto{cv: cv, vacante: titulo, contacto: utilidades.ExtraerContacto(c.textoHojaDeVida(cv), cv.Cedula)})
		}
	}
	if len(aptos) == 0 {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "No hay candidatos aptos para exportar"})
		return
	}

	zw := abrirZip(w, fmt.Sprintf("candidatos-aptos-%s.zip", time.Now().Format("2006-01-02")))
	tabla := crearCSV(zw, "aptos.csv", []string{"nombre", "cedula", "telefono", "correo", "vacante", "score_ia", "carpeta"})
	if tabla == nil {
		return
	}
	for _, a := range aptos {
		carpeta := nombreSeguro(a.cv.Nombre) + " - " + soloDigitos(a.cv.Cedula)
		_ = tabla.Write([]string{a.cv.Nombre, a.cv.Cedula, a.contacto.Telefono, a.contacto.Correo, a.vacante, scoreIA(a.cv), carpeta})
		c.agregarArchivosCandidato(zw, carpeta, a.cv)
	}
	tabla.Flush()
	if err := zw.Close(); err != nil {
		log.Printf("[!] Exportar aptos: no se pudo cerrar el zip: %v", err)
	}
}

// ManejarExportarCandidato entrega un .zip con la información de UNA persona: datos.csv (nombre,
// cédula, teléfono, correo, resultado y motivos) y sus hojas de vida. La persona se identifica por
// ?ficha=<url de su ficha> dentro de las vacantes de ?ids=a,b,c (un grupo trae varias).
func (c *ControladorRRHH) ManejarExportarCandidato(w http.ResponseWriter, r *http.Request) {
	ids, ok := idsDeVacantes(w, r)
	if !ok {
		return
	}
	ficha := r.URL.Query().Get("ficha")
	if ficha == "" {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "Parámetro 'ficha' requerido"})
		return
	}

	var cv *CandidatoVacante
	var vacante string
	for _, id := range ids {
		titulo, lista, err := c.candidatosGuardados(r, id)
		if err != nil {
			responderErrorExportar(w, err)
			return
		}
		for i := range lista {
			if lista[i].URLDetalle == ficha {
				cv, vacante = &lista[i], titulo
				break
			}
		}
		if cv != nil {
			break
		}
	}
	if cv == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Candidato no encontrado"})
		return
	}

	nombre := nombreSeguro(cv.Nombre)
	if d := soloDigitos(cv.Cedula); d != "" {
		nombre += " - " + d
	}
	zw := abrirZip(w, nombre+".zip")
	contacto := utilidades.ExtraerContacto(c.textoHojaDeVida(cv), cv.Cedula)
	resultado, motivos := "No apto", ""
	switch {
	case esAptoParaExportar(&cv.Candidato):
		resultado = "Apto"
	case cv.EvaluacionIdoneidad != nil && cv.EvaluacionIdoneidad.VerificacionPendiente:
		resultado = "Verificación pendiente"
	}
	if cv.EvaluacionIdoneidad != nil {
		motivos = strings.Join(cv.EvaluacionIdoneidad.MotivosDescarte, " · ")
	}
	sintesis := ""
	if ia := cv.EvaluacionIA; ia != nil {
		sintesis = ia.PorQueEsElHombre
		if sintesis == "" {
			sintesis = ia.Dictamen
		}
	}
	escribirFicha(zw, "datos.txt", [][2]string{
		{"Nombre", cv.Nombre},
		{"Cédula", cv.Cedula},
		{"Teléfono", contacto.Telefono},
		{"Correo", contacto.Correo},
		{"Vacante", vacante},
		{"Resultado", resultado},
		{"Motivos de descarte", motivos},
		{"Score IA", scoreIA(cv)},
		{"Síntesis IA", sintesis},
		{"Exportado", time.Now().Format("2006-01-02 15:04")},
	})
	c.agregarArchivosCandidato(zw, "", cv)
	if err := zw.Close(); err != nil {
		log.Printf("[!] Exportar candidato: no se pudo cerrar el zip: %v", err)
	}
}

func scoreIA(cv *CandidatoVacante) string {
	if cv.EvaluacionIA == nil {
		return ""
	}
	return strconv.Itoa(int(cv.EvaluacionIA.ScoreTotal))
}

// abrirZip prepara la respuesta como descarga de un .zip.
func abrirZip(w http.ResponseWriter, nombre string) *zip.Writer {
	// Armar un zip puede tardar (muchos PDF): se amplía el plazo de esta respuesta.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": nombre}))
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition") // para que la pantalla lea el nombre
	w.Header().Set("Cache-Control", "no-store")
	return zip.NewWriter(w)
}

// crearCSV agrega al zip una tabla csv con su fila de títulos.
func crearCSV(zw *zip.Writer, nombre string, titulos []string) *csv.Writer {
	archivo, err := zw.Create(nombre)
	if err != nil {
		return nil
	}
	_, _ = archivo.Write([]byte("\xEF\xBB\xBF")) // BOM: Excel abre las tildes bien
	tabla := csv.NewWriter(archivo)
	tabla.Comma = ';' // Excel en español separa con punto y coma
	_ = tabla.Write(titulos)
	return tabla
}

// escribirFicha agrega al zip la ficha de una persona como texto plano, legible en el Bloc de notas.
func escribirFicha(zw *zip.Writer, nombre string, lineas [][2]string) {
	archivo, err := zw.Create(nombre)
	if err != nil {
		return
	}
	_, _ = archivo.Write([]byte("\xEF\xBB\xBF")) // BOM: el Bloc de notas respeta las tildes
	var sb strings.Builder
	for _, l := range lineas {
		if l[1] == "" {
			l[1] = "—"
		}
		sb.WriteString(l[0] + ": " + l[1] + "\r\n")
	}
	_, _ = archivo.Write([]byte(sb.String()))
}

// agregarArchivosCandidato copia al zip las hojas de vida de la persona (original, PDF y perfil), dentro
// de carpeta si se indica.
func (c *ControladorRRHH) agregarArchivosCandidato(zw *zip.Writer, carpeta string, cv *CandidatoVacante) {
	prefijo := ""
	if carpeta != "" {
		prefijo = carpeta + "/"
	}
	copiados := map[string]bool{}
	for _, f := range []struct{ etiqueta, ruta string }{
		{"hoja_de_vida_original", cv.RutaCVOriginal}, {"hoja_de_vida", cv.RutaPDF}, {"perfil", cv.RutaPerfilPDF},
	} {
		if f.ruta == "" {
			continue
		}
		seguro, err := utilidades.RutaDentroDe(c.cfg.DirectorioDescargas, f.ruta)
		if err != nil || copiados[seguro] {
			continue
		}
		copiados[seguro] = true
		if err := agregarArchivoZip(zw, prefijo+f.etiqueta+filepath.Ext(seguro), seguro); err != nil {
			log.Printf("[!] Exportar: no se pudo agregar %s de %q: %v", f.etiqueta, cv.Nombre, err)
		}
	}
}

// esAptoParaExportar: apto con el análisis completo (sin error ni verificación pendiente ni pausa por datos).
func esAptoParaExportar(c *modelos.Candidato) bool {
	if !c.EsApto || c.ErrorProceso != "" || modelos.EnPausa(c.EstadoProceso) {
		return false
	}
	return c.EvaluacionIdoneidad == nil || !c.EvaluacionIdoneidad.VerificacionPendiente
}

func agregarArchivoZip(zw *zip.Writer, nombre, ruta string) error {
	f, err := os.Open(ruta)
	if err != nil {
		return err
	}
	defer f.Close()
	dst, err := zw.Create(nombre)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, f)
	return err
}
