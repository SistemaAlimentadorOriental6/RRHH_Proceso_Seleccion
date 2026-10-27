package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
	"go-rrhh/utilidades"
)

const (
	// timeoutAnalisis acota un análisis completo de una vacante.
	timeoutAnalisis = 12 * time.Hour
	// intervaloPing mantiene viva la conexión SSE (proxies y navegadores cierran las conexiones mudas).
	intervaloPing = 15 * time.Second
	// maxEventosJob protege la memoria: un análisis normal genera unas decenas de eventos por candidato.
	maxEventosJob = 200000
	// jobsRetenidos es cuántos análisis terminados se conservan para volver a ver su historial.
	jobsRetenidos = 5
	// vigenciaListaFresca es cuánto se reutiliza la lista de vacantes al pedirla desde la pantalla.
	vigenciaListaFresca = 2 * time.Minute
	// vigenciaCacheOfertas es cuánto se reutiliza la última lista de vacantes al buscar un id.
	vigenciaCacheOfertas = 10 * time.Minute
	// plazoResolverOferta acota la búsqueda de la vacante (puede abrir Chrome e iniciar sesión).
	plazoResolverOferta = 3 * time.Minute
)

var reIDOferta = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)

// Estados de un job de análisis.
const (
	jobEnCurso    = "en_curso"
	jobCompletado = "completado"
	jobError      = "error"
	jobCancelado  = "cancelado"
)

type eventoSSE struct {
	id    int
	tipo  string
	datos []byte
	// indice y etapa (de los eventos de candidato) permiten compactar la repetición; indice -1 si no aplica.
	indice int
	etapa  string
}

// minimoParaCompactar es desde cuántos eventos pendientes se compacta lo que se envía: al unirse a un
// análisis en marcha (o volver de una pestaña oculta) llegaban miles de eventos, casi todos etapas
// intermedias de personas que ya terminaron. En vivo llegan de a pocos y se envían tal cual.
const minimoParaCompactar = 50

// compactar deja, de una tanda de eventos pendientes, lo necesario para reconstruir el estado: de cada
// persona su descubrimiento, su resultado y, si aún no terminó, la última noticia de cada etapa; del
// resumen, el último. El último evento se conserva siempre (su id es desde donde se reconecta).
func compactar(eventos []eventoSSE) []eventoSSE {
	if len(eventos) < minimoParaCompactar {
		return eventos
	}
	ultimoCompleto := map[int]int{}
	ultimaEtapa := map[string]int{}
	ultimoResumen := -1
	for i, ev := range eventos {
		switch ev.tipo {
		case "candidato.completo":
			ultimoCompleto[ev.indice] = i
		case "candidato.etapa":
			ultimaEtapa[strconv.Itoa(ev.indice)+"|"+ev.etapa] = i
		case "resumen":
			ultimoResumen = i
		}
	}
	salida := make([]eventoSSE, 0, len(eventos)/4)
	for i, ev := range eventos {
		conservar := true
		switch ev.tipo {
		case "resumen":
			conservar = i == ultimoResumen
		case "candidato.etapa":
			fin, termino := ultimoCompleto[ev.indice]
			conservar = (!termino || fin < i) && ultimaEtapa[strconv.Itoa(ev.indice)+"|"+ev.etapa] == i
		}
		if conservar || i == len(eventos)-1 {
			salida = append(salida, ev)
		}
	}
	return salida
}

// JobAnalisis es el análisis de una vacante. Vive aparte de la conexión HTTP: si el cliente se cae
// el análisis sigue, y al reconectar (Last-Event-ID) recibe lo que se perdió.
type JobAnalisis struct {
	ID       string
	OfertaID string
	Titulo   string
	Inicio   time.Time
	cancel   context.CancelFunc

	mu       sync.Mutex
	eventos  []eventoSSE
	estado   string
	fin      time.Time
	resumen  *orquestador.ResumenAnalisis
	oyentes  map[chan struct{}]struct{}
	truncado bool
}

func nuevoJobAnalisis(ofertaID, titulo string) *JobAnalisis {
	return &JobAnalisis{
		ID:       strconv.FormatInt(time.Now().UnixNano(), 36),
		OfertaID: ofertaID,
		Titulo:   titulo,
		Inicio:   time.Now(),
		estado:   jobEnCurso,
		oyentes:  make(map[chan struct{}]struct{}),
	}
}

