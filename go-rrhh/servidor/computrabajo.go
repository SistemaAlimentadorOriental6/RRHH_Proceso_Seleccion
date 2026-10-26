package servidor

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
)

const (
	timeoutVacantesComputrabajo = 6 * time.Minute
	// conteosSimultaneos es cuántas vacantes se cuentan a la vez (una pestaña de Chrome cada una).
	conteosSimultaneos = 4
)

// FuenteOfertas es lo que el servidor necesita de Computrabajo; el agente real la implementa
// y las pruebas usan una versión falsa.
type FuenteOfertas interface {
	AsegurarAutenticacion(ctx context.Context) error
	ObtenerOfertasActivas(ctx context.Context) ([]modelos.OfertaLaboral, error)
	ContarCandidatosOferta(ctx context.Context, urlCandidatos string) (int, error)
	Cerrar()
}

// VacanteComputrabajo es una vacante activa en Computrabajo, cruzada con lo ya procesado en local.
type VacanteComputrabajo struct {
	ID            string `json:"id,omitempty"`
	Slug          string `json:"slug"`
	Titulo        string `json:"titulo"`
	URLCandidatos string `json:"url_candidatos"`
	// TotalCandidatos es el número real de personas inscritas a la oferta en Computrabajo (conteo
	// del listado en vivo, sin quienes abandonaron el proceso). Es null si no se pudo contar.
	TotalCandidatos *int   `json:"total_candidatos"`
	ErrorConteo     string `json:"error_conteo,omitempty"`
	// Excluida es verdadera si el título coincide con OfertasExcluidas: el motor la omite.
	Excluida bool `json:"excluida"`
	// Procesada es verdadera si ya hay candidatos descargados y validados de esta vacante.
	Procesada bool `json:"procesada"`
	// Origen dice de dónde vienen las personas: "computrabajo" o "correo" (hojas de vida recibidas por
	// el buzón, que se cuentan aparte).
	Origen string `json:"origen"`
}

// fuenteComputrabajo devuelve el agente de Computrabajo, creándolo la primera vez. Debe llamarse
// con muCT tomado. El navegador queda abierto entre peticiones para no pagar el arranque de
// Chrome ni el login cada vez; los datos de las vacantes se piden en vivo en cada llamada.
// Solo hay un navegador abierto a la vez: pedir otra cuenta cierra el de la anterior (sesión por sesión).
func (c *ControladorRRHH) fuenteComputrabajo(cuenta int) FuenteOfertas {
	if c.ofertasCT != nil && c.cuentaCT != cuenta {
		c.descartarFuenteComputrabajo()
	}
	if c.ofertasCT == nil {
		c.ofertasCT = c.fabricaOfertas(cuenta)
		c.cuentaCT = cuenta
	}
	return c.ofertasCT
}

// descartarFuenteComputrabajo cierra el navegador para que la próxima petición arranque uno limpio.
func (c *ControladorRRHH) descartarFuenteComputrabajo() {
	if c.ofertasCT != nil {
		c.ofertasCT.Cerrar()
		c.ofertasCT = nil
	}
}

// ofertaConConteo es una oferta de Computrabajo con su número de inscritos.
type ofertaConConteo struct {
	modelos.OfertaLaboral
	Total    *int
	ErrorMsg string
}

