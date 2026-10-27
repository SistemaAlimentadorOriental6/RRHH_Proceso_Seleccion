package servidor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/notificaciones"
	"go-rrhh/orquestador"
	"go-rrhh/utilidades"
)

const (
	cargoPorDefecto     = "Operador(a) de buses alimentadores - Metro de Medellín"
	timeoutEvaluacionIA = 20 * time.Minute
	timeoutPreguntaIA   = 180 * time.Second
	maxCuerpoPregunta   = 64 << 10
	maxLongitudPregunta = 2000
	maxLimitePagina     = 500
)

// EstadoEvaluacion describe la evaluación con IA en curso o la última ejecutada.
type EstadoEvaluacion struct {
	ID      string     `json:"id,omitempty"`
	Estado  string     `json:"estado"` // inactiva | en_curso | completada | error
	Inicio  *time.Time `json:"inicio,omitempty"`
	Fin     *time.Time `json:"fin,omitempty"`
	Total   int        `json:"total"`
	Mensaje string     `json:"mensaje,omitempty"`
}

// ControladorRRHH gestiona los endpoints de la API web.
type ControladorRRHH struct {
	cfg         *configuracion.Configuracion
	repositorio RepositorioCandidatos
	agenteIA    *agentes.AgenteIA
	notificador notificaciones.Notificador // canal de correo; los tests lo reemplazan
	whatsapp    notificaciones.Notificador // canal de WhatsApp; los tests lo reemplazan

	// base es el contexto de vida del servidor: las evaluaciones en segundo plano lo heredan y se
	// cancelan al apagar el servidor (no dependen de la conexión HTTP que las inició).
	base       context.Context
	cancelBase context.CancelFunc

	muEval sync.Mutex
	eval   EstadoEvaluacion

	// Computrabajo en vivo: el agente se crea al primer uso (arranca Chrome) y se serializa con semCT.
	// Hay un solo navegador abierto a la vez: el de la cuenta cuentaCT (ver fuenteComputrabajo).
	fabricaOfertas func(cuenta int) FuenteOfertas
	numCuentas     int
	ofertasCT      FuenteOfertas
	cuentaCT       int
	semCT          chan struct{}
	refrescandoCT  atomic.Bool    // hay un refresco de la lista de vacantes en segundo plano
	refrescos      sync.WaitGroup // refrescos en curso (Detener los espera)

	// Buzón de hojas de vida (se crea al primer uso). Ver correo.go.
	muCorreo sync.Mutex
	correo   *agentes.AgenteCorreo

	// Análisis de vacantes en streaming (SSE): un solo job activo a la vez.
	muJob             sync.Mutex
	jobActual         *JobAnalisis
	jobs              []*JobAnalisis // recientes, el más viejo primero
	fabricaAnalizador FabricaAnalizador

	// Última lista de vacantes con su conteo: se sirve mientras un análisis ocupa el navegador.
	muCache           sync.Mutex
	cacheOfertas      []ofertaConConteo
	cacheOfertasEn    time.Time
	ultimoAvisoConteo time.Time // último aviso de conteo parcial (computrabajo.go)

	// Datos faltantes (requieren base de datos; son nil sin ella). Ver datos_faltantes.go y
	// reanudacion.go.
	solicitudes solicitudesPublicas
	envios      enviosBase
	trabajos    trabajosBase
	historiales historialBase
	limite      *limitadorIP  // límite de peticiones por dirección en las rutas públicas
	despertar   chan struct{} // avisa al trabajador de segundo plano
	cambios     *difusor      // avisa a las pantallas abiertas cuando el flujo avanza (cambios.go)

	// Inicio de sesión (nil sin base de datos: la API queda abierta). Ver autenticacion.go.
	sesiones       sesionesBase
	sesionesVistas sesionesVistas // sesiones ya validadas, por unos segundos (autenticacion.go)
	limiteLogin    *limitadorIP
}

