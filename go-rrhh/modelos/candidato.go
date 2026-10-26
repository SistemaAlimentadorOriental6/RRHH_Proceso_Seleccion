package modelos

import "time"

// DetalleNoConsultado es el detalle de una regla que no se evaluó por descarte temprano. La base de
// datos lo guarda como "sin evaluar" (NULL) y no como "no cumple".
const DetalleNoConsultado = "No consultado (descarte temprano)"

// OfertaLaboral representa una vacante activa en Computrabajo.
type OfertaLaboral struct {
	Titulo        string `json:"titulo"`
	URLCandidatos string `json:"url_candidatos"`
	// ID es el identificador de la oferta en Computrabajo (parámetro "oi" del enlace).
	ID string `json:"id,omitempty"`
	// Cuenta es el índice (desde 0) de la cuenta de Computrabajo dueña de la oferta.
	Cuenta int `json:"-"`
}

// Candidato representa a una persona postulada con sus datos y resultados de validacion.
type Candidato struct {
	Indice         int    `json:"indice"`
	Nombre         string `json:"nombre"`
	Cedula         string `json:"cedula"`
	URLDetalle     string `json:"url_detalle"`
	URLFoto        string `json:"foto_url,omitempty"`
	RutaFoto       string `json:"ruta_foto,omitempty"`
	RutaPDF        string `json:"ruta_pdf,omitempty"`
	RutaCVOriginal string `json:"ruta_cv_original,omitempty"`
	RutaPerfilPDF  string `json:"ruta_perfil_pdf,omitempty"`
	URLDescargaCV  string `json:"url_descarga_cv,omitempty"`
	Genero         string `json:"genero,omitempty"`
	TextoPerfil    string `json:"texto_perfil,omitempty"`

	// CarpetaCandidato es donde se guardan los archivos del candidato; los agentes dejan allí sus capturas.
	CarpetaCandidato string `json:"-"`
	// Capturas relaciona cada etapa (perfil, adres, runt...) con la imagen JPEG guardada en disco.
	Capturas map[string]string `json:"capturas,omitempty"`

	// Resultados de validaciones externas
	ResultadoSarlaft      *ResultadoSarlaft      `json:"sarlaft_onu,omitempty"`
	ResultadoOfac         *ResultadoOfac         `json:"ofac,omitempty"`
	ResultadoProcuraduria *ResultadoProcuraduria `json:"procuraduria,omitempty"`
	ResultadoAdres        *ResultadoAdres        `json:"adres,omitempty"`
	ResultadoRunt         *ResultadoRunt         `json:"runt,omitempty"`
	ResultadoSimit        *ResultadoSimit        `json:"simit,omitempty"`
	ResultadoReintegro    *ResultadoReintegro    `json:"reintegro,omitempty"`

	// Calificacion final de idoneidad
	EsApto              bool                 `json:"es_apto"`
	EvaluacionIdoneidad *EvaluacionIdoneidad `json:"evaluacion_idoneidad,omitempty"`

	// EvaluacionIA es la calificación de la IA sobre la hoja de vida y los datos oficiales; solo la
	// tienen los candidatos aptos y puede faltar si la IA no pudo evaluarlos (se reintenta al reanalizar).
	EvaluacionIA *EvaluacionIA `json:"evaluacion_ia,omitempty"`

	// ErrorProceso se llena cuando el candidato no pudo analizarse por completo (ficha ilegible o
	// consulta externa caída). Un candidato con error se reprocesa en el siguiente análisis.
	ErrorProceso string `json:"error_proceso,omitempty"`

	// EstadoProceso dice en qué punto quedó el candidato cuando su análisis está en pausa o recién
	// se reanudó (esperando_datos, sin_contacto, datos_recibidos, solicitud_vencida,
	// identidad_en_revision). Vacío si el análisis siguió su curso normal.
	EstadoProceso string `json:"estado_proceso,omitempty"`
	// PostulacionID identifica al candidato en la base de datos (UUID).
	PostulacionID string `json:"postulacion_id,omitempty"`
	// Edad se calcula a partir de la fecha de nacimiento que declaró el candidato; la fecha no se expone.
	Edad int `json:"edad,omitempty"`
	// IntentosCedula son los métodos con los que se buscó la cédula (no se guardan en el JSON).
	IntentosCedula []IntentoCedula `json:"-"`
}

