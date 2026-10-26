package orquestador

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// OpcionesEjecucion contiene los parametros de ejecucion del motor de automatizacion.
type OpcionesEjecucion struct {
	Concurrencia        int
	LimiteCandidatos    int
	ValidarAntecedentes bool
	ValidarSarlaft      bool
	ValidarOfac         bool
	ValidarAdres        bool
	ValidarRunt         bool
	ValidarSimit        bool
	EvaluarIA           bool
	ModeloIA            string
}

// MotorOrquestador coordina los agentes concurrentes y las fuentes gubernamentales.
type MotorOrquestador struct {
	cfg          *configuracion.Configuracion
	opciones     OpcionesEjecucion
	computrabajo *agentes.AgenteComputrabajo
	sarlaft      *agentes.AgenteSarlaft
	ofac         *agentes.AgenteOfac
	procuraduria *agentes.AgenteProcuraduria
	adres        *agentes.AgenteAdres
	runt         *agentes.AgenteRunt
	simit        *agentes.AgenteSimit
	clasificador *agentes.AgenteClasificador
	evaluadorIA  *agentes.AgenteIA

	persistencia Persistencia // opcional: base de datos

	semComputrabajo chan struct{}
	semCandidatos   chan struct{}
	semProcuraduria chan struct{}
	semSimit        chan struct{}
	semOfac         chan struct{}
	semAdres        chan struct{}
	semRunt         chan struct{}
}

// NuevoMotorOrquestador inicializa todos los agentes y canales de concurrencia.
func NuevoMotorOrquestador(cfg *configuracion.Configuracion, opts OpcionesEjecucion) *MotorOrquestador {
	if opts.Concurrencia <= 0 {
		opts.Concurrencia = 20
	}

	motor := &MotorOrquestador{
		cfg:          cfg,
		opciones:     opts,
		computrabajo: agentes.NuevoAgenteComputrabajo(cfg),
		clasificador: agentes.NuevoAgenteClasificador(cfg),

		semComputrabajo: make(chan struct{}, 3),
		semCandidatos:   make(chan struct{}, opts.Concurrencia),
		semProcuraduria: make(chan struct{}, min(opts.Concurrencia, 3)),
		semSimit:        make(chan struct{}, min(opts.Concurrencia, 3)),
		semOfac:         make(chan struct{}, min(opts.Concurrencia, 4)),
		semAdres:        make(chan struct{}, 1),
		semRunt:         make(chan struct{}, min(opts.Concurrencia, 4)),
	}

	if opts.ValidarSarlaft {
		motor.sarlaft = agentes.NuevoAgenteSarlaft(cfg)
	}
	if opts.ValidarOfac {
		motor.ofac = agentes.NuevoAgenteOfac(cfg)
	}
	if opts.ValidarAntecedentes {
		motor.procuraduria = agentes.NuevoAgenteProcuraduria(cfg)
	}
	if opts.ValidarAdres {
		motor.adres = agentes.NuevoAgenteAdres(cfg)
	}
	if opts.ValidarRunt {
		motor.runt = agentes.NuevoAgenteRunt(cfg)
	}
	if opts.ValidarSimit {
		motor.simit = agentes.NuevoAgenteSimit(cfg)
	}
	if opts.EvaluarIA {
		motor.evaluadorIA = agentes.NuevoAgenteIA(cfg, opts.ModeloIA)
	}

	return motor
}

// ConPersistencia hace que cada candidato, al terminar, se guarde también en la base de datos.
func (m *MotorOrquestador) ConPersistencia(p Persistencia) *MotorOrquestador {
	m.persistencia = p
	return m
}

// Cerrar libera todos los navegadores y agentes asignados.
func (m *MotorOrquestador) Cerrar() {
	if m.computrabajo != nil {
		m.computrabajo.Cerrar()
	}
	if m.ofac != nil {
		m.ofac.Cerrar()
	}
	if m.procuraduria != nil {
		m.procuraduria.Cerrar()
	}
	if m.adres != nil {
		m.adres.Cerrar()
	}
	if m.simit != nil {
		m.simit.Cerrar()
	}
}