// NuevoControladorRRHH instancia el controlador HTTP con sus dependencias inyectadas.
func NuevoControladorRRHH(cfg *configuracion.Configuracion, repo RepositorioCandidatos, modeloIA string) *ControladorRRHH {
	base, cancel := context.WithCancel(context.Background())
	cuentas := cfg.CuentasComputrabajo()
	c := &ControladorRRHH{
		cfg:         cfg,
		repositorio: repo,
		agenteIA:    agentes.NuevoAgenteIA(cfg, modeloIA), // un solo cliente HTTP reutilizado
		notificador: notificaciones.NuevoSMTP(notificaciones.ConfigSMTP{
			Host: cfg.SMTPHost, Puerto: cfg.SMTPPuerto, Usuario: cfg.SMTPUsuario, Clave: cfg.SMTPClave, Remitente: cfg.SMTPRemitente,
		}),
		whatsapp:    notificaciones.NuevoWhatsApp(notificaciones.ConfigWhatsApp{URL: cfg.WhatsAppURL, Token: cfg.WhatsAppToken}),
		base:        base,
		cancelBase:  cancel,
		eval:        EstadoEvaluacion{Estado: "inactiva"},
		semCT:       make(chan struct{}, 1),
		limite:      nuevoLimitadorIP(60, time.Minute),
		limiteLogin: nuevoLimitadorIP(10, time.Minute),
		despertar:   make(chan struct{}, 1),
		cambios:     nuevoDifusor(),
		numCuentas:  len(cuentas),
		fabricaOfertas: func(cuenta int) FuenteOfertas {
			return agentes.NuevoAgenteComputrabajo(cuentas[cuenta])
		},
		fabricaAnalizador: func(fuente orquestador.FuenteCandidatos, opts orquestador.OpcionesAnalisis, rep orquestador.Reportero) *orquestador.Analizador {
			if cfg.SolicitudSoloPrueba {
				opts.SimularSinCedula = esPrueba
			}
			return orquestador.NuevoAnalizador(cfg, fuente, opts, rep)
		},
	}
	c.cargarOfertasGuardadas()
	return c
}

// ConPersistencia hace que los análisis en vivo también guarden a cada candidato en la base de
// datos. Se llama una vez al arrancar, antes de atender peticiones.
func (c *ControladorRRHH) ConPersistencia(p orquestador.Persistencia) {
	fabrica := c.fabricaAnalizador
	c.fabricaAnalizador = func(fuente orquestador.FuenteCandidatos, opts orquestador.OpcionesAnalisis, rep orquestador.Reportero) *orquestador.Analizador {
		return fabrica(fuente, opts, rep).ConPersistencia(p)
	}
}

// ConBase hace que el servidor guarde lo que analiza en la base de datos Y lea de ella las
// vacantes y sus candidatos (en vez de los JSON). Se llama una vez al arrancar.
func (c *ControladorRRHH) ConBase(b Base) {
	c.ConPersistencia(b)
	c.repositorio = NuevoRepositorioMySQL(NuevoRepositorioDisco(c.cfg.DirectorioDescargas), b, c.cfg.DirectorioDescargas)
	c.solicitudes, c.envios, c.trabajos, c.historiales = b, b, b, b
	if s, ok := b.(sesionesBase); ok {
		c.sesiones = s
	}
	c.iniciarSegundoPlano()
}

// Detener cancela las tareas en segundo plano (llamar al apagar el servidor).
func (c *ControladorRRHH) Detener() {
	c.cancelBase()
	c.refrescos.Wait()
	// Cerrar el Chrome de Computrabajo si se llegó a abrir.
	c.semCT <- struct{}{}
	c.descartarFuenteComputrabajo()
	<-c.semCT
}

// ResponderJSON serializa el objeto antes de escribir: si la serialización falla se responde 500
// limpio en vez de un 200 con el cuerpo cortado. Las cabeceras CORS las pone el middleware.
func ResponderJSON(w http.ResponseWriter, codigo int, datos interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(datos); err != nil {
		log.Printf("[!] Error serializando respuesta JSON: %v", err)
		codigo = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"error":"error interno serializando la respuesta"}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_, _ = w.Write(buf.Bytes())
}

