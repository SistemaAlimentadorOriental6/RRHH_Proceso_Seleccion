package orquestador

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// Reportero recibe el avance del análisis. Emitir se llama desde varias goroutines y debe
// serializar los datos antes de volver (el candidato sigue mutando después de cada llamada).
type Reportero interface {
	Emitir(tipo string, datos any)
}

// FuenteCandidatos es lo que el análisis necesita de Computrabajo.
type FuenteCandidatos interface {
	ObtenerCandidatosStream(ctx context.Context, urlCandidatos string, limite int, alDescubrir func(*modelos.Candidato)) error
	ExtraerDetalleCandidato(ctx context.Context, c *modelos.Candidato, carpetaCandidato string) error
}

// OpcionesAnalisis configura un análisis de una vacante.
type OpcionesAnalisis struct {
	Concurrencia int  `json:"concurrencia"`
	Limite       int  `json:"limite"`   // 0 = todos los postulados
	Completo     bool `json:"completo"` // true: consulta todo, sin descarte temprano
	Forzar       bool `json:"forzar"`   // true: reprocesa aunque el candidato ya esté analizado
	Antecedentes bool `json:"antecedentes"`
	Sarlaft      bool `json:"sarlaft"`
	Ofac         bool `json:"ofac"`

	// SimularSinCedula dice si a esta persona se le trata como si no tuviera cédula aunque la tenga, para
	// que pase por la rama de contacto y el formulario antes de las consultas (solo personas de prueba).
	SimularSinCedula func(nombre, cedula string) bool `json:"-"`
}

// FuncionValidar consulta una fuente externa y deja el resultado en el candidato.
type FuncionValidar func(ctx context.Context, c *modelos.Candidato) error

// Validadores son las consultas externas. Runt, Simit y Adres definen la idoneidad y son obligatorios;
// el resto son informativos (alertas) y pueden ser nil.
type Validadores struct {
	Runt, Simit, Adres          FuncionValidar
	Procuraduria, Sarlaft, Ofac FuncionValidar
	// Reintegro (Administremos) mira si la cédula ya figura como empleado: quien ya fue empleado no puede
	// volver a entrar, así que es una regla que descarta. Corre antes de ADRES; nil si no hay credenciales.
	Reintegro FuncionValidar
	Cerrar    func()
}

// ResumenAnalisis son los totales de un análisis.
type ResumenAnalisis struct {
	Descubiertos       int    `json:"descubiertos"`
	Procesados         int    `json:"procesados"`
	Cache              int    `json:"cache"`
	Aptos              int    `json:"aptos"`
	NoAptos            int    `json:"no_aptos"`
	Errores            int    `json:"errores"`
	Pendientes         int    `json:"pendientes"` // sin dictamen por una consulta fallida; se reintentan al reanalizar
	DescartesTempranos int    `json:"descartes_tempranos"`
	ListadoCompleto    bool   `json:"listado_completo"`
	Carpeta            string `json:"carpeta,omitempty"`
}

// PayloadCandidato es el cuerpo del evento candidato.completo.
type PayloadCandidato struct {
	modelos.Candidato
	Cache bool `json:"cache"`
}

// Analizador procesa los postulados de UNA vacante en streaming: cada candidato se analiza apenas
// aparece en el listado y cada paso se informa al Reportero.
type Analizador struct {
	cfg    *configuracion.Configuracion
	opts   OpcionesAnalisis
	fuente FuenteCandidatos
	clasif *agentes.AgenteClasificador
	val    Validadores
	rep    Reportero

	semPerfil chan struct{}
	semRunt   chan struct{}
	semSimit  chan struct{}
	semAdres  chan struct{}
	semProcu  chan struct{}
	semOfac   chan struct{}

	ia           EvaluadorIA           // opcional: calificación de la hoja de vida
	generosIA    sync.Map              // nombre → género decidido por la IA
	persistencia Persistencia          // opcional: base de datos
	oferta       modelos.OfertaLaboral // vacante que se está analizando

	mu  sync.Mutex
	res ResumenAnalisis
}

// ConPersistencia hace que cada candidato, al terminar, se guarde también en la base de datos.
func (a *Analizador) ConPersistencia(p Persistencia) *Analizador {
	a.persistencia = p
	return a
}

