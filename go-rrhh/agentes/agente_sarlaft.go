package agentes

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Expresiones del parseo de la lista consolidada. Se compilan una sola vez: antes algunas se
// recompilaban dentro del bucle por cada fila de la lista (miles de veces).
var (
	reSarlaftNoAlfanum = regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	reSarlaftFila      = regexp.MustCompile(`(?s)<tr class="rowtext">.*?</tr>`)
	reSarlaftID        = regexp.MustCompile(`<strong>([A-Za-z0-9\.\_]+)&nbsp;</strong>`)
	reSarlaftNombre    = regexp.MustCompile(`(?s)<strong>Nombre:\s*</strong>(.*?)(?:<br>|<span>|$)`)
	reSarlaftAliasB    = regexp.MustCompile(`(?s)<strong>Alias de buena calidad:\s*</strong>(.*?)(?:<strong>|$)`)
	reSarlaftDoc       = regexp.MustCompile(`(?s)<strong>N[úu]mero nacional de identidad:\s*</strong>(.*?)(?:<strong>|$)`)
	reSarlaftTags      = regexp.MustCompile(`<[^>]+>`)
	reSarlaftPartes    = regexp.MustCompile(`[1-4]:\s*([^1-4<]+)`)
	reSarlaftItems     = regexp.MustCompile(`\s+[a-z]\)\s*|\;`)
)

type registroSarlaft struct {
	ID                string
	Nombre            string
	NombreNorm        string
	TokensNombre      map[string]struct{}
	AliasesNorm       []string
	DocumentosDigitos []string
}

// AgenteSarlaft valida personas contra la Lista Consolidada de Sanciones ONU en memoria.
type AgenteSarlaft struct {
	cfg       *configuracion.Configuracion
	registros []registroSarlaft
	mu        sync.RWMutex
	cargado   bool
	errCarga  error
	// ultimoIntento evita reintentar la descarga en cada candidato cuando la lista no está disponible.
	ultimoIntento time.Time
}

// NuevoAgenteSarlaft inicializa y pre-carga la lista en memoria.
func NuevoAgenteSarlaft(cfg *configuracion.Configuracion) *AgenteSarlaft {
	ag := &AgenteSarlaft{cfg: cfg}
	if err := ag.cargarLista(); err != nil {
		fmt.Printf("[!] No se pudo cargar la lista SARLAFT ONU: %v\n", err)
	}
	return ag
}

func (a *AgenteSarlaft) Nombre() string {
	return "SARLAFT_ONU"
}

func (a *AgenteSarlaft) normalizar(texto string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	res, _, _ := transform.String(t, texto)
	limpio := reSarlaftNoAlfanum.ReplaceAllString(res, " ")
	return strings.ToLower(strings.TrimSpace(reEspacios.ReplaceAllString(limpio, " ")))
}

// descargarLista baja la lista consolidada y la guarda de forma atómica. Valida el TLS por defecto:
// una lista de sanciones alterada en tránsito ocultaría coincidencias. Si el entorno realmente lo
// necesita, SARLAFT_TLS_INSECURE=true desactiva la verificación (con aviso).
func (a *AgenteSarlaft) descargarLista(ruta string) error {
	cliente := nuevoClienteHTTP(60 * time.Second)
	if strings.EqualFold(os.Getenv("SARLAFT_TLS_INSECURE"), "true") {
		fmt.Println("[!] SARLAFT_TLS_INSECURE=true: se descargará la lista ONU SIN verificar el certificado TLS.")
		if tr, ok := cliente.Transport.(*http.Transport); ok {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in explícito del usuario
		}
	}

	return reintentar(context.Background(), 3, 2*time.Second, func(int) error {
		req, err := http.NewRequest(http.MethodGet, a.cfg.URLSarlaftONU, nil)
		if err != nil {
			return permanente(err)
		}
		req.Header.Set("User-Agent", userAgentChrome)

		resp, err := cliente.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("la ONU respondió HTTP %d", resp.StatusCode)
		}
		cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		if err != nil {
			return err
		}
		// No pisar una lista buena con una página de error o vacía.
		if !strings.Contains(string(cuerpo), `class="rowtext"`) {
			return fmt.Errorf("la respuesta de la ONU no parece la lista consolidada (%d bytes)", len(cuerpo))
		}
		if err := utilidades.EscribirArchivoAtomico(ruta, cuerpo, 0o644); err != nil {
			return permanente(err)
		}
		fmt.Printf("[+] Lista SARLAFT ONU actualizada (%d KB).\n", len(cuerpo)/1024)
		return nil
	})
}