// ResponderJSONConETag responde como ResponderJSON pero con un ETag del contenido: si el cliente
// manda If-None-Match con esa misma versión se responde 304 sin cuerpo (no hay nada que transferir,
// parsear ni comparar en la pantalla).
func ResponderJSONConETag(w http.ResponseWriter, r *http.Request, datos any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(datos); err != nil {
		ResponderJSON(w, http.StatusOK, datos) // misma ruta de error que siempre
		return
	}
	suma := sha256.Sum256(buf.Bytes())
	etag := `"` + hex.EncodeToString(suma[:12]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	for _, v := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "W/")) == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// responderError traduce errores de dominio a códigos HTTP. Los errores internos se registran con
// detalle pero al cliente solo se le da un mensaje genérico (los mensajes del sistema de archivos
// incluyen rutas absolutas del servidor).
func responderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoEncontrado):
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, ErrParametro):
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		log.Printf("[!] Error interno: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno al procesar la solicitud"})
	}
}

func soloMetodo(w http.ResponseWriter, r *http.Request, metodos ...string) bool {
	for _, m := range metodos {
		if r.Method == m {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(metodos, ", "))
	ResponderJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
	return false
}

// paginar recorta la lista según ?limite=&offset= y devuelve el total sin paginar.
// Sin parámetros devuelve la lista completa.
func paginar[T any](lista []T, q url.Values) ([]T, int, error) {
	total := len(lista)
	leer := func(nombre string) (int, error) {
		v := q.Get(nombre)
		if v == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%w: %s debe ser un entero >= 0", ErrParametro, nombre)
		}
		return n, nil
	}

	limite, err := leer("limite")
	if err != nil {
		return nil, 0, err
	}
	offset, err := leer("offset")
	if err != nil {
		return nil, 0, err
	}
	if limite > maxLimitePagina {
		limite = maxLimitePagina
	}
	if offset > total {
		offset = total
	}
	fin := total
	if limite > 0 && offset+limite < total {
		fin = offset + limite
	}
	return lista[offset:fin], total, nil
}

// ManejarIndice lista los endpoints disponibles (documentación mínima en la raíz).
func (c *ControladorRRHH) ManejarIndice(w http.ResponseWriter, r *http.Request) {
	ResponderJSON(w, http.StatusOK, map[string]any{
		"servicio": "go-rrhh API",
		"endpoints": []string{
			"GET  /api/salud",
			"GET  /api/computrabajo/vacantes",
			"GET  /api/computrabajo/vacantes/{id}/analizar?completo=0&forzar=0&limite=0&concurrencia=1&antecedentes=1&sarlaft=1&ofac=1  (SSE)",
			"GET  /api/computrabajo/vacantes/{id}/candidatos  (analizados y guardados)",
			"GET  /api/computrabajo/vacantes/{id}/candidatos/{cedula|slug-persona}/contacto  (correo y celular de la hoja de vida)",
			"POST /api/computrabajo/vacantes/{id}/candidatos/{cedula|slug-persona}/solicitar-datos  (correo SMTP con el enlace a la plataforma)",
			"GET  /api/jobs",
			"GET  /api/jobs/{id}",
			"GET  /api/jobs/{id}/eventos  (SSE, admite Last-Event-ID)",
			"DELETE /api/jobs/{id}",
			"GET  /api/vacantes",
			"GET  /api/vacantes/{slug}",
			"GET  /api/vacantes/{slug}/candidatos?filtro=aptos|no_aptos&limite=&offset=&incluir_cv=1",
			"GET  /api/vacantes/{slug}/candidatos/{cedula|slug-persona}",
			"GET  /api/vacantes/{slug}/ranking",
			"GET  /api/ofertas",
			"GET  /api/candidatos?oferta=<id>&filtro=",
			"GET  /api/ranking?oferta=<slug|id>",
			"POST /api/evaluar-ia?async=1",
			"GET  /api/evaluar-ia/estado",
			"GET  /api/pdf?ruta=",
			"POST /api/candidato/preguntar",
		},
	})
}