// NuevoAnalizador crea los agentes reales (RUNT, SIMIT, ADRES y los informativos pedidos).
// Cerrar libera sus navegadores; la fuente de Computrabajo no es suya y no se cierra.
func NuevoAnalizador(cfg *configuracion.Configuracion, fuente FuenteCandidatos, opts OpcionesAnalisis, rep Reportero) *Analizador {
	runt := agentes.NuevoAgenteRunt(cfg)
	simit := agentes.NuevoAgenteSimit(cfg)
	adres := agentes.NuevoAgenteAdres(cfg)
	v := Validadores{
		// El RUNT pide el primer apellido y adivinarlo del nombre del perfil falla con nombres de uno o dos
		// palabras o con el orden cambiado (APELLIDO_NO_COINCIDE para siempre). ADRES corre antes y devuelve
		// los apellidos oficiales de esa cédula: Validar los usa primero.
		Runt:  runt.Validar,
		Simit: simit.Validar,
		Adres: adres.Validar,
	}
	cerrar := []func(){simit.Cerrar, adres.Cerrar}
	if reintegro := agentes.NuevoAgenteReintegro(cfg); reintegro.Disponible() {
		v.Reintegro = reintegro.Validar
	}
	if opts.Antecedentes {
		a := agentes.NuevoAgenteProcuraduria(cfg)
		v.Procuraduria = a.Validar
		cerrar = append(cerrar, a.Cerrar)
	}
	if opts.Sarlaft {
		v.Sarlaft = agentes.NuevoAgenteSarlaft(cfg).Validar
	}
	if opts.Ofac {
		a := agentes.NuevoAgenteOfac(cfg)
		v.Ofac = a.Validar
		cerrar = append(cerrar, a.Cerrar)
	}
	agentes.PrecalentarRenderizador() // Chrome de las tarjetas de evidencia (RUNT, ONU)
	v.Cerrar = func() {
		for _, f := range cerrar {
			f()
		}
		agentes.CerrarRenderizador()
	}
	an := NuevoAnalizadorConValidadores(cfg, fuente, opts, rep, v)
	if ia := agentes.NuevoAgenteIA(cfg, ""); ia.Disponible() {
		an.ia = ia
	}
	return an
}

// NuevoAnalizadorConValidadores permite inyectar las consultas (pruebas).
func NuevoAnalizadorConValidadores(cfg *configuracion.Configuracion, fuente FuenteCandidatos, opts OpcionesAnalisis, rep Reportero, v Validadores) *Analizador {
	if opts.Concurrencia <= 0 {
		opts.Concurrencia = 1 // persona por persona
	}
	return &Analizador{
		cfg:       cfg,
		opts:      opts,
		fuente:    fuente,
		clasif:    agentes.NuevoAgenteClasificador(cfg),
		val:       v,
		rep:       rep,
		semPerfil: make(chan struct{}, 3),
		semRunt:   make(chan struct{}, min(opts.Concurrencia, 4)),
		semSimit:  make(chan struct{}, min(opts.Concurrencia, 3)),
		semAdres:  make(chan struct{}, 1), // un navegador visible a la vez (captcha)
		semProcu:  make(chan struct{}, min(opts.Concurrencia, 3)),
		semOfac:   make(chan struct{}, min(opts.Concurrencia, 4)),
	}
}

// Cerrar libera los navegadores de las consultas.
func (a *Analizador) Cerrar() {
	if a.val.Cerrar != nil {
		a.val.Cerrar()
	}
}

// Resumen devuelve los totales al momento de la llamada.
func (a *Analizador) Resumen() ResumenAnalisis {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.res
}

func (a *Analizador) contar(f func(*ResumenAnalisis)) ResumenAnalisis {
	a.mu.Lock()
	defer a.mu.Unlock()
	f(&a.res)
	return a.res
}

// ---- Etapas -------------------------------------------------------------------------------

// Estados de una etapa en el evento candidato.etapa.
const (
	estadoIniciada   = "iniciada"
	estadoOK         = "ok"
	estadoRechazado  = "rechazado" // la regla de negocio no se cumple
	estadoAlerta     = "alerta"    // coincidencia en listas / antecedentes (informativo)
	estadoError      = "error"     // la consulta falló: no hay evidencia
	estadoOmitida    = "omitida"
	estadoReintento  = "reintentando" // la consulta falló y se vuelve a intentar
	detalleNoConsult = modelos.DetalleNoConsultado
)

type etapa struct {
	nombre   string
	sem      chan struct{}
	validar  FuncionValidar
	evaluar  func(c *modelos.Candidato) (estado string, cumple bool, detalle string)
	esNucleo bool
}

type resultadoEtapa struct {
	estado  string
	cumple  bool
	detalle string
}

