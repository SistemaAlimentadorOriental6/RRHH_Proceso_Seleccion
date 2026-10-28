package servidor

import (
	"context"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
)

const (
	// sondeoJobAutomatico es cada cuánto se mira si el análisis en curso ya terminó.
	sondeoJobAutomatico = 3 * time.Second
	// reintentosVacante y esperaReintentoVacante: cuántas veces y cuánto después se reintenta una vacante
	// cuyo análisis no pudo arrancar.
	reintentosVacante      = 3
	esperaReintentoVacante = 3 * time.Minute
	// reintentoPendientesPorDefecto es cada cuánto se vuelven a analizar las vacantes con candidatos que
	// quedaron pendientes por una consulta fallida (portal caído, captcha). Antes solo se reintentaban al
	// llegar gente nueva, y ese reintento podía no llegar nunca.
	reintentoPendientesPorDefecto = 10 * time.Minute
	// cadaCorreoPorDefecto es cada cuánto se revisa el buzón en busca de hojas de vida nuevas.
	cadaCorreoPorDefecto = time.Minute
	// ultimasCorreoPorDefecto: en la primera lectura del buzón solo se guardan las N hojas más recientes.
	ultimasCorreoPorDefecto = 20
)

// ofertaCorreo es la vacante virtual donde caen las hojas de vida recibidas por correo.
func ofertaCorreo() modelos.OfertaLaboral {
	return modelos.OfertaLaboral{ID: agentes.IDOfertaCorreo, Titulo: agentes.TituloOfertaCorreo, URLCandidatos: "correo://buzon", Cuenta: -1}
}

// agenteCorreo devuelve el lector del buzón, creándolo la primera vez.
func (c *ControladorRRHH) agenteCorreo() *agentes.AgenteCorreo {
	c.muCorreo.Lock()
	defer c.muCorreo.Unlock()
	if c.correo == nil {
		c.correo = agentes.NuevoAgenteCorreo(c.cfg, c.agenteIA)
		c.correo.UltimasEnPrimeraLectura = enteroEnv("RRHH_CORREO_ULTIMAS", ultimasCorreoPorDefecto)
	}
	return c.correo
}

func enteroEnv(nombre string, porDefecto int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(nombre))); err == nil && n >= 0 {
		return n
	}
	return porDefecto
}

// vacanteCorreo es la vacante virtual con el número de hojas de vida que entran al flujo, o nil si
// el buzón no está configurado. Solo lee un archivo local: no toca la red.
func (c *ControladorRRHH) vacanteCorreo() *ofertaConConteo {
	if !c.cfg.CorreoActivo() {
		return nil
	}
	v := ofertaConConteo{OfertaLaboral: ofertaCorreo()}
	if hojas, err := c.agenteCorreo().HojasParaAnalizar(); err != nil {
		log.Printf("[!] No se pudo contar las hojas de vida del correo: %v", err)
		v.ErrorMsg = "no se pudo contar las hojas de vida del correo"
	} else {
		n := len(hojas)
		v.Total = &n
	}
	return &v
}

// IniciarAutomatico arranca en segundo plano el recorrido completo, sin intervención:
//  1. cada vacante activa de Computrabajo (todas las cuentas), una tras otra;
//  2. en paralelo desde el inicio, el buzón: se lee cada RRHH_CORREO_CADA_MIN minutos (1 por defecto)
//     y sus hojas de vida se analizan con el mismo flujo cuando llega alguna;
//  3. y de ahí en adelante, con la misma frecuencia, revisa los inscritos de cada vacante de
//     Computrabajo: solo si llegó algo nuevo lo analiza (lo ya analizado queda como está).
//
// RRHH_AUTOMATICO=false lo apaga (se puede lanzar cada análisis a mano desde la pantalla).
func (c *ControladorRRHH) IniciarAutomatico() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RRHH_AUTOMATICO")), "false") {
		log.Printf("[auto] Proceso automático apagado (RRHH_AUTOMATICO=false)")
		return
	}
	if !c.cfg.ValidarCredenciales() && !c.cfg.CorreoActivo() {
		return
	}
	go c.cicloAutomatico()
}

