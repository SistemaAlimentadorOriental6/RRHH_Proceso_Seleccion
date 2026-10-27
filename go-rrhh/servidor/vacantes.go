package servidor

import (
	"fmt"
	"net/http"
	"strconv"

	"go-rrhh/modelos"
)

// CandidatoVacante es un candidato con el slug de su nombre, para rutas amigables en el frontend
// (/vacantes/{slug}/candidatos/{slug-persona}). El Candidato embebido se serializa aplanado.
type CandidatoVacante struct {
	modelos.Candidato
	Slug string `json:"slug"`
}

// DetalleCandidatoVacante agrega, si existe, la evaluación de IA de la persona dentro de la vacante.
type DetalleCandidatoVacante struct {
	CandidatoVacante
	Ranking *modelos.CandidatoCalificadoIA `json:"ranking,omitempty"`
}

func conSlug(lista []modelos.Candidato) []CandidatoVacante {
	salida := make([]CandidatoVacante, len(lista))
	for i, c := range lista {
		salida[i] = CandidatoVacante{Candidato: c, Slug: SlugPersona(c.Nombre)}
	}
	return salida
}

// ManejarVacantes lista las vacantes (una por slug; si hay carpetas duplicadas gana la más reciente).
func (c *ControladorRRHH) ManejarVacantes(w http.ResponseWriter, r *http.Request) {
	ofertas, err := c.repositorio.ListarOfertas()
	if err != nil {
		responderError(w, err)
		return
	}

	vistas := make(map[string]int, len(ofertas))
	unicas := make([]ResumenOferta, 0, len(ofertas))
	for _, o := range ofertas {
		if i, existe := vistas[o.Slug]; existe {
			if o.ActualizadoEn.After(unicas[i].ActualizadoEn) {
				unicas[i] = o
			}
			continue
		}
		vistas[o.Slug] = len(unicas)
		unicas = append(unicas, o)
	}
	ResponderJSON(w, http.StatusOK, unicas)
}

// ManejarVacante devuelve el resumen de una vacante por slug.
func (c *ControladorRRHH) ManejarVacante(w http.ResponseWriter, r *http.Request) {
	oferta, err := c.repositorio.ResolverOferta(r.PathValue("slug"))
	if err != nil {
		responderError(w, err)
		return
	}
	ResponderJSON(w, http.StatusOK, oferta)
}

// ManejarVacanteCandidatos lista los candidatos de una vacante (?filtro=aptos|no_aptos, ?limite=&offset=,
// ?incluir_cv=1 para incluir el texto del perfil).
func (c *ControladorRRHH) ManejarVacanteCandidatos(w http.ResponseWriter, r *http.Request) {
	oferta, err := c.repositorio.ResolverOferta(r.PathValue("slug"))
	if err != nil {
		responderError(w, err)
		return
	}

	q := r.URL.Query()
	candidatos, err := c.candidatosOferta(oferta.ID, q.Get("filtro"), quiereCV(q))
	if err != nil {
		responderError(w, err)
		return
	}
	pagina, total, err := paginar(conSlug(candidatos), q)
	if err != nil {
		responderError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	ResponderJSONConETag(w, r, pagina)
}

// buscarCandidato localiza a una persona de la vacante por cédula o por el slug de su nombre.
func (c *ControladorRRHH) buscarCandidato(idOferta, id string) (*CandidatoVacante, error) {
	candidatos, err := c.repositorio.ObtenerCandidatosOferta(idOferta, "")
	if err != nil {
		return nil, err
	}
	return buscarEn(candidatos, id)
}

// buscarEn localiza a una persona, por cédula o por el slug de su nombre, en candidatos ya leídos.
func buscarEn(candidatos []modelos.Candidato, id string) (*CandidatoVacante, error) {
	idDigitos := soloDigitos(id)
	esCedula := idDigitos != "" && idDigitos == id
	for _, cv := range conSlug(candidatos) {
		if (esCedula && soloDigitos(cv.Cedula) == idDigitos) || (!esCedula && cv.Slug == id) {
			return &cv, nil
		}
	}
	return nil, fmt.Errorf("%w: candidato %q en la vacante", ErrNoEncontrado, id)
}

// ManejarVacanteCandidato devuelve una persona de la vacante, identificada por cédula o por el slug de su nombre.
func (c *ControladorRRHH) ManejarVacanteCandidato(w http.ResponseWriter, r *http.Request) {
	oferta, err := c.repositorio.ResolverOferta(r.PathValue("slug"))
	if err != nil {
		responderError(w, err)
		return
	}
	// Una sola lectura de la vacante: de ella salen la persona y los aptos para su puesto en el ranking
	// (antes se leía completa dos veces).
	candidatos, err := c.repositorio.ObtenerCandidatosOferta(oferta.ID, "")
	if err != nil {
		responderError(w, err)
		return
	}
	encontrado, err := buscarEn(candidatos, r.PathValue("candidato"))
	if err != nil {
		responderError(w, err)
		return
	}

	detalle := DetalleCandidatoVacante{CandidatoVacante: *encontrado}
	if global, err := c.repositorio.ObtenerRankingIA(); err == nil {
		ranking := rankingDeAptos(candidatos, global)
		for i := range ranking {
			if soloDigitos(ranking[i].Candidato.Cedula) == soloDigitos(encontrado.Cedula) {
				item := sinTextoCV(ranking[i : i+1])[0]
				detalle.Ranking = &item
				break
			}
		}
	}
	ResponderJSON(w, http.StatusOK, detalle)
}

// ManejarVacanteRanking devuelve el ranking de IA de una vacante (?incluir_cv=1 agrega el texto del CV).
func (c *ControladorRRHH) ManejarVacanteRanking(w http.ResponseWriter, r *http.Request) {
	oferta, err := c.repositorio.ResolverOferta(r.PathValue("slug"))
	if err != nil {
		responderError(w, err)
		return
	}
	ranking, err := c.repositorio.ObtenerRankingOferta(oferta.ID)
	if err != nil {
		responderError(w, err)
		return
	}

	q := r.URL.Query()
	if !quiereCV(q) {
		ranking = sinTextoCV(ranking)
	}
	pagina, total, err := paginar(ranking, q)
	if err != nil {
		responderError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	ResponderJSON(w, http.StatusOK, pagina)
}