// ManejarSalud responde si el servicio está vivo (útil para el frontend y monitoreo).
func (c *ControladorRRHH) ManejarSalud(w http.ResponseWriter, r *http.Request) {
	ResponderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
}

// ManejarOfertas devuelve el listado de ofertas de empleo detectadas.
func (c *ControladorRRHH) ManejarOfertas(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodGet) {
		return
	}

	ofertas, err := c.repositorio.ListarOfertas()
	if err != nil {
		responderError(w, err)
		return
	}
	ResponderJSON(w, http.StatusOK, ofertas)
}

// ManejarCandidatos lista los candidatos de una oferta seleccionada, con opción de filtrado.
func (c *ControladorRRHH) ManejarCandidatos(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodGet) {
		return
	}

	q := r.URL.Query()
	idOferta := q.Get("oferta")
	if idOferta == "" {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "El parámetro 'oferta' es obligatorio"})
		return
	}

	candidatos, err := c.candidatosOferta(idOferta, q.Get("filtro"), quiereCV(q))
	if err != nil {
		responderError(w, err)
		return
	}
	pagina, total, err := paginar(candidatos, q)
	if err != nil {
		responderError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	ResponderJSON(w, http.StatusOK, pagina)
}

// sinTextoCV devuelve una copia del ranking sin los textos largos de la hoja de vida (texto_cv y
// texto_perfil), que pesan decenas de KB por persona y los listados no los necesitan.
func sinTextoCV(ranking []modelos.CandidatoCalificadoIA) []modelos.CandidatoCalificadoIA {
	copia := make([]modelos.CandidatoCalificadoIA, len(ranking))
	copy(copia, ranking)
	for i := range copia {
		copia[i].TextoCV = ""
		copia[i].Candidato.TextoPerfil = ""
	}
	return copia
}

// candidatosOferta lee los candidatos de una vacante; sin conTexto no trae el texto del perfil, y la
// base ni siquiera lo lee del disco (lo que piden las pantallas en cada carga).
func (c *ControladorRRHH) candidatosOferta(idOferta, filtro string, conTexto bool) ([]modelos.Candidato, error) {
	if conTexto {
		return c.repositorio.ObtenerCandidatosOferta(idOferta, filtro)
	}
	if r, ok := c.repositorio.(interface {
		ObtenerCandidatosOfertaSinTexto(idOferta, filtro string) ([]modelos.Candidato, error)
	}); ok {
		return r.ObtenerCandidatosOfertaSinTexto(idOferta, filtro)
	}
	lista, err := c.repositorio.ObtenerCandidatosOferta(idOferta, filtro)
	return sinTextoPerfil(lista), err
}

// sinTextoPerfil devuelve una copia de los candidatos sin el texto del perfil (hasta 5 KB c/u).
func sinTextoPerfil(candidatos []modelos.Candidato) []modelos.Candidato {
	copia := make([]modelos.Candidato, len(candidatos))
	copy(copia, candidatos)
	for i := range copia {
		copia[i].TextoPerfil = ""
	}
	return copia
}

func quiereCV(q url.Values) bool {
	v := strings.ToLower(q.Get("incluir_cv"))
	return v == "1" || v == "true" || v == "si"
}

