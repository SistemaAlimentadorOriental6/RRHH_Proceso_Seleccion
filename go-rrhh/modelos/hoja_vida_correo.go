package modelos

import "time"

// HojaVidaCorreo es una hoja de vida (o algo que lo parece) recibida por correo, con lo que se pudo
// leer de ella. Cédula, teléfono y correo pueden estar vacíos si la persona no los escribió.
type HojaVidaCorreo struct {
	UID       uint32    `json:"uid"`
	MessageID string    `json:"message_id,omitempty"`
	Fecha     time.Time `json:"fecha"`
	Asunto    string    `json:"asunto"`
	// Remitente es la dirección desde la que escribió; NombreRemitente, el nombre que muestra su correo.
	Remitente       string `json:"remitente"`
	NombreRemitente string `json:"nombre_remitente,omitempty"`

	// Nombre es el de la persona: el que puso en el formulario o, si no, el que muestra su correo.
	Nombre string `json:"nombre,omitempty"`

	Cedula   string `json:"cedula,omitempty"`
	Telefono string `json:"telefono,omitempty"` // 10 dígitos, sin +57
	Correo   string `json:"correo,omitempty"`   // el de la hoja de vida; si no trae, el del remitente

	// Motivo explica por qué el correo se tomó como hoja de vida.
	Motivo string `json:"motivo"`
	// ParecidoCV es verdadero si el texto leído tiene la estructura típica de una hoja de vida.
	ParecidoCV bool `json:"parecido_cv"`

	// InteresConductor: "si", "dudoso" o "no" (¿busca trabajo como conductor u operador?).
	InteresConductor string `json:"interes_conductor"`
	MotivoConductor  string `json:"motivo_conductor"`
	DecididoPorIA    bool   `json:"decidido_por_ia,omitempty"`

	// TextoCV es lo leído de la hoja de vida y del correo (hasta 20 000 caracteres): de aquí salen el
	// género, la cédula y el contacto cuando la persona entra al análisis.
	TextoCV string `json:"texto_cv,omitempty"`

	// CuerpoCorreo es el comienzo de lo que escribió la persona (para revisar el caso a mano).
	CuerpoCorreo string `json:"cuerpo_correo,omitempty"`
	// HuellaAdjuntos identifica los adjuntos: el mismo CV reenviado o enviado dos veces no se repite.
	HuellaAdjuntos string `json:"huella_adjuntos,omitempty"`
	// LeidoConIA es verdadero si el texto de algún adjunto escaneado o fotografiado se transcribió con IA.
	LeidoConIA bool `json:"leido_con_ia,omitempty"`

	Archivos        []string `json:"archivos,omitempty"`          // adjuntos guardados en disco
	AdjuntosSinLeer []string `json:"adjuntos_sin_leer,omitempty"` // imágenes u otros formatos que no se pueden leer
	TextoLeido      bool     `json:"texto_leido"`                 // falso si no se pudo sacar texto de nada
}
