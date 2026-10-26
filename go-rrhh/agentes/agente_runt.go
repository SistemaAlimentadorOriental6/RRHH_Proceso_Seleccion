package agentes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/ocr"
)

const (
	runtOrigen     = "https://portalpublico.runt.gov.co"
	maxBytesRespRT = 4 << 20
)

// AgenteRunt consulta y valida información de ciudadanos, licencias y multas en el RUNT.
type AgenteRunt struct {
	cfg          *configuracion.Configuracion
	solucionador *ocr.SolucionadorCaptcha
	cliente      *http.Client
	maxIntentos  int
}

// NuevoAgenteRunt inicializa el agente RUNT con su solucionador OCR.
func NuevoAgenteRunt(cfg *configuracion.Configuracion) *AgenteRunt {
	return &AgenteRunt{
		cfg:          cfg,
		solucionador: ocr.NuevoSolucionador(""),
		cliente:      nuevoClienteHTTP(18 * time.Second),
		maxIntentos:  30, // el OCR del captcha acierta solo en una parte de los intentos
	}
}

func (a *AgenteRunt) Nombre() string {
	return "RUNT"
}

// NormalizarApellido aplica las reglas oficiales del RUNT para el primer apellido.
func (a *AgenteRunt) NormalizarApellido(primerApellido string) string {
	if primerApellido == "" {
		return ""
	}
	apellido := strings.ToUpper(strings.TrimSpace(primerApellido))
	prohibidos := []string{"'", "-", "/", "*", ".", ",", ";", "|", "&", "#", "!", "?", `"`, "_", "\\", "(", ")"}
	for _, p := range prohibidos {
		apellido = strings.ReplaceAll(apellido, p, "")
	}
	return strings.TrimSpace(apellido)
}

// DeducirApellidos extrae los candidatos más probables a primer apellido.
func (a *AgenteRunt) DeducirApellidos(nombreCompleto string, apellidosRef string) []string {
	var lista []string
	agregar := func(ap string) {
		norm := a.NormalizarApellido(ap)
		if norm == "" {
			return
		}
		for _, e := range lista {
			if e == norm {
				return
			}
		}
		lista = append(lista, norm)
	}

	agregarConVariantes := func(ap string) {
		agregar(ap)
		if strings.ContainsAny(ap, "ñÑ") {
			agregar(strings.ReplaceAll(strings.ReplaceAll(ap, "ñ", "n"), "Ñ", "N"))
		}
		if strings.Contains(strings.ToLower(ap), "montan") {
			agregar("MONTAÑA")
		}
	}

	// 1. Apellido proveniente de ADRES
	if apellidosRef != "" {
		for _, p := range strings.Fields(apellidosRef) {
			agregarConVariantes(p)
		}
	}

	// 2. Deducir desde nombre completo
	palabras := strings.Fields(nombreCompleto)
	switch len(palabras) {
	case 2:
		agregarConVariantes(palabras[1])
		agregarConVariantes(palabras[0])
	case 3:
		// En formato Nombre ApellidoPaterno ApellidoMaterno, priorizar paterno
		agregarConVariantes(palabras[1])
		agregarConVariantes(palabras[2])
		agregarConVariantes(palabras[0])
	default:
		if len(palabras) >= 4 {
			agregarConVariantes(palabras[len(palabras)-2])
			agregarConVariantes(palabras[len(palabras)-1])
			agregarConVariantes(palabras[1])
			agregarConVariantes(palabras[0])
		} else if len(palabras) == 1 {
			agregarConVariantes(palabras[0])
		}
	}

	return lista
}

// Validar ejecuta la consulta del candidato en el RUNT con deducción inteligente de apellido,
// usando el apellido que ADRES ya reportó para esa cédula cuando está disponible.
func (a *AgenteRunt) Validar(ctx context.Context, c *modelos.Candidato) error {
	apellidosRef := ""
	if c.ResultadoAdres != nil && c.ResultadoAdres.Documento == soloDigitos(c.Cedula) {
		apellidosRef = c.ResultadoAdres.Apellidos
	}
	return a.ValidarConApellidos(ctx, c, apellidosRef)
}

