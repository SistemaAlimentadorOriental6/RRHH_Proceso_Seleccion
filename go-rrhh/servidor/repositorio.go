package servidor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"go-rrhh/modelos"
	"go-rrhh/utilidades"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	carpetaAptos      = "candidatos_aptos"
	archivoCandidatos = "candidatos.json"
	archivoAptos      = "candidatos_aptos.json"
	archivoRanking    = "ranking_candidatos_ia.json"
)

// Errores de dominio que el controlador traduce a códigos HTTP.
var (
	ErrNoEncontrado  = errors.New("recurso no encontrado")
	ErrParametro     = errors.New("parámetro inválido")
	ErrFiltroInvalid = fmt.Errorf("%w: filtro debe ser todos, aptos o no_aptos", ErrParametro)
)

// ResumenOferta contiene información consolidada de una convocatoria laboral.
type ResumenOferta struct {
	ID              string `json:"id"`
	Slug            string `json:"slug"`
	Titulo          string `json:"titulo"`
	Carpeta         string `json:"carpeta"`
	TotalCandidatos int    `json:"total_candidatos"`
	TotalAptos      int    `json:"total_aptos"`
	TotalNoAptos    int    `json:"total_no_aptos"`
	// TotalPendientes son los no aptos con verificación pendiente (una fuente no respondió, falta un dato).
	TotalPendientes int       `json:"total_pendientes"`
	TieneRankingIA  bool      `json:"tiene_ranking_ia"`
	ActualizadoEn   time.Time `json:"actualizado_en"`
	// Error se informa cuando el candidatos.json de la oferta existe pero está dañado, en lugar
	// de mostrar la oferta como si no tuviera postulados.
	Error string `json:"error,omitempty"`
}

// RepositorioCandidatos define las operaciones para consultar datos de postulaciones en disco.
type RepositorioCandidatos interface {
	ListarOfertas() ([]ResumenOferta, error)
	ResolverOferta(slugOID string) (*ResumenOferta, error)
	ObtenerCandidatosOferta(idOferta string, filtro string) ([]modelos.Candidato, error)
	ObtenerRankingIA() ([]modelos.CandidatoCalificadoIA, error)
	ObtenerRankingOferta(idOferta string) ([]modelos.CandidatoCalificadoIA, error)
	GuardarRankingIA(ranking []modelos.CandidatoCalificadoIA) error
	ObtenerRutaAptos() string
	ObtenerCandidatoAptoPorCedula(cedula string) (*modelos.RegistroApto, error)
}

// RepositorioDisco implementa RepositorioCandidatos leyendo desde el sistema de archivos local.
// Lee siempre del disco (sin caché): las escrituras del motor son atómicas, así que una lectura
// nunca ve un archivo a medias.
type RepositorioDisco struct {
	directorioDescargas string
}

// NuevoRepositorioDisco crea una nueva instancia del repositorio en disco.
func NuevoRepositorioDisco(directorioDescargas string) *RepositorioDisco {
	return &RepositorioDisco{directorioDescargas: directorioDescargas}
}

// quitarAcentos crea la cadena en cada uso: transform.Chain guarda estado y no admite llamadas concurrentes.
func quitarAcentos() transform.Transformer {
	return transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
}

var palabrasVacias = map[string]struct{}{
	"de": {}, "del": {}, "la": {}, "las": {}, "el": {}, "los": {}, "y": {}, "en": {}, "para": {}, "a": {},
}