// consultarVacantesComputrabajo lee las vacantes activas de todas las cuentas de Computrabajo, una
// cuenta a la vez, y cuenta los inscritos de cada una (si contar es verdadero). Si una cuenta falla
// pero otra respondió, se devuelve lo que hay; solo es error si ninguna respondió.
func (c *ControladorRRHH) consultarVacantesComputrabajo(ctx context.Context, contar bool) ([]ofertaConConteo, error) {
	// Un solo navegador con el perfil de Computrabajo: las consultas se serializan.
	select {
	case c.semCT <- struct{}{}:
		defer func() { <-c.semCT }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var todas []ofertaConConteo
	vistas := map[string]bool{}
	var errUltimo error
	respondieron := 0
	for cuenta := 0; cuenta < max(c.numCuentas, 1); cuenta++ {
		resultado, err := c.consultarCuenta(ctx, cuenta, contar, vistas)
		if ctx.Err() != nil {
			c.descartarFuenteComputrabajo()
			return nil, ctx.Err()
		}
		if err != nil {
			log.Printf("[!] Cuenta %d de Computrabajo: %v", cuenta+1, err)
			errUltimo = err
			continue
		}
		respondieron++
		todas = append(todas, resultado...)
	}
	if respondieron == 0 {
		return nil, errUltimo
	}
	// Las hojas de vida del correo son una vacante más; el número sale de un archivo local, sin red.
	if v := c.vacanteCorreo(); v != nil {
		todas = append(todas, *v)
	}
	// Siempre debe haber una lista guardada: mientras un análisis usa el navegador de Computrabajo, la
	// pantalla se sirve de ella. Una lista sin conteos no pisa a otra que sí los tiene.
	if previa, _ := c.ofertasEnCache(); contar || len(previa) == 0 {
		c.guardarOfertasEnCache(todas)
	}
	return todas, nil
}

// conCorreoActual devuelve la lista con el conteo del correo al día: ese número sale de un archivo
// local y cambia cuando llegan hojas de vida, aunque la lista de Computrabajo sea de hace rato.
func (c *ControladorRRHH) conCorreoActual(lista []ofertaConConteo) []ofertaConConteo {
	salida := make([]ofertaConConteo, 0, len(lista)+1)
	for _, o := range lista {
		if o.ID != agentes.IDOfertaCorreo {
			salida = append(salida, o)
		}
	}
	if v := c.vacanteCorreo(); v != nil {
		salida = append(salida, *v)
	}
	return salida
}

// tieneConteos dice si la lista trae el número de inscritos (o el motivo de que falte) de cada vacante.
func tieneConteos(lista []ofertaConConteo) bool {
	for _, o := range lista {
		if o.ID != agentes.IDOfertaCorreo && o.Total == nil && o.ErrorMsg == "" {
			return false
		}
	}
	return len(lista) > 0
}

// consultarCuenta lee (y cuenta) las vacantes de una sola cuenta. vistas guarda los ids ya listados
// por cuentas anteriores: cuentas de una misma empresa pueden ver la misma oferta, y esa se omite
// (antes de contarla) porque el id debe ser único: el frontend lo usa como clave.
func (c *ControladorRRHH) consultarCuenta(ctx context.Context, cuenta int, contar bool, vistas map[string]bool) ([]ofertaConConteo, error) {
	fuente := c.fuenteComputrabajo(cuenta)
	ofertas, err := fuente.ObtenerOfertasActivas(ctx)
	if errors.Is(err, agentes.ErrSesionExpirada) {
		if err = fuente.AsegurarAutenticacion(ctx); err == nil {
			ofertas, err = fuente.ObtenerOfertasActivas(ctx)
		}
	}
	if err != nil {
		// Chrome pudo quedar en mal estado: la próxima petición arranca uno nuevo.
		c.descartarFuenteComputrabajo()
		return nil, err
	}

	resultado := make([]ofertaConConteo, 0, len(ofertas))
	for _, o := range ofertas {
		if !c.cfg.OfertaIncluida(o.Titulo) {
			continue // solo conductores y operadores: las demás ni se listan ni se cuentan
		}
		clave := strings.ToLower(o.ID)
		if clave != "" && vistas[clave] {
			log.Printf("[*] Vacante %q ya la trae otra cuenta; se omite en la cuenta %d", o.Titulo, cuenta+1)
			continue
		}
		vistas[clave] = true
		o.Cuenta = cuenta
		resultado = append(resultado, ofertaConConteo{OfertaLaboral: o})
	}

	if contar {
		// Cada vacante se cuenta en su propia pestaña y en paralelo (máx. conteosSimultaneos): el
		// tiempo total pasa de la suma de las vacantes a la de la más lenta. Un fallo en una
		// vacante no invalida a las demás.
		inicio := time.Now()
		cupos := make(chan struct{}, conteosSimultaneos)
		var wg sync.WaitGroup
		for i := range resultado {
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case cupos <- struct{}{}:
					defer func() { <-cupos }()
				case <-ctx.Done():
					return
				}

				inicioVacante := time.Now()
				total, errConteo := fuente.ContarCandidatosOferta(ctx, resultado[i].URLCandidatos)
				if errConteo != nil {
					log.Printf("[!] No se pudo contar los inscritos de %q: %v", resultado[i].Titulo, errConteo)
					resultado[i].ErrorMsg = "no se pudo contar los inscritos de esta vacante"
					return
				}
				resultado[i].Total = &total
				log.Printf("[conteo] %q: %d inscritos en %s", resultado[i].Titulo, total, time.Since(inicioVacante).Round(time.Millisecond))
			}()
		}
		wg.Wait()
		log.Printf("[conteo] %d vacantes contadas en %s", len(resultado), time.Since(inicio).Round(time.Millisecond))
	}
	return resultado, nil
}

func (c *ControladorRRHH) estaExcluida(titulo string) bool {
	if !c.cfg.OfertaIncluida(titulo) {
		return true
	}
	bajo := strings.ToLower(titulo)
	for _, patron := range c.cfg.OfertasExcluidas {
		if strings.Contains(bajo, strings.ToLower(patron)) {
			return true
		}
	}
	return false
}

