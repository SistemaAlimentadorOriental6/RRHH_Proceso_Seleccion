package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"go-rrhh/modelos"
	"go-rrhh/orquestador"
)

// persistenciaFalsa hace de base de datos: recuerda a quién se le pidió guardar.
type persistenciaFalsa struct {
	mu        sync.Mutex
	guardados []modelos.Candidato
	ofertas   []modelos.OfertaLaboral
	err       error

	previos       map[string]*modelos.Candidato // lo ya analizado que devuelve la "base"
	errPrevios    error
	vigentes      []string // URLs recibidas en MarcarVigentes
	vigentesLlamo bool
}

func (p *persistenciaFalsa) GuardarCandidato(ctx context.Context, o modelos.OfertaLaboral, c *modelos.Candidato) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.guardados = append(p.guardados, *c)
	p.ofertas = append(p.ofertas, o)
	return p.err
}

func (p *persistenciaFalsa) CandidatosPrevios(ctx context.Context, o modelos.OfertaLaboral) (map[string]*modelos.Candidato, error) {
	return p.previos, p.errPrevios
}

func (p *persistenciaFalsa) MarcarVigentes(ctx context.Context, o modelos.OfertaLaboral, urls []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vigentesLlamo = true
	p.vigentes = urls
	return nil
}

func validadoresQueCumplen() orquestador.Validadores {
	return orquestador.Validadores{
		Runt: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoRunt = &modelos.ResultadoRunt{
				Estado: "INSCRITO_ACTIVO", Inscrito: true, FechaInscripcion: "2015-01-01",
				Licencias: []modelos.LicenciaRunt{{Estado: "ACTIVA", Categorias: []string{"C2"}}},
			}
			return nil
		},
		Simit: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoSimit = &modelos.ResultadoSimit{Estado: "PAZ_Y_SALVO", PazYSalvo: true, TotalDeuda: "$ 0"}
			return nil
		},
		Adres: func(ctx context.Context, c *modelos.Candidato) error {
			c.ResultadoAdres = &modelos.ResultadoAdres{Estado: "OK", EPS: "SURA", EstadoAfiliacion: "ACTIVO"}
			return nil
		},
	}
}

func analizarConPersistencia(t *testing.T, p orquestador.Persistencia) []eventoLeido {
	t.Helper()
	return analizarConFuente(t, &fuenteAnalisisFalsa{}, p)
}

func analizarConFuente(t *testing.T, fuente *fuenteAnalisisFalsa, p orquestador.Persistencia) []eventoLeido {
	t.Helper()
	return analizarCon(t, fuente, validadoresQueCumplen(), p)
}

// analizarCon lanza un análisis en vivo con la fuente y las consultas dadas y devuelve sus eventos.
func analizarCon(t *testing.T, fuente FuenteOfertas, validadores orquestador.Validadores, p orquestador.Persistencia) []eventoLeido {
	t.Helper()
	ctrl, srv := montarControladorAnalisis(t, fuente, validadores)
	ctrl.ConPersistencia(p)
	rr := pedir(srv.rutas(), "GET", "/api/computrabajo/vacantes/"+idOfertaPrueba+"/analizar?antecedentes=0&sarlaft=0&ofac=0", nil)
	if rr.Code != 200 {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	return leerSSE(t, rr.Body.String())
}

func TestAnalisisGuardaCadaCandidatoEnLaBase(t *testing.T) {
	p := &persistenciaFalsa{}
	eventos := analizarConPersistencia(t, p)

	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.completado" {
		t.Fatalf("último evento = %s", ultimo)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.guardados) != 3 {
		t.Fatalf("se guardaron %d candidatos; quería 3", len(p.guardados))
	}
	for i, c := range p.guardados {
		if c.EvaluacionIdoneidad == nil {
			t.Errorf("%s se guardó sin evaluación de idoneidad", c.Nombre)
		}
		if p.ofertas[i].ID != idOfertaPrueba || p.ofertas[i].Titulo == "" {
			t.Errorf("oferta recibida = %+v", p.ofertas[i])
		}
	}
}

func TestFalloDeLaBaseNoDetieneElAnalisis(t *testing.T) {
	p := &persistenciaFalsa{err: errors.New("base caída")}
	eventos := analizarConPersistencia(t, p)

	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.completado" {
		t.Fatalf("el análisis debe terminar aunque la base falle; último evento = %s", ultimo)
	}
	avisos, completos := 0, 0
	for _, ev := range eventos {
		switch ev.tipo {
		case "candidato.completo":
			completos++
		case "candidato.etapa":
			var cuerpo struct{ Etapa, Estado string }
			_ = json.Unmarshal([]byte(ev.datos), &cuerpo)
			if cuerpo.Etapa == "guardado" && cuerpo.Estado == "error" {
				avisos++
			}
		}
	}
	if completos != 3 {
		t.Errorf("candidatos completos = %d; quería 3", completos)
	}
	if avisos != 3 {
		t.Errorf("avisos de error al guardar = %d; quería 3 (uno por candidato)", avisos)
	}
}