// Emitir implementa orquestador.Reportero. Serializa en el momento de la llamada.
func (j *JobAnalisis) Emitir(tipo string, datos any) {
	cuerpo, err := json.Marshal(datos)
	if err != nil {
		log.Printf("[!] No se pudo serializar el evento %s: %v", tipo, err)
		return
	}
	ev := eventoSSE{tipo: tipo, datos: cuerpo, indice: -1}
	if strings.HasPrefix(tipo, "candidato.") {
		var clave struct {
			Indice *int   `json:"indice"`
			Etapa  string `json:"etapa"`
		}
		if json.Unmarshal(cuerpo, &clave) == nil && clave.Indice != nil {
			ev.indice, ev.etapa = *clave.Indice, clave.Etapa
		}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.eventos) >= maxEventosJob {
		j.truncado = true
		return
	}
	ev.id = len(j.eventos) + 1
	j.eventos = append(j.eventos, ev)
	j.avisarLocked()
}

func (j *JobAnalisis) avisarLocked() {
	for ch := range j.oyentes {
		select {
		case ch <- struct{}{}:
		default: // ya tiene un aviso pendiente
		}
	}
}

// terminar deja el job en su estado final y despierta a los oyentes.
func (j *JobAnalisis) terminar(estado string, resumen *orquestador.ResumenAnalisis) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.estado = estado
	j.fin = time.Now()
	j.resumen = resumen
	j.avisarLocked()
}

func (j *JobAnalisis) enCurso() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.estado == jobEnCurso
}

// desde devuelve los eventos con id mayor que ultimo y si el job ya terminó (con todo emitido).
func (j *JobAnalisis) desde(ultimo int) ([]eventoSSE, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if ultimo < 0 {
		ultimo = 0
	}
	var pendientes []eventoSSE
	if ultimo < len(j.eventos) {
		pendientes = compactar(j.eventos[ultimo:])
		if len(pendientes) == len(j.eventos)-ultimo {
			pendientes = append([]eventoSSE(nil), pendientes...) // copia: el job sigue agregando
		}
	}
	return pendientes, j.estado != jobEnCurso
}

func (j *JobAnalisis) suscribir() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	j.mu.Lock()
	j.oyentes[ch] = struct{}{}
	j.mu.Unlock()
	return ch, func() {
		j.mu.Lock()
		delete(j.oyentes, ch)
		j.mu.Unlock()
	}
}

// EstadoJob es la vista JSON de un job (sin los eventos).
type EstadoJob struct {
	ID        string                       `json:"id"`
	OfertaID  string                       `json:"oferta_id"`
	Titulo    string                       `json:"titulo"`
	Estado    string                       `json:"estado"`
	Inicio    time.Time                    `json:"inicio"`
	Fin       *time.Time                   `json:"fin,omitempty"`
	Eventos   int                          `json:"eventos"`
	Resumen   *orquestador.ResumenAnalisis `json:"resumen,omitempty"`
	Truncado  bool                         `json:"eventos_truncados,omitempty"`
	URLStream string                       `json:"url_eventos"`
}

func (j *JobAnalisis) estadoJob() EstadoJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	e := EstadoJob{
		ID: j.ID, OfertaID: j.OfertaID, Titulo: j.Titulo, Estado: j.estado, Inicio: j.Inicio,
		Eventos: len(j.eventos), Resumen: j.resumen, Truncado: j.truncado,
		URLStream: "/api/jobs/" + j.ID + "/eventos",
	}
	if !j.fin.IsZero() {
		fin := j.fin
		e.Fin = &fin
	}
	return e
}

// FabricaAnalizador construye el analizador de un job (las pruebas inyectan uno con consultas falsas).
type FabricaAnalizador func(fuente orquestador.FuenteCandidatos, opts orquestador.OpcionesAnalisis, rep orquestador.Reportero) *orquestador.Analizador

// --- Estado del controlador asociado a los jobs ---

// jobEnCursoActual devuelve el job activo o nil. Solo hay uno a la vez: hay un único Chrome con la
// sesión de Computrabajo y la ADRES atiende una consulta por vez.
func (c *ControladorRRHH) jobEnCursoActual() *JobAnalisis {
	c.muJob.Lock()
	defer c.muJob.Unlock()
	if c.jobActual != nil && c.jobActual.enCurso() {
		return c.jobActual
	}
	return nil
}