// tokens normaliza un texto a palabras en minúscula, sin acentos ni signos.
func tokens(texto string) []string {
	sinAcentos, _, _ := transform.String(quitarAcentos(), texto)
	return strings.FieldsFunc(strings.ToLower(sinAcentos), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// SlugVacante genera el slug de una vacante: sin acentos, sin "(a)" ni conectores.
// "Operador(a) de buses alimentadores - Metro de Medellín" -> "operador-buses-alimentadores-metro-medellin".
func SlugVacante(titulo string) string {
	titulo = strings.NewReplacer("(a)", "", "(A)", "").Replace(titulo)
	var salida []string
	for _, t := range tokens(titulo) {
		if _, vacia := palabrasVacias[t]; !vacia {
			salida = append(salida, t)
		}
	}
	return strings.Join(salida, "-")
}

// SlugPersona genera el slug de una persona: "Julián Andrés Castañeda" -> "julian-andres-castaneda".
func SlugPersona(nombre string) string {
	return strings.Join(tokens(nombre), "-")
}

// carpetaOferta valida el identificador y devuelve la ruta absoluta de la carpeta de la oferta.
// Rechaza separadores y ".." para que el parámetro no pueda salir de descargas/.
func (r *RepositorioDisco) carpetaOferta(id string) (string, error) {
	if id == "" || id == carpetaAptos || id == "." || id == ".." ||
		strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", fmt.Errorf("%w: identificador de oferta inválido", ErrParametro)
	}
	ruta := filepath.Join(r.directorioDescargas, id)
	if info, err := os.Stat(ruta); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: oferta %q", ErrNoEncontrado, id)
	}
	return ruta, nil
}

// registroLigero deserializa solo lo necesario para contar: evita materializar el objeto completo
// de cada candidato (RUNT, SIMIT, texto del perfil, etc.) al listar ofertas.
type registroLigero struct {
	EsApto     bool `json:"es_apto"`
	Evaluacion *struct {
		VerificacionPendiente bool `json:"verificacion_pendiente"`
	} `json:"evaluacion_idoneidad"`
}

// ListarOfertas busca todas las carpetas de ofertas laborales en el directorio de descargas.
func (r *RepositorioDisco) ListarOfertas() ([]ResumenOferta, error) {
	entradas, err := os.ReadDir(r.directorioDescargas)
	if err != nil {
		return nil, fmt.Errorf("error leyendo directorio de descargas: %w", err)
	}

	_, errRanking := os.Stat(filepath.Join(r.directorioDescargas, carpetaAptos, archivoRanking))
	tieneRanking := errRanking == nil

	ofertas := make([]ResumenOferta, 0, len(entradas))
	for _, e := range entradas {
		if !e.IsDir() || e.Name() == carpetaAptos {
			continue
		}

		carpeta := filepath.Join(r.directorioDescargas, e.Name())
		rutaCandidatos := filepath.Join(carpeta, archivoCandidatos)
		info, err := os.Stat(rutaCandidatos)
		if err != nil {
			continue // carpeta sin candidatos.json: no es una oferta procesada
		}

		oferta := ResumenOferta{
			ID:             e.Name(),
			Slug:           SlugVacante(strings.ReplaceAll(e.Name(), "_", " ")),
			Titulo:         strings.ReplaceAll(e.Name(), "_", " "),
			Carpeta:        carpeta,
			TieneRankingIA: tieneRanking,
			ActualizadoEn:  info.ModTime(),
		}

		var lista []registroLigero
		if err := utilidades.LeerJSON(rutaCandidatos, &lista); err != nil {
			oferta.Error = err.Error()
		} else {
			for _, c := range lista {
				if c.EsApto {
					oferta.TotalAptos++
				} else if c.Evaluacion != nil && c.Evaluacion.VerificacionPendiente {
					oferta.TotalPendientes++
				}
			}
			oferta.TotalCandidatos = len(lista)
			oferta.TotalNoAptos = len(lista) - oferta.TotalAptos
		}
		ofertas = append(ofertas, oferta)
	}

	sort.Slice(ofertas, func(i, j int) bool { return ofertas[i].Titulo < ofertas[j].Titulo })
	return ofertas, nil
}

