const fs=require("fs");
function ed(f,pairs){let s=fs.readFileSync(f,"utf8");const crlf=s.includes("\r\n");if(crlf)s=s.replace(/\r\n/g,"\n");for(const [a,b] of pairs){if(!s.includes(a))throw new Error(f+": "+a.slice(0,60));s=s.replace(a,()=>b)}if(crlf)s=s.replace(/\n/g,"\r\n");fs.writeFileSync(f,s)}
ed("servidor/cambios.go",[
[`import (
	"fmt"`,`import (
	"encoding/json"
	"fmt"`],
[`type difusor struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func nuevoDifusor() *difusor { return &difusor{subs: map[chan string]struct{}{}} }

func (d *difusor) suscribir() (<-chan string, func()) {
	ch := make(chan string, 8)`,`type difusor struct {
	mu   sync.Mutex
	subs map[chan aviso]struct{}
}

// aviso es un evento SSE ya armado: nombre y datos JSON.
type aviso struct{ evento, datos string }

func nuevoDifusor() *difusor { return &difusor{subs: map[chan aviso]struct{}{}} }

func (d *difusor) suscribir() (<-chan aviso, func()) {
	ch := make(chan aviso, 64)`],
[`// publicar nunca bloquea: a quien va lento se le descarta el aviso (el siguiente lo pone al día).
func (d *difusor) publicar(tipo string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.subs {
		select {
		case ch <- tipo:
		default:
		}
	}
}`,`// publicar avisa que algo cambió; solo viaja el tipo y la pantalla vuelve a leer a la API.
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
}`],
[`		case tipo := <-avisos:
			if _, err := fmt.Fprintf(w, "event: cambio\ndata: {\\"tipo\\":%q}\n\n", tipo); err != nil || rc.Flush() != nil {`,`		case av := <-avisos:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", av.evento, av.datos); err != nil || rc.Flush() != nil {`],
]);
ed("servidor/reanudacion.go",[
[`	analizador := c.fabricaAnalizador(nil, opts, reporteroNulo{})`,`	analizador := c.fabricaAnalizador(nil, opts, reporteroReanudacion{d: c.cambios, postulacionID: t.PostulacionID})`],
[`	fallo := c.ejecutarReanudacion(t)
	if fallo != nil {
		log.Printf("[!] La reanudación de una postulación falló (intento %d): %v", t.Intentos, fallo)
	}`,`	fallo := c.ejecutarReanudacion(t)
	if fallo != nil {
		log.Printf("[!] La reanudación de una postulación falló (intento %d): %v", t.Intentos, fallo)
		c.cambios.emitir("reanudacion.fallida", map[string]any{"postulacion_id": t.PostulacionID, "intento": t.Intentos, "mensaje": fallo.Error()})
	}`],
]);