// ManejarRanking devuelve el ranking de candidatos evaluados con IA. Con ?oferta=<slug|id> se
// restringe a una vacante; con ?incluir_cv=1 se agrega el texto de la hoja de vida.
func (c *ControladorRRHH) ManejarRanking(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodGet) {
		return
	}

	q := r.URL.Query()
	var (
		ranking []modelos.CandidatoCalificadoIA
		err     error
	)
	if oferta := q.Get("oferta"); oferta != "" {
		var res *ResumenOferta
		if res, err = c.repositorio.ResolverOferta(oferta); err == nil {
			ranking, err = c.repositorio.ObtenerRankingOferta(res.ID)
		}
	} else {
		ranking, err = c.repositorio.ObtenerRankingIA()
	}
	if err != nil {
		responderError(w, err)
		return
	}

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

// --- Evaluación con IA ---

// iniciarEvaluacion reserva el único cupo de evaluación. Dos evaluaciones simultáneas se pisarían
// el archivo del ranking y saturarían Ollama, así que la segunda se rechaza con 409.
func (c *ControladorRRHH) iniciarEvaluacion() (string, bool) {
	c.muEval.Lock()
	defer c.muEval.Unlock()
	if c.eval.Estado == "en_curso" {
		return "", false
	}
	ahora := time.Now()
	c.eval = EstadoEvaluacion{
		ID:     strconv.FormatInt(ahora.UnixNano(), 36),
		Estado: "en_curso",
		Inicio: &ahora,
	}
	return c.eval.ID, true
}

func (c *ControladorRRHH) finalizarEvaluacion(total int, err error) {
	c.muEval.Lock()
	defer c.muEval.Unlock()
	fin := time.Now()
	c.eval.Fin = &fin
	c.eval.Total = total
	if err != nil {
		c.eval.Estado = "error"
		c.eval.Mensaje = err.Error()
		return
	}
	c.eval.Estado = "completada"
	c.eval.Mensaje = "Evaluación de candidatos con IA completada exitosamente"
}

// ejecutarEvaluacion califica a los aptos y persiste el ranking.
func (c *ControladorRRHH) ejecutarEvaluacion(ctx context.Context, cargo string) ([]modelos.CandidatoCalificadoIA, error) {
	rutaAptos := c.repositorio.ObtenerRutaAptos()
	if _, err := os.Stat(rutaAptos); err != nil {
		return nil, fmt.Errorf("%w: archivo de candidatos aptos para evaluar", ErrNoEncontrado)
	}

	resultados, err := c.agenteIA.EvaluarCandidatosAptos(ctx, rutaAptos, cargo)
	if err != nil {
		return nil, fmt.Errorf("error evaluando candidatos con IA: %w", err)
	}
	if err := c.repositorio.GuardarRankingIA(resultados); err != nil {
		return nil, fmt.Errorf("evaluación completada pero no se pudo guardar el ranking: %w", err)
	}
	return resultados, nil
}

// ManejarEvaluarIA lanza la evaluación con IA sobre los candidatos aptos.
// Por defecto responde cuando termina (compatibilidad); con ?async=1 responde 202 de inmediato y
// el avance se consulta en GET /api/evaluar-ia/estado.
func (c *ControladorRRHH) ManejarEvaluarIA(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodPost) {
		return
	}

	cargo := r.URL.Query().Get("cargo")
	if cargo == "" {
		cargo = cargoPorDefecto
	}
	if _, err := os.Stat(c.repositorio.ObtenerRutaAptos()); err != nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{
			"error": "No se encontró el archivo de candidatos aptos para evaluar",
		})
		return
	}

	id, ok := c.iniciarEvaluacion()
	if !ok {
		ResponderJSON(w, http.StatusConflict, map[string]string{
			"error": "Ya hay una evaluación con IA en curso; consulte /api/evaluar-ia/estado",
		})
		return
	}

	if r.URL.Query().Get("async") == "1" {
		go func() {
			ctx, cancel := context.WithTimeout(c.base, timeoutEvaluacionIA)
			defer cancel()
			resultados, err := c.ejecutarEvaluacion(ctx, cargo)
			c.finalizarEvaluacion(len(resultados), err)
			if err != nil {
				log.Printf("[!] Evaluación IA %s falló: %v", id, err)
			}
		}()
		ResponderJSON(w, http.StatusAccepted, map[string]string{"id": id, "estado": "en_curso"})
		return
	}

	// Modo síncrono: la conexión debe poder esperar toda la evaluación.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeoutEvaluacionIA + time.Minute))
	ctx, cancel := context.WithTimeout(r.Context(), timeoutEvaluacionIA)
	defer cancel()

	resultados, err := c.ejecutarEvaluacion(ctx, cargo)
	c.finalizarEvaluacion(len(resultados), err)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			responderError(w, err)
			return
		}
		log.Printf("[!] Evaluación IA falló: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	ResponderJSON(w, http.StatusOK, map[string]interface{}{
		"mensaje":    "Evaluación de candidatos con IA completada exitosamente",
		"total":      len(resultados),
		"candidatos": sinTextoCV(resultados),
	})
}

