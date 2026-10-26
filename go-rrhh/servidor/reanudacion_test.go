package servidor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
)

// trabajosFalsos hace de base de datos para el trabajador de segundo plano.
type trabajosFalsos struct {
	mu             sync.Mutex
	pendiente      *almacen.TrabajoReanudacion
	oferta         modelos.OfertaLaboral
	candidato      *modelos.Candidato
	errCargar      error
	terminados     []error // fallo con el que se cerró cada trabajo (nil = bien)
	cerrados       int
	reclamos       int
	vencimientos   int
	recuperaciones int
}

func (t *trabajosFalsos) ReclamarReanudacion(context.Context, string, time.Duration) (*almacen.TrabajoReanudacion, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reclamos++
	trabajo := t.pendiente
	t.pendiente = nil
	return trabajo, nil
}

func (t *trabajosFalsos) TerminarReanudacion(_ context.Context, _ *almacen.TrabajoReanudacion, fallo error) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.terminados = append(t.terminados, fallo)
	t.cerrados++
	return nil
}

func (t *trabajosFalsos) RecuperarReanudaciones(context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recuperaciones++
	return nil
}

func (t *trabajosFalsos) VencerSolicitudes(context.Context) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.vencimientos++
	return 0, nil
}

func (t *trabajosFalsos) CandidatoPorPostulacion(context.Context, string) (modelos.OfertaLaboral, *modelos.Candidato, error) {
	return t.oferta, t.candidato, t.errCargar
}

func montarTrabajador(t *testing.T, consultas *consultasContadas) (*ControladorRRHH, *trabajosFalsos, *persistenciaConPausas) {
	t.Helper()
	ctrl, _ := montarControladorAnalisis(t, &fuenteAnalisisFalsa{}, consultas.validadores)
	p := nuevaPersistenciaConPausas()
	ctrl.ConPersistencia(p)
	trabajos := &trabajosFalsos{
		pendiente: &almacen.TrabajoReanudacion{ID: []byte("id"), PostulacionID: postulacionPrueba, Intentos: 1},
		oferta:    modelos.OfertaLaboral{ID: idOfertaPrueba, Titulo: "Conductor de pruebas SSE"},
		candidato: candidatoQueEntregoSusDatos(),
	}
	ctrl.trabajos = trabajos
	return ctrl, trabajos, p
}

func TestElTrabajadorReanudaAlCandidatoQueEntregoSusDatos(t *testing.T) {
	consultas := nuevasConsultas("CARLOS", "PEREZ")
	ctrl, trabajos, p := montarTrabajador(t, consultas)

	if !ctrl.reanudarUna("prueba") {
		t.Fatal("debía haber procesado la reanudación pendiente")
	}
	if len(trabajos.terminados) != 1 || trabajos.terminados[0] != nil {
		t.Fatalf("el trabajo debía cerrarse sin error: %v", trabajos.terminados)
	}
	if consultas.adres.Load() != 1 || consultas.runt.Load() != 1 || consultas.simit.Load() != 1 {
		t.Errorf("consultas: adres=%d runt=%d simit=%d", consultas.adres.Load(), consultas.runt.Load(), consultas.simit.Load())
	}
	if guardado := p.guardado("Carlos Perez"); guardado == nil || guardado.EstadoProceso != "" || !guardado.EsApto {
		t.Errorf("resultado guardado: %+v", guardado)
	}
	if len(ctrl.semCT) != 0 {
		t.Error("el trabajador debe liberar el navegador al terminar")
	}
	// No queda nada más: el siguiente intento no encuentra trabajo.
	if ctrl.reanudarUna("prueba") {
		t.Error("sin trabajos pendientes debe devolver falso")
	}
}

// Las reanudaciones y los análisis de vacantes comparten navegadores: si hay uno en curso se espera.
func TestElTrabajadorEsperaSiHayUnAnalisisEnCurso(t *testing.T) {
	consultas := nuevasConsultas("CARLOS", "PEREZ")
	ctrl, trabajos, _ := montarTrabajador(t, consultas)

	ctrl.semCT <- struct{}{} // un análisis de vacante tiene el navegador
	if ctrl.reanudarUna("prueba") {
		t.Error("con el navegador ocupado no debe procesar")
	}
	if trabajos.reclamos != 0 {
		t.Errorf("tampoco debe reclamar el trabajo (reclamos=%d): lo tomaría y lo dejaría varado", trabajos.reclamos)
	}
	<-ctrl.semCT
	if !ctrl.reanudarUna("prueba") {
		t.Error("liberado el navegador debe procesar")
	}
}