// ValidarConApellidos ejecuta la consulta del candidato en el RUNT con apellidos de referencia explícitos.
func (a *AgenteRunt) ValidarConApellidos(ctx context.Context, c *modelos.Candidato, apellidosRef string) error {
	cedulaLimpia := soloDigitos(c.Cedula)

	if len(cedulaLimpia) < 6 {
		c.ResultadoRunt = &modelos.ResultadoRunt{
			Estado: "NO_CONSULTADO_SIN_CEDULA",
			Error:  "Cédula no especificada para consulta en RUNT",
		}
		return nil
	}

	candidatosApellidos := a.DeducirApellidos(c.Nombre, apellidosRef)
	if len(candidatosApellidos) == 0 {
		c.ResultadoRunt = &modelos.ResultadoRunt{
			Estado: "SIN_APELLIDO",
			Alerta: true,
			Error:  "No se pudo deducir el apellido del candidato",
		}
		return nil
	}

	var resFinal *modelos.ResultadoRunt
	for _, ap := range candidatosApellidos {
		res, err := a.consultarIndividual(ctx, cedulaLimpia, ap)
		if err == nil && res != nil && (res.Estado == "INSCRITO_ACTIVO" || res.Estado == "NO_INSCRITO") {
			c.ResultadoRunt = res
			capturarEvidenciaRunt(ctx, c)
			return nil
		}
		if res != nil && resFinal == nil {
			resFinal = res
		}
		// Solo se prueba el siguiente apellido cuando el RUNT dijo que este no coincide.
		if res == nil || res.Estado != "APELLIDO_NO_COINCIDE" {
			break
		}
	}

	if resFinal != nil {
		c.ResultadoRunt = resFinal
	} else {
		c.ResultadoRunt = &modelos.ResultadoRunt{
			Estado: "ERROR_DESCONOCIDO",
			Alerta: true,
			Error:  "No se pudo completar la validación en RUNT",
		}
	}
	return nil
}

// ConsultarCedulaConApellido ejecuta la consulta de una cédula con su primer apellido conocido.
func (a *AgenteRunt) ConsultarCedulaConApellido(ctx context.Context, cedula string, apellido string) (*modelos.ResultadoRunt, error) {
	return a.consultarIndividual(ctx, soloDigitos(cedula), a.NormalizarApellido(apellido))
}

// solicitar construye y envía una petición al API del RUNT con las cabeceras esperadas.
func (a *AgenteRunt) solicitar(ctx context.Context, metodo, ruta, token string, cuerpo []byte) (*http.Response, error) {
	var lector io.Reader
	if cuerpo != nil {
		lector = bytes.NewReader(cuerpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, strings.TrimRight(a.cfg.URLRuntAPI, "/")+ruta, lector)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Origin", runtOrigen)
	req.Header.Set("Referer", runtOrigen+"/")
	req.Header.Set("User-Agent", userAgentChrome)
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Auth-Token", "Bearer "+token)
	}
	return a.cliente.Do(req)
}

// obtenerJSON hace GET autenticado con reintentos ante errores transitorios (red, 429, 5xx).
func (a *AgenteRunt) obtenerJSON(ctx context.Context, ruta, token string, destino any) error {
	return reintentar(ctx, 3, 400*time.Millisecond, func(int) error {
		resp, err := a.solicitar(ctx, http.MethodGet, ruta, token, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return fmt.Errorf("RUNT %s respondió HTTP %d", ruta, resp.StatusCode)
		case resp.StatusCode != http.StatusOK:
			return permanente(fmt.Errorf("RUNT %s respondió HTTP %d", ruta, resp.StatusCode))
		}
		return json.NewDecoder(io.LimitReader(resp.Body, maxBytesRespRT)).Decode(destino)
	})
}

