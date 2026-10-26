package configuracion

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Configuracion centralizada del sistema de automatizacion RRHH en Go.
type Configuracion struct {
	URLBase             string
	URLLogin            string
	URLOfertas          string
	UsuarioComputrabajo string
	ClaveComputrabajo   string

	URLProcuraduria string
	URLSarlaftONU   string
	URLOfac         string
	URLAdres        string
	URLRuntAPI      string
	URLSimit        string

	DirectorioRaiz        string
	DirectorioDescargas   string
	DirectorioPerfilCP    string
	DirectorioPerfilAdres string

	ArchivoSarlaftONU string

	EPSAutorizadas   []string
	OfertasExcluidas []string
	// OfertasIncluidas son las palabras que debe tener el título de una vacante para listarla y
	// analizarla: solo conductor u operador. Las demás ni se cuentan. RRHH_OFERTAS_INCLUIDAS
	// (separadas por coma) la cambia; "todas" quita el filtro.
	OfertasIncluidas []string

	// Correo saliente (SMTP) y plataforma a la que se invita al candidato a completar sus datos.
	SMTPHost, SMTPPuerto, SMTPUsuario, SMTPClave, SMTPRemitente string
	URLPlataformaValidacion                                     string

	// WhatsApp (API de nube de Meta): URL del número emisor, token y plantilla aprobada.
	WhatsAppURL, WhatsAppToken, WhatsAppPlantilla string

	// Base de datos MySQL/MariaDB. Si MySQLHost está vacío la base queda desactivada y todo sigue
	// guardándose solo en archivos JSON, como antes.
	MySQLHost, MySQLPuerto, MySQLBase, MySQLUsuario, MySQLClave string
	// Claves de privacidad (64 caracteres hexadecimales cada una, distintas entre sí): la primera
	// pseudonimiza cédulas para buscarlas (HMAC-SHA256) y la segunda las cifra (AES-256-GCM).
	ClavePseudonimos, ClaveCifrado string

	// Buzón de correo (IMAP) donde la gente envía su hoja de vida. Es otra cuenta distinta del SMTP
	// de salida. Sin CORREO_USUARIO y CORREO_CLAVE la lectura del buzón queda apagada.
	CorreoHost, CorreoPuerto, CorreoUsuario, CorreoClave string

	// Administremos (servicio web SOAP de SIGMA): de ahí sale la validación de reintegro, que mira si la
	// cédula ya figura como empleado. Sin URL y credenciales la validación se omite.
	AdministremosURL, AdministremosUsuario, AdministremosClave, AdministremosKey, AdministremosToken string
	AdministremosIDEmpresa                                                                           int

	// IA (Gemini): clave y modelo de la calificación de hojas de vida. Sin clave no hay calificación.
	APIKeyIA, ModeloIA string
	// ProveedorIA elige dónde está la clave: "vertex" (Vertex AI, claves AQ.…), "aistudio" (Google AI Studio)
	// o vacío (por defecto): se prueba el más probable según la clave y, si no tiene acceso, el otro.
	ProveedorIA string

	// Datos faltantes: cuando no se puede leer la cédula de un candidato se le pide con un enlace
	// de un solo uso (token).
	//   - SolicitudVigenciaDias: cuántos días sirve el enlace (7).
	//   - SolicitudAutomatica: si el sistema envía solo el mensaje al pausar a un candidato. Por
	//     defecto NO: el envío es manual desde la interfaz, para no escribirle a nadie por sorpresa.
	//   - SolicitudSoloPrueba: mientras sea verdadero, los mensajes solo salen hacia las personas de
	//     prueba reconocidas en el código (servidor.esPersonaPrueba).
	//   - EdadMinima: edad mínima para continuar el proceso, validada con la fecha de nacimiento (18).
	//   - PoliticaVersion: identifica la política de datos personales que acepta el candidato.
	SolicitudVigenciaDias  int
	SolicitudAutomatica    bool
	SolicitudSoloPrueba    bool
	EdadMinima             int
	EdadMinimaContratacion int
	EdadMaximaContratacion int
	PoliticaVersion        string
}

// entero lee un entero de una variable de entorno; si falta o no está en [min, max] usa porDefecto.
func entero(nombre string, porDefecto, min, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(nombre)))
	if err != nil || n < min || n > max {
		return porDefecto
	}
	return n
}

// booleano lee una variable de entorno verdadero/falso; si falta usa porDefecto.
func booleano(nombre string, porDefecto bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(nombre))) {
	case "1", "true", "si", "sí":
		return true
	case "0", "false", "no":
		return false
	}
	return porDefecto
}

// MySQLActivo indica si hay una base de datos configurada.
func (c *Configuracion) MySQLActivo() bool {
	return strings.TrimSpace(c.MySQLHost) != ""
}