// ManejarEstadoEvaluacion informa el estado de la evaluación con IA actual o la última.
func (c *ControladorRRHH) ManejarEstadoEvaluacion(w http.ResponseWriter, r *http.Request) {
	c.muEval.Lock()
	estado := c.eval
	c.muEval.Unlock()
	ResponderJSON(w, http.StatusOK, estado)
}

// --- Archivos ---

// ManejarServirPDF sirve o descarga la hoja de vida (PDF, DOCX o DOC) y las capturas de las validaciones (JPG/PNG). Solo entrega archivos que
// estén dentro del directorio de descargas: antes cualquier ruta del disco terminada en .pdf podía
// leerse con ?ruta=.
func (c *ControladorRRHH) ManejarServirPDF(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodGet, http.MethodHead) {
		return
	}

	ruta := r.URL.Query().Get("ruta")
	if ruta == "" {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "Parámetro 'ruta' requerido"})
		return
	}

	seguro, err := utilidades.RutaDentroDe(c.cfg.DirectorioDescargas, ruta)
	if err != nil {
		ResponderJSON(w, http.StatusForbidden, map[string]string{"error": "ruta no permitida"})
		return
	}

	var tipoContenido, disposicion string
	switch strings.ToLower(filepath.Ext(seguro)) {
	case ".pdf":
		tipoContenido, disposicion = "application/pdf", "inline"
	case ".docx":
		tipoContenido, disposicion = "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "attachment"
	case ".doc":
		tipoContenido, disposicion = "application/msword", "attachment"
	case ".jpg", ".jpeg":
		// Capturas de pantalla de las validaciones (se guardan como JPEG en la carpeta del candidato).
		tipoContenido, disposicion = "image/jpeg", "inline"
	case ".png":
		tipoContenido, disposicion = "image/png", "inline"
	default:
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "Formato no soportado (solo PDF, DOCX, DOC y capturas JPG/PNG)"})
		return
	}

	archivo, err := os.Open(seguro)
	if err != nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Archivo no encontrado"})
		return
	}
	defer archivo.Close()

	info, err := archivo.Stat()
	if err != nil || info.IsDir() {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Archivo no encontrado"})
		return
	}

	// Archivos grandes: ampliar el plazo de escritura de esta petición en lugar del global.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))

	w.Header().Set("Content-Type", tipoContenido)
	// FormatMediaType codifica correctamente nombres con tildes o comillas (filename*=UTF-8'').
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposicion, map[string]string{"filename": filepath.Base(seguro)}))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	// ServeContent resuelve HEAD y Range (visores de PDF que piden por tramos) sin cargar el archivo en memoria.
	http.ServeContent(w, r, filepath.Base(seguro), info.ModTime(), archivo)
}

// --- Pregunta a la IA sobre un candidato ---

// SolicitudPreguntaCandidato estructura la consulta interactiva enviada desde el frontend.
type SolicitudPreguntaCandidato struct {
	Cedula   string `json:"cedula"`
	Pregunta string `json:"pregunta"`
}