// Ejecutar inicia el ciclo completo de procesamiento en Computrabajo y fuentes externas.
func (m *MotorOrquestador) Ejecutar(ctx context.Context) error {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  AUTOMATIZACIÓN RRHH - COMPUTRABAJO, PROCURADURÍA, ONU, OFAC, ADRES, RUNT & SIMIT (MOTOR GO)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("[*] INICIANDO MOTOR ASÍNCRONO CONCURRENTE (Concurrencia máxima: %d)\n", m.opciones.Concurrencia)
	fmt.Println(strings.Repeat("=", 60))

	if err := m.computrabajo.AsegurarAutenticacion(ctx); err != nil {
		return fmt.Errorf("error de inicio de sesión: %w", err)
	}

	ofertas, err := m.computrabajo.ObtenerOfertasActivas(ctx)
	if err != nil {
		return fmt.Errorf("error al obtener ofertas: %w", err)
	}

	if len(ofertas) == 0 {
		fmt.Println("[!] No se detectaron ofertas activas para procesar.")
		return nil
	}

	for _, oferta := range ofertas {
		if ctx.Err() != nil {
			fmt.Println("\n[!] Ejecución cancelada; se detiene el procesamiento de ofertas.")
			break
		}

		// Filtrar ofertas excluidas por configuración
		excluir := false
		tituloLower := strings.ToLower(oferta.Titulo)
		for _, patron := range m.cfg.OfertasExcluidas {
			if strings.Contains(tituloLower, strings.ToLower(patron)) {
				excluir = true
				break
			}
		}
		if excluir {
			fmt.Printf("\n[~] Oferta '%s' excluida por configuración (no aplica).\n", oferta.Titulo)
			continue
		}

		if err := m.procesarOferta(ctx, oferta); err != nil {
			fmt.Printf("[!] Error en oferta '%s': %v\n", oferta.Titulo, err)
		}
	}

	// Si está activada la evaluación con IA, generar el ranking consolidado
	if ctx.Err() == nil && m.opciones.EvaluarIA && m.evaluadorIA != nil {
		rutaGlobalAptos := filepath.Join(m.cfg.DirectorioDescargas, "candidatos_aptos", "candidatos_aptos.json")
		if _, err := os.Stat(rutaGlobalAptos); err == nil {
			fmt.Printf("\n[*] Generando ranking global consolidado de candidatos aptos con IA...\n")
			if _, err := m.evaluadorIA.EvaluarCandidatosAptos(ctx, rutaGlobalAptos, "Consolidado de Candidatos Aptos"); err != nil {
				fmt.Printf("[!] Error generando ranking con IA: %v\n", err)
			}
		}
	}

	return nil
}

// EvaluarAptosExistentes ejecuta el análisis y scoring con IA sobre un archivo de candidatos aptos existente.
func (m *MotorOrquestador) EvaluarAptosExistentes(ctx context.Context, rutaJSON string, cargo string) error {
	if m.evaluadorIA == nil {
		m.evaluadorIA = agentes.NuevoAgenteIA(m.cfg, m.opciones.ModeloIA)
	}
	_, err := m.evaluadorIA.EvaluarCandidatosAptos(ctx, rutaJSON, cargo)
	return err
}

func (m *MotorOrquestador) procesarOferta(ctx context.Context, oferta modelos.OfertaLaboral) error {
	tituloLimpio := agentes.LimpiarNombre(oferta.Titulo)
	carpetaOferta := filepath.Join(m.cfg.DirectorioDescargas, tituloLimpio)
	_ = os.MkdirAll(carpetaOferta, 0755)
	rutaResumen := filepath.Join(carpetaOferta, "candidatos.json")

	fmt.Printf("\n[*] Procesando oferta: %s\n", oferta.Titulo)
	fmt.Printf("[*] Carpeta: %s\n", carpetaOferta)

	candidatos, err := m.computrabajo.ObtenerCandidatosDeOferta(ctx, oferta.URLCandidatos, m.opciones.LimiteCandidatos)
	if err != nil {
		return fmt.Errorf("error obteniendo candidatos: %w", err)
	}

	total := len(candidatos)
	fmt.Printf("[+] Total candidatos a procesar en paralelo: %d (Concurrencia: %d)\n", total, m.opciones.Concurrencia)
	if total == 0 {
		return nil
	}

	t0 := time.Now()
	acumulador := &acumuladorResultados{ruta: rutaResumen}
	var wg sync.WaitGroup

	for _, cand := range candidatos {
		liberar := adquirirCupo(ctx, m.semCandidatos)
		if liberar == nil {
			break // cancelado: no se lanzan más candidatos
		}
		wg.Add(1)
		go func(c *modelos.Candidato) {
			defer wg.Done()
			defer liberar()
			defer func() {
				// Un fallo inesperado con un candidato no debe tumbar toda la ejecución.
				if r := recover(); r != nil {
					fmt.Printf("[X] Error inesperado procesando a %s: %v\n%s\n", c.Nombre, r, debug.Stack())
				}
			}()

			m.procesarCandidatoIndividual(ctx, oferta, c, total, carpetaOferta, acumulador)
		}(cand)
	}

	wg.Wait()
	acumulador.finalizar()
	duracion := time.Since(t0)

	totalAptos := 0
	for _, c := range acumulador.lista {
		if c.EsApto {
			totalAptos++
		}
	}

	fmt.Printf("\n[OK] Oferta '%s' completada: %d candidatos en %.2f segundos.\n", oferta.Titulo, len(acumulador.lista), duracion.Seconds())
	fmt.Printf("[*] Resumen de selección: %d/%d candidatos APTOS guardados en carpeta 'candidatos_aptos/'.\n", totalAptos, total)
	return nil
}