func TestUnaReanudacionFallidaSeCierraConElError(t *testing.T) {
	consultas := nuevasConsultas("CARLOS", "PEREZ")

	// No se pudo cargar al candidato.
	ctrl, trabajos, _ := montarTrabajador(t, consultas)
	trabajos.errCargar = errors.New("la base no responde")
	ctrl.reanudarUna("prueba")
	if len(trabajos.terminados) != 1 || trabajos.terminados[0] == nil {
		t.Errorf("debía cerrarse con error: %v", trabajos.terminados)
	}

	// El candidato no trae cédula (algo salió mal al guardar sus datos).
	ctrl, trabajos, _ = montarTrabajador(t, consultas)
	trabajos.candidato.Cedula = "No especificada"
	ctrl.reanudarUna("prueba")
	if len(trabajos.terminados) != 1 || trabajos.terminados[0] == nil {
		t.Errorf("sin cédula debía cerrarse con error: %v", trabajos.terminados)
	}
	if len(ctrl.semCT) != 0 {
		t.Error("aun fallando debe liberar el navegador")
	}
}

func TestUnPanicEnLaReanudacionNoTumbaAlTrabajador(t *testing.T) {
	consultas := nuevasConsultas("CARLOS", "PEREZ")
	ctrl, trabajos, _ := montarTrabajador(t, consultas)
	trabajos.candidato = nil // la base devolvió nil sin error: provoca un panic al usarlo

	if !ctrl.reanudarUna("prueba") {
		t.Error("debía procesar (y cerrar con error) el trabajo")
	}
	if len(trabajos.terminados) != 1 || trabajos.terminados[0] == nil {
		t.Errorf("el panic debe convertirse en un error del trabajo: %v", trabajos.terminados)
	}
}

func TestElCicloHaceElMantenimiento(t *testing.T) {
	consultas := nuevasConsultas("CARLOS", "PEREZ")
	ctrl, trabajos, _ := montarTrabajador(t, consultas)
	trabajos.pendiente = nil
	ctrl.cicloSegundoPlano("prueba")
	if trabajos.vencimientos != 1 || trabajos.recuperaciones != 1 {
		t.Errorf("cada ciclo debe vencer enlaces (%d) y recuperar reanudaciones detenidas (%d)", trabajos.vencimientos, trabajos.recuperaciones)
	}
}

// El sistema no le escribe a nadie por su cuenta salvo que se active expresamente y fuera del modo de prueba.
func TestElEnvioAutomaticoSoloSeActivaExpresamente(t *testing.T) {
	casos := []struct {
		nombre                 string
		automatica, soloPrueba bool
		esperadosEnvios        int
	}{
		{"por defecto", false, true, 0},
		{"automática pero en modo de prueba", true, true, 0},
		{"automática sin modo de prueba", true, false, 1},
		{"manual fuera del modo de prueba", false, false, 0},
	}
	for _, c := range casos {
		consultas := nuevasConsultas("CARLOS", "PEREZ")
		ctrl, trabajos, _ := montarTrabajador(t, consultas)
		trabajos.pendiente = nil
		envios := &enviosFalsos{
			contacto: almacen.Contacto{Correo: "c@mail.com"}, pendientes: []string{postulacionPrueba},
		}
		ctrl.envios = envios
		ctrl.cfg.URLPlataformaValidacion = "https://plataforma.test"
		ctrl.cfg.SolicitudAutomatica, ctrl.cfg.SolicitudSoloPrueba = c.automatica, c.soloPrueba
		ctrl.notificador, ctrl.whatsapp = &notificadorFalso{}, &notificadorFalso{}

		ctrl.cicloSegundoPlano("prueba")
		if len(envios.preparados) != c.esperadosEnvios {
			t.Errorf("%s: %d envíos; quería %d", c.nombre, len(envios.preparados), c.esperadosEnvios)
		}
	}
}