// RespuestaPreguntaCandidato estructura la respuesta generada por la IA sobre el candidato.
type RespuestaPreguntaCandidato struct {
	Cedula           string  `json:"cedula"`
	Nombre           string  `json:"nombre"`
	Pregunta         string  `json:"pregunta"`
	Respuesta        string  `json:"respuesta"`
	DuracionSegundos float64 `json:"duracion_segundos"`
}

// ManejarPreguntaCandidato procesa preguntas de los reclutadores sobre un candidato específico usando la IA.
func (c *ControladorRRHH) ManejarPreguntaCandidato(w http.ResponseWriter, r *http.Request) {
	if !soloMetodo(w, r, http.MethodPost) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCuerpoPregunta)
	var sol SolicitudPreguntaCandidato
	if err := json.NewDecoder(r.Body).Decode(&sol); err != nil {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON de solicitud inválido o demasiado grande"})
		return
	}

	sol.Pregunta = strings.TrimSpace(sol.Pregunta)
	if strings.TrimSpace(sol.Cedula) == "" || sol.Pregunta == "" {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "La cédula y la pregunta son obligatorias"})
		return
	}
	if len([]rune(sol.Pregunta)) > maxLongitudPregunta {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("La pregunta no puede superar %d caracteres", maxLongitudPregunta),
		})
		return
	}

	ranking, err := c.repositorio.ObtenerRankingIA()
	if err != nil {
		responderError(w, err)
		return
	}

	buscada := soloDigitos(sol.Cedula)
	var seleccionado *modelos.CandidatoCalificadoIA
	for i := range ranking {
		if soloDigitos(ranking[i].Candidato.Cedula) == buscada {
			seleccionado = &ranking[i]
			break
		}
	}
	if seleccionado == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "Candidato no encontrado en el ranking de idoneidad"})
		return
	}

	registroParaIA := seleccionado.Candidato
	if apto, err := c.repositorio.ObtenerCandidatoAptoPorCedula(sol.Cedula); err == nil && apto != nil {
		registroParaIA = *apto
	}

	textoCV := c.textoCVParaPregunta(seleccionado, registroParaIA)

	inicio := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), timeoutPreguntaIA)
	defer cancel()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(timeoutPreguntaIA + 10*time.Second))

	respuestaIA, err := c.agenteIA.ResponderPreguntaCandidato(ctx, registroParaIA, seleccionado.EvaluacionIA, textoCV, sol.Pregunta)
	if err != nil {
		log.Printf("[!] Error consultando a la IA: %v", err)
		ResponderJSON(w, http.StatusBadGateway, map[string]string{"error": "No fue posible obtener respuesta de la IA (revise APIKEY_IA e IA_MODEL)"})
		return
	}

	ResponderJSON(w, http.StatusOK, RespuestaPreguntaCandidato{
		Cedula:           seleccionado.Candidato.Cedula,
		Nombre:           seleccionado.Candidato.Nombre,
		Pregunta:         sol.Pregunta,
		Respuesta:        respuestaIA,
		DuracionSegundos: time.Since(inicio).Seconds(),
	})
}

// textoCVParaPregunta reutiliza el texto que la IA ya usó al calificar (guardado en el ranking):
// evita lanzar un proceso Python por cada pregunta. Solo extrae del PDF si ese texto no existe.
func (c *ControladorRRHH) textoCVParaPregunta(sel *modelos.CandidatoCalificadoIA, registro modelos.RegistroApto) string {
	if len(sel.TextoCV) >= 50 {
		return sel.TextoCV
	}
	for _, ruta := range []string{registro.RutaCVOriginal, registro.RutaPDF, registro.RutaPerfilPDF} {
		if ruta == "" {
			continue
		}
		seguro, err := utilidades.RutaDentroDe(c.cfg.DirectorioDescargas, ruta)
		if err != nil {
			continue
		}
		if txt, err := utilidades.ExtraerTextoPDF(seguro); err == nil && len(txt) > 20 {
			return txt
		}
	}
	return registro.TextoPerfil
}