func (m *MotorOrquestador) procesarCandidatoIndividual(
	ctx context.Context,
	oferta modelos.OfertaLaboral,
	c *modelos.Candidato,
	total int,
	carpetaOferta string,
	acumulador *acumuladorResultados,
) {
	nombreLimpio := agentes.LimpiarNombre(c.Nombre)
	carpetaCandidato := filepath.Join(carpetaOferta, fmt.Sprintf("%d_%s", c.Indice, nombreLimpio))
	_ = os.MkdirAll(carpetaCandidato, 0755)

	// 1. Extraer cédula, género y generar PDF en Computrabajo con semáforo para evitar sobrecarga en Chrome
	liberarCT := adquirirCupo(ctx, m.semComputrabajo)
	if liberarCT == nil {
		return
	}
	if err := m.computrabajo.ExtraerDetalleCandidato(ctx, c, carpetaCandidato); err != nil {
		fmt.Printf("[!] Detalle Computrabajo (%s): %v\n", c.Nombre, err)
	}
	liberarCT()

	// 2. Disparar validaciones gubernamentales en paralelo
	m.ejecutarValidacionesParalelo(ctx, c)

	if ctx.Err() != nil {
		return // no clasificar con validaciones incompletas por una cancelación
	}

	// 3. Evaluar reglas de idoneidad (Hombre, EPS activa autorizada, Licencia C2/C3 >= 2 años en RUNT, SIMIT 0 multas o <= $1.000.000 con acuerdo)
	evaluacion := m.clasificador.EvaluarCandidato(c)
	c.EvaluacionIdoneidad = evaluacion
	c.EsApto = evaluacion.EsApto
	c.Genero = evaluacion.Genero

	// 4. Si es APTO, segregar en carpetas dedicadas
	if c.EsApto {
		if err := m.clasificador.GuardarCandidatoApto(c, carpetaOferta); err != nil {
			fmt.Printf("[!] No se pudo guardar el candidato apto %s en la carpeta de oferta: %v\n", c.Nombre, err)
		}
		if err := m.clasificador.GuardarCandidatoApto(c, m.cfg.DirectorioDescargas); err != nil {
			fmt.Printf("[!] No se pudo guardar el candidato apto %s en el consolidado: %v\n", c.Nombre, err)
		}
	}

	// 5. Persistir en candidatos.json (guardado diferido y atómico)
	acumulador.agregar(c)
	if err := guardarEnBase(m.persistencia, oferta, c); err != nil {
		fmt.Printf("[!] No se pudo guardar a %s en la base de datos: %v\n", c.Nombre, err)
	}

	// 6. Log informativo en consola
	m.imprimirLogProgreso(c, total)
}

func (m *MotorOrquestador) ejecutarValidacionesParalelo(ctx context.Context, c *modelos.Candidato) {
	var wgVal sync.WaitGroup

	// lanzar ejecuta una validación con su semáforo (nil = sin límite) respetando la cancelación.
	lanzar := func(sem chan struct{}, validar func() error) {
		wgVal.Add(1)
		go func() {
			defer wgVal.Done()
			if sem != nil {
				liberar := adquirirCupo(ctx, sem)
				if liberar == nil {
					return
				}
				defer liberar()
			}
			_ = validar()
		}()
	}

	// SARLAFT (memoria, sin cupo)
	if m.sarlaft != nil {
		lanzar(nil, func() error { return m.sarlaft.Validar(ctx, c) })
	}
	if m.ofac != nil {
		lanzar(m.semOfac, func() error { return m.ofac.Validar(ctx, c) })
	}
	if m.procuraduria != nil {
		lanzar(m.semProcuraduria, func() error { return m.procuraduria.Validar(ctx, c) })
	}
	// ADRES (BDUA EPS): un solo navegador visible a la vez
	if m.adres != nil {
		lanzar(m.semAdres, func() error { return m.adres.Validar(ctx, c) })
	}
	if m.simit != nil {
		lanzar(m.semSimit, func() error { return m.simit.Validar(ctx, c) })
	}
	// RUNT: se lanza en paralelo (no necesita esperar ADRES dado que ADRES tiene captcha)
	if m.runt != nil {
		lanzar(m.semRunt, func() error { return m.runt.ValidarConApellidos(ctx, c, "") })
	}

	wgVal.Wait()
}

