package agentes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

// AgenteOfac valida identificaciones contra la lista de sanciones de la OFAC (EE. UU.).
type AgenteOfac struct {
	cfg         *configuracion.Configuracion
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
}

// NuevoAgenteOfac inicializa el agente OFAC.
func NuevoAgenteOfac(cfg *configuracion.Configuracion) *AgenteOfac {
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opcionesChrome(true)...)
	return &AgenteOfac{
		cfg:         cfg,
		allocCtx:    allocCtx,
		cancelAlloc: cancel,
	}
}

func (a *AgenteOfac) Nombre() string {
	return "OFAC"
}

// Cerrar libera los recursos del asignador de procesos Chrome.
func (a *AgenteOfac) Cerrar() {
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
}

// Validar implementa el contrato de AgenteValidador para enriquecer el modelo Candidato.
func (a *AgenteOfac) Validar(ctx context.Context, candidato *modelos.Candidato) error {
	cedulaLimpia := soloDigitos(candidato.Cedula)
	if cedulaLimpia == "" || candidato.Cedula == "No especificada" {
		candidato.ResultadoOfac = &modelos.ResultadoOfac{
			Coincidencia:       false,
			Alerta:             false,
			Estado:             "NO_CONSULTADO_SIN_CEDULA",
			TotalCoincidencias: 0,
			Registros:          []modelos.RegistroOfac{},
			Detalles:           "Cédula no especificada para consulta en OFAC.",
		}
		return nil
	}

	var captura []byte
	resultado, err := a.consultarCedula(ctx, cedulaLimpia, candidato.Nombre, &captura)
	if err != nil {
		candidato.ResultadoOfac = &modelos.ResultadoOfac{
			Coincidencia:       false,
			Alerta:             false,
			Estado:             "ERROR_CONSULTA",
			TotalCoincidencias: 0,
			Registros:          []modelos.RegistroOfac{},
			Detalles:           fmt.Sprintf("Error en consulta OFAC: %v", err),
		}
		return nil
	}

	candidato.ResultadoOfac = resultado
	GuardarCaptura(candidato, "ofac", captura)
	return nil
}

// scriptExtraccionOfac lee la tabla de resultados descartando la fila de encabezado.
const scriptExtraccionOfac = `
	(() => {
		const trs = Array.from(document.querySelectorAll('#gvSearchResults tr'));
		if (trs.length <= 1) return [];
		return trs.slice(1).map(tr => {
			const tds = Array.from(tr.querySelectorAll('td'));
			return {
				nombre: tds[0] ? tds[0].innerText.trim() : '',
				direccion: tds[1] ? tds[1].innerText.trim() : '',
				tipo: tds[2] ? tds[2].innerText.trim() : '',
				programa: tds[3] ? tds[3].innerText.trim() : '',
				lista: tds[4] ? tds[4].innerText.trim() : '',
				score: tds[5] ? tds[5].innerText.trim() : ''
			};
		}).filter(r => r.nombre !== '');
	})()
`

// scriptOfacTerminada es verdadera cuando el portal ya respondió: pintó la tabla de resultados,
// el contador "Lookup Results" (caso de 0 coincidencias) o recargó la página (la marca
// __ofacEnviado, puesta antes del clic, desaparece con el postback).
const scriptOfacTerminada = `
	(() => {
		if (document.querySelector('#gvSearchResults')) return true;
		const lbl = document.querySelector('#ctl00_MainContent_lblResults');
		if (lbl && (lbl.innerText || '').trim().length > 0) return true;
		return !window.__ofacEnviado;
	})()
`

// ConsultarCedula ejecuta la búsqueda web automatizada en el portal OFAC con reintentos.
func (a *AgenteOfac) ConsultarCedula(ctx context.Context, cedula string, nombre string) (*modelos.ResultadoOfac, error) {
	return a.consultarCedula(ctx, cedula, nombre, nil)
}