func (a *Analizador) etapasNucleo() []etapa {
	return []etapa{
		{nombre: "adres", sem: a.semAdres, validar: a.val.Adres, esNucleo: true, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoAdres
			cumple, det := a.clasif.EvaluarEPS(r)
			tecnico := r == nil || strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA"
			if tecnico && r != nil {
				det = fmt.Sprintf("ADRES no verificado (%s): %s", r.Estado, textoOr(r.Detalles, "sin detalle"))
			}
			return clasificar(cumple, tecnico), cumple, det
		}},
		{nombre: "runt", sem: a.semRunt, validar: a.val.Runt, esNucleo: true, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoRunt
			cumple, det := a.clasif.EvaluarRunt(r)
			tecnico := r == nil || strings.HasPrefix(r.Estado, "ERROR") ||
				r.Estado == "NO_CONSULTADO_SIN_CEDULA" || r.Estado == "SIN_APELLIDO" || r.Estado == "APELLIDO_NO_COINCIDE"
			if tecnico && r != nil {
				det = fmt.Sprintf("RUNT no verificado (%s): %s", r.Estado, textoOr(r.Error, "sin detalle"))
			}
			return clasificar(cumple, tecnico), cumple, det
		}},
		{nombre: "simit", sem: a.semSimit, validar: a.val.Simit, esNucleo: true, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoSimit
			cumple, det := a.clasif.EvaluarSimit(r)
			tecnico := r == nil || strings.HasPrefix(r.Estado, "ERROR") || r.Estado == "NO_APLICA" || r.Estado == "SIN_CEDULA"
			if tecnico && r != nil {
				det = fmt.Sprintf("SIMIT no verificado (%s): %s", r.Estado, textoOr(r.Detalles, "sin detalle"))
			}
			return clasificar(cumple, tecnico), cumple, det
		}},
	}
}

// consultarReintegro corre la validación de reintegro (Administremos) antes de ADRES. Quien ya fue (o es)
// empleado no puede volver a entrar: el resultado es "rechazado" y el proceso con esa persona termina. Si
// el servicio falla queda "error" (verificación pendiente, como cualquier consulta fallida). El segundo
// valor es false solo si se canceló el análisis. Sin credenciales de Administremos la etapa no existe.
func (a *Analizador) consultarReintegro(ctx context.Context, c *modelos.Candidato) (resultadoEtapa, bool) {
	if a.val.Reintegro == nil {
		return resultadoEtapa{estado: estadoOK, cumple: true}, true
	}
	return a.correrEtapa(ctx, c, etapa{nombre: "reintegro", validar: a.val.Reintegro, evaluar: evaluarEtapaReintegro, esNucleo: true})
}

func evaluarEtapaReintegro(c *modelos.Candidato) (string, bool, string) {
	r := c.ResultadoReintegro
	switch {
	case r == nil:
		return estadoError, false, "Sin resultado de Administremos"
	case r.Estado == modelos.ReintegroErrorConsulta:
		return estadoError, false, "Reintegro no verificado: " + r.Detalles
	case r.Alerta:
		return estadoRechazado, false, r.Detalles + ": no se admiten reintegros"
	}
	return estadoOK, true, r.Detalles
}

func clasificar(cumple, tecnico bool) string {
	switch {
	case tecnico:
		return estadoError
	case cumple:
		return estadoOK
	default:
		return estadoRechazado
	}
}

// etapasInformativas son las consultas que no cambian la idoneidad pero alertan al reclutador.
func (a *Analizador) etapasInformativas() []etapa {
	var lista []etapa
	if a.val.Procuraduria != nil {
		lista = append(lista, etapa{nombre: "procuraduria", sem: a.semProcu, validar: a.val.Procuraduria, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoProcuraduria
			if r == nil {
				return estadoError, true, "Sin resultado de la Procuraduría"
			}
			if r.Alerta {
				return estadoAlerta, true, textoOr(r.Detalles, r.Estado)
			}
			return estadoOK, true, textoOr(r.Detalles, r.Estado)
		}})
	}
	if a.val.Sarlaft != nil {
		lista = append(lista, etapa{nombre: "sarlaft", validar: a.val.Sarlaft, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoSarlaft
			if r == nil {
				return estadoError, true, "Sin resultado de SARLAFT"
			}
			if r.Alerta {
				return estadoAlerta, true, r.Detalles
			}
			return estadoOK, true, r.Detalles
		}})
	}
	if a.val.Ofac != nil {
		lista = append(lista, etapa{nombre: "ofac", sem: a.semOfac, validar: a.val.Ofac, evaluar: func(c *modelos.Candidato) (string, bool, string) {
			r := c.ResultadoOfac
			if r == nil {
				return estadoError, true, "Sin resultado de OFAC"
			}
			if r.Alerta {
				return estadoAlerta, true, textoOr(r.Detalles, r.Estado)
			}
			return estadoOK, true, textoOr(r.Detalles, r.Estado)
		}})
	}
	return lista
}

