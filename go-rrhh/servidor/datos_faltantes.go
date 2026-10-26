package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
	"go-rrhh/notificaciones"
)

// ---- Lo que el flujo de datos faltantes necesita de la base de datos ----

// solicitudesPublicas es lo que atiende la página que abre el candidato.
type solicitudesPublicas interface {
	AbrirSolicitud(ctx context.Context, token string) (almacen.InfoSolicitud, error)
	CompletarSolicitud(ctx context.Context, token string, d almacen.DatosCandidato) error
	RegistrarFallo(ctx context.Context, token string) error
	PseudonimoIP(ip string) string
}

// enviosBase es lo que necesita el envío de la solicitud al candidato.
type enviosBase interface {
	// cualquierEtapa permite pedirle datos a quien no está en pausa (solo para pruebas).
	PrepararEnvio(ctx context.Context, postulacionID string, vigencia time.Duration, cualquierEtapa bool) (almacen.Envio, error)
	GuardarContacto(ctx context.Context, postulacionID, correo, telefono string) error
	// EnvioPrevio: canales por los que ya salió la solicitud vigente (para no repetir el mensaje).
	EnvioPrevio(ctx context.Context, postulacionID string) (correo, whatsapp bool, err error)
	EstadoSolicitud(ctx context.Context, postulacionID string) (almacen.EstadoSolicitudDatos, error)
	MarcarEnvio(ctx context.Context, solicitudID string, resultados []almacen.ResultadoEnvio) error
	EnviosPendientes(ctx context.Context) ([]string, error)
}

// trabajosBase es lo que necesita el trabajador que reanuda los análisis.
type trabajosBase interface {
	ReclamarReanudacion(ctx context.Context, trabajador string, lease time.Duration) (*almacen.TrabajoReanudacion, error)
	TerminarReanudacion(ctx context.Context, t *almacen.TrabajoReanudacion, fallo error) error
	RecuperarReanudaciones(ctx context.Context) error
	VencerSolicitudes(ctx context.Context) (int, error)
	CandidatoPorPostulacion(ctx context.Context, postulacionID string) (modelos.OfertaLaboral, *modelos.Candidato, error)
}

// historialBase entrega la línea de tiempo de un candidato.
type historialBase interface {
	HistorialDe(ctx context.Context, postulacionID string) ([]almacen.EventoHistorial, error)
}

// ---- Límite de peticiones por dirección (las rutas públicas no tienen sesión) ----

type limitadorIP struct {
	mu      sync.Mutex
	max     int
	ventana time.Duration
	cuentas map[string]*cuentaIP
}

type cuentaIP struct {
	n     int
	desde time.Time
}

func nuevoLimitadorIP(max int, ventana time.Duration) *limitadorIP {
	return &limitadorIP{max: max, ventana: ventana, cuentas: map[string]*cuentaIP{}}
}

// permitir cuenta una petición de ip y dice si todavía está dentro del límite de la ventana.
func (l *limitadorIP) permitir(ip string) bool {
	ahora := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.cuentas) > 10000 { // limpieza oportunista para que el mapa no crezca sin límite
		for k, c := range l.cuentas {
			if ahora.Sub(c.desde) > l.ventana {
				delete(l.cuentas, k)
			}
		}
	}
	c := l.cuentas[ip]
	if c == nil || ahora.Sub(c.desde) > l.ventana {
		l.cuentas[ip] = &cuentaIP{n: 1, desde: ahora}
		return true
	}
	c.n++
	return c.n <= l.max
}

