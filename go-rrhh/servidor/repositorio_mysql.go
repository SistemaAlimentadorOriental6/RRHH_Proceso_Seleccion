package servidor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/almacen"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
)

const plazoLecturaBase = 30 * time.Second

// LectorBase es lo que la API lee de la base de datos (lo implementa almacen.Almacen).
type LectorBase interface {
	OfertasProcesadas(ctx context.Context) ([]almacen.OfertaProcesada, error)
	CandidatosDeOferta(ctx context.Context, idComputrabajo string) ([]modelos.Candidato, error)
}

// Base es la base de datos completa del servidor: guarda lo que se analiza, lo devuelve y atiende el
// flujo de datos faltantes (pausas, solicitudes con token y reanudaciones).
type Base interface {
	orquestador.Persistencia
	orquestador.Pausas
	LectorBase
	solicitudesPublicas
	enviosBase
	trabajosBase
	historialBase
}

// RepositorioMySQL sirve las vacantes y sus candidatos desde la base de datos, con la misma forma
// de respuesta que RepositorioDisco. El ranking de IA y los registros de aptos siguen en archivos
// hasta que se migren (se heredan de RepositorioDisco).
//
// El id de una vacante es el nombre que tenía su carpeta (el título sin caracteres inválidos),
// para que las rutas y enlaces de siempre sigan funcionando.
type RepositorioMySQL struct {
	*RepositorioDisco
	base      LectorBase
	descargas string
}

// NuevoRepositorioMySQL crea el repositorio; disco aporta lo que aún vive en archivos.
func NuevoRepositorioMySQL(disco *RepositorioDisco, base LectorBase, descargas string) *RepositorioMySQL {
	return &RepositorioMySQL{RepositorioDisco: disco, base: base, descargas: descargas}
}

// ofertaLeida es una vacante con la clave con la que se buscan sus candidatos en la base.
type ofertaLeida struct {
	ResumenOferta
	clave string
}

func (r *RepositorioMySQL) ofertas() ([]ofertaLeida, error) {
	ctx, cancel := context.WithTimeout(context.Background(), plazoLecturaBase)
	defer cancel()
	lista, err := r.base.OfertasProcesadas(ctx)
	if err != nil {
		return nil, fmt.Errorf("error leyendo las vacantes de la base de datos: %w", err)
	}
	_, errRanking := os.Stat(filepath.Join(r.descargas, carpetaAptos, archivoRanking))

	salida := make([]ofertaLeida, 0, len(lista))
	for _, o := range lista {
		id := agentes.LimpiarNombre(o.Titulo)
		salida = append(salida, ofertaLeida{
			clave: o.IDComputrabajo,
			ResumenOferta: ResumenOferta{
				ID:              id,
				Slug:            o.Slug,
				Titulo:          o.Titulo,
				Carpeta:         filepath.Join(r.descargas, id),
				TotalCandidatos: o.Total,
				TotalAptos:      o.Aptos,
				TotalNoAptos:    o.Total - o.Aptos,
				TieneRankingIA:  errRanking == nil,
				ActualizadoEn:   o.ActualizadoEn,
			},
		})
	}
	sort.Slice(salida, func(i, j int) bool { return salida[i].Titulo < salida[j].Titulo })
	return salida, nil
}

// ListarOfertas lista las vacantes con candidatos vigentes en la base.
func (r *RepositorioMySQL) ListarOfertas() ([]ResumenOferta, error) {
	ofertas, err := r.ofertas()
	if err != nil {
		return nil, err
	}
	salida := make([]ResumenOferta, len(ofertas))
	for i, o := range ofertas {
		salida[i] = o.ResumenOferta
	}
	return salida, nil
}

// ResolverOferta localiza una vacante por slug o por id. Si varias comparten slug gana la de datos
// más recientes.
func (r *RepositorioMySQL) ResolverOferta(slugOID string) (*ResumenOferta, error) {
	ofertas, err := r.ListarOfertas()
	if err != nil {
		return nil, err
	}
	var elegida *ResumenOferta
	for i := range ofertas {
		o := &ofertas[i]
		if o.ID == slugOID {
			return o, nil
		}
		if o.Slug == slugOID && (elegida == nil || o.ActualizadoEn.After(elegida.ActualizadoEn)) {
			elegida = o
		}
	}
	if elegida == nil {
		return nil, fmt.Errorf("%w: vacante %q", ErrNoEncontrado, slugOID)
	}
	return elegida, nil
}

// ObtenerCandidatosOferta devuelve los candidatos de una vacante con filtro opcional (todos, aptos, no_aptos).
func (r *RepositorioMySQL) ObtenerCandidatosOferta(idOferta string, filtro string) ([]modelos.Candidato, error) {
	var soloAptos, soloNoAptos bool
	switch filtro {
	case "", "todos":
	case "aptos":
		soloAptos = true
	case "no_aptos":
		soloNoAptos = true
	default:
		return nil, ErrFiltroInvalid
	}

	ofertas, err := r.ofertas()
	if err != nil {
		return nil, err
	}
	var elegida *ofertaLeida
	for i := range ofertas {
		o := &ofertas[i]
		if o.ID == idOferta && (elegida == nil || o.ActualizadoEn.After(elegida.ActualizadoEn)) {
			elegida = o
		}
	}
	if elegida == nil {
		return nil, fmt.Errorf("%w: no hay candidatos guardados para la oferta %q", ErrNoEncontrado, idOferta)
	}

	ctx, cancel := context.WithTimeout(context.Background(), plazoLecturaBase)
	defer cancel()
	todos, err := r.base.CandidatosDeOferta(ctx, elegida.clave)
	if err != nil {
		return nil, fmt.Errorf("error leyendo los candidatos de %q de la base de datos: %w", idOferta, err)
	}
	if todos == nil {
		todos = []modelos.Candidato{}
	}
	if !soloAptos && !soloNoAptos {
		return todos, nil
	}
	filtrados := make([]modelos.Candidato, 0, len(todos))
	for _, c := range todos {
		if (soloAptos && c.EsApto) || (soloNoAptos && !c.EsApto) {
			filtrados = append(filtrados, c)
		}
	}
	return filtrados, nil
}

// ObtenerRankingOferta devuelve el ranking restringido a las personas aptas de la vacante,
// reordenado por puntaje con puestos 1..n dentro de ella.
func (r *RepositorioMySQL) ObtenerRankingOferta(idOferta string) ([]modelos.CandidatoCalificadoIA, error) {
	candidatos, err := r.ObtenerCandidatosOferta(idOferta, "aptos")
	if err != nil {
		return nil, err
	}
	enOferta := make(map[string]struct{}, len(candidatos))
	for _, c := range candidatos {
		enOferta[soloDigitos(c.Cedula)] = struct{}{}
	}

	global, err := r.RepositorioDisco.ObtenerRankingIA()
	if err != nil {
		return nil, err
	}
	filtrado := make([]modelos.CandidatoCalificadoIA, 0, len(global))
	for _, item := range global {
		if _, ok := enOferta[soloDigitos(item.Candidato.Cedula)]; ok {
			filtrado = append(filtrado, item)
		}
	}
	sort.SliceStable(filtrado, func(i, j int) bool {
		return filtrado[i].EvaluacionIA.ScoreTotal > filtrado[j].EvaluacionIA.ScoreTotal
	})
	for i := range filtrado {
		filtrado[i].Puesto = i + 1
	}
	return filtrado, nil
}