// consultarCedula es ConsultarCedula con salida opcional de la captura de pantalla del resultado.
func (a *AgenteOfac) consultarCedula(ctx context.Context, cedula string, nombre string, captura *[]byte) (*modelos.ResultadoOfac, error) {
	cedulaLimpia := soloDigitos(cedula)
	if cedulaLimpia == "" {
		return &modelos.ResultadoOfac{
			Coincidencia:       false,
			Alerta:             false,
			Estado:             "SIN_CEDULA",
			TotalCoincidencias: 0,
			Registros:          []modelos.RegistroOfac{},
		}, nil
	}

	var registros []modelos.RegistroOfac
	err := reintentar(ctx, 3, 2*time.Second, func(intento int) error {
		var err error
		if captura != nil {
			*captura = nil
		}
		registros, err = a.consultarUnaVez(ctx, cedulaLimpia, intento == 3, captura)
		return err
	})
	if err != nil {
		return nil, err
	}

	if len(registros) > 0 {
		a.emitirAlertaConsola(nombre, cedulaLimpia, registros)
		return &modelos.ResultadoOfac{
			Coincidencia:       true,
			Alerta:             true,
			Estado:             "ALERTA: COINCIDENCIA EN LISTA DE SANCIONES OFAC",
			TotalCoincidencias: len(registros),
			Registros:          registros,
			Detalles:           fmt.Sprintf("Se encontraron %d coincidencias en OFAC para cédula %s.", len(registros), cedulaLimpia),
		}, nil
	}

	return &modelos.ResultadoOfac{
		Coincidencia:       false,
		Alerta:             false,
		Estado:             "SIN COINCIDENCIAS EN LISTA OFAC",
		TotalCoincidencias: 0,
		Registros:          []modelos.RegistroOfac{},
		Detalles:           "Sin coincidencias en la lista de sanciones de la OFAC (EE. UU.)",
	}, nil
}

// consultarUnaVez ejecuta una búsqueda en una pestaña nueva. Si el portal no da ninguna señal de
// haber respondido devuelve un error reintentable; en el último intento (ultimo=true) lo
// interpreta como "sin resultados", igual que el comportamiento histórico.
func (a *AgenteOfac) consultarUnaVez(ctx context.Context, cedula string, ultimo bool, captura *[]byte) ([]modelos.RegistroOfac, error) {
	taskCtx, cancelTask := chromedp.NewContext(a.allocCtx, chromedp.WithErrorf(func(string, ...interface{}) {}))
	defer cancelTask()
	detener := context.AfterFunc(ctx, cancelTask)
	defer detener()

	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, 45*time.Second)
	defer cancelTimeout()

	scriptBuscar := fmt.Sprintf(`
		(() => {
			const ln = document.querySelector('#ctl00_MainContent_txtLastName');
			if (ln) ln.value = '';
			const id = document.querySelector('#ctl00_MainContent_txtID');
			if (id) {
				id.value = %s;
				id.dispatchEvent(new Event('input', { bubbles: true }));
				id.dispatchEvent(new Event('change', { bubbles: true }));
			}
			const btn = document.querySelector('#ctl00_MainContent_btnSearch');
			window.__ofacEnviado = true;
			if (btn) btn.click();
		})()
	`, literalJS(cedula))

	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(a.cfg.URLOfac),
		chromedp.WaitVisible("#ctl00_MainContent_txtID", chromedp.ByID),
		chromedp.Evaluate(scriptBuscar, nil),
	); err != nil {
		return nil, fmt.Errorf("error navegando portal OFAC: %w", err)
	}

	respondio := esperarCondicion(taskCtx, scriptOfacTerminada, 15*time.Second, 250*time.Millisecond)
	if !respondio && !ultimo {
		return nil, fmt.Errorf("el portal OFAC no devolvió resultados a tiempo")
	}

	var registros []modelos.RegistroOfac
	if err := chromedp.Run(taskCtx, chromedp.Evaluate(scriptExtraccionOfac, &registros)); err != nil {
		return nil, fmt.Errorf("error leyendo resultados OFAC: %w", err)
	}
	if captura != nil && respondio {
		*captura = capturarPagina(taskCtx)
	}
	return registros, nil
}

func (a *AgenteOfac) emitirAlertaConsola(nombre string, cedula string, registros []modelos.RegistroOfac) {
	fmt.Println("\n" + strings.Repeat("!", 65))
	fmt.Println("  [!] ALERTA CRÍTICA OFAC (LISTA CLINTON / SDN)")
	fmt.Printf("  Candidato: %s | Cédula: %s\n", nombre, cedula)
	fmt.Printf("  Total registros sancionados encontrados: %d\n", len(registros))
	for _, r := range registros {
		fmt.Printf("   -> %s | Tipo: %s | Programa: %s | Lista: %s\n", r.Nombre, r.Tipo, r.Programa, r.Lista)
	}
	fmt.Println(strings.Repeat("!", 65) + "\n")
}