// ipDe es la dirección del cliente. Detrás de un proxy inverso hay que activar
// RRHH_PROXY_CONFIABLE=true para leerla de X-Forwarded-For; si no, sería falsificable.
func ipDe(r *http.Request) string {
	if strings.EqualFold(os.Getenv("RRHH_PROXY_CONFIABLE"), "true") {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- Página pública del candidato ----

// vistaSolicitud es lo que ve la página del candidato al abrir su enlace.
type vistaSolicitud struct {
	Estado          string     `json:"estado"` // pendiente | completada | vencida | invalida
	Nombre          string     `json:"nombre,omitempty"`
	Correo          string     `json:"correo,omitempty"` // enmascarado
	Cargo           string     `json:"cargo,omitempty"`
	VenceEn         *time.Time `json:"vence_en,omitempty"`
	PoliticaVersion string     `json:"politica_version,omitempty"`
	EdadMinima      int        `json:"edad_minima,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// respuestaPrivada evita que el navegador o un intermediario guarden o indexen la página.
func respuestaPrivada(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

// admitirPublico aplica a las rutas públicas el límite por dirección y comprueba que la base esté
// disponible. Devuelve falso si ya respondió.
func (c *ControladorRRHH) admitirPublico(w http.ResponseWriter, r *http.Request) bool {
	respuestaPrivada(w)
	if !c.limite.permitir(ipDe(r)) {
		w.Header().Set("Retry-After", "60")
		ResponderJSON(w, http.StatusTooManyRequests, vistaSolicitud{Estado: "invalida", Error: "Demasiados intentos. Espera un minuto e inténtalo de nuevo."})
		return false
	}
	if c.solicitudes == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, vistaSolicitud{Estado: "invalida", Error: "El servicio no está disponible por ahora."})
		return false
	}
	return true
}

// ManejarVerSolicitudDatos entrega al formulario del candidato lo que ya sabemos de él, a partir
// del token del enlace. Nada de eso viaja en la URL.
func (c *ControladorRRHH) ManejarVerSolicitudDatos(w http.ResponseWriter, r *http.Request) {
	if !c.admitirPublico(w, r) {
		return
	}
	info, err := c.solicitudes.AbrirSolicitud(r.Context(), r.PathValue("token"))
	switch {
	case err == nil:
		vence := info.VenceEn
		ResponderJSON(w, http.StatusOK, vistaSolicitud{
			Estado: "pendiente", Nombre: info.Nombre, Correo: info.Correo, Cargo: info.Cargo, VenceEn: &vence,
			PoliticaVersion: c.cfg.PoliticaVersion, EdadMinima: c.cfg.EdadMinima,
		})
	case errors.Is(err, almacen.ErrYaCompletada):
		ResponderJSON(w, http.StatusOK, vistaSolicitud{Estado: "completada", Nombre: info.Nombre})
	case errors.Is(err, almacen.ErrEnlaceVencido):
		ResponderJSON(w, http.StatusGone, vistaSolicitud{Estado: "vencida", Error: "Este enlace venció. Pide a quien te contactó que te envíe uno nuevo."})
	case errors.Is(err, almacen.ErrEnlaceInvalido):
		ResponderJSON(w, http.StatusNotFound, vistaSolicitud{Estado: "invalida", Error: "Este enlace no es válido."})
	default:
		log.Printf("[!] No se pudo abrir una solicitud de datos: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, vistaSolicitud{Estado: "invalida", Error: "No pudimos cargar tus datos. Inténtalo de nuevo en unos minutos."})
	}
}

type cuerpoDatosCandidato struct {
	Cedula          string `json:"cedula"`
	FechaNacimiento string `json:"fecha_nacimiento"`
	Autoriza        bool   `json:"autoriza"`
	PoliticaVersion string `json:"politica_version"`
}

type respuestaEnvioDatos struct {
	OK     bool              `json:"ok"`
	Estado string            `json:"estado,omitempty"`
	Error  string            `json:"error,omitempty"`
	Campos map[string]string `json:"campos,omitempty"`
}

// ManejarEnviarDatosSolicitud recibe lo que escribió el candidato. Valida, guarda y responde de
// inmediato; la consulta a las fuentes continúa en segundo plano.
func (c *ControladorRRHH) ManejarEnviarDatosSolicitud(w http.ResponseWriter, r *http.Request) {
	if !c.admitirPublico(w, r) {
		return
	}
	token := r.PathValue("token")

	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var cuerpo cuerpoDatosCandidato
	if err := dec.Decode(&cuerpo); err != nil {
		ResponderJSON(w, http.StatusBadRequest, respuestaEnvioDatos{Error: "No pudimos leer los datos enviados."})
		return
	}

	campos := map[string]string{}
	datos, err := almacen.ValidarDatosCandidato(cuerpo.Cedula, cuerpo.FechaNacimiento, c.cfg.EdadMinima, time.Now().UTC())
	var errCampos almacen.ErrCampos
	if errors.As(err, &errCampos) {
		for campo, mensaje := range errCampos {
			campos[campo] = mensaje
		}
	}
	if !cuerpo.Autoriza {
		campos["autoriza"] = "Debes autorizar el tratamiento de tus datos personales para continuar"
	}
	if cuerpo.PoliticaVersion != c.cfg.PoliticaVersion {
		campos["autoriza"] = "La política de tratamiento de datos cambió. Recarga la página para verla y aceptarla."
	}
	if len(campos) > 0 {
		_ = c.solicitudes.RegistrarFallo(r.Context(), token)
		ResponderJSON(w, http.StatusUnprocessableEntity, respuestaEnvioDatos{Error: "Revisa los datos marcados.", Campos: campos})
		return
	}

	err = c.solicitudes.CompletarSolicitud(r.Context(), token, almacen.DatosCandidato{
		DatosValidados:  datos,
		PoliticaVersion: cuerpo.PoliticaVersion,
		IPHuella:        c.solicitudes.PseudonimoIP(ipDe(r)),
	})
	switch {
	case err == nil:
		c.despertarSegundoPlano() // que el trabajador reanude el análisis sin esperar al siguiente ciclo
		c.cambios.publicar("formulario_completado")
		ResponderJSON(w, http.StatusOK, respuestaEnvioDatos{OK: true, Estado: "completada"})
	case errors.Is(err, almacen.ErrYaCompletada):
		ResponderJSON(w, http.StatusOK, respuestaEnvioDatos{OK: true, Estado: "completada"}) // idempotente: doble clic
	case errors.Is(err, almacen.ErrEnlaceVencido):
		ResponderJSON(w, http.StatusGone, respuestaEnvioDatos{Estado: "vencida", Error: "Este enlace venció. Pide a quien te contactó que te envíe uno nuevo."})
	case errors.Is(err, almacen.ErrEnlaceInvalido):
		ResponderJSON(w, http.StatusNotFound, respuestaEnvioDatos{Estado: "invalida", Error: "Este enlace no es válido."})
	default:
		log.Printf("[!] No se pudieron guardar los datos de un candidato: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, respuestaEnvioDatos{Error: "No pudimos guardar tus datos. Inténtalo de nuevo en unos minutos."})
	}
}

// ---- Rutas del reclutador ----

// ManejarHistorialPostulacion devuelve la línea de tiempo de un candidato: qué se intentó, dónde
// se detuvo su análisis y qué pasó después.
func (c *ControladorRRHH) ManejarHistorialPostulacion(w http.ResponseWriter, r *http.Request) {
	if c.historiales == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El historial requiere la base de datos"})
		return
	}
	eventos, err := c.historiales.HistorialDe(r.Context(), r.PathValue("id"))
	if err != nil {
		log.Printf("[!] No se pudo leer el historial de una postulación: %v", err)
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "Postulación inválida"})
		return
	}
	ResponderJSON(w, http.StatusOK, eventos)
}

// vistaEstadoSolicitud es lo que la pantalla del reclutador pinta en la rama de contacto.
type vistaEstadoSolicitud struct {
	Existe        bool   `json:"existe"`
	Estado        string `json:"estado,omitempty"`
	Correo        string `json:"correo,omitempty"`
	Telefono      string `json:"telefono,omitempty"`
	EnvioCorreo   string `json:"envio_correo,omitempty"`   // enviada | fallida | pendiente
	EnvioWhatsapp string `json:"envio_whatsapp,omitempty"` // enviada | fallida | pendiente
	Completada    bool   `json:"completada"`
	Cedula        string `json:"cedula,omitempty"` // la declarada por el candidato, en claro
	Edad          int    `json:"edad,omitempty"`
}

// ManejarEstadoSolicitud informa en qué va la solicitud de datos de un candidato. Solo lee: lo que
// envía y recibe pasa en segundo plano. Mientras dure la prueba, solo para las personas de prueba.
func (c *ControladorRRHH) ManejarEstadoSolicitud(w http.ResponseWriter, r *http.Request) {
	if c.envios == nil || c.trabajos == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "La solicitud de datos requiere la base de datos"})
		return
	}
	id := r.PathValue("id")
	if c.cfg.SolicitudSoloPrueba {
		_, cand, err := c.trabajos.CandidatoPorPostulacion(r.Context(), id)
		if err != nil {
			ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Postulación no encontrada"})
			return
		}
		if !esPrueba(cand.Nombre, cand.Cedula) {
			ResponderJSON(w, http.StatusForbidden, map[string]string{"error": "Esta función aún está habilitada solo para candidatos de prueba"})
			return
		}
	}
	e, err := c.envios.EstadoSolicitud(r.Context(), id)
	if err != nil {
		log.Printf("[!] No se pudo leer el estado de la solicitud de datos: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible leer la solicitud"})
		return
	}
	ResponderJSON(w, http.StatusOK, vistaEstadoSolicitud{
		Existe: e.Existe, Estado: e.Estado, Correo: e.Contacto.Correo, Telefono: e.Contacto.Telefono,
		EnvioCorreo: e.Correo, EnvioWhatsapp: e.Whatsapp, Completada: e.Completada, Cedula: e.Cedula, Edad: e.Edad,
	})
}

// ManejarSolicitarDatosPostulacion envía (o reenvía) al candidato la solicitud de sus datos, con un
// enlace nuevo que reemplaza al anterior.
func (c *ControladorRRHH) ManejarSolicitarDatosPostulacion(w http.ResponseWriter, r *http.Request) {
	if c.envios == nil || c.trabajos == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El envío de solicitudes requiere la base de datos"})
		return
	}
	id := r.PathValue("id")
	if c.cfg.SolicitudSoloPrueba {
		// Mientras se prueba, los mensajes solo salen hacia las personas de prueba.
		_, cand, err := c.trabajos.CandidatoPorPostulacion(r.Context(), id)
		if err != nil {
			ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Postulación no encontrada"})
			return
		}
		if !esPersonaPrueba(&CandidatoVacante{Candidato: *cand, Slug: SlugPersona(cand.Nombre)}) {
			ResponderJSON(w, http.StatusForbidden, map[string]string{"error": "Esta función aún está habilitada solo para candidatos de prueba"})
			return
		}
	}
	c.responderEnvio(w, r.Context(), id)
}

// ---- Envío de la solicitud ----

var (
	errEnvioNoConfigurado = errors.New("el canal de envío no está configurado")
	errEnvioFallido       = errors.New("no fue posible enviar el mensaje")
)

// responderEnvio envía la solicitud y traduce el resultado a la respuesta HTTP.
func (c *ControladorRRHH) responderEnvio(w http.ResponseWriter, ctx context.Context, postulacionID string) {
	resp, err := c.enviarSolicitud(ctx, postulacionID)
	switch {
	case err == nil:
		resp.Mensaje = "Mensaje enviado"
		ResponderJSON(w, http.StatusOK, resp)
	case errors.Is(err, almacen.ErrEstadoNoPermite):
		ResponderJSON(w, http.StatusConflict, map[string]string{"error": "El candidato no está esperando datos"})
	case errors.Is(err, almacen.ErrSinContacto):
		resp.Mensaje = "No se encontró correo ni teléfono en la hoja de vida"
		ResponderJSON(w, http.StatusUnprocessableEntity, resp)
	case errors.Is(err, errEnvioNoConfigurado):
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	case errors.Is(err, errEnvioFallido):
		ResponderJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
	default:
		log.Printf("[!] No se pudo preparar la solicitud de datos: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible preparar la solicitud"})
	}
}

// enviarSolicitud genera un enlace nuevo con su token y se lo manda al candidato por correo y por
// WhatsApp. El token solo existe en memoria durante este envío: en la base queda su huella. Si
// ningún canal funciona devuelve error, pero el resultado de cada intento queda registrado.
func (c *ControladorRRHH) enviarSolicitud(ctx context.Context, postulacionID string) (RespuestaSolicitudDatos, error) {
	if c.cfg.URLPlataformaValidacion == "" {
		return RespuestaSolicitudDatos{}, errors.New("URL_PLATAFORMA_VALIDACION no está configurada")
	}
	vigencia := time.Duration(c.cfg.SolicitudVigenciaDias) * 24 * time.Hour
	envio, err := c.envios.PrepararEnvio(ctx, postulacionID, vigencia, c.cfg.SolicitudSoloPrueba)
	if err != nil {
		return RespuestaSolicitudDatos{}, err
	}

	enlace := strings.TrimRight(c.cfg.URLPlataformaValidacion, "/") + "/validar-datos/" + envio.Token
	resp := RespuestaSolicitudDatos{ContactoCandidato: ContactoCandidato{Correo: envio.Contacto.Correo, Telefono: envio.Contacto.Telefono}}

	// El envío no depende de la conexión del cliente: si se cierra a mitad, el mensaje puede salir igual.
	ctxEnvio, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeoutEnvioCorreo)
	defer cancel()

	var resultados []almacen.ResultadoEnvio
	configurado := true
	if envio.Contacto.Correo != "" {
		errCorreo := c.enviarCorreoSolicitud(ctxEnvio, envio, enlace)
		resultados = append(resultados, resultadoDe("correo", errCorreo))
		resp.CorreoEnviado = errCorreo == nil
		if errors.Is(errCorreo, notificaciones.ErrNoConfigurado) {
			configurado = false
		}
	}
	if envio.Contacto.Telefono != "" {
		errWA := c.whatsapp.Enviar(ctxEnvio, notificaciones.Mensaje{
			Destino:   envio.Contacto.Telefono,
			Plantilla: c.cfg.WhatsAppPlantilla,
			Variables: map[string]string{
				"nombre_minimo": primerNombre(envio.Nombre), "nombre": envio.Nombre, "vacante": envio.Cargo,
				"fecha": time.Now().Format("02/01/2006"), "correo": envio.Contacto.Correo, "url": enlace,
			},
		})
		resultados = append(resultados, resultadoDe("whatsapp", errWA))
		resp.WhatsappEnviado = errWA == nil
		switch {
		case errors.Is(errWA, notificaciones.ErrNoConfigurado):
			resp.WhatsappError = "WhatsApp no está configurado"
		case errWA != nil:
			resp.WhatsappError = "No fue posible enviar el WhatsApp"
		}
	}

	ctxMarca, cancelMarca := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelMarca()
	if err := c.envios.MarcarEnvio(ctxMarca, envio.SolicitudID, resultados); err != nil {
		log.Printf("[!] No se pudo registrar el resultado del envío: %v", err)
	}

	if !resp.CorreoEnviado && !resp.WhatsappEnviado {
		if !configurado {
			return resp, errEnvioNoConfigurado
		}
		return resp, errEnvioFallido
	}
	return resp, nil
}

func (c *ControladorRRHH) enviarCorreoSolicitud(ctx context.Context, envio almacen.Envio, enlace string) error {
	texto, html, err := notificaciones.SolicitudDatos(notificaciones.DatosSolicitudDatos{
		Nombre: envio.Nombre, Correo: envio.Contacto.Correo, Vacante: envio.Cargo, URL: enlace,
	})
	if err != nil {
		return err
	}
	return c.notificador.Enviar(ctx, notificaciones.Mensaje{
		Destino: envio.Contacto.Correo,
		Asunto:  "Completa tus datos para continuar en el proceso de selección",
		Cuerpo:  texto,
		HTML:    html,
	})
}

// resultadoDe resume el intento de envío por un canal sin guardar el texto del error (podría
// traer datos del destinatario).
func resultadoDe(canal string, err error) almacen.ResultadoEnvio {
	switch {
	case err == nil:
		return almacen.ResultadoEnvio{Canal: canal, OK: true}
	case errors.Is(err, notificaciones.ErrNoConfigurado):
		return almacen.ResultadoEnvio{Canal: canal, Error: "canal no configurado"}
	}
	log.Printf("[!] Falló el envío por %s: %v", canal, err)
	return almacen.ResultadoEnvio{Canal: canal, Error: "falló el envío"}
}

func primerNombre(nombre string) string {
	if partes := strings.Fields(nombre); len(partes) > 0 {
		return partes[0]
	}
	return ""
}