func (m *MotorOrquestador) imprimirLogProgreso(c *modelos.Candidato, total int) {
	estados := []string{}
	if c.ResultadoProcuraduria != nil {
		estados = append(estados, fmt.Sprintf("Procuraduría: %s", c.ResultadoProcuraduria.Estado))
	}
	if c.ResultadoSarlaft != nil {
		alerta := "OK"
		if c.ResultadoSarlaft.Alerta {
			alerta = "ALERTA"
		}
		estados = append(estados, fmt.Sprintf("SARLAFT: %s", alerta))
	}
	if c.ResultadoOfac != nil {
		alerta := "OK"
		if c.ResultadoOfac.Alerta {
			alerta = "ALERTA"
		}
		estados = append(estados, fmt.Sprintf("OFAC: %s", alerta))
	}
	if c.ResultadoAdres != nil {
		estados = append(estados, fmt.Sprintf("EPS: %s", c.ResultadoAdres.EPS))
	}
	if c.ResultadoRunt != nil {
		estados = append(estados, fmt.Sprintf("RUNT: %s", c.ResultadoRunt.Estado))
	}
	if c.ResultadoSimit != nil {
		estados = append(estados, fmt.Sprintf("SIMIT: %s", c.ResultadoSimit.Estado))
	}

	estadosStr := "OK"
	if len(estados) > 0 {
		estadosStr = strings.Join(estados, " | ")
	}

	fmt.Printf("[+] [%d/%d] %s (CC: %s) -> %s\n", c.Indice, total, c.Nombre, c.Cedula, estadosStr)
	if c.EsApto {
		fmt.Printf("    [*** CANDIDATO APTO ***] Cumple todos los requisitos (Género: %s | %s | %s | %s) -> Guardado en 'candidatos_aptos/'.\n",
			c.EvaluacionIdoneidad.Genero,
			c.EvaluacionIdoneidad.DetalleEPS,
			c.EvaluacionIdoneidad.DetalleRunt,
			c.EvaluacionIdoneidad.DetalleSimit,
		)
	} else if c.EvaluacionIdoneidad != nil && len(c.EvaluacionIdoneidad.MotivosDescarte) > 0 {
		fmt.Printf("    [-] No apto: %s\n", strings.Join(c.EvaluacionIdoneidad.MotivosDescarte, "; "))
	}
}

// acumuladorResultados reúne los candidatos procesados desde muchas goroutines y los persiste en
// candidatos.json sin reescribir el archivo completo por cada candidato (antes O(n²)): durante la
// ejecución se guarda como máximo cada 2 s en formato compacto; al terminar queda ordenado y legible.
type acumuladorResultados struct {
	mu     sync.Mutex
	ruta   string
	lista  []*modelos.Candidato
	ultimo time.Time
}

func (p *acumuladorResultados) agregar(c *modelos.Candidato) {
	p.mu.Lock()
	defer p.mu.Unlock()
	reemplazado := false
	if c.URLDetalle != "" {
		for i, previo := range p.lista {
			if previo.URLDetalle == c.URLDetalle {
				p.lista[i] = c // un reanálisis sustituye al registro anterior del mismo postulante
				reemplazado = true
				break
			}
		}
	}
	if !reemplazado {
		p.lista = append(p.lista, c)
	}
	if time.Since(p.ultimo) >= 2*time.Second {
		p.escribirLocked(false)
	}
}

// conservarSoloVistos descarta los registros cuyo postulante ya no aparece en la oferta.
func (p *acumuladorResultados) conservarSoloVistos(vistos map[string]bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	filtrada := p.lista[:0]
	for _, c := range p.lista {
		if c.URLDetalle == "" || vistos[c.URLDetalle] {
			filtrada = append(filtrada, c)
		}
	}
	p.lista = filtrada
}

func (p *acumuladorResultados) finalizar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	sort.Slice(p.lista, func(i, j int) bool { return p.lista[i].Indice < p.lista[j].Indice })
	p.escribirLocked(true)
}

func (p *acumuladorResultados) escribirLocked(indentar bool) {
	if p.ruta == "" {
		return // acumulador sin archivo (reanudaciones): el resultado va a la base de datos
	}
	if err := utilidades.EscribirJSONAtomico(p.ruta, p.lista, indentar); err != nil {
		fmt.Printf("[!] No se pudo guardar %s: %v\n", p.ruta, err)
	}
	p.ultimo = time.Now()
}

// adquirirCupo toma un cupo del semáforo respetando la cancelación del contexto.
// Devuelve la función de liberación o nil si el contexto se canceló antes de obtenerlo.
func adquirirCupo(ctx context.Context, sem chan struct{}) func() {
	select {
	case sem <- struct{}{}:
		return func() { <-sem }
	case <-ctx.Done():
		return nil
	}
}