func textoOr(texto, alternativa string) string {
	if strings.TrimSpace(texto) != "" {
		return texto
	}
	return alternativa
}

// ---- Análisis de la vacante -----------------------------------------------------------------

// Analizar recorre los postulados de la oferta y analiza a cada uno en cuanto aparece.
// Devuelve los totales aunque haya error o cancelación. Solo debe llamarse una vez por Analizador.
func (a *Analizador) Analizar(ctx context.Context, oferta modelos.OfertaLaboral) (ResumenAnalisis, error) {
	if a.val.Runt == nil || a.val.Simit == nil || a.val.Adres == nil {
		return a.Resumen(), fmt.Errorf("faltan las consultas RUNT, SIMIT y ADRES: son las que definen la idoneidad")
	}
	a.oferta = oferta // se fija antes de lanzar las goroutines que guardan a cada candidato

	carpetaOferta := filepath.Join(a.cfg.DirectorioDescargas, agentes.LimpiarNombre(oferta.Titulo))
	if err := os.MkdirAll(carpetaOferta, 0o755); err != nil {
		return a.Resumen(), fmt.Errorf("no se pudo crear la carpeta de la oferta: %w", err)
	}
	a.contar(func(r *ResumenAnalisis) { r.Carpeta = carpetaOferta })

	rutaResumen := filepath.Join(carpetaOferta, "candidatos.json")
	// Con base de datos, lo ya analizado sale de ahí. Si no se puede leer se detiene el análisis en
	// vez de volver a consultar a todos en silencio (cuesta captchas y tiempo).
	var previos map[string]*modelos.Candidato
	if a.persistencia != nil {
		var err error
		if previos, err = a.persistencia.CandidatosPrevios(ctx, oferta); err != nil {
			return a.Resumen(), fmt.Errorf("no se pudieron leer los candidatos anteriores de la base de datos: %w", err)
		}
	} else {
		previos = cargarPrevios(rutaResumen)
	}
	acum := &acumuladorResultados{ruta: rutaResumen}
	for _, p := range previos {
		acum.lista = append(acum.lista, p)
	}
	defer acum.finalizar()

	vistos := make(map[string]bool)
	cola := make(chan turno, 4096)
	var wg sync.WaitGroup
	for i := 0; i < a.opts.Concurrencia; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range cola {
				if ctx.Err() != nil {
					continue // vaciar la cola sin procesar
				}
				if t.previo != nil {
					a.reutilizar(ctx, t.previo, t.c.Indice, acum)
					continue
				}
				a.procesarSeguro(ctx, t.c, carpetaOferta, acum)
			}
		}()
	}

	errListado := a.fuente.ObtenerCandidatosStream(ctx, oferta.URLCandidatos, a.opts.Limite, func(c *modelos.Candidato) {
		vistos[c.URLDetalle] = true
		total := a.contar(func(r *ResumenAnalisis) { r.Descubiertos++ }).Descubiertos
		// Todos pasan por la misma fila, en orden de aparición: con concurrencia 1 se termina a una
		// persona antes de empezar con la siguiente, también cuando su análisis viene de la caché.
		t := turno{c: c}
		if previo, ok := previos[c.URLDetalle]; ok && a.cacheValido(previo) {
			t.previo = previo
		}
		// "cache" avisa a la pantalla que no es una persona nueva: ya tiene resultado y solo se confirma.
		a.rep.Emitir("candidato.descubierto", map[string]any{"indice": c.Indice, "nombre": c.Nombre, "descubiertos": total, "cache": t.previo != nil})
		select {
		case cola <- t:
		case <-ctx.Done():
		}
	})
	close(cola)
	wg.Wait()

	listadoCompleto := errListado == nil && ctx.Err() == nil
	if listadoCompleto && a.opts.Limite == 0 {
		acum.conservarSoloVistos(vistos)
		if a.persistencia != nil {
			urls := make([]string, 0, len(vistos))
			for u := range vistos {
				urls = append(urls, u)
			}
			ctxBase, cancel := context.WithTimeout(context.Background(), plazoPersistencia)
			if err := a.persistencia.MarcarVigentes(ctxBase, oferta, urls); err != nil {
				fmt.Printf("[!] No se pudo actualizar en la base de datos quiénes siguen postulados: %v\n", err)
			}
			cancel()
		}
	}
	res := a.contar(func(r *ResumenAnalisis) { r.ListadoCompleto = listadoCompleto })
	if listadoCompleto {
		a.rep.Emitir("listado.completo", map[string]any{"total": res.Descubiertos})
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	if errListado != nil {
		return res, fmt.Errorf("error leyendo los postulados de la oferta: %w", errListado)
	}
	return res, nil
}