func (c *ControladorRRHH) buscarJob(id string) *JobAnalisis {
	c.muJob.Lock()
	defer c.muJob.Unlock()
	for _, j := range c.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// guardarOfertasEnCache recuerda la última lista de vacantes leída con su conteo de inscritos.
// También la deja en disco (sobrevive a un reinicio) y avisa a las pantallas para que la vuelvan a pedir.
func (c *ControladorRRHH) guardarOfertasEnCache(ofertas []ofertaConConteo) {
	c.muCache.Lock()
	c.cacheOfertas = ofertas
	c.cacheOfertasEn = time.Now()
	guardada := listaGuardada{En: c.cacheOfertasEn}
	c.muCache.Unlock()
	for _, o := range ofertas {
		guardada.Ofertas = append(guardada.Ofertas, ofertaGuardada{Oferta: o.OfertaLaboral, Cuenta: o.Cuenta, Total: o.Total, Error: o.ErrorMsg})
	}

	if c.cfg.DirectorioDescargas != "" {
		if err := utilidades.EscribirJSONAtomico(filepath.Join(c.cfg.DirectorioDescargas, archivoListaVacantes), guardada, false); err != nil {
			log.Printf("[!] No se pudo guardar la lista de vacantes en disco: %v", err)
		}
	}
	c.cambios.emitir("vacantes", map[string]any{})
}

// archivoListaVacantes guarda la última lista de vacantes para servirla al instante tras un reinicio.
const archivoListaVacantes = "vacantes_computrabajo.json"

type listaGuardada struct {
	En      time.Time        `json:"en"`
	Ofertas []ofertaGuardada `json:"ofertas"`
}

// ofertaGuardada lleva la cuenta aparte: OfertaLaboral no la serializa y sin ella el análisis abriría
// la sesión de otra cuenta.
type ofertaGuardada struct {
	Oferta modelos.OfertaLaboral `json:"oferta"`
	Cuenta int                   `json:"cuenta"`
	Total  *int                  `json:"total"`
	Error  string                `json:"error,omitempty"`
}

// cargarOfertasGuardadas recupera la lista que quedó en disco. Queda con su fecha real: es vieja, así
// que la primera petición la sirve y pide una nueva en segundo plano.
func (c *ControladorRRHH) cargarOfertasGuardadas() {
	if c.cfg.DirectorioDescargas == "" {
		return
	}
	var guardada listaGuardada
	if err := utilidades.LeerJSON(filepath.Join(c.cfg.DirectorioDescargas, archivoListaVacantes), &guardada); err != nil || len(guardada.Ofertas) == 0 {
		return
	}
	lista := make([]ofertaConConteo, 0, len(guardada.Ofertas))
	for _, g := range guardada.Ofertas {
		g.Oferta.Cuenta = g.Cuenta
		lista = append(lista, ofertaConConteo{OfertaLaboral: g.Oferta, Total: g.Total, ErrorMsg: g.Error})
	}
	c.muCache.Lock()
	c.cacheOfertas, c.cacheOfertasEn = lista, guardada.En
	c.muCache.Unlock()
}

func (c *ControladorRRHH) ofertasEnCache() ([]ofertaConConteo, time.Time) {
	c.muCache.Lock()
	defer c.muCache.Unlock()
	return c.cacheOfertas, c.cacheOfertasEn
}

// traducirErrorComputrabajo convierte un error de lectura en código HTTP y mensaje para el cliente.
// codigo 0 significa que el cliente cerró la conexión y no hay nada que responder.
func traducirErrorComputrabajo(err error) (int, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return 0, ""
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "Computrabajo tardó demasiado en responder"
	case errors.Is(err, agentes.ErrSesionExpirada):
		return http.StatusBadGateway, "No fue posible iniciar sesión en Computrabajo; revise usuario y contraseña"
	default:
		return http.StatusBadGateway, "No fue posible leer las vacantes de Computrabajo"
	}
}

// resolverOferta busca la vacante activa por su id (parámetro oi). Usa la lista reciente si existe;
// si no, la lee en vivo.
func (c *ControladorRRHH) resolverOferta(ctx context.Context, id string) (*modelos.OfertaLaboral, error) {
	buscar := func(lista []ofertaConConteo) *modelos.OfertaLaboral {
		for _, o := range lista {
			if strings.EqualFold(o.ID, id) {
				oferta := o.OfertaLaboral
				return &oferta
			}
		}
		return nil
	}

	if lista, en := c.ofertasEnCache(); len(lista) > 0 && time.Since(en) < vigenciaCacheOfertas {
		if o := buscar(lista); o != nil {
			return o, nil
		}
	}
	ctx, cancel := context.WithTimeout(ctx, plazoResolverOferta)
	defer cancel()
	lista, err := c.consultarVacantesComputrabajo(ctx, false)
	if err != nil {
		return nil, err
	}
	return buscar(lista), nil
}