// ResolverOferta localiza una oferta por slug o por id de carpeta. Si varias carpetas comparten
// slug (p. ej. "Operador(a)_…" y "Operador_…") gana la de datos más recientes.
func (r *RepositorioDisco) ResolverOferta(slugOID string) (*ResumenOferta, error) {
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

// ObtenerCandidatosOferta carga los candidatos de una oferta con filtro opcional (todos, aptos, no_aptos).
func (r *RepositorioDisco) ObtenerCandidatosOferta(idOferta string, filtro string) ([]modelos.Candidato, error) {
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

	carpeta, err := r.carpetaOferta(idOferta)
	if err != nil {
		return nil, err
	}

	var todos []modelos.Candidato
	if err := utilidades.LeerJSON(filepath.Join(carpeta, archivoCandidatos), &todos); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: no existe candidatos.json para la oferta %q", ErrNoEncontrado, idOferta)
		}
		return nil, fmt.Errorf("error leyendo candidatos de %q: %w", idOferta, err)
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

// ObtenerRankingIA recupera el ranking calificado de candidatos aptos. Si todavía no se ha
// evaluado con IA devuelve una lista vacía: no se inventan puntajes.
func (r *RepositorioDisco) ObtenerRankingIA() ([]modelos.CandidatoCalificadoIA, error) {
	ruta := filepath.Join(r.directorioDescargas, carpetaAptos, archivoRanking)
	ranking := []modelos.CandidatoCalificadoIA{}
	if err := utilidades.LeerJSON(ruta, &ranking); err != nil {
		if os.IsNotExist(err) {
			return []modelos.CandidatoCalificadoIA{}, nil
		}
		return nil, fmt.Errorf("error leyendo el ranking de IA: %w", err)
	}
	return ranking, nil
}

// ObtenerRankingOferta devuelve el ranking restringido a las personas postuladas a una oferta,
// reordenado por puntaje con puestos 1..n dentro de esa vacante.
func (r *RepositorioDisco) ObtenerRankingOferta(idOferta string) ([]modelos.CandidatoCalificadoIA, error) {
	candidatos, err := r.ObtenerCandidatosOferta(idOferta, "aptos")
	if err != nil {
		return nil, err
	}
	global, err := r.ObtenerRankingIA()
	if err != nil {
		return nil, err
	}
	return rankingDeAptos(candidatos, global), nil
}

// rankingDeAptos deja del ranking global solo a los aptos de la vacante (los que lleguen en candidatos
// con EsApto), ordenados por puntaje con puestos 1..n dentro de ella.
func rankingDeAptos(candidatos []modelos.Candidato, global []modelos.CandidatoCalificadoIA) []modelos.CandidatoCalificadoIA {
	enOferta := make(map[string]struct{}, len(candidatos))
	for _, c := range candidatos {
		if c.EsApto {
			enOferta[soloDigitos(c.Cedula)] = struct{}{}
		}
	}
	filtrado := make([]modelos.CandidatoCalificadoIA, 0, len(enOferta))
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
	return filtrado
}

// GuardarRankingIA almacena el ranking evaluado en disco de forma atómica.
func (r *RepositorioDisco) GuardarRankingIA(ranking []modelos.CandidatoCalificadoIA) error {
	ruta := filepath.Join(r.directorioDescargas, carpetaAptos, archivoRanking)
	return utilidades.ConBloqueoArchivo(ruta, func() error {
		return utilidades.EscribirJSONAtomico(ruta, ranking, true)
	})
}

// ObtenerRutaAptos devuelve la ruta del archivo candidatos_aptos.json.
func (r *RepositorioDisco) ObtenerRutaAptos() string {
	return filepath.Join(r.directorioDescargas, carpetaAptos, archivoAptos)
}

// ObtenerCandidatoAptoPorCedula recupera el registro completo y detallado de un candidato apto.
func (r *RepositorioDisco) ObtenerCandidatoAptoPorCedula(cedula string) (*modelos.RegistroApto, error) {
	var aptos []modelos.RegistroApto
	if err := utilidades.LeerJSON(r.ObtenerRutaAptos(), &aptos); err != nil {
		return nil, fmt.Errorf("error leyendo %s: %w", archivoAptos, err)
	}

	buscada := soloDigitos(cedula)
	for i := range aptos {
		if soloDigitos(aptos[i].Cedula) == buscada {
			return &aptos[i], nil
		}
	}
	return nil, fmt.Errorf("%w: candidato con cédula %s", ErrNoEncontrado, cedula)
}

func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