// turno es un candidato en la fila de análisis; previo != nil indica que se reutiliza su análisis anterior.
type turno struct {
	c      *modelos.Candidato
	previo *modelos.Candidato
}

// reutilizar da por bueno el análisis anterior de un candidato y lo informa como caché.
func (a *Analizador) reutilizar(ctx context.Context, previo *modelos.Candidato, indice int, acum *acumuladorResultados) {
	previo.Indice = indice
	// Un apto guardado antes de la IA (o cuya calificación falló) solo completa ese paso; no se repiten las consultas.
	if a.evaluarConIA(ctx, previo) {
		a.guardarIA(previo)
	}
	acum.agregar(previo)
	a.contar(func(r *ResumenAnalisis) {
		r.Procesados++
		r.Cache++
		if previo.EsApto {
			r.Aptos++
		} else {
			r.NoAptos++
		}
	})
	a.informarCompleto(previo, true)
}

// cargarPrevios lee el candidatos.json de una corrida anterior, indexado por la URL de la ficha.
func cargarPrevios(ruta string) map[string]*modelos.Candidato {
	var lista []*modelos.Candidato
	previos := make(map[string]*modelos.Candidato)
	if err := utilidades.LeerJSON(ruta, &lista); err != nil {
		return previos
	}
	for _, c := range lista {
		if c != nil && c.URLDetalle != "" {
			previos[c.URLDetalle] = c
		}
	}
	return previos
}

// cacheValido decide si un análisis previo se reutiliza: debe estar terminado, sin errores
// de consulta, y con las validaciones que ahora se piden (no un descarte temprano si se pide completo).
func (a *Analizador) cacheValido(c *modelos.Candidato) bool {
	if a.opts.Forzar {
		return false
	}
	// En pausa por un dato que falta: ya está en manos del proceso de solicitud (esperando datos,
	// reanudándose o en revisión). No se vuelve a leer de Computrabajo en cada análisis; con
	// forzar=1 sí se reprocesa desde cero.
	if modelos.EnPausa(c.EstadoProceso) && c.EvaluacionIdoneidad != nil {
		return true
	}
	if c.EvaluacionIdoneidad == nil || c.ErrorProceso != "" {
		return false
	}
	// Sin cédula nunca se pudo consultar nada: vale la pena volver a leer el perfil.
	if len(soloDigitos(c.Cedula)) < 6 {
		return false
	}
	// Un análisis anterior a las capturas de pantalla no tiene evidencia que mostrar: se rehace una vez.
	if len(c.Capturas) == 0 {
		return false
	}
	if a.opts.Completo && c.EvaluacionIdoneidad.DescarteTemprano {
		return false
	}
	return true
}

func (a *Analizador) procesarSeguro(ctx context.Context, c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados) {
	defer func() {
		if r := recover(); r != nil {
			c.ErrorProceso = fmt.Sprintf("error inesperado: %v", r)
			c.EsApto = false
			a.cerrarCandidato(c, carpetaOferta, acum)
		}
	}()
	a.procesar(ctx, c, carpetaOferta, acum)
}