// opcionesAnalisis lee los parámetros de la URL del análisis.
func opcionesAnalisis(q url.Values) (orquestador.OpcionesAnalisis, error) {
	opts := orquestador.OpcionesAnalisis{Concurrencia: 1, Antecedentes: true, Sarlaft: true, Ofac: true}

	booleano := func(clave string, destino *bool) error {
		valor := strings.ToLower(strings.TrimSpace(q.Get(clave)))
		switch valor {
		case "":
		case "1", "true", "si", "sí":
			*destino = true
		case "0", "false", "no":
			*destino = false
		default:
			return fmt.Errorf("%w: %s debe ser 1 o 0", ErrParametro, clave)
		}
		return nil
	}
	for clave, destino := range map[string]*bool{
		"completo": &opts.Completo, "forzar": &opts.Forzar,
		"antecedentes": &opts.Antecedentes, "sarlaft": &opts.Sarlaft, "ofac": &opts.Ofac,
	} {
		if err := booleano(clave, destino); err != nil {
			return opts, err
		}
	}
	if v := q.Get("limite"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return opts, fmt.Errorf("%w: limite debe ser un entero >= 0", ErrParametro)
		}
		opts.Limite = n
	}
	if v := q.Get("concurrencia"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 20 {
			return opts, fmt.Errorf("%w: concurrencia debe estar entre 1 y 20", ErrParametro)
		}
		opts.Concurrencia = n
	}
	return opts, nil
}

// iniciarJobAnalisis registra y lanza el job. Si ya hay uno en curso lo devuelve con creado=false.
func (c *ControladorRRHH) iniciarJobAnalisis(oferta modelos.OfertaLaboral, opts orquestador.OpcionesAnalisis) (*JobAnalisis, bool) {
	c.muJob.Lock()
	defer c.muJob.Unlock()
	if c.jobActual != nil && c.jobActual.enCurso() {
		return c.jobActual, false
	}

	job := nuevoJobAnalisis(oferta.ID, oferta.Titulo)
	ctx, cancel := context.WithTimeout(c.base, timeoutAnalisis)
	job.cancel = cancel
	c.jobActual = job
	c.jobs = append(c.jobs, job)
	if len(c.jobs) > jobsRetenidos {
		c.jobs = c.jobs[len(c.jobs)-jobsRetenidos:]
	}

	go c.ejecutarJobAnalisis(ctx, job, oferta, opts)
	return job, true
}