func (a *AgenteSarlaft) cargarLista() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cargado && len(a.registros) > 0 {
		return nil
	}
	if a.errCarga != nil && time.Since(a.ultimoIntento) < 5*time.Minute {
		return a.errCarga
	}
	a.ultimoIntento = time.Now()

	ruta := a.cfg.ArchivoSarlaftONU
	info, err := os.Stat(ruta)
	if err != nil || time.Since(info.ModTime()) > 24*time.Hour {
		fmt.Println("[*] Descargando Lista Consolidada de Sanciones ONU (SARLAFT)...")
		if errDesc := a.descargarLista(ruta); errDesc != nil {
			// Con una copia previa (aunque vieja) es preferible usarla a quedarse sin validación.
			if err == nil {
				fmt.Printf("[!] No se pudo actualizar la lista ONU (%v); se usa la copia local del %s.\n",
					errDesc, info.ModTime().Format("2006-01-02"))
			} else {
				a.errCarga = fmt.Errorf("no se pudo descargar la lista ONU y no hay copia local: %w "+
					"(si el error es de certificado TLS, defina SARLAFT_TLS_INSECURE=true)", errDesc)
				return a.errCarga
			}
		}
	}

	cuerpo, err := os.ReadFile(ruta)
	if err != nil {
		a.errCarga = err
		return err
	}

	a.registros = a.parsearLista(string(cuerpo))
	if len(a.registros) == 0 {
		a.errCarga = fmt.Errorf("la lista ONU no contiene registros reconocibles")
		return a.errCarga
	}
	a.cargado = true
	a.errCarga = nil
	fmt.Printf("[+] Total sancionados cargados en memoria SARLAFT: %d\n", len(a.registros))
	return nil
}

// parsearLista extrae los sancionados del HTML de la lista consolidada.
func (a *AgenteSarlaft) parsearLista(html string) []registroSarlaft {
	filas := reSarlaftFila.FindAllString(html, -1)
	registros := make([]registroSarlaft, 0, len(filas))

	for _, f := range filas {
		idONU := "DESCONOCIDO"
		if m := reSarlaftID.FindStringSubmatch(f); len(m) > 1 {
			idONU = m[1]
		}

		nombre := ""
		if m := reSarlaftNombre.FindStringSubmatch(f); len(m) > 1 {
			nomRaw := m[1]
			if partes := reSarlaftPartes.FindAllStringSubmatch(nomRaw, -1); len(partes) > 0 {
				var tokens []string
				for _, p := range partes {
					v := strings.TrimSpace(p[1])
					if !strings.EqualFold(v, "nd") && !strings.EqualFold(v, "na") && v != "" {
						tokens = append(tokens, v)
					}
				}
				nombre = strings.Join(tokens, " ")
			} else {
				nombre = strings.TrimSpace(reSarlaftTags.ReplaceAllString(nomRaw, ""))
			}
		}
		if nombre == "" {
			continue
		}

		var aliasesNorm []string
		if m := reSarlaftAliasB.FindStringSubmatch(f); len(m) > 1 {
			txt := reSarlaftTags.ReplaceAllString(m[1], " ")
			for _, item := range reSarlaftItems.Split(txt, -1) {
				it := strings.TrimSpace(strings.ReplaceAll(item, "&nbsp;", " "))
				if it != "" && !strings.EqualFold(it, "nd") && !strings.EqualFold(it, "na") {
					aliasesNorm = append(aliasesNorm, a.normalizar(it))
				}
			}
		}

		var docsDigitos []string
		if m := reSarlaftDoc.FindStringSubmatch(f); len(m) > 1 {
			dig := soloDigitos(strings.TrimSpace(reSarlaftTags.ReplaceAllString(m[1], " ")))
			if len(dig) >= 6 {
				docsDigitos = append(docsDigitos, dig)
			}
		}

		nomNorm := a.normalizar(nombre)
		tokensMap := make(map[string]struct{})
		for _, tok := range strings.Fields(nomNorm) {
			tokensMap[tok] = struct{}{}
		}

		registros = append(registros, registroSarlaft{
			ID:                idONU,
			Nombre:            nombre,
			NombreNorm:        nomNorm,
			TokensNombre:      tokensMap,
			AliasesNorm:       aliasesNorm,
			DocumentosDigitos: docsDigitos,
		})
	}
	return registros
}