// ManejarVacantesComputrabajo lista, en vivo, las vacantes activas en Computrabajo y marca cuáles
// ya fueron procesadas localmente.
func (c *ControladorRRHH) ManejarVacantesComputrabajo(w http.ResponseWriter, r *http.Request) {
	if !c.cfg.ValidarCredenciales() {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Credenciales de Computrabajo no configuradas (COMPUTRABAJO_USUARIO y COMPUTRABAJO_CONTRASENA en .env)",
		})
		return
	}

	var ofertas []ofertaConConteo
	if job := c.jobEnCursoActual(); job != nil {
		// Un análisis tiene ocupado el navegador de Computrabajo: se responde con la última lista leída.
		cache, _ := c.ofertasEnCache()
		if len(cache) == 0 {
			// Aún no se pudo leer la lista (el análisis arrancó antes): se muestra lo que ya se sabe, la
			// vacante en análisis y la del correo, en vez de dejar la pantalla sin nada.
			if job.OfertaID != agentes.IDOfertaCorreo {
				cache = []ofertaConConteo{{OfertaLaboral: modelos.OfertaLaboral{ID: job.OfertaID, Titulo: job.Titulo}}}
			}
			w.Header().Set("X-Vacantes-Fuente", "parcial-analisis-en-curso")
		} else {
			w.Header().Set("X-Vacantes-Fuente", "cache-analisis-en-curso")
		}
		ofertas = c.conCorreoActual(cache)
	} else if cache, en := c.ofertasEnCache(); len(cache) > 0 && time.Since(en) < vigenciaListaFresca && (r.URL.Query().Get("contar") == "0" || tieneConteos(cache)) {
		// Hace instantes se leyó la lista (p. ej. el proceso automático al arrancar): no se vuelve a
		// abrir el navegador, que además puede estar ocupado.
		ofertas = c.conCorreoActual(cache)
		w.Header().Set("X-Vacantes-Fuente", "cache-reciente")
	} else {
		// El plazo crece con las cuentas: cada una abre su propia sesión y cuenta sus vacantes.
		plazo := timeoutVacantesComputrabajo * time.Duration(max(c.numCuentas, 1))
		ctx, cancel := context.WithTimeout(r.Context(), plazo)
		defer cancel()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(plazo + 10*time.Second))

		// ?contar=0 omite el conteo de inscritos (más rápido: no abre el listado de cada vacante).
		contar := r.URL.Query().Get("contar") != "0"
		var err error
		ofertas, err = c.consultarVacantesComputrabajo(ctx, contar)
		if err != nil {
			log.Printf("[!] Error consultando vacantes en Computrabajo: %v", err)
			codigo, mensaje := traducirErrorComputrabajo(err)
			if codigo == 0 {
				return // el cliente cerró la conexión
			}
			ResponderJSON(w, codigo, map[string]string{"error": mensaje})
			return
		}
	}

	// Cruce con lo procesado en local (por slug; si hay carpetas duplicadas gana la más reciente).
	locales := map[string]ResumenOferta{}
	if lista, err := c.repositorio.ListarOfertas(); err == nil {
		for _, o := range lista {
			if previa, ok := locales[o.Slug]; !ok || o.ActualizadoEn.After(previa.ActualizadoEn) {
				locales[o.Slug] = o
			}
		}
	}

	vacantes := make([]VacanteComputrabajo, 0, len(ofertas))
	for _, o := range ofertas {
		v := VacanteComputrabajo{
			ID:              o.ID,
			Slug:            SlugVacante(o.Titulo),
			Titulo:          o.Titulo,
			URLCandidatos:   o.URLCandidatos,
			TotalCandidatos: o.Total,
			ErrorConteo:     o.ErrorMsg,
			Excluida:        c.estaExcluida(o.Titulo),
			Origen:          "computrabajo",
		}
		if o.ID == agentes.IDOfertaCorreo {
			v.Origen = "correo"
		}
		if local, ok := locales[v.Slug]; ok && local.Error == "" {
			v.Procesada = true
		}
		vacantes = append(vacantes, v)
	}

	ResponderJSON(w, http.StatusOK, vacantes)
}

// PrecalentarSiConfigurado abre Chrome e inicia sesión en Computrabajo al arrancar el servidor
// cuando RRHH_PRECALENTAR_COMPUTRABAJO=true, para que la primera consulta no pague ese arranque
// (varios segundos). Por defecto está apagado: arrancar Chrome cierra el que use el mismo perfil.
func (c *ControladorRRHH) PrecalentarSiConfigurado() {
	if !strings.EqualFold(os.Getenv("RRHH_PRECALENTAR_COMPUTRABAJO"), "true") || !c.cfg.ValidarCredenciales() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(c.base, 2*time.Minute)
		defer cancel()
		select {
		case c.semCT <- struct{}{}:
			defer func() { <-c.semCT }()
		case <-ctx.Done():
			return
		}
		if err := c.fuenteComputrabajo(0).AsegurarAutenticacion(ctx); err != nil {
			log.Printf("[!] No se pudo precalentar Computrabajo: %v", err)
			c.descartarFuenteComputrabajo()
			return
		}
		log.Printf("[*] Computrabajo precalentado: sesión lista")
	}()
}