func (c *ControladorRRHH) cicloAutomatico() {
	ctx := c.base
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] proceso automático: %v", r)
		}
	}()
	log.Printf("[auto] Proceso automático iniciado: primero Computrabajo, después el correo")

	cada := time.Duration(enteroEnv("RRHH_CORREO_CADA_MIN", int(cadaCorreoPorDefecto/time.Minute))) * time.Minute
	if cada <= 0 {
		cada = cadaCorreoPorDefecto
	}
	// El buzón se vigila desde ya, en paralelo: no espera a que termine el recorrido inicial de Computrabajo.
	if c.cfg.CorreoActivo() {
		log.Printf("[auto] Vigilando el buzón %s cada %s", c.cfg.CorreoUsuario, cada)
		go c.vigilarBuzon(ctx, cada)
	} else {
		log.Printf("[auto] Buzón NO vigilado: faltan CORREO_USUARIO y CORREO_CLAVE en el .env")
	}

	vistos := map[string]int{} // inscritos de cada vacante la última vez que se miró
	if c.cfg.ValidarCredenciales() {
		consulta, cancel := context.WithTimeout(ctx, timeoutVacantesComputrabajo*time.Duration(max(c.numCuentas, 1)))
		ofertas, err := c.consultarVacantesComputrabajo(consulta, true) // con conteos: la pantalla se sirve de esta lista mientras se analiza
		cancel()
		if err != nil {
			log.Printf("[auto] No se pudieron leer las vacantes de Computrabajo: %v", err)
		}
		for _, o := range ofertas {
			if o.Total != nil {
				vistos[o.ID] = *o.Total
			}
		}
		var fallidas []modelos.OfertaLaboral
		for _, o := range ofertas {
			if ctx.Err() != nil {
				return
			}
			if o.ID == agentes.IDOfertaCorreo || c.estaExcluida(o.Titulo) {
				continue
			}
			log.Printf("[auto] Analizando la vacante %q (cuenta %d)", o.Titulo, o.Cuenta+1)
			if !c.analizarYEsperar(ctx, o.OfertaLaboral) {
				fallidas = append(fallidas, o.OfertaLaboral)
			}
		}
		// Una vacante que no pudo arrancar (sesión caída, red) se reintenta después de las demás, sin
		// repetir las que sí terminaron.
		for intento := 1; intento <= reintentosVacante && len(fallidas) > 0 && ctx.Err() == nil; intento++ {
			log.Printf("[auto] %d vacante(s) fallaron; reintento %d/%d en %s", len(fallidas), intento, reintentosVacante, esperaReintentoVacante)
			select {
			case <-ctx.Done():
				return
			case <-time.After(esperaReintentoVacante):
			}
			pendientes := fallidas
			fallidas = nil
			for _, o := range pendientes {
				if ctx.Err() != nil {
					return
				}
				if !c.analizarYEsperar(ctx, o) {
					fallidas = append(fallidas, o)
				}
			}
		}
		for _, o := range fallidas {
			log.Printf("[auto] La vacante %q no se pudo analizar; se puede lanzar a mano desde la pantalla", o.Titulo)
		}
	}

	log.Printf("[auto] Revisión de novedades (correo y Computrabajo) cada %s", cada)
	reintentoCada := time.Duration(enteroEnv("RRHH_REINTENTO_PENDIENTES_MIN", int(reintentoPendientesPorDefecto/time.Minute))) * time.Minute
	if reintentoCada <= 0 {
		reintentoCada = reintentoPendientesPorDefecto
	}
	ultimoReintento := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(cada):
		}
		// Un fallo inesperado en una vuelta no debe matar la vigilancia para siempre.
		protegido("revisión de novedades", func() {
			// La pantalla muestra al asistente despierto mientras dura la revisión y dormido el resto del minuto.
			c.cambios.emitir("revision", map[string]string{"estado": "inicio"})
			defer c.cambios.emitir("revision", map[string]string{"estado": "fin"})
			if c.cfg.ValidarCredenciales() {
				c.revisarNovedadesComputrabajo(ctx, vistos)
			}
			if time.Since(ultimoReintento) >= reintentoCada {
				ultimoReintento = time.Now()
				c.reintentarPendientes(ctx)
			}
		})
	}
}

// protegido ejecuta una vuelta de un ciclo de fondo registrando el pánico en vez de tumbar el ciclo.
func protegido(nombre string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] %s: %v", nombre, r)
		}
	}()
	f()
}