func (a *Analizador) procesar(ctx context.Context, c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados) {
	carpetaCandidato := filepath.Join(carpetaOferta, fmt.Sprintf("%d_%s", c.Indice, agentes.LimpiarNombre(c.Nombre)))
	_ = os.MkdirAll(carpetaCandidato, 0o755)
	c.CarpetaCandidato = carpetaCandidato // aquí dejan sus capturas los agentes

	// 1. Perfil de Computrabajo + hoja de vida original
	a.etapa(c, "perfil", estadoIniciada, "", 0)
	liberar := adquirirCupo(ctx, a.semPerfil)
	if liberar == nil {
		return
	}
	t0 := time.Now()
	err := a.fuente.ExtraerDetalleCandidato(ctx, c, carpetaCandidato)
	liberar()
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		c.ErrorProceso = "no se pudo leer el perfil en Computrabajo"
		a.etapa(c, "perfil", estadoError, err.Error(), time.Since(t0))
		a.dejarPendiente(c, carpetaOferta, acum, nil, []string{"no se pudo leer el perfil en Computrabajo: " + err.Error()})
		return
	}
	cv := "sin hoja de vida original"
	if c.RutaCVOriginal != "" {
		cv = "hoja de vida descargada"
	}
	a.etapa(c, "perfil", estadoOK, fmt.Sprintf("Cédula %s · %s", c.Cedula, cv), time.Since(t0))

	// 2. Género (sale del perfil; no cuesta ninguna consulta externa)
	genero, origenGenero := a.determinarGenero(ctx, c)
	c.Genero = genero
	if genero == "Hombre" {
		a.etapa(c, "genero", estadoOK, genero+origenGenero, 0)
	} else {
		a.etapa(c, "genero", estadoRechazado, fmt.Sprintf("Género (%s%s) no cumple requisito (requerido: Hombre)", genero, origenGenero), 0)
	}

	simulada := genero == "Hombre" && a.opts.SimularSinCedula != nil && a.opts.SimularSinCedula(c.Nombre, c.Cedula)
	sinCedula := len(soloDigitos(c.Cedula)) < 6 || simulada

	// Sin cédula en la ficha ni en la hoja de vida: antes de molestar al candidato se mira si la
	// misma persona ya se postuló y la dejó. Esa cédula no se da por buena sin comprobar que
	// ADRES la entrega a alguien con el mismo nombre.
	verificarIdentidad := false
	if sinCedula && genero == "Hombre" && !simulada { // la simulada no busca en otras postulaciones: debe pasar por el formulario
		if cedula := a.cedulaDePostulacionPrevia(c); cedula != "" {
			c.Cedula, sinCedula, verificarIdentidad = cedula, false, true
		}
	}

	// La consulta completa no descarta, pero tampoco se le pide la cédula a quien las reglas ya
	// excluyen: solo se pausa a quien pasaría el género.
	if a.opts.Completo && !verificarIdentidad && !(sinCedula && genero == "Hombre") {
		a.analizarCompleto(ctx, c, carpetaOferta, acum)
		return
	}

	// Descarte temprano: se corta en la primera regla que falla, de la más barata a la más cara.
	if genero != "Hombre" {
		a.descartar(c, carpetaOferta, acum, nil, fmt.Sprintf("Género (%s) no cumple requisito (requerido: Hombre)", genero))
		return
	}
	if sinCedula {
		// No es un rechazo: falta un dato que solo el candidato puede dar. Se pausa y se le pide.
		a.pausarPorDatos(c, carpetaOferta, acum)
		return
	}

	a.continuarDesdeCedula(ctx, c, carpetaOferta, acum, verificarIdentidad)
}

// continuarDesdeCedula aplica las reglas que dependen de la cédula (ADRES, RUNT y SIMIT, de la más
// barata a la más cara, cortando en la primera que falla) y luego las consultas informativas. Es el
// tramo que se reanuda cuando el candidato entrega la cédula que faltaba. verificarIdentidad pide
// comprobar con ADRES que la cédula pertenece a quien dice el perfil.
func (a *Analizador) continuarDesdeCedula(ctx context.Context, c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados, verificarIdentidad bool) {
	// La edad es la de la ficha de Computrabajo o la del formulario; el rango de contratación se revisa antes de
	// gastar consultas en las fuentes.
	if motivo := a.motivoEdad(c); motivo != "" {
		a.descartar(c, carpetaOferta, acum, nil, motivo)
		return
	}
	reintegro, continuar := a.consultarReintegro(ctx, c)
	if !continuar {
		return // cancelado
	}
	var pendientes []string
	switch reintegro.estado {
	case estadoRechazado:
		// Ya trabajó aquí: no puede volver a entrar. Se termina con él sin gastar más consultas.
		c.ErrorProceso = ""
		a.descartar(c, carpetaOferta, acum, nil, "Reintegro: "+reintegro.detalle)
		return
	case estadoError:
		// No se pudo comprobar: se sigue con las demás reglas (un rechazo firme gana) y, si no, queda pendiente.
		c.ErrorProceso = "falló la consulta de REINTEGRO"
		pendientes = append(pendientes, reintegro.detalle)
	}
	hechas := make(map[string]resultadoEtapa)
	for _, e := range a.etapasNucleo() {
		r, continuar := a.correrEtapa(ctx, c, e)
		if !continuar {
			return // cancelado
		}
		hechas[e.nombre] = r
		if verificarIdentidad && e.nombre == "adres" && r.estado != estadoError {
			if confirmada, hayTitular := identidadDe(c); !confirmada {
				a.identidadEnRevision(c, carpetaOferta, acum, hechas, hayTitular)
				return
			}
		}
		switch r.estado {
		case estadoOK:
		case estadoError:
			// No hay evidencia ni a favor ni en contra: se sigue con las demás reglas y se deja pendiente.
			c.ErrorProceso = "falló la consulta de " + strings.ToUpper(e.nombre)
			pendientes = append(pendientes, r.detalle)
		default:
			// Rechazo firme: aunque otra consulta haya fallado, el candidato ya no cumple.
			c.ErrorProceso = ""
			a.descartar(c, carpetaOferta, acum, hechas, r.detalle)
			return
		}
	}
	if len(pendientes) > 0 {
		a.dejarPendiente(c, carpetaOferta, acum, hechas, pendientes)
		return
	}

	// Cumple las cuatro reglas: ahora sí las consultas informativas.
	a.correrParalelo(ctx, c, a.etapasInformativas())
	if ctx.Err() != nil {
		return
	}
	a.clasif.Evaluar(c)
	a.evaluarConIA(ctx, c)
	a.cerrarCandidato(c, carpetaOferta, acum)
}