func (c *ControladorRRHH) ejecutarJobAnalisis(ctx context.Context, job *JobAnalisis, oferta modelos.OfertaLaboral, opts orquestador.OpcionesAnalisis) {
	// Las pantallas se enteran del inicio y del final por /api/cambios, sin sondear /api/jobs. Este
	// defer corre el último, con el estado final ya puesto.
	c.avisarJob(job)
	defer c.avisarJob(job)
	defer job.cancel()
	t0 := time.Now()

	fallar := func(mensaje string, resumen *orquestador.ResumenAnalisis) {
		job.Emitir("job.error", map[string]any{"mensaje": mensaje, "resumen": resumen})
		job.terminar(jobError, resumen)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] análisis %s: %v", job.ID, r)
			c.semCT <- struct{}{} // el defer del cupo del job ya corrió: se retoma para cerrar Chrome
			c.descartarFuenteComputrabajo()
			<-c.semCT
			fallar("error interno durante el análisis", nil)
		}
	}()

	job.Emitir("job.iniciado", map[string]any{
		"job_id":   job.ID,
		"oferta":   map[string]any{"id": oferta.ID, "titulo": oferta.Titulo, "slug": SlugVacante(oferta.Titulo), "url_candidatos": oferta.URLCandidatos},
		"opciones": opts,
	})

	var candidatos orquestador.FuenteCandidatos
	if oferta.ID == agentes.IDOfertaCorreo {
		// Las hojas de vida del correo no usan el navegador de Computrabajo: primero se leen los
		// correos que hayan llegado y luego pasan por el mismo flujo.
		buzon := c.agenteCorreo()
		job.Emitir("correo.leyendo", map[string]string{"buzon": c.cfg.CorreoUsuario})
		revisados, guardadas, err := buzon.ProcesarNuevos(ctx)
		if err != nil && ctx.Err() == nil {
			// Sin red o con el buzón caído se analiza lo que ya estaba guardado.
			log.Printf("[!] Análisis %s: no se pudo leer el buzón: %v", job.ID, err)
			job.Emitir("correo.error", map[string]string{"mensaje": err.Error()})
		} else {
			job.Emitir("correo.leido", map[string]int{"correos_revisados": revisados, "hojas_de_vida_nuevas": guardadas})
		}
		candidatos = buzon
	} else {
		// Un solo navegador con el perfil de Computrabajo: si una consulta de vacantes lo tiene, se espera.
		select {
		case c.semCT <- struct{}{}:
		default:
			job.Emitir("job.en_espera", map[string]string{"motivo": "esperando que termine una consulta de vacantes en Computrabajo"})
			select {
			case c.semCT <- struct{}{}:
			case <-ctx.Done():
				job.Emitir("job.cancelado", map[string]any{"duracion_ms": time.Since(t0).Milliseconds()})
				job.terminar(jobCancelado, nil)
				return
			}
		}
		defer func() { <-c.semCT }()

		fuente := c.fuenteComputrabajo(oferta.Cuenta)
		var ok bool
		candidatos, ok = fuente.(orquestador.FuenteCandidatos)
		if !ok {
			fallar("la fuente de Computrabajo configurada no permite analizar candidatos", nil)
			return
		}
		if err := fuente.AsegurarAutenticacion(ctx); err != nil {
			log.Printf("[!] Análisis %s: autenticación en Computrabajo: %v", job.ID, err)
			c.descartarFuenteComputrabajo()
			if ctx.Err() != nil {
				job.Emitir("job.cancelado", map[string]any{"duracion_ms": time.Since(t0).Milliseconds()})
				job.terminar(jobCancelado, nil)
				return
			}
			fallar("No fue posible iniciar sesión en Computrabajo ("+err.Error()+"); revise usuario y contraseña", nil)
			return
		}
	}

	analizador := c.fabricaAnalizador(candidatos, opts, job)
	defer analizador.Cerrar()
	resumen, err := analizador.Analizar(ctx, oferta)
	duracion := time.Since(t0).Milliseconds()

	switch {
	case ctx.Err() != nil:
		// Chrome puede haber quedado a medias: la próxima petición abre uno limpio.
		if oferta.ID != agentes.IDOfertaCorreo { // el correo no usa ese navegador
			c.descartarFuenteComputrabajo()
		}
		job.Emitir("job.cancelado", map[string]any{"resumen": resumen, "duracion_ms": duracion})
		job.terminar(jobCancelado, &resumen)
	case err != nil:
		log.Printf("[!] Análisis %s falló: %v", job.ID, err)
		if oferta.ID != agentes.IDOfertaCorreo {
			c.descartarFuenteComputrabajo()
		}
		fallar("El análisis terminó con errores: "+err.Error(), &resumen)
	default:
		job.Emitir("job.completado", map[string]any{"resumen": resumen, "duracion_ms": duracion})
		job.terminar(jobCompletado, &resumen)
	}
}

// avisarJob publica el estado de un análisis (en_curso, completado, error, cancelado).
func (c *ControladorRRHH) avisarJob(job *JobAnalisis) {
	e := job.estadoJob()
	c.cambios.emitir("job", map[string]string{"id": e.ID, "oferta_id": e.OfertaID, "titulo": e.Titulo, "estado": e.Estado})
}