// Validar ejecuta la verificacion del candidato contra la lista ONU en memoria.
func (a *AgenteSarlaft) Validar(ctx context.Context, c *modelos.Candidato) error {
	if err := a.cruzar(c); err != nil {
		return err
	}
	capturarEvidenciaSarlaft(ctx, c)
	return nil
}

// cruzar compara al candidato contra la lista y deja el resultado en c.ResultadoSarlaft.
func (a *AgenteSarlaft) cruzar(c *modelos.Candidato) error {
	ahora := time.Now().Format("2006-01-02 15:04:05")

	if err := a.cargarLista(); err != nil {
		// Sin lista no se puede afirmar que no haya coincidencias: se deja constancia en vez de
		// reportar "sin coincidencias".
		c.ResultadoSarlaft = &modelos.ResultadoSarlaft{
			Alerta:            true,
			TipoCoincidencia:  "LISTA_NO_DISPONIBLE",
			Detalles:          fmt.Sprintf("No se pudo verificar contra la lista ONU: %v", err),
			FechaVerificacion: ahora,
		}
		return nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	candNorm := a.normalizar(c.Nombre)
	candTokensMap := make(map[string]struct{})
	for _, t := range strings.Fields(candNorm) {
		candTokensMap[t] = struct{}{}
	}
	candCedula := soloDigitos(c.Cedula)

	coincidencia := func(tipo string, reg registroSarlaft, detalle string) {
		c.ResultadoSarlaft = &modelos.ResultadoSarlaft{
			Coincidencia:      true,
			Alerta:            true,
			TipoCoincidencia:  tipo,
			IDONU:             reg.ID,
			NombreSancionado:  reg.Nombre,
			Detalles:          detalle,
			FechaVerificacion: ahora,
		}
	}

	for _, reg := range a.registros {
		// 1. Coincidencia por documento de identidad
		if len(candCedula) >= 6 {
			for _, doc := range reg.DocumentosDigitos {
				if candCedula == doc {
					coincidencia("DOCUMENTO_IDENTIDAD", reg,
						fmt.Sprintf("Cédula %s coincide con documento registrado en ONU (%s)", candCedula, reg.ID))
					return nil
				}
			}
		}

		// 2. Coincidencia exacta de nombre
		if candNorm != "" && candNorm == reg.NombreNorm {
			coincidencia("NOMBRE_EXACTO", reg,
				fmt.Sprintf("Nombre completo coincide exactamente con sancionado ONU (%s)", reg.ID))
			return nil
		}

		// 3. Coincidencia con alias
		for _, aliasNorm := range reg.AliasesNorm {
			if candNorm != "" && candNorm == aliasNorm {
				coincidencia("ALIAS_REGISTRADO", reg,
					fmt.Sprintf("Nombre coincide con alias en registro ONU (%s)", reg.ID))
				return nil
			}
		}

		// 4. Inclusión de tokens si el sancionado tiene 2 o más palabras
		if len(reg.TokensNombre) >= 2 {
			todosEstan := true
			for tok := range reg.TokensNombre {
				if _, ok := candTokensMap[tok]; !ok {
					todosEstan = false
					break
				}
			}
			if todosEstan {
				coincidencia("SIMILITUD_NOMBRE", reg,
					fmt.Sprintf("Todos los nombres de '%s' (%s) están presentes en el candidato", reg.Nombre, reg.ID))
				return nil
			}
		}
	}

	c.ResultadoSarlaft = &modelos.ResultadoSarlaft{
		Detalles:          "Sin coincidencias en la Lista Consolidada de Sanciones ONU (SARLAFT)",
		FechaVerificacion: ahora,
	}
	return nil
}

// ValidarNombre consulta directamente un nombre contra la lista ONU en memoria.
func (a *AgenteSarlaft) ValidarNombre(nombre string) *modelos.ResultadoSarlaft {
	c := &modelos.Candidato{Nombre: nombre}
	_ = a.Validar(context.Background(), c)
	return c.ResultadoSarlaft
}
