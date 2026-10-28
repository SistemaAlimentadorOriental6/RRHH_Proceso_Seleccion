package servidor

import (
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

const timeoutEnvioCorreo = 30 * time.Second

// Personas a las que se les envía la solicitud de datos mientras RRHH_SOLICITUD_SOLO_PRUEBA esté
// activo (por defecto lo está). Se reconocen por nombre (sin acentos) o por cédula. Para habilitar a
// otra persona, agregarla aquí; para abrirlo a todos, RRHH_SOLICITUD_SOLO_PRUEBA=false.
var (
	nombresPrueba = map[string]bool{SlugPersona("Daniel Ochoa Castrillón"): true}
	cedulasPrueba = map[string]bool{"1035126774": true}
)

func esPersonaPrueba(cv *CandidatoVacante) bool { return esPrueba(cv.Nombre, cv.Cedula) }

func esPrueba(nombre, cedula string) bool {
	return nombresPrueba[SlugPersona(nombre)] || cedulasPrueba[soloDigitos(cedula)]
}

// ContactoCandidato son los datos de contacto hallados en la hoja de vida y el perfil.
type ContactoCandidato struct {
	Correo   string `json:"correo,omitempty"`
	Telefono string `json:"telefono,omitempty"`
}

// RespuestaSolicitudDatos informa el contacto hallado y qué se envió por cada canal.
type RespuestaSolicitudDatos struct {
	ContactoCandidato
	CorreoEnviado bool `json:"correo_enviado"`
	// YaEnviado / WhatsappYaEnviado: el mensaje ya había salido por ese canal y no se repitió.
	YaEnviado         bool   `json:"ya_enviado,omitempty"`
	WhatsappEnviado   bool   `json:"whatsapp_enviado,omitempty"`
	WhatsappYaEnviado bool   `json:"whatsapp_ya_enviado,omitempty"`
	WhatsappError     string `json:"whatsapp_error,omitempty"`
	Mensaje           string `json:"mensaje"`
}

// textoHojaDeVida junta el texto del perfil con el de los archivos de la hoja de vida del candidato (PDF o Word).
func (c *ControladorRRHH) textoHojaDeVida(cv *CandidatoVacante) string {
	partes := []string{cv.TextoPerfil}
	for _, ruta := range []string{cv.RutaCVOriginal, cv.RutaPDF, cv.RutaPerfilPDF} {
		if ruta == "" {
			continue
		}
		seguro, err := utilidades.RutaDentroDe(c.cfg.DirectorioDescargas, ruta)
		if err != nil {
			continue
		}
		leer := utilidades.ExtraerTextoPDF
		if strings.EqualFold(filepath.Ext(seguro), ".docx") {
			leer = utilidades.ExtraerTextoDocx
		}
		if txt, err := leer(seguro); err == nil {
			partes = append(partes, txt)
		}
	}
	return strings.Join(partes, "\n")
}

// candidatoParaSolicitud resuelve vacante (id de Computrabajo) y candidato (cédula o slug) de la ruta.
// Mientras RRHH_SOLICITUD_SOLO_PRUEBA esté activo exige que sea una persona de prueba. Si falla ya
// respondió al cliente y devuelve ok=false.
func (c *ControladorRRHH) candidatoParaSolicitud(w http.ResponseWriter, r *http.Request) (titulo string, cv *CandidatoVacante, ok bool) {
	id := r.PathValue("id")
	if !reIDOferta.MatchString(id) {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "identificador de vacante inválido"})
		return
	}
	titulo, err := c.tituloDeOferta(r.Context(), id)
	if err == nil {
		var lista []modelos.Candidato
		if lista, err = c.candidatosDeVacante(id, titulo, true); err == nil {
			cv, err = buscarEn(lista, r.PathValue("candidato"))
		}
	}
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) || errors.Is(err, ErrParametro) {
			responderError(w, err)
		} else if codigo, mensaje := traducirErrorComputrabajo(err); codigo != 0 { // 0: el cliente cerró la conexión
			ResponderJSON(w, codigo, map[string]string{"error": mensaje})
		}
		return
	}
	if c.cfg.SolicitudSoloPrueba && !esPersonaPrueba(cv) {
		ResponderJSON(w, http.StatusForbidden, map[string]string{"error": "Esta función aún está habilitada solo para candidatos de prueba"})
		return
	}
	return titulo, cv, true
}

// ManejarContactoCandidato detecta el correo y el celular en la hoja de vida (solo lectura, no envía nada).
func (c *ControladorRRHH) ManejarContactoCandidato(w http.ResponseWriter, r *http.Request) {
	_, cv, ok := c.candidatoParaSolicitud(w, r)
	if !ok {
		return
	}
	contacto := utilidades.ExtraerContacto(c.textoHojaDeVida(cv), cv.Cedula)
	ResponderJSON(w, http.StatusOK, ContactoCandidato{Correo: contacto.Correo, Telefono: contacto.Telefono})
}

// ManejarSolicitarDatosFaltantes le escribe al candidato (correo y WhatsApp) para que complete sus
// datos en la plataforma. El enlace lleva un token de un solo uso: nombre, correo y cargo ya no
// viajan en la URL. Cada llamada genera un enlace nuevo que invalida el anterior.
func (c *ControladorRRHH) ManejarSolicitarDatosFaltantes(w http.ResponseWriter, r *http.Request) {
	_, cv, ok := c.candidatoParaSolicitud(w, r)
	if !ok {
		return
	}
	if c.envios == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El envío de solicitudes requiere la base de datos"})
		return
	}
	if cv.PostulacionID == "" {
		log.Printf("[!] Solicitud de datos para un candidato que no está en la base de datos")
		ResponderJSON(w, http.StatusConflict, map[string]string{"error": "El candidato todavía no está guardado en la base de datos: analiza la vacante primero"})
		return
	}
	// Abrir el flujo de nuevo no debe volver a escribirle al candidato: lo ya enviado no se repite, ni
	// siquiera con otro enlace. Para reenviar a propósito se pide con ?reenviar=1.
	if r.URL.Query().Get("reenviar") != "1" {
		correo, whatsapp, err := c.envios.EnvioPrevio(r.Context(), cv.PostulacionID)
		if err != nil {
			log.Printf("[!] No se pudo consultar los envíos previos: %v", err)
			ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible preparar la solicitud"})
			return
		}
		if correo || whatsapp {
			ResponderJSON(w, http.StatusOK, RespuestaSolicitudDatos{
				YaEnviado: correo, WhatsappYaEnviado: whatsapp, Mensaje: "La solicitud ya se había enviado",
			})
			return
		}
	}
	// Quien no pasó por la pausa no tiene contacto guardado: se toma de su hoja de vida.
	contacto := utilidades.ExtraerContacto(c.textoHojaDeVida(cv), cv.Cedula)
	if err := c.envios.GuardarContacto(r.Context(), cv.PostulacionID, contacto.Correo, contacto.Telefono); err != nil {
		log.Printf("[!] No se pudo guardar el contacto del candidato: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible preparar la solicitud"})
		return
	}
	c.responderEnvio(w, r.Context(), cv.PostulacionID)
}
