package modelos

// Estados de ResultadoReintegro.
const (
	ReintegroSinRegistro    = "SIN_REGISTRO"    // la cédula no figura como empleado
	ReintegroExEmpleado     = "EX_EMPLEADO"     // fue empleado y está retirado
	ReintegroEmpleadoActivo = "EMPLEADO_ACTIVO" // figura como empleado vigente
	ReintegroErrorConsulta  = "ERROR_CONSULTA"  // el servicio no respondió bien: no hay evidencia
)

// ResultadoReintegro es lo que dice Administremos sobre una cédula: si esa persona ya trabajó (o trabaja)
// en la empresa. Quien ya fue (o es) empleado no puede volver a entrar: el análisis lo descarta.
type ResultadoReintegro struct {
	Estado string `json:"estado"`
	Alerta bool   `json:"alerta"`
	// Detalles es la frase que se muestra en el flujo.
	Detalles string `json:"detalles,omitempty"`
	// Datos del registro más reciente de esa cédula (vacíos si no figura).
	EstadoEmpleado string `json:"estado_empleado,omitempty"`
	Cargo          string `json:"cargo,omitempty"`
	TipoContrato   string `json:"tipo_contrato,omitempty"`
	FechaIngreso   string `json:"fecha_ingreso,omitempty"`
	FechaRetiro    string `json:"fecha_retiro,omitempty"`
	Sanciones      string `json:"sanciones,omitempty"`
	// Registros es cuántas veces figura esa cédula (reingresos).
	Registros int `json:"registros,omitempty"`
}