// Estados de un candidato cuyo análisis está en pausa o se reanuda por falta de datos.
const (
	EstadoEsperandoDatos      = "esperando_datos"       // se le pidieron sus datos y no ha respondido
	EstadoSinContacto         = "sin_contacto"          // no hay correo ni celular a quien pedírselos
	EstadoDatosRecibidos      = "datos_recibidos"       // respondió; falta reanudar la consulta a las fuentes
	EstadoSolicitudVencida    = "solicitud_vencida"     // el enlace venció sin respuesta
	EstadoIdentidadEnRevision = "identidad_en_revision" // la cédula declarada no cuadra con su nombre
)

// EnPausa indica que el candidato no se debe volver a leer de Computrabajo: ya está en manos del
// proceso de datos faltantes.
func EnPausa(estado string) bool {
	switch estado {
	case EstadoEsperandoDatos, EstadoSinContacto, EstadoDatosRecibidos, EstadoSolicitudVencida, EstadoIdentidadEnRevision:
		return true
	}
	return false
}

// IntentoCedula es un método con el que se buscó la cédula y si la encontró.
type IntentoCedula struct {
	Metodo  string `json:"metodo"` // ficha_html | texto_pdf | texto_perfil | texto_docx | postulacion_previa
	Hallada bool   `json:"hallada"`
}

// PausaDatos describe por qué y dónde se dejó en pausa el análisis de un candidato.
type PausaDatos struct {
	Motivo       string // cedula_no_encontrada
	EtapaPausada string // primera etapa que depende del dato que falta (adres)
	Correo       string // contacto hallado en la hoja de vida (puede estar vacío)
	Telefono     string
}

// EvaluacionIdoneidad contiene el dictamen de las 4 reglas de negocio de la vacante.
type EvaluacionIdoneidad struct {
	EsApto          bool     `json:"es_apto"`
	Genero          string   `json:"genero"`
	CumpleGenero    bool     `json:"cumple_genero"`
	CumpleEPS       bool     `json:"cumple_eps"`
	DetalleEPS      string   `json:"detalle_eps"`
	CumpleRunt      bool     `json:"cumple_runt"`
	DetalleRunt     string   `json:"detalle_runt"`
	CumpleSimit     bool     `json:"cumple_simit"`
	DetalleSimit    string   `json:"detalle_simit"`
	MotivosDescarte []string `json:"motivos_descarte"`
	// DescarteTemprano indica que se dejó de consultar en cuanto una regla falló: los detalles de
	// las reglas no evaluadas dicen "No consultado".
	DescarteTemprano bool `json:"descarte_temprano,omitempty"`
	// VerificacionPendiente indica que NO se pudo comprobar alguna regla porque una consulta externa
	// falló (portal caído, captcha, etc.). No es un rechazo: el candidato se reintenta en el siguiente análisis.
	VerificacionPendiente bool `json:"verificacion_pendiente,omitempty"`
}

// RegistroApto estructura la informacion final que se consolida en candidatos_aptos.json.
type RegistroApto struct {
	Nombre            string                 `json:"nombre"`
	Cedula            string                 `json:"cedula"`
	Genero            string                 `json:"genero"`
	Evaluacion        map[string]string      `json:"evaluacion"`
	TextoPerfil       string                 `json:"texto_perfil,omitempty"`
	RutaPDF           string                 `json:"ruta_pdf"`
	RutaCVOriginal    string                 `json:"ruta_cv_original,omitempty"`
	RutaPerfilPDF     string                 `json:"ruta_perfil_pdf,omitempty"`
	Adres             *ResultadoAdres        `json:"adres,omitempty"`
	Runt              *ResultadoRunt         `json:"runt,omitempty"`
	Simit             *ResultadoSimit        `json:"simit,omitempty"`
	Procuraduria      *ResultadoProcuraduria `json:"procuraduria,omitempty"`
	SarlaftONU        *ResultadoSarlaft      `json:"sarlaft_onu,omitempty"`
	Ofac              *ResultadoOfac         `json:"ofac,omitempty"`
	FechaCalificacion time.Time              `json:"fecha_calificacion"`
}

// ResultadoSarlaft almacena la coincidencia contra la lista ONU.
type ResultadoSarlaft struct {
	Coincidencia      bool   `json:"coincidencia"`
	Alerta            bool   `json:"alerta"`
	TipoCoincidencia  string `json:"tipo_coincidencia,omitempty"`
	IDONU             string `json:"id_onu,omitempty"`
	NombreSancionado  string `json:"nombre_sancionado,omitempty"`
	Detalles          string `json:"detalles"`
	FechaVerificacion string `json:"fecha_verificacion"`
}