// cadena convierte un valor dinámico del JSON en texto, devolviendo "" para nil.
func cadena(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// soloFecha recorta la parte horaria de una fecha ISO ("2023-12-13T00:00:00" -> "2023-12-13").
func soloFecha(f string) string {
	if i := strings.Index(f, "T"); i >= 0 {
		return f[:i]
	}
	return f
}

// parsearLicencias convierte la respuesta cruda del RUNT en licencias y categorías activas.
func parsearLicencias(raw []map[string]any) ([]modelos.LicenciaRunt, []string) {
	var licencias []modelos.LicenciaRunt
	var categoriasActivas []string

	for _, licRaw := range raw {
		estadoLic := cadena(licRaw["estadoLicencia"])
		if estadoLic == "" {
			estadoLic = cadena(licRaw["estado"])
		}

		var catsLic []string
		var detsLic []modelos.DetalleCategoria

		if subDets, ok := licRaw["detalleLicencia"].([]any); ok {
			for _, sd := range subDets {
				sdMap, okMap := sd.(map[string]any)
				if !okMap {
					continue
				}
				catNom := cadena(sdMap["categoria"])
				fVenc := cadena(sdMap["fechaVencimientoLimite"])
				if fVenc == "" {
					fVenc = cadena(sdMap["fechaVencimiento"])
				}
				fVenc = soloFecha(fVenc)

				catsLic = append(catsLic, catNom)
				detsLic = append(detsLic, modelos.DetalleCategoria{
					Categoria:        catNom,
					FechaVencimiento: fVenc,
				})

				if estadoLic == "ACTIVA" {
					textoCat := catNom
					if fVenc != "" {
						textoCat = fmt.Sprintf("%s (%s)", catNom, fVenc)
					}
					categoriasActivas = append(categoriasActivas, textoCat)
				}
			}
		}

		licencias = append(licencias, modelos.LicenciaRunt{
			NumeroLicencia:     cadena(licRaw["numeroLicencia"]),
			Categorias:         catsLic,
			DetallesCategorias: detsLic,
			FechaExpedicion:    soloFecha(cadena(licRaw["fechaExpedicion"])),
			Estado:             estadoLic,
			Retenida:           estadoLic == "RETENIDA",
			OTExpide:           cadena(licRaw["otExpide"]),
			Restricciones:      cadena(licRaw["restricciones"]),
		})
	}
	return licencias, categoriasActivas
}

// autenticar resuelve el captcha y autentica con cédula+apellido. Devuelve la respuesta de /auth.
// Los intentos con captcha inválido o fallos de red se repiten hasta maxIntentos.
func (a *AgenteRunt) autenticar(ctx context.Context, cedula, apellido string) (map[string]any, error) {
	var ultimoErr error
	for intento := 1; intento <= a.maxIntentos; intento++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if intento > 1 && !dormir(ctx, min(time.Duration(intento-1)*150*time.Millisecond, 400*time.Millisecond)) {
			return nil, ctx.Err()
		}

		// 1. Generar LibreCaptcha
		respCap, err := a.solicitar(ctx, http.MethodGet, "/captcha/libre-captcha/generar", "", nil)
		if err != nil {
			ultimoErr = err
			continue
		}
		var datosCap struct {
			ID     string `json:"id"`
			Imagen string `json:"imagen"`
		}
		okCap := respCap.StatusCode == http.StatusOK &&
			json.NewDecoder(io.LimitReader(respCap.Body, maxBytesRespRT)).Decode(&datosCap) == nil
		_ = respCap.Body.Close()
		if !okCap || datosCap.ID == "" || datosCap.Imagen == "" {
			ultimoErr = fmt.Errorf("el RUNT no entregó un captcha válido")
			continue
		}

		// 2. Resolver Captcha
		textoCaptcha, err := a.solucionador.Resolver(datosCap.Imagen)
		if err != nil || len(textoCaptcha) != 5 {
			ultimoErr = fmt.Errorf("OCR no logró leer el captcha")
			continue
		}

		// 3. Autenticación en /auth
		cuerpoJSON, _ := json.Marshal(map[string]any{
			"tipoConsulta":          "1",
			"tipoDocumento":         "C",
			"documento":             cedula,
			"noDocumento":           cedula,
			"primerApellido":        apellido,
			"primerApelido":         apellido,
			"captcha":               textoCaptcha,
			"idLibreCaptcha":        datosCap.ID,
			"valueCaptchaEncripted": "",
			"reCaptcha":             nil,
		})

		respAuth, err := a.solicitar(ctx, http.MethodPost, "/auth", "", cuerpoJSON)
		if err != nil {
			ultimoErr = err
			continue
		}
		var datosAuth map[string]any
		errDec := json.NewDecoder(io.LimitReader(respAuth.Body, maxBytesRespRT)).Decode(&datosAuth)
		codigo := respAuth.StatusCode
		_ = respAuth.Body.Close()

		if codigo >= 500 || codigo == http.StatusTooManyRequests {
			ultimoErr = fmt.Errorf("RUNT /auth respondió HTTP %d", codigo)
			continue
		}
		if errDec != nil || datosAuth == nil {
			ultimoErr = fmt.Errorf("respuesta de /auth ilegible")
			continue
		}

		desc := strings.ToLower(cadena(datosAuth["descripcionRespuesta"]))
		if strings.Contains(desc, "captcha no es valido") || strings.Contains(desc, "error en el captcha") {
			ultimoErr = fmt.Errorf("captcha rechazado por el RUNT (leído %q; respuesta: %s)", textoCaptcha, cadena(datosAuth["descripcionRespuesta"]))
			continue
		}
		return datosAuth, nil
	}

	if ultimoErr == nil {
		ultimoErr = fmt.Errorf("intentos agotados")
	}
	return nil, ultimoErr
}

