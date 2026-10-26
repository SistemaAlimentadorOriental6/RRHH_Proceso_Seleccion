package servidor

import (
	"strings"
	"testing"
	"time"
)

func TestDifusorEntregaAvisosYNoBloqueaAlLento(t *testing.T) {
	d := nuevoDifusor()
	rapido, salirRapido := d.suscribir()
	defer salirRapido()
	_, salirLento := d.suscribir() // nunca lee
	defer salirLento()

	hecho := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ { // más que el búfer del suscriptor lento
			d.publicar("formulario_completado")
		}
		close(hecho)
	}()
	select {
	case <-hecho:
	case <-time.After(2 * time.Second):
		t.Fatal("publicar se bloqueó por un suscriptor lento")
	}
	select {
	case av := <-rapido:
		if av.evento != "cambio" || av.datos != `{"tipo":"formulario_completado"}` {
			t.Fatalf("aviso = %+v", av)
		}
	default:
		t.Fatal("el suscriptor no recibió el aviso")
	}

	var nulo *difusor
	nulo.publicar("x") // sin base de datos no hay difusor: no debe fallar
}

func TestReanudacionLlevaSusEtapasALaPantallaConSuPostulacion(t *testing.T) {
	d := nuevoDifusor()
	ch, salir := d.suscribir()
	defer salir()
	rep := reporteroReanudacion{d: d, postulacionID: "p-1"}

	rep.Emitir("candidato.etapa", map[string]any{"indice": 3, "etapa": "adres", "estado": "iniciada"})
	rep.Emitir("resumen", map[string]any{"x": 1}) // el resto de eventos del análisis no sale

	av := <-ch
	if av.evento != "candidato.etapa" || !strings.Contains(av.datos, `"postulacion_id":"p-1"`) || !strings.Contains(av.datos, `"etapa":"adres"`) {
		t.Fatalf("aviso = %+v", av)
	}
	select {
	case extra := <-ch:
		t.Fatalf("no debía salir otro evento: %+v", extra)
	default:
	}
}