// Cargar inicializa la configuracion leyendo variables de entorno y definiendo rutas.
func Cargar() (*Configuracion, error) {
	// Intentar cargar .env desde el directorio actual o el directorio padre
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	dirRaiz, err := os.Getwd()
	if err != nil {
		dirRaiz = "."
	}

	dirDescargas := filepath.Join(dirRaiz, "descargas")
	_ = os.MkdirAll(dirDescargas, 0755)

	urlBase := "https://empresa.co.computrabajo.com"

	cfg := &Configuracion{
		URLBase:             urlBase,
		URLLogin:            urlBase + "/Login",
		URLOfertas:          urlBase + "/Company/Offers",
		UsuarioComputrabajo: os.Getenv("COMPUTRABAJO_USUARIO"),
		ClaveComputrabajo:   os.Getenv("COMPUTRABAJO_CONTRASENA"),

		URLProcuraduria: "https://apps.procuraduria.gov.co/webcert/inicio.aspx?tpo=1",
		URLSarlaftONU:   "https://scsanctions.un.org/consolidated-sp/",
		URLOfac:         "https://sanctionssearch.ofac.treas.gov/Default.aspx",
		URLAdres: func() string {
			if u := os.Getenv("URL_ADRES"); strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
			return "https://aplicaciones.adres.gov.co/BDUA_Internet/Pages/ConsultarAfiliadoWeb_2.aspx"
		}(),
		URLRuntAPI: "https://runtproapi.runt.gov.co/CYRConsultaCiudadanoMS",
		URLSimit:   "https://www.fcm.org.co/simit/#/estado-cuenta",

		DirectorioRaiz:        dirRaiz,
		DirectorioDescargas:   dirDescargas,
		DirectorioPerfilCP:    filepath.Join(dirRaiz, "perfil_computrabajo"),
		DirectorioPerfilAdres: filepath.Join(dirRaiz, "perfil_adres"),

		ArchivoSarlaftONU: filepath.Join(dirDescargas, "sarlaft_onu.html"),

		SMTPHost:                strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPuerto:              strings.TrimSpace(os.Getenv("SMTP_PUERTO")),
		SMTPUsuario:             strings.TrimSpace(os.Getenv("SMTP_USUARIO")),
		SMTPClave:               os.Getenv("SMTP_CLAVE"),
		SMTPRemitente:           strings.TrimSpace(os.Getenv("SMTP_REMITENTE")),
		URLPlataformaValidacion: strings.TrimSpace(os.Getenv("URL_PLATAFORMA_VALIDACION")),
		WhatsAppURL:             strings.TrimSpace(os.Getenv("WHATSAPP_API_URL")),
		WhatsAppToken:           strings.TrimSpace(os.Getenv("ACCESS_TOKEN_WHATSAPP")),
		WhatsAppPlantilla: func() string {
			if p := strings.TrimSpace(os.Getenv("WHATSAPP_PLANTILLA")); p != "" {
				return p
			}
			return "plantilla_validar_datos_seleccion_2"
		}(),

		MySQLHost:        strings.TrimSpace(os.Getenv("RRHH_MYSQL_HOST")),
		MySQLPuerto:      strings.TrimSpace(os.Getenv("RRHH_MYSQL_PUERTO")),
		MySQLBase:        strings.TrimSpace(os.Getenv("RRHH_MYSQL_BASE")),
		MySQLUsuario:     strings.TrimSpace(os.Getenv("RRHH_MYSQL_USUARIO")),
		MySQLClave:       os.Getenv("RRHH_MYSQL_CLAVE"),
		ClavePseudonimos: strings.TrimSpace(os.Getenv("RRHH_CLAVE_PSEUDONIMOS")),
		ClaveCifrado:     strings.TrimSpace(os.Getenv("RRHH_CLAVE_CIFRADO")),

		CorreoHost: func() string {
			if h := strings.TrimSpace(os.Getenv("CORREO_IMAP_HOST")); h != "" {
				return h
			}
			return strings.TrimSpace(os.Getenv("SMTP_HOST")) // mismo servidor de correo
		}(),
		CorreoPuerto: func() string {
			if p := strings.TrimSpace(os.Getenv("CORREO_IMAP_PUERTO")); p != "" {
				return p
			}
			return "993"
		}(),
		CorreoUsuario: strings.TrimSpace(os.Getenv("CORREO_USUARIO")),
		CorreoClave:   os.Getenv("CORREO_CLAVE"),

		AdministremosURL:       strings.TrimSpace(os.Getenv("ADMINISTREMOS_URL")),
		AdministremosUsuario:   strings.TrimSpace(os.Getenv("ADMINISTREMOS_USUARIO")),
		AdministremosClave:     os.Getenv("ADMINISTREMOS_CLAVE"),
		AdministremosKey:       os.Getenv("ADMINISTREMOS_KEY"),
		AdministremosToken:     strings.TrimSpace(os.Getenv("ADMINISTREMOS_TOKEN")),
		AdministremosIDEmpresa: entero("ADMINISTREMOS_ID_EMPRESA", 1, 1, 1_000_000),

		APIKeyIA:    strings.TrimSpace(os.Getenv("APIKEY_IA")),
		ProveedorIA: strings.ToLower(strings.TrimSpace(os.Getenv("IA_PROVEEDOR"))),
		ModeloIA: func() string {
			if m := strings.TrimSpace(os.Getenv("IA_MODEL")); m != "" {
				return m
			}
			return "gemini-2.5-flash-lite"
		}(),

		SolicitudVigenciaDias: entero("RRHH_SOLICITUD_VIGENCIA_DIAS", 7, 1, 60),
		SolicitudAutomatica:   booleano("RRHH_SOLICITUD_AUTOMATICA", false),
		SolicitudSoloPrueba:   booleano("RRHH_SOLICITUD_SOLO_PRUEBA", true),
		EdadMinima:            entero("RRHH_EDAD_MINIMA", 18, 14, 70),
		// Regla de contratación: mayor de 21 y menor de 50 (22 a 49 años cumplidos).
		EdadMinimaContratacion: entero("RRHH_EDAD_MIN_CONTRATACION", 22, 14, 70),
		EdadMaximaContratacion: entero("RRHH_EDAD_MAX_CONTRATACION", 49, 18, 99),
		PoliticaVersion: func() string {
			if v := strings.TrimSpace(os.Getenv("RRHH_POLITICA_VERSION")); v != "" {
				return v
			}
			return "sao6-politica-datos-v1"
		}(),

		EPSAutorizadas: []string{
			"SURA", "SURAMERICANA", "NUEVA", "NUEVA EPS", "SAVIA", "SAVIA SALUD", "SANITAS",
		},
		OfertasIncluidas: ofertasIncluidas(),
		OfertasExcluidas: []string{
			"técnico mecánico", "tecnico mecanico", "vehículos pesados", "vehiculos pesados",
		},
	}

	return cfg, nil
}

