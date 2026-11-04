package orquestador

import (
	"strings"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
)

// Por qué un descartado no se vuelve a analizar (agrupa el resumen que ve el usuario).
const (
	SigueYaEvaluado     = "Ya se evaluaron con los parámetros vigentes"
	SigueSinCedula      = "Sin cédula: no se puede reanalizar sin pedirle sus datos"
	SigueGenero         = "Género no aceptado"
	SigueEdad           = "Mayor a la edad tope"
	SigueEdadMinima     = "Menor a la edad mínima"
	SigueReintegro      = "Reintegro y no se aceptan reintegros"
	SigueEPS            = "Su EPS sigue sin aceptarse"
	SigueRunt           = "RUNT o licencia siguen sin cumplir"
	SigueSimit          = "SIMIT con deuda mayor a $2.000.000"
	SigueSimitPendiente = "Comparendos o multas en SIMIT siguen sin cumplir"
	SigueIncidentes     = "Incidentes (Procuraduría, ONU u OFAC)"
	SigueSinResultado   = "Sin evaluación guardada"
)

// clasificadorReglas juzga los resultados guardados contra unas reglas (no usa la configuración).
var clasificadorReglas = agentes.NuevoAgenteClasificador(nil)

// MotivoSigueDescartado dice por qué un descartado seguiría sin cumplir con estas reglas, mirando solo lo que
// ya se sabe de él (sin consultar ninguna fuente); "" si con ellas cumple todo lo que se sabe y vale la pena
// reanalizarlo. Cada regla que ya se comprobó (edad, género, reintegro, EPS, RUNT, SIMIT, incidentes) se
// vuelve a juzgar con los parámetros nuevos: si la que lo descartó sigue sin cumplirse, no se le reanaliza.
func MotivoSigueDescartado(c *modelos.Candidato, reglas *modelos.ReglasIdoneidad) string {
	ev := c.EvaluacionIdoneidad
	switch {
	case ev == nil:
		return SigueSinResultado
	case reglas.Version > 0 && ev.ReglaVersion == reglas.Version:
		return SigueYaEvaluado // nada cambió desde su evaluación
	case len(soloDigitos(c.Cedula)) < 6:
		return SigueSinCedula
	}
	genero := ev.Genero
	if genero == "" {
		genero = c.Genero
	}
	// Un género desconocido no descarta aquí: el reanálisis lo vuelve a determinar.
	if (genero == "Hombre" || genero == "Mujer") && !reglas.AceptaGenero(genero) {
		return SigueGenero
	}
	switch nivel, _ := reglas.NivelEdad(c.Edad); nivel {
	case modelos.NivelDescarte:
		return SigueEdad
	case modelos.NivelSemillero:
		return SigueEdadMinima
	}
	// Reintegro: el resultado de Administremos o, si se evaluó antes de guardarlo, el motivo del descarte.
	if agentes.MotivoReintegro(c, reglas) != "" || (!reglas.AceptaReintegros && tieneMotivo(ev, "Reintegro")) {
		return SigueReintegro
	}
	// Cada fuente ya consultada se juzga otra vez con las reglas nuevas; una consulta fallida no cuenta.
	if r := c.ResultadoAdres; r != nil && !adresSinVerificar(r) {
		if ok, _ := clasificadorReglas.EvaluarEPS(r, reglas); !ok {
			return SigueEPS
		}
	}
	if r := c.ResultadoRunt; r != nil && !runtSinVerificar(r) {
		if ok, _ := clasificadorReglas.EvaluarRunt(r, reglas); !ok {
			return SigueRunt
		}
	}
	if r := c.ResultadoSimit; r != nil && !simitSinVerificar(r) {
		switch nivel, _ := agentes.NivelSimit(r); nivel {
		case modelos.NivelDescarte:
			return SigueSimit
		case modelos.NivelSemillero:
			return SigueSimitPendiente
		}
	}
	if len(agentes.MotivoIncidentes(c)) > 0 {
		return SigueIncidentes
	}
	return ""
}