// ManejarAnalizarVacante analiza los postulados de una vacante y transmite el avance por SSE.
// Si ya hay un análisis en curso de la misma vacante, se une a él en vez de duplicar el trabajo.
func (c *ControladorRRHH) ManejarAnalizarVacante(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !reIDOferta.MatchString(id) {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "identificador de vacante inválido"})
		return
	}
	if !c.cfg.ValidarCredenciales() {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Credenciales de Computrabajo no configuradas (COMPUTRABAJO_USUARIO y COMPUTRABAJO_CONTRASENA en .env)",
		})
		return
	}
	opts, err := opcionesAnalisis(r.URL.Query())
	if err != nil {
		responderError(w, err)
		return
	}

	if activo := c.jobEnCursoActual(); activo != nil {
		if strings.EqualFold(activo.OfertaID, id) {
			c.transmitirJob(w, r, activo)
			return
		}
		c.responderJobOcupado(w, activo)
		return
	}

	oferta, err := c.resolverOferta(r.Context(), id)
	if err != nil {
		log.Printf("[!] Error buscando la vacante %s en Computrabajo: %v", id, err)
		if codigo, mensaje := traducirErrorComputrabajo(err); codigo != 0 {
			ResponderJSON(w, codigo, map[string]string{"error": mensaje})
		}
		return
	}
	if oferta == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "La vacante no existe entre las ofertas activas de Computrabajo"})
		return
	}
	if c.estaExcluida(oferta.Titulo) {
		ResponderJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "La vacante está excluida por configuración (OfertasExcluidas) y no se analiza"})
		return
	}

	job, creado := c.iniciarJobAnalisis(*oferta, opts)
	if !creado && !strings.EqualFold(job.OfertaID, id) {
		c.responderJobOcupado(w, job)
		return
	}
	c.transmitirJob(w, r, job)
}

func (c *ControladorRRHH) responderJobOcupado(w http.ResponseWriter, activo *JobAnalisis) {
	ResponderJSON(w, http.StatusConflict, map[string]any{
		"error":      "Ya hay un análisis en curso de otra vacante; solo se analiza una a la vez",
		"job_activo": activo.estadoJob(),
	})
}

// ultimoEventoID lee desde dónde continuar: Last-Event-ID (reconexión automática de EventSource)
// o ?desde= (clientes que no son EventSource).
func ultimoEventoID(r *http.Request) int {
	valor := r.Header.Get("Last-Event-ID")
	if valor == "" {
		valor = r.URL.Query().Get("desde")
	}
	n, err := strconv.Atoi(strings.TrimSpace(valor))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// transmitirJob escribe los eventos del job como Server-Sent Events hasta que termina o el cliente se va.
// Cerrar la conexión NO cancela el análisis.
func (c *ControladorRRHH) transmitirJob(w http.ResponseWriter, r *http.Request, job *JobAnalisis) {
	rc := http.NewResponseController(w)
	// Sin plazo de escritura: el servidor limita las respuestas normales a 60 s y cortaría el stream.
	_ = rc.SetWriteDeadline(time.Time{})

	ultimo := ultimoEventoID(r)
	if pendientes, terminado := job.desde(ultimo); terminado && len(pendientes) == 0 {
		// El cliente ya recibió todo (típicamente EventSource reconectando tras el final):
		// 204 le indica que no vuelva a intentarlo.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cab := w.Header()
	cab.Set("Content-Type", "text/event-stream; charset=utf-8")
	cab.Set("Cache-Control", "no-cache, no-transform")
	cab.Set("Connection", "keep-alive")
	cab.Set("X-Accel-Buffering", "no") // nginx: no almacenar la respuesta
	cab.Set("X-Job-ID", job.ID)
	cab.Set("Access-Control-Expose-Headers", "X-Job-ID, X-Total-Count, Content-Disposition")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprintf(w, "retry: 3000\n\n"); err != nil || rc.Flush() != nil {
		return
	}

	avisos, desuscribir := job.suscribir()
	defer desuscribir()
	ping := time.NewTicker(intervaloPing)
	defer ping.Stop()

	marcado := false
	for {
		pendientes, terminado := job.desde(ultimo)
		for _, ev := range pendientes {
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.id, ev.tipo, ev.datos); err != nil {
				return
			}
			ultimo = ev.id
		}
		// Si el job ya terminó todo es repetición y el evento final cierra el stream: el marcador iría
		// después de él (y el cliente ya no lo necesita).
		if !marcado && len(pendientes) > 0 && !terminado {
			// Lo anterior es la repetición de lo ya ocurrido (al recargar o unirse a un análisis en curso);
			// a partir de aquí los eventos son en vivo. La pantalla no anima como "nuevo" lo repetido.
			marcado = true
			if _, err := fmt.Fprint(w, "event: repeticion.fin\ndata: {}\n\n"); err != nil {
				return
			}
			if rc.Flush() != nil {
				return
			}
		} else if len(pendientes) > 0 && rc.Flush() != nil {
			return
		}
		if terminado && len(pendientes) == 0 {
			return
		}
		if len(pendientes) > 0 {
			continue // puede haber más (o el cierre) antes de dormir
		}

		select {
		case <-avisos:
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case <-r.Context().Done():
			return
		case <-c.base.Done():
			return
		}
	}
}

// ManejarListaJobs lista los análisis recientes (el último primero).
func (c *ControladorRRHH) ManejarListaJobs(w http.ResponseWriter, r *http.Request) {
	c.muJob.Lock()
	jobs := append([]*JobAnalisis(nil), c.jobs...)
	c.muJob.Unlock()

	lista := make([]EstadoJob, 0, len(jobs))
	for i := len(jobs) - 1; i >= 0; i-- {
		lista = append(lista, jobs[i].estadoJob())
	}
	ResponderJSON(w, http.StatusOK, lista)
}

// ManejarJob devuelve el estado de un análisis.
func (c *ControladorRRHH) ManejarJob(w http.ResponseWriter, r *http.Request) {
	job := c.buscarJob(r.PathValue("id"))
	if job == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "análisis no encontrado"})
		return
	}
	ResponderJSON(w, http.StatusOK, job.estadoJob())
}