// vigilarBuzon lee el buzón cada intervalo sin depender de que haya un análisis en curso (antes la
// lectura esperaba a que terminara el análisis de Computrabajo, que puede tardar horas). Si llegaron
// hojas de vida las analiza cuando le toque el turno; si ese análisis falla, lo reintenta en la
// siguiente vuelta.
func (c *ControladorRRHH) vigilarBuzon(ctx context.Context, cada time.Duration) {
	porAnalizar := true // al arrancar se analiza lo que ya estuviera guardado sin analizar
	for {
		protegido("vigilancia del buzón", func() {
			revisados, guardadas, err := c.agenteCorreo().ProcesarNuevos(ctx)
			if err != nil && ctx.Err() == nil {
				log.Printf("[auto] Buzón: %v", err)
			}
			if revisados > 0 {
				log.Printf("[auto] Buzón: %d correos nuevos revisados, %d hojas de vida", revisados, guardadas)
			}
			if guardadas > 0 {
				porAnalizar = true
			}
			if porAnalizar && ctx.Err() == nil {
				porAnalizar = !c.analizarYEsperar(ctx, ofertaCorreo())
			}
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(cada):
		}
	}
}

// hayPendientesTecnicos dice si algún candidato quedó pendiente por una consulta fallida: esos se
// reintentan solos al volver a analizar la vacante. Los que esperan datos del propio candidato (pausa) no
// cuentan: no se resuelven reintentando.
func hayPendientesTecnicos(lista []modelos.Candidato) bool {
	for i := range lista {
		if lista[i].ErrorProceso != "" && !modelos.EnPausa(lista[i].EstadoProceso) {
			return true
		}
	}
	return false
}

// reintentarPendientes vuelve a analizar las vacantes que tienen candidatos pendientes por una consulta
// fallida; el análisis salta a todos los demás y solo rehace a esos.
func (c *ControladorRRHH) reintentarPendientes(ctx context.Context) {
	ofertas, _ := c.ofertasEnCache()
	hayCorreo := false
	for _, o := range ofertas {
		hayCorreo = hayCorreo || o.ID == agentes.IDOfertaCorreo
	}
	if !hayCorreo && c.cfg.CorreoActivo() {
		ofertas = append(ofertas, ofertaConConteo{OfertaLaboral: ofertaCorreo()})
	}
	for _, o := range ofertas {
		if ctx.Err() != nil || c.jobEnCursoActual() != nil {
			return
		}
		if o.ID != agentes.IDOfertaCorreo && c.estaExcluida(o.Titulo) {
			continue
		}
		lista, err := c.candidatosDeVacante(o.ID, o.Titulo, false)
		if err != nil || !hayPendientesTecnicos(lista) {
			continue
		}
		log.Printf("[auto] Vacante %q: hay candidatos pendientes por una consulta fallida; se reintentan", o.Titulo)
		c.analizarYEsperar(ctx, o.OfertaLaboral)
	}
}

// revisarNovedadesComputrabajo cuenta los inscritos de cada vacante y analiza las que crecieron desde
// la última revisión; el análisis salta lo ya guardado y solo procesa a los nuevos.
func (c *ControladorRRHH) revisarNovedadesComputrabajo(ctx context.Context, vistos map[string]int) {
	if c.jobEnCursoActual() != nil {
		return // el navegador está ocupado; se mira en la siguiente vuelta
	}
	consulta, cancel := context.WithTimeout(ctx, timeoutVacantesComputrabajo*time.Duration(max(c.numCuentas, 1)))
	ofertas, err := c.consultarVacantesComputrabajo(consulta, true)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("[auto] Computrabajo: no se pudieron contar las vacantes: %v", err)
		}
		return
	}
	for _, o := range ofertas {
		if ctx.Err() != nil {
			return
		}
		if o.ID == agentes.IDOfertaCorreo || o.Total == nil || c.estaExcluida(o.Titulo) {
			continue
		}
		previo, conocida := vistos[o.ID]
		vistos[o.ID] = *o.Total
		if conocida && *o.Total <= previo {
			continue
		}
		log.Printf("[auto] Vacante %q: %d inscritos (antes %d); se analizan los nuevos", o.Titulo, *o.Total, previo)
		if !c.analizarYEsperar(ctx, o.OfertaLaboral) {
			delete(vistos, o.ID) // no se pudo: se reintenta en la siguiente vuelta
		}
	}
}

// analizarYEsperar lanza el análisis de la vacante y espera a que termine. Solo hay un análisis a la
// vez: si ya hay otro en curso (p. ej. uno lanzado a mano) espera su turno.
func (c *ControladorRRHH) analizarYEsperar(ctx context.Context, oferta modelos.OfertaLaboral) bool {
	opts, _ := opcionesAnalisis(url.Values{}) // las mismas opciones que usa la pantalla por defecto
	for ctx.Err() == nil {
		job, creado := c.iniciarJobAnalisis(oferta, opts)
		esperarJob(ctx, job)
		if creado || strings.EqualFold(job.OfertaID, oferta.ID) {
			estado := job.estadoJob()
			if estado.Estado != jobCompletado {
				log.Printf("[auto] El análisis de %q terminó como %s", oferta.Titulo, estado.Estado)
			}
			return estado.Estado == jobCompletado
		}
	}
	return false
}

func esperarJob(ctx context.Context, job *JobAnalisis) {
	for job.enCurso() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(sondeoJobAutomatico):
		}
	}
}
