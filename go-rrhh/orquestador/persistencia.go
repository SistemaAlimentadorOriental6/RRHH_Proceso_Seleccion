package orquestador

import (
	"context"
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
	// MarcarVigentes deja vigentes solo a los postulantes de urls y retira a los demás.
	MarcarVigentes(ctx context.Context, oferta modelos.OfertaLaboral, urls []string) error
}

const plazoPersistencia = 30 * time.Second

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
