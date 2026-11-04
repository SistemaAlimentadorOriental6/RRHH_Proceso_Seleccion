package servidor

import (
	"context"
	"log"
	"net/http"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
)

// reanalisisBase lo implementa almacen.Almacen: los descartados y la cola para reanalizarlos.
type reanalisisBase interface {
	ParametrosVigentes(ctx context.Context) (almacen.ParametrosAceptacion, error)
	DescartadosVigentes(ctx context.Context) ([]modelos.Candidato, error)
	EncolarReanalisis(ctx context.Context, postulaciones []string) (int, error)
	ReanalisisEnCola(ctx context.Context) ([]string, error)
}

// ManejarColaReanalisis devuelve las postulaciones con un reanálisis pendiente o en curso
// (GET /api/descartados/reanalisis): la cola es la misma para todos los usuarios.
func (c *ControladorRRHH) ManejarColaReanalisis(w http.ResponseWriter, r *http.Request) {
	if c.reanalisis == nil {
		ResponderJSON(w, http.StatusOK, map[string][]string{"en_cola": {}})
		return
	}
	ids, err := c.reanalisis.ReanalisisEnCola(r.Context())
	if err != nil {
		log.Printf("[!] No se pudo leer la cola de reanálisis: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo leer la cola de reanálisis"})
		return
	}
	ResponderJSON(w, http.StatusOK, map[string][]string{"en_cola": ids})
}

// respuestaReanalisis es el resumen de la segunda oportunidad; Encolados solo viene si no se simuló.
type respuestaReanalisis struct {
	orquestador.ResumenSegundaOportunidad
	Simulado  bool `json:"simulado"`
	Encolados int  `json:"encolados"`
}

// ManejarReanalizarDescartados da una segunda oportunidad a los descartados con los parámetros vigentes
// (POST /api/descartados/reanalizar). Solo reanaliza a quien podría cambiar de resultado: quien sigue
// descartado por lo que ya se sabe de él (edad, género, reintegro, SIMIT, incidentes) no gasta consultas.
// Con ?simular=1 solo devuelve el resumen, sin encolar nada.
func (c *ControladorRRHH) ManejarReanalizarDescartados(w http.ResponseWriter, r *http.Request) {
	if c.reanalisis == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Reanalizar necesita la base de datos, que no está configurada"})
		return
	}
	reglas, err := c.reanalisis.ParametrosVigentes(r.Context())
	if err != nil {
		responderParametros(w, reglas, err)
		return
	}
	descartados, err := c.reanalisis.DescartadosVigentes(r.Context())
	if err != nil {
		log.Printf("[!] No se pudieron leer los descartados: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudieron leer los descartados de la base de datos"})
		return
	}

	// Solo descartados firmes: un pendiente o en pausa no tiene dictamen que revisar.
	firmes := descartados[:0]
	for _, d := range descartados {
		if orquestador.EsDescartadoFirme(&d) {
			firmes = append(firmes, d)
		}
	}
	res := respuestaReanalisis{ResumenSegundaOportunidad: orquestador.SegundaOportunidad(firmes, &reglas)}
	res.Simulado = r.URL.Query().Get("simular") == "1"
	if !res.Simulado && len(res.Reanalizar) > 0 {
		ids := make([]string, len(res.Reanalizar))
		for i, p := range res.Reanalizar {
			ids[i] = p.PostulacionID
		}
		if res.Encolados, err = c.reanalisis.EncolarReanalisis(r.Context(), ids); err != nil {
			log.Printf("[!] No se pudo encolar el reanálisis de descartados: %v", err)
			ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No se pudo poner en cola el reanálisis"})
			return
		}
		log.Printf("[+] Reanálisis con parámetros v%d: %d descartados en cola", reglas.Version, res.Encolados)
		c.cambios.publicar("reanalisis_encolado") // las demás pantallas abiertas ven la cola nueva
		c.despertarSegundoPlano()
	}
	ResponderJSON(w, http.StatusOK, res)
}
