package orquestador

import (
	"context"
	"sync"
	"time"

	"go-rrhh/modelos"
)

// Persistencia es la base de datos del análisis (la implementa almacen.Almacen): guarda a cada
// candidato al terminar y devuelve lo ya analizado para no repetirlo. Es opcional: sin ella todo
// se guarda y se lee de los JSON, como antes.
type Persistencia interface {
	GuardarCandidato(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato) error
	// CandidatosPrevios devuelve lo ya analizado de la vacante, indexado por la URL de la ficha.
	CandidatosPrevios(ctx context.Context, oferta modelos.OfertaLaboral) (map[string]*modelos.Candidato, error)
	// MarcarVigentes deja vigentes a los postulantes de urls (en el orden del listado, que es su número)
	// y retira a los de retirados; los demás quedan como estaban.
	MarcarVigentes(ctx context.Context, oferta modelos.OfertaLaboral, urls, retirados []string) error
}

const plazoPersistencia = 30 * time.Second

// ausentes recuerda, por vacante, quién faltó en el último listado completo. Solo se retira a quien falta
// dos veces seguidas: Computrabajo a veces entrega el listado corto (el "cargar más" no responde a tiempo)
// y retirar a la primera hacía que los aptos desaparecieran de la pantalla y volvieran en el ciclo siguiente.
// shortcut: vive en memoria; tras reiniciar, el primer listado no retira a nadie. Pasarlo a la base si
// hiciera falta retirar antes.
var (
	muAusentes sync.Mutex
	ausentes   = map[string]map[string]bool{}
)

// ausentesConfirmados devuelve las fichas de previos que faltan en vistos y ya faltaban la vez anterior.
func ausentesConfirmados(idOferta string, previos map[string]*modelos.Candidato, vistos map[string]bool) []string {
	muAusentes.Lock()
	defer muAusentes.Unlock()
	antes := ausentes[idOferta]
	ahora := map[string]bool{}
	var confirmados []string
	for u := range previos {
		if vistos[u] {
			continue
		}
		ahora[u] = true
		if antes[u] {
			confirmados = append(confirmados, u)
		}
	}
	ausentes[idOferta] = ahora
	return confirmados
}

// guardarEnBase guarda al candidato sin heredar la cancelación del análisis: un candidato que ya
// terminó se conserva aunque el usuario cancele el job justo después.
func guardarEnBase(p Persistencia, oferta modelos.OfertaLaboral, c *modelos.Candidato) error {
	if p == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), plazoPersistencia)
	defer cancel()
	return p.GuardarCandidato(ctx, oferta, c)
}
