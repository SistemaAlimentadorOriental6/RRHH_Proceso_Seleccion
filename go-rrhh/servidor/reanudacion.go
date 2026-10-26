package servidor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"go-rrhh/almacen"
	"go-rrhh/orquestador"
)

const (
	// cicloReanudador es cada cuánto revisa el trabajador si hay algo que hacer cuando nadie lo despierta.
	cicloReanudador = 15 * time.Second
	// leaseReanudacion es el tiempo que se reserva una reanudación mientras se procesa.
	leaseReanudacion = 20 * time.Minute
	// plazoReanudacion es lo máximo que puede tardar la consulta a las fuentes de un candidato.
	plazoReanudacion = 15 * time.Minute
)

// reporteroNulo descarta el avance del análisis: una reanudación no tiene a nadie mirando por SSE
// (su resultado queda en la base de datos y en la línea de tiempo del candidato).
type reporteroNulo struct{}

func (reporteroNulo) Emitir(string, any) {}

// despertarSegundoPlano avisa al trabajador que hay algo nuevo sin esperar a su siguiente ciclo.
func (c *ControladorRRHH) despertarSegundoPlano() {
	select {
	case c.despertar <- struct{}{}:
	default: // ya hay un aviso pendiente
	}
}

// iniciarSegundoPlano arranca el trabajador que reanuda los análisis de los candidatos que
// entregaron sus datos. Se detiene con el servidor (c.base).
func (c *ControladorRRHH) iniciarSegundoPlano() {
	if c.trabajos == nil {
		return
	}
	if c.cfg.SolicitudAutomatica && c.cfg.SolicitudSoloPrueba {
		log.Printf("[!] RRHH_SOLICITUD_AUTOMATICA está activa pero RRHH_SOLICITUD_SOLO_PRUEBA también: solo se escribe solo a las personas de prueba")
	}
	go c.bucleSegundoPlano()
}

func (c *ControladorRRHH) bucleSegundoPlano() {
	trabajador := fmt.Sprintf("reanudador-%d", os.Getpid())
	if err := c.trabajos.RecuperarReanudaciones(c.base); err != nil {
		log.Printf("[!] No se pudieron recuperar las reanudaciones pendientes: %v", err)
	}
	ticker := time.NewTicker(cicloReanudador)
	defer ticker.Stop()
	for {
		c.cicloSegundoPlano(trabajador)
		select {
		case <-c.base.Done():
			return
		case <-ticker.C:
		case <-c.despertar:
		}
		if c.base.Err() != nil {
			return
		}
	}
}

// cicloSegundoPlano hace el mantenimiento (vencer enlaces, enviar solicitudes automáticas) y luego
// reanuda, de a una, las reservas pendientes.
func (c *ControladorRRHH) cicloSegundoPlano(trabajador string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] trabajador de reanudaciones: %v", r)
		}
	}()

	if n, err := c.trabajos.VencerSolicitudes(c.base); err != nil {
		log.Printf("[!] No se pudieron vencer las solicitudes caducadas: %v", err)
	} else if n > 0 {
		log.Printf("[+] %d solicitudes de datos vencieron sin respuesta", n)
	}
	if err := c.trabajos.RecuperarReanudaciones(c.base); err != nil {
		log.Printf("[!] No se pudieron recuperar las reanudaciones detenidas: %v", err)
	}

	// Envío automático: en modo de prueba solo a las personas de prueba; fuera de él, solo si se activó
	// expresamente RRHH_SOLICITUD_AUTOMATICA.
	if (c.cfg.SolicitudSoloPrueba || c.cfg.SolicitudAutomatica) && c.envios != nil {
		c.enviarSolicitudesPendientes()
	}

	for c.base.Err() == nil {
		if !c.reanudarUna(trabajador) {
			return
		}
	}
}

// enviarSolicitudesPendientes manda la solicitud a los candidatos que quedaron en pausa y a los que
// todavía no se les ha escrito.
func (c *ControladorRRHH) enviarSolicitudesPendientes() {
	ids, err := c.envios.EnviosPendientes(c.base)
	if err != nil {
		log.Printf("[!] No se pudieron listar las solicitudes por enviar: %v", err)
		return
	}
	for _, id := range ids {
		if c.base.Err() != nil {
			return
		}
		if c.cfg.SolicitudSoloPrueba {
			_, cand, err := c.trabajos.CandidatoPorPostulacion(c.base, id)
			if err != nil || !esPrueba(cand.Nombre, cand.Cedula) {
				continue
			}
		}
		_, err := c.enviarSolicitud(c.base, id)
		if err != nil && !errors.Is(err, almacen.ErrSinContacto) {
			log.Printf("[!] No se pudo enviar la solicitud de datos de una postulación: %v", err)
		}
		c.cambios.publicar("solicitud_enviada")
	}
}

// reanudarUna reserva y procesa una reanudación. Devuelve falso cuando no hay más que hacer por ahora
// (no hay pendientes o el navegador está ocupado con otro análisis).
func (c *ControladorRRHH) reanudarUna(trabajador string) bool {
	// Los análisis de vacantes y las reanudaciones comparten navegadores (ADRES atiende una consulta
	// por vez): si hay un análisis en curso, la reanudación espera al siguiente ciclo.
	select {
	case c.semCT <- struct{}{}:
	default:
		return false
	}
	defer func() { <-c.semCT }()

	t, err := c.trabajos.ReclamarReanudacion(c.base, trabajador, leaseReanudacion)
	if err != nil {
		log.Printf("[!] No se pudo reclamar una reanudación: %v", err)
		return false
	}
	if t == nil {
		return false
	}

	c.cambios.publicar("reanudacion_iniciada")
	fallo := c.ejecutarReanudacion(t)
	if fallo != nil {
		log.Printf("[!] La reanudación de una postulación falló (intento %d): %v", t.Intentos, fallo)
		c.cambios.emitir("reanudacion.fallida", map[string]any{"postulacion_id": t.PostulacionID, "intento": t.Intentos, "mensaje": fallo.Error()})
	}
	if err := c.trabajos.TerminarReanudacion(context.WithoutCancel(c.base), t, fallo); err != nil {
		log.Printf("[!] No se pudo cerrar la reanudación: %v", err)
		return false
	}
	c.cambios.publicar("reanudacion_terminada")
	return true
}

// ejecutarReanudacion continúa el análisis del candidato justo donde se detuvo.
func (c *ControladorRRHH) ejecutarReanudacion(t *almacen.TrabajoReanudacion) (fallo error) {
	defer func() {
		if r := recover(); r != nil {
			fallo = fmt.Errorf("error interno: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(c.base, plazoReanudacion)
	defer cancel()

	oferta, candidato, err := c.trabajos.CandidatoPorPostulacion(ctx, t.PostulacionID)
	if err != nil {
		return fmt.Errorf("no se pudo cargar al candidato: %w", err)
	}
	opts := orquestador.OpcionesAnalisis{Concurrencia: 1, Antecedentes: true, Sarlaft: true, Ofac: true}
	analizador := c.fabricaAnalizador(nil, opts, reporteroReanudacion{d: c.cambios, postulacionID: t.PostulacionID})
	defer analizador.Cerrar()
	return analizador.Reanudar(ctx, oferta, candidato)
}