// analizarCompleto consulta todas las fuentes en paralelo aunque el candidato ya esté descartado.
func (a *Analizador) analizarCompleto(ctx context.Context, c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados) {
	reintegro, continuar := a.consultarReintegro(ctx, c)
	if !continuar {
		return
	}
	todas := append(a.etapasNucleo(), a.etapasInformativas()...)
	a.correrParalelo(ctx, c, todas)
	if ctx.Err() != nil {
		return
	}
	rechazo := c.Genero != "Hombre" || reintegro.estado == estadoRechazado
	for _, e := range a.etapasNucleo() {
		switch estado, _, _ := e.evaluar(c); estado {
		case estadoError:
			c.ErrorProceso = "falló la consulta de " + strings.ToUpper(e.nombre)
		case estadoRechazado:
			rechazo = true
		}
	}
	ev := a.clasif.Evaluar(c)
	if reintegro.estado == estadoRechazado {
		ev.EsApto, c.EsApto = false, false
		ev.MotivosDescarte = append(ev.MotivosDescarte, "Reintegro: "+reintegro.detalle)
	}
	if rechazo {
		c.ErrorProceso = "" // hay un rechazo firme: reintentar no cambia el resultado
	} else if !ev.EsApto && c.ErrorProceso != "" {
		ev.VerificacionPendiente = true
	}
	a.evaluarConIA(ctx, c)
	a.cerrarCandidato(c, carpetaOferta, acum)
}

func (a *Analizador) correrParalelo(ctx context.Context, c *modelos.Candidato, etapas []etapa) {
	var wg sync.WaitGroup
	for _, e := range etapas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.correrEtapa(ctx, c, e)
		}()
	}
	wg.Wait()
}

// correrEtapa ejecuta una consulta respetando su cupo. El segundo valor es falso si se canceló.
func (a *Analizador) correrEtapa(ctx context.Context, c *modelos.Candidato, e etapa) (resultadoEtapa, bool) {
	a.etapa(c, e.nombre, estadoIniciada, "", 0)
	if e.sem != nil {
		liberar := adquirirCupo(ctx, e.sem)
		if liberar == nil {
			return resultadoEtapa{}, false
		}
		defer liberar()
	}

	// Una falla técnica (captcha mal leído, portal lento) se reintenta una vez antes de darse por vencida.
	intentos := 1
	if e.esNucleo {
		intentos = 2
	}
	t0 := time.Now()
	var estado, detalle string
	var cumple bool
	for intento := 1; intento <= intentos; intento++ {
		errValidar := e.validar(ctx, c)
		if ctx.Err() != nil {
			return resultadoEtapa{}, false
		}
		estado, cumple, detalle = e.evaluar(c)
		if errValidar != nil {
			estado, cumple, detalle = estadoError, false, fmt.Sprintf("Error en la consulta: %v", errValidar)
		}
		if estado != estadoError || intento == intentos {
			break
		}
		a.etapa(c, e.nombre, estadoReintento, detalle, time.Since(t0))
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return resultadoEtapa{}, false
		}
	}
	a.etapa(c, e.nombre, estado, detalle, time.Since(t0))
	return resultadoEtapa{estado: estado, cumple: cumple, detalle: detalle}, true
}

// descartar cierra al candidato como NO APTO sin consultar lo que falta. hechas son las etapas
// que sí alcanzaron a correr; motivo es la razón del descarte.
func (a *Analizador) descartar(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados, hechas map[string]resultadoEtapa, motivo string) {
	a.contar(func(r *ResumenAnalisis) { r.DescartesTempranos++ })
	a.cerrarSinAptitud(c, carpetaOferta, acum, hechas, []string{motivo}, false)
}

// dejarPendiente cierra al candidato sin dictamen: alguna consulta falló y no se pudo comprobar la
// regla. No se rechaza; el siguiente análisis lo reintenta solo (ErrorProceso).
func (a *Analizador) dejarPendiente(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados, hechas map[string]resultadoEtapa, pendientes []string) {
	motivos := make([]string, len(pendientes))
	for i, p := range pendientes {
		motivos[i] = "Verificación pendiente: " + p
	}
	a.cerrarSinAptitud(c, carpetaOferta, acum, hechas, motivos, true)
}