func (a *AgenteRunt) consultarIndividual(ctx context.Context, cedula string, apellido string) (*modelos.ResultadoRunt, error) {
	fmt.Printf("[*] Consultando RUNT para documento %s (%s)...\n", cedula, apellido)

	// Tope total de la autenticación: muchos intentos de captcha cortos, pero no indefinidos.
	ctxAuth, cancelAuth := context.WithTimeout(ctx, 90*time.Second)
	defer cancelAuth()
	datosAuth, err := a.autenticar(ctxAuth, cedula, apellido)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return &modelos.ResultadoRunt{
			Estado:             "ERROR_INTENTOS",
			ApellidoConsultado: apellido,
			Error:              fmt.Sprintf("No se pudo autenticar en el RUNT: %v", err),
			Alerta:             true,
		}, nil
	}

	// Verificar si no está inscrito
	noInscrito, _ := datosAuth["noInscrito"].(bool)
	estadoPersona, _ := datosAuth["estadoPersona"].(string)
	if noInscrito || estadoPersona == "NO INSCRITO" {
		return &modelos.ResultadoRunt{
			Estado:             "NO_INSCRITO",
			ApellidoConsultado: apellido,
			Documento:          cedula,
			EstadoPersona:      "NO INSCRITO",
			PazYSalvo:          true,
		}, nil
	}

	token, _ := datosAuth["token"].(string)
	idPersona := cadena(datosAuth["idPersona"])
	if token == "" || idPersona == "" {
		return &modelos.ResultadoRunt{
			Estado:             "APELLIDO_NO_COINCIDE",
			ApellidoConsultado: apellido,
			Alerta:             true,
		}, nil
	}

	nombres, _ := datosAuth["nombres"].(string)
	apellidos, _ := datosAuth["apellidos"].(string)
	fechaInscripcion, _ := datosAuth["fechaInscripcion"].(string)
	estadoConductor, _ := datosAuth["estadoConductor"].(string)
	esConductorActivo := estadoConductor == "ACTIVO"
	tieneLicencias, _ := datosAuth["tieneLicencias"].(bool)

	// Licencias y multas son independientes: se consultan en paralelo con el mismo token.
	var (
		wg                sync.WaitGroup
		licencias         []modelos.LicenciaRunt
		categoriasActivas []string
		errLic            error
		mulRaw            map[string]any
		errMul            error
	)

	if tieneLicencias {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var raw []map[string]any
			if errLic = a.obtenerJSON(ctx, "/consulta-ciudadano/licencias", token, &raw); errLic == nil {
				licencias, categoriasActivas = parsearLicencias(raw)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		errMul = a.obtenerJSON(ctx, "/consulta-ciudadano/multas", token, &mulRaw)
	}()
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Sin licencias no se puede decidir la idoneidad: antes esto se reportaba como INSCRITO_ACTIVO
	// con 0 licencias y la persona quedaba rechazada por "no tiene C2/C3" aunque sí la tuviera.
	if tieneLicencias && errLic != nil {
		return &modelos.ResultadoRunt{
			Estado:             "ERROR_LICENCIAS",
			Inscrito:           true,
			ApellidoConsultado: apellido,
			IDPersona:          idPersona,
			Nombres:            nombres,
			Apellidos:          apellidos,
			Documento:          cedula,
			FechaInscripcion:   fechaInscripcion,
			EstadoPersona:      estadoPersona,
			ConductorActivo:    esConductorActivo,
			EstadoConductor:    estadoConductor,
			TieneLicencias:     true,
			Error:              fmt.Sprintf("No se pudo leer el listado de licencias: %v", errLic),
			Alerta:             true,
		}, nil
	}

	tieneMultas := false
	pazYSalvo := true
	numPazSalvo := ""
	var avisoMultas string
	if errMul == nil && mulRaw != nil {
		tieneMultas = strings.EqualFold(cadena(mulRaw["tieneMultas"]), "SI")
		numPazSalvo = cadena(mulRaw["nroPazYSalvo"])
		pazYSalvo = !tieneMultas
	} else if errMul != nil {
		avisoMultas = fmt.Sprintf("No se pudo consultar multas en RUNT: %v", errMul)
	}

	fmt.Printf("    [+] RUNT: %s | Conductor: %s | Licencias: %d | Multas: %v\n", estadoPersona, estadoConductor, len(licencias), tieneMultas)

	return &modelos.ResultadoRunt{
		Estado:             "INSCRITO_ACTIVO",
		Inscrito:           true,
		ApellidoConsultado: apellido,
		IDPersona:          idPersona,
		Nombres:            nombres,
		Apellidos:          apellidos,
		Documento:          cedula,
		FechaInscripcion:   fechaInscripcion,
		EstadoPersona:      estadoPersona,
		ConductorActivo:    esConductorActivo,
		EstadoConductor:    estadoConductor,
		TieneLicencias:     tieneLicencias,
		Licencias:          licencias,
		CategoriasActivas:  categoriasActivas,
		TieneMultas:        tieneMultas,
		PazYSalvo:          pazYSalvo,
		NumeroPazYSalvo:    numPazSalvo,
		Error:              avisoMultas,
		Alerta:             tieneMultas || (!esConductorActivo && tieneLicencias),
	}, nil
}