// ResultadoOfac almacena las coincidencias con la lista de la OFAC (EE. UU.).
type ResultadoOfac struct {
	Coincidencia       bool           `json:"coincidencia"`
	Alerta             bool           `json:"alerta"`
	Estado             string         `json:"estado"`
	TotalCoincidencias int            `json:"total_coincidencias"`
	Registros          []RegistroOfac `json:"registros"`
	Detalles           string         `json:"detalles,omitempty"`
}

// RegistroOfac fila de resultado OFAC.
type RegistroOfac struct {
	Nombre    string `json:"nombre"`
	Direccion string `json:"direccion"`
	Tipo      string `json:"tipo"`
	Programa  string `json:"programa"`
	Lista     string `json:"lista"`
	Score     string `json:"score"`
}

// ResultadoProcuraduria almacena antecedentes disciplinarios.
type ResultadoProcuraduria struct {
	Estado       string `json:"estado"`
	Alerta       bool   `json:"alerta"`
	Coincidencia bool   `json:"coincidencia"`
	Detalles     string `json:"detalles,omitempty"`
}

// ResultadoAdres informacion de afiliacion en salud EPS.
type ResultadoAdres struct {
	Estado           string `json:"estado"`
	Documento        string `json:"documento"`
	Nombres          string `json:"nombres"`
	Apellidos        string `json:"apellidos"`
	Departamento     string `json:"departamento"`
	Municipio        string `json:"municipio"`
	EPS              string `json:"eps"`
	Regimen          string `json:"regimen"`
	EstadoAfiliacion string `json:"estado_afiliacion"`
	TipoAfiliado     string `json:"tipo_afiliado"`
	FechaAfiliacion  string `json:"fecha_afiliacion"`
	CumpleEPS        bool   `json:"cumple_eps"`
	Alerta           bool   `json:"alerta"`
	Detalles         string `json:"detalles,omitempty"`
}

// ResultadoSimit informacion de comparendos y multas de transito.
type ResultadoSimit struct {
	Estado              string `json:"estado"`
	PazYSalvo           bool   `json:"paz_y_salvo"`
	CantidadComparendos int    `json:"cantidad_comparendos"`
	CantidadMultas      int    `json:"cantidad_multas"`
	CantidadAcuerdos    int    `json:"cantidad_acuerdos"`
	TotalDeuda          string `json:"total_deuda"`
	Alerta              bool   `json:"alerta"`
	Detalles            string `json:"detalles,omitempty"`
}

// ResultadoRunt informacion del ciudadano en el RUNT.
type ResultadoRunt struct {
	Estado             string         `json:"estado"`
	Inscrito           bool           `json:"inscrito"`
	ApellidoConsultado string         `json:"apellido_consultado"`
	IDPersona          string         `json:"id_persona,omitempty"`
	Nombres            string         `json:"nombres,omitempty"`
	Apellidos          string         `json:"apellidos,omitempty"`
	Documento          string         `json:"documento,omitempty"`
	FechaInscripcion   string         `json:"fecha_inscripcion,omitempty"`
	EstadoPersona      string         `json:"estado_persona,omitempty"`
	ConductorActivo    bool           `json:"conductor_activo"`
	EstadoConductor    string         `json:"estado_conductor,omitempty"`
	TieneLicencias     bool           `json:"tiene_licencias"`
	Licencias          []LicenciaRunt `json:"licencias"`
	CategoriasActivas  []string       `json:"categorias_activas"`
	TieneMultas        bool           `json:"tiene_multas"`
	PazYSalvo          bool           `json:"paz_y_salvo"`
	NumeroPazYSalvo    string         `json:"numero_paz_y_salvo,omitempty"`
	Alerta             bool           `json:"alerta"`
	Error              string         `json:"error,omitempty"`
}

// LicenciaRunt detalle de licencia individual en RUNT.
type LicenciaRunt struct {
	NumeroLicencia     string             `json:"numero_licencia"`
	Categorias         []string           `json:"categorias"`
	DetallesCategorias []DetalleCategoria `json:"detalles_categorias"`
	FechaExpedicion    string             `json:"fecha_expedicion"`
	Estado             string             `json:"estado"`
	Retenida           bool               `json:"retenida"`
	OTExpide           string             `json:"ot_expide"`
	Restricciones      string             `json:"restricciones,omitempty"`
}

// DetalleCategoria especificacion por categoria de vehiculo.
type DetalleCategoria struct {
	Categoria        string `json:"categoria"`
	FechaVencimiento string `json:"fecha_vencimiento"`
	Restricciones    string `json:"restricciones,omitempty"`
	Limitaciones     string `json:"limitaciones,omitempty"`
}
