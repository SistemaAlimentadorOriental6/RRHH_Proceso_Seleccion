package servidor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// difusor avisa a las pantallas abiertas del reclutador que algo cambió en la base de datos (el
// candidato envió su formulario, terminó una reanudación). Solo viaja el tipo de cambio: la
// pantalla vuelve a leer el estado a la API. Vive en memoria: si el servidor se reinicia, la
// pantalla se reconecta sola y relee, así que no se pierde nada.
type difusor struct {
	mu   sync.Mutex
	subs map[chan aviso]struct{}
}

// aviso es un evento SSE ya armado: nombre y datos JSON.
type aviso struct{ evento, datos string }

func nuevoDifusor() *difusor { return &difusor{subs: map[chan aviso]struct{}{}} }

func (d *difusor) suscribir() (<-chan aviso, func()) {
	ch := make(chan aviso, 64)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	d.mu.Unlock()
	return ch, func() {
		d.mu.Lock()
		delete(d.subs, ch)
		d.mu.Unlock()
	}
}

// publicar avisa que algo cambió; solo viaja el tipo y la pantalla vuelve a leer a la API.
func (d *difusor) publicar(tipo string) {
	d.emitir("cambio", map[string]string{"tipo": tipo})
}

// emitir nunca bloquea: a quien va lento se le descarta el aviso (el siguiente lo pone al día).
func (d *difusor) emitir(evento string, datos any) {
	if d == nil {
		return
	}
	cuerpo, err := json.Marshal(datos)
	if err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.subs {
		select {
		case ch <- aviso{evento, string(cuerpo)}:
		default:
		}
	}
}

// reporteroReanudacion lleva el avance de una reanudación (etapa por etapa) a las pantallas abiertas,
// igual que el análisis en vivo, marcándolo con la postulación a la que pertenece.
type reporteroReanudacion struct {
	d             *difusor
	postulacionID string
}

func (r reporteroReanudacion) Emitir(tipo string, datos any) {
	if tipo != "candidato.etapa" && tipo != "candidato.completo" {
		return
	}
	cuerpo, err := json.Marshal(datos)
	if err != nil {
		return
	}
	var mapa map[string]any
	if json.Unmarshal(cuerpo, &mapa) != nil {
		return
	}
	mapa["postulacion_id"] = r.postulacionID
	r.d.emitir(tipo, mapa)
}

// ManejarCambios transmite como SSE los avisos de cambio del flujo de datos faltantes.
func (c *ControladorRRHH) ManejarCambios(w http.ResponseWriter, r *http.Request) {
	if c.cambios == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Avisos no disponibles"})
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // el límite de 60 s de las respuestas normales cortaría el stream

	cab := w.Header()
	cab.Set("Content-Type", "text/event-stream; charset=utf-8")
	cab.Set("Cache-Control", "no-cache, no-transform")
	cab.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil || rc.Flush() != nil {
		return
	}

	avisos, desuscribir := c.cambios.suscribir()
	defer desuscribir()
	ping := time.NewTicker(intervaloPing)
	defer ping.Stop()
	for {
		select {
		case av := <-avisos:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", av.evento, av.datos); err != nil || rc.Flush() != nil {
				return
			}
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