func (a *Analizador) cerrarSinAptitud(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados, hechas map[string]resultadoEtapa, motivos []string, pendiente bool) {
	genero := c.Genero
	if genero == "" {
		genero = a.clasif.DeterminarGenero(c.Nombre, c.TextoPerfil)
	}
	detalle := func(nombre string) (bool, string) {
		if r, ok := hechas[nombre]; ok {
			return r.cumple && r.estado == estadoOK, r.detalle
		}
		return false, detalleNoConsult
	}
	cumpleRunt, detRunt := detalle("runt")
	cumpleSimit, detSimit := detalle("simit")
	cumpleEPS, detEPS := detalle("adres")

	c.Genero = genero
	c.EsApto = false
	c.EvaluacionIdoneidad = &modelos.EvaluacionIdoneidad{
		EsApto:                false,
		Genero:                genero,
		CumpleGenero:          genero == "Hombre",
		CumpleEPS:             cumpleEPS,
		DetalleEPS:            detEPS,
		CumpleRunt:            cumpleRunt,
		DetalleRunt:           detRunt,
		CumpleSimit:           cumpleSimit,
		DetalleSimit:          detSimit,
		MotivosDescarte:       motivos,
		DescarteTemprano:      !pendiente,
		VerificacionPendiente: pendiente,
	}
	a.cerrarCandidato(c, carpetaOferta, acum)
}

// cerrarCandidato persiste al candidato, actualiza los totales y lo informa.
func (a *Analizador) cerrarCandidato(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados) {
	if c.EvaluacionIdoneidad != nil {
		c.EsApto = c.EvaluacionIdoneidad.EsApto
	}
	if c.EsApto && a.cfg != nil {
		if err := a.clasif.GuardarCandidatoApto(c, carpetaOferta); err != nil {
			a.etapa(c, "guardado", estadoError, "no se pudo guardar en candidatos_aptos: "+err.Error(), 0)
		}
		if err := a.clasif.GuardarCandidatoApto(c, a.cfg.DirectorioDescargas); err != nil {
			a.etapa(c, "guardado", estadoError, "no se pudo guardar en el consolidado: "+err.Error(), 0)
		}
	}
	acum.agregar(c)
	if err := guardarEnBase(a.persistencia, a.oferta, c); err != nil {
		a.etapa(c, "guardado", estadoError, "no se pudo guardar en la base de datos: "+err.Error(), 0)
	}
	a.guardarIA(c)
	a.contar(func(r *ResumenAnalisis) {
		r.Procesados++
		if c.EsApto {
			r.Aptos++
		} else {
			r.NoAptos++
		}
		if c.ErrorProceso != "" {
			r.Errores++
		}
		if c.EvaluacionIdoneidad != nil && c.EvaluacionIdoneidad.VerificacionPendiente {
			r.Pendientes++
		}
	})
	a.informarCompleto(c, false)
}

func (a *Analizador) informarCompleto(c *modelos.Candidato, cache bool) {
	copia := *c
	copia.TextoPerfil = "" // pesa miles de caracteres y ya está en /api/vacantes/{slug}/candidatos
	a.rep.Emitir("candidato.completo", PayloadCandidato{Candidato: copia, Cache: cache})
	a.rep.Emitir("resumen", a.Resumen())
}

func (a *Analizador) etapa(c *modelos.Candidato, nombre, estado, detalle string, d time.Duration) {
	datos := map[string]any{"indice": c.Indice, "nombre": c.Nombre, "etapa": nombre, "estado": estado}
	if detalle != "" {
		datos["detalle"] = detalle
	}
	if d > 0 {
		datos["ms"] = d.Milliseconds()
	}
	if estado != estadoIniciada && estado != estadoReintento {
		if ruta := agentes.CapturaDe(c, nombre); ruta != "" {
			datos["captura"] = ruta
		}
	}
	a.rep.Emitir("candidato.etapa", datos)
}

func soloDigitos(texto string) string {
	var b strings.Builder
	for _, r := range texto {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// motivoEdad devuelve por qué la edad declarada queda fuera del rango de contratación, o "" si cumple
// o no se conoce (no se inventa una edad).
func (a *Analizador) motivoEdad(c *modelos.Candidato) string {
	if a.cfg == nil || c.Edad <= 0 {
		return ""
	}
	if c.Edad < a.cfg.EdadMinimaContratacion || c.Edad > a.cfg.EdadMaximaContratacion {
		return fmt.Sprintf("Edad (%d años) fuera del rango de contratación (%d a %d años)", c.Edad, a.cfg.EdadMinimaContratacion, a.cfg.EdadMaximaContratacion)
	}
	return ""
}