// ManejarEventosJob transmite (o reproduce) los eventos de un análisis ya creado.
func (c *ControladorRRHH) ManejarEventosJob(w http.ResponseWriter, r *http.Request) {
	job := c.buscarJob(r.PathValue("id"))
	if job == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "análisis no encontrado"})
		return
	}
	c.transmitirJob(w, r, job)
}

// ManejarCancelarJob detiene un análisis en curso. Lo ya analizado queda guardado.
func (c *ControladorRRHH) ManejarCancelarJob(w http.ResponseWriter, r *http.Request) {
	job := c.buscarJob(r.PathValue("id"))
	if job == nil {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "análisis no encontrado"})
		return
	}
	if !job.enCurso() {
		ResponderJSON(w, http.StatusConflict, map[string]any{"error": "el análisis ya terminó", "job": job.estadoJob()})
		return
	}
	job.cancel()
	ResponderJSON(w, http.StatusAccepted, map[string]string{"id": job.ID, "estado": "cancelando"})
}

// tituloDeOferta encuentra el título de la vacante sin abrir Chrome si es posible: primero el job
// activo, luego la última lista leída (aunque esté vencida) y, por último, una lectura en vivo.
func (c *ControladorRRHH) tituloDeOferta(ctx context.Context, id string) (string, error) {
	if job := c.jobEnCursoActual(); job != nil && strings.EqualFold(job.OfertaID, id) {
		return job.Titulo, nil
	}
	if lista, _ := c.ofertasEnCache(); len(lista) > 0 {
		for _, o := range lista {
			if strings.EqualFold(o.ID, id) {
				return o.Titulo, nil
			}
		}
	}
	if c.jobEnCursoActual() != nil {
		return "", fmt.Errorf("%w: hay un análisis en curso y la vacante no está en la lista guardada", ErrNoEncontrado)
	}
	oferta, err := c.resolverOferta(ctx, id)
	if err != nil {
		return "", err
	}
	if oferta == nil {
		return "", fmt.Errorf("%w: vacante %q", ErrNoEncontrado, id)
	}
	return oferta.Titulo, nil
}

// ManejarCandidatosGuardadosVacante devuelve los candidatos ya analizados de una vacante, buscada por
// su id de Computrabajo. A diferencia de /api/vacantes/{slug}/candidatos no se confunde cuando dos
// vacantes comparten slug ("Operador(a)…" y "Operador…").
func (c *ControladorRRHH) ManejarCandidatosGuardadosVacante(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !reIDOferta.MatchString(id) {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "identificador de vacante inválido"})
		return
	}
	titulo, err := c.tituloDeOferta(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			responderError(w, err)
			return
		}
		if codigo, mensaje := traducirErrorComputrabajo(err); codigo != 0 {
			ResponderJSON(w, codigo, map[string]string{"error": mensaje})
		}
		return
	}

	candidatos, err := c.candidatosOferta(agentes.LimpiarNombre(titulo), "", false)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			log.Printf("[!] Sin candidatos guardados para la vacante %s (%q): %v", id, titulo, err)
			ResponderJSON(w, http.StatusOK, []CandidatoVacante{}) // vacante sin análisis previo
			return
		}
		responderError(w, err)
		return
	}
	ResponderJSONConETag(w, r, conSlug(candidatos))
}