// EsDescartadoFirme dice si el candidato está descartado con dictamen (no pendiente, ni en pausa, ni
// semillero ni reclutado): solo a esos se les da una segunda oportunidad.
func EsDescartadoFirme(c *modelos.Candidato) bool {
	ev := c.EvaluacionIdoneidad
	return ev != nil && !ev.EsApto && !c.EsApto && !ev.VerificacionPendiente && !modelos.EnPausa(c.EstadoProceso) &&
		c.ErrorProceso == "" && ev.Clasificacion != modelos.ClasificacionSemillero && ev.Clasificacion != modelos.ClasificacionReclutadoP1
}

func tieneMotivo(ev *modelos.EvaluacionIdoneidad, prefijo string) bool {
	for _, m := range ev.MotivosDescarte {
		if strings.HasPrefix(m, prefijo) {
			return true
		}
	}
	return false
}

// Las mismas condiciones de "consulta fallida" de las etapas de ADRES, RUNT y SIMIT (etapasNucleo).
func adresSinVerificar(r *modelos.ResultadoAdres) bool {
	return strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" || r.Estado == "PORTAL_EN_MANTENIMIENTO"
}

func runtSinVerificar(r *modelos.ResultadoRunt) bool {
	return strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_CONSULTADO_SIN_CEDULA" ||
		r.Estado == "SIN_APELLIDO" || r.Estado == "APELLIDO_NO_COINCIDE"
}

func simitSinVerificar(r *modelos.ResultadoSimit) bool {
	return strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" || r.Estado == "SIN_CEDULA"
}

// ResumenSegundaOportunidad cuenta, entre los descartados, a quiénes vale la pena reanalizar.
type ResumenSegundaOportunidad struct {
	Version    int                 `json:"version"`
	Total      int                 `json:"total"`
	Reanalizar []PersonaReanalisis `json:"reanalizar"`
	Omitidos   []GrupoOmitidos     `json:"omitidos"`
}

// PersonaReanalisis es un descartado que se vuelve a analizar con los parámetros vigentes.
type PersonaReanalisis struct {
	PostulacionID string `json:"postulacion_id"`
	Nombre        string `json:"nombre"`
	ReglaVersion  int    `json:"regla_version"` // con la que se descartó
	Motivo        string `json:"motivo"`        // por qué se descartó entonces
}

// GrupoOmitidos son los descartados que siguen igual, por motivo.
type GrupoOmitidos struct {
	Motivo   string `json:"motivo"`
	Cantidad int    `json:"cantidad"`
}

// SegundaOportunidad separa a los descartados entre los que vale la pena reanalizar con reglas y los que no.
func SegundaOportunidad(descartados []modelos.Candidato, reglas *modelos.ReglasIdoneidad) ResumenSegundaOportunidad {
	res := ResumenSegundaOportunidad{Version: reglas.Version, Total: len(descartados), Reanalizar: []PersonaReanalisis{}, Omitidos: []GrupoOmitidos{}}
	cuenta := map[string]int{}
	var orden []string
	for i := range descartados {
		c := &descartados[i]
		if m := MotivoSigueDescartado(c, reglas); m != "" {
			if cuenta[m] == 0 {
				orden = append(orden, m)
			}
			cuenta[m]++
			continue
		}
		motivo := "No cumple los requisitos"
		if ev := c.EvaluacionIdoneidad; len(ev.MotivosDescarte) > 0 {
			motivo = ev.MotivosDescarte[0]
		}
		res.Reanalizar = append(res.Reanalizar, PersonaReanalisis{PostulacionID: c.PostulacionID, Nombre: c.Nombre,
			ReglaVersion: c.EvaluacionIdoneidad.ReglaVersion, Motivo: motivo})
	}
	for _, m := range orden {
		res.Omitidos = append(res.Omitidos, GrupoOmitidos{Motivo: m, Cantidad: cuenta[m]})
	}
	return res
}
