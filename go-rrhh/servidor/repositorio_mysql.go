package servidor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/almacen"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
)

const (
	plazoLecturaBase = 30 * time.Second
	// vigenciaOfertasBase es cuánto se reutiliza la lista de vacantes leída de la base: cada petición de
	// candidatos la necesita y es un agregado sobre todas las postulaciones.
	vigenciaOfertasBase = 3 * time.Second
)

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

	muOfertas sync.Mutex
	ofertasEn time.Time
	cacheOf   []ofertaLeida
}

// lectorSinTexto lo implementa almacen.Almacen: lee a los candidatos sin el texto de cada perfil.
type lectorSinTexto interface {
	CandidatosDeOfertaSinTexto(ctx context.Context, idComputrabajo string) ([]modelos.Candidato, error)
}

// NuevoRepositorioMySQL crea el repositorio; disco aporta lo que aún vive en archivos.
func NuevoRepositorioMySQL(disco *RepositorioDisco, base LectorBase, descargas string) *RepositorioMySQL {
	return &RepositorioMySQL{RepositorioDisco: disco, base: base, descargas: descargas}
}

// ofertaLeida es una vacante con la clave con la que se buscan sus candidatos en la base. slugBase es
// el slug sin distinguir, que guardan los enlaces de antes.
type ofertaLeida struct {
	ResumenOferta
	clave, slugBase string
}

// distinguirRepetidas da slug e id propios a las vacantes que los comparten: Computrabajo repite el título
// al publicar otra vez una oferta (o solo cambia mayúsculas), y antes la API mostraba una sola, la de
// cambios más recientes, así que la pantalla saltaba de una a otra con otros totales. El sufijo sale de
// la clave de la vacante, así que no cambia entre lecturas.
func distinguirRepetidas(ofertas []ofertaLeida) {
	slugs, ids := map[string]int{}, map[string]int{}
	for _, o := range ofertas {
		slugs[o.Slug]++
		ids[strings.ToLower(o.ID)]++ // en Windows las carpetas no distinguen mayúsculas
	}
	for i := range ofertas {
		o := &ofertas[i]
		o.slugBase = o.Slug
		sufijo := fmt.Sprintf("%x", sha256.Sum256([]byte(o.clave)))[:6]
		if slugs[o.Slug] > 1 {
			o.Slug += "-" + sufijo
		}
		if ids[strings.ToLower(o.ID)] > 1 {
			o.ID += "_" + sufijo
		}
	}
}

// ofertas devuelve una copia de la lista de vacantes de la base, reutilizada por unos segundos.
func (r *RepositorioMySQL) ofertas() ([]ofertaLeida, error) {
	r.muOfertas.Lock()
	defer r.muOfertas.Unlock()
	if r.cacheOf == nil || time.Since(r.ofertasEn) >= vigenciaOfertasBase {
		lista, err := r.leerOfertas()
		if err != nil {
			return nil, err
		}
		r.cacheOf, r.ofertasEn = lista, time.Now()
	}
	return append([]ofertaLeida(nil), r.cacheOf...), nil
}

func (r *RepositorioMySQL) leerOfertas() ([]ofertaLeida, error) {
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
				IDComputrabajo:  o.IDComputrabajo,
				Slug:            o.Slug,
				Titulo:          o.Titulo,
				Carpeta:         filepath.Join(r.descargas, id),
				TotalCandidatos: o.Total,
				TotalAptos:      o.Aptos,
				TotalNoAptos:    o.Total - o.Aptos,
				TotalPendientes: o.Pendientes,
				TieneRankingIA:  errRanking == nil,
				ActualizadoEn:   o.ActualizadoEn,
			},
		})
	}
	distinguirRepetidas(salida)
	sort.Slice(salida, func(i, j int) bool {
		if salida[i].Titulo != salida[j].Titulo {
			return salida[i].Titulo < salida[j].Titulo
		}
		return salida[i].clave < salida[j].clave
	})
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

// ResolverOferta localiza una vacante por slug, por id o por su id de Computrabajo. Un enlace viejo con
// el slug que comparten varias vacantes lleva a la de datos más recientes.
func (r *RepositorioMySQL) ResolverOferta(slugOID string) (*ResumenOferta, error) {
	ofertas, err := r.ofertas()
	if err != nil {
		return nil, err
	}
	var elegida *ResumenOferta
	for i := range ofertas {
		o := &ofertas[i]
		if o.ID == slugOID || o.Slug == slugOID || o.clave == slugOID {
			return &o.ResumenOferta, nil
		}
		if o.slugBase == slugOID && (elegida == nil || o.ActualizadoEn.After(elegida.ActualizadoEn)) {
			elegida = &o.ResumenOferta
		}
	}
	if elegida == nil {
		return nil, fmt.Errorf("%w: vacante %q", ErrNoEncontrado, slugOID)
	}
	return elegida, nil
}

// ObtenerCandidatosOferta devuelve los candidatos de una vacante con filtro opcional (todos, aptos, no_aptos).
func (r *RepositorioMySQL) ObtenerCandidatosOferta(idOferta string, filtro string) ([]modelos.Candidato, error) {
	return r.candidatosOferta(idOferta, filtro, true)
}

// ObtenerCandidatosOfertaSinTexto es ObtenerCandidatosOferta sin el texto del perfil (para las pantallas).
func (r *RepositorioMySQL) ObtenerCandidatosOfertaSinTexto(idOferta string, filtro string) ([]modelos.Candidato, error) {
	return r.candidatosOferta(idOferta, filtro, false)
}

func (r *RepositorioMySQL) candidatosOferta(idOferta string, filtro string, conTexto bool) ([]modelos.Candidato, error) {
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
		if ofertas[i].ID == idOferta || ofertas[i].clave == idOferta {
			elegida = &ofertas[i]
			break
		}
	}
	if elegida == nil {
		return nil, fmt.Errorf("%w: no hay candidatos guardados para la oferta %q", ErrNoEncontrado, idOferta)
	}

	ctx, cancel := context.WithTimeout(context.Background(), plazoLecturaBase)
	defer cancel()
	leer := r.base.CandidatosDeOferta
	if l, ok := r.base.(lectorSinTexto); ok && !conTexto {
		leer = l.CandidatosDeOfertaSinTexto
	}
	todos, err := leer(ctx, elegida.clave)
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
	candidatos, err := r.candidatosOferta(idOferta, "aptos", false)
	if err != nil {
		return nil, err
	}
	global, err := r.RepositorioDisco.ObtenerRankingIA()
	if err != nil {
		return nil, err
	}
	return rankingDeAptos(candidatos, global), nil
}