// CuentasComputrabajo devuelve una configuración por cuenta de Computrabajo: la principal y las
// extra COMPUTRABAJO_USUARIO_2/CONTRASENA_2, _3… (se corta en la primera que falte). Cada cuenta
// tiene su propio perfil de Chrome (perfil_computrabajo_2…) para que las sesiones no se mezclen.
func (c *Configuracion) CuentasComputrabajo() []*Configuracion {
	cuentas := []*Configuracion{c}
	for n := 2; ; n++ {
		sufijo := "_" + strconv.Itoa(n)
		usuario := strings.TrimSpace(os.Getenv("COMPUTRABAJO_USUARIO" + sufijo))
		clave := os.Getenv("COMPUTRABAJO_CONTRASENA" + sufijo)
		if usuario == "" || strings.TrimSpace(clave) == "" {
			return cuentas
		}
		extra := *c
		extra.UsuarioComputrabajo, extra.ClaveComputrabajo = usuario, clave
		extra.DirectorioPerfilCP = c.DirectorioPerfilCP + sufijo
		cuentas = append(cuentas, &extra)
	}
}

// ofertasIncluidas lee RRHH_OFERTAS_INCLUIDAS; por defecto solo conductor y operador.
func ofertasIncluidas() []string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("RRHH_OFERTAS_INCLUIDAS")))
	switch v {
	case "":
		return []string{"conductor", "operador"}
	case "todas":
		return nil
	}
	var lista []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			lista = append(lista, p)
		}
	}
	return lista
}

// OfertaIncluida dice si el título de la vacante tiene alguna de las palabras de OfertasIncluidas
// (sin lista, todas entran).
func (c *Configuracion) OfertaIncluida(titulo string) bool {
	if len(c.OfertasIncluidas) == 0 {
		return true
	}
	bajo := strings.ToLower(titulo)
	for _, p := range c.OfertasIncluidas {
		if strings.Contains(bajo, p) {
			return true
		}
	}
	return false
}

// CorreoActivo indica si hay un buzón configurado para leer hojas de vida.
func (c *Configuracion) CorreoActivo() bool {
	return c.CorreoHost != "" && c.CorreoUsuario != "" && c.CorreoClave != ""
}

// AdministremosActivo dice si hay con qué consultar el servicio web de Administremos.
func (c *Configuracion) AdministremosActivo() bool {
	return c.AdministremosURL != "" && c.AdministremosUsuario != "" && c.AdministremosClave != "" &&
		c.AdministremosKey != "" && c.AdministremosToken != ""
}

// ValidarCredenciales verifica que las credenciales obligatorias de Computrabajo existan.
func (c *Configuracion) ValidarCredenciales() bool {
	return strings.TrimSpace(c.UsuarioComputrabajo) != "" && strings.TrimSpace(c.ClaveComputrabajo) != ""
}
