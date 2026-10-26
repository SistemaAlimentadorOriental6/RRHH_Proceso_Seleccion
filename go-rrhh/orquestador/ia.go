package orquestador

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-rrhh/modelos"
)

// EvaluadorIA califica la hoja de vida de un candidato apto contra los datos oficiales.
type EvaluadorIA interface {
	Calificar(ctx context.Context, cand modelos.RegistroApto, textoCV, cargo string) (*modelos.EvaluacionIA, error)
}

// PersistenciaIA guarda la calificación de la IA de una postulación (la implementa almacen.Almacen).
type PersistenciaIA interface {
	GuardarEvaluacionIA(ctx context.Context, postulacionID string, ev *modelos.EvaluacionIA) error
}

// GeneroIA decide el género por el nombre cuando el diccionario no lo conoce. Lo implementa el mismo
// agente de IA; es opcional: si el EvaluadorIA no lo cumple, se usa solo el método sin IA.
type GeneroIA interface {
	DeterminarGenero(ctx context.Context, nombre string) (string, error)
}

// determinarGenero decide el género de la persona y de dónde salió ("" si fue el diccionario o el
// perfil, " (IA)" si lo decidió la IA). Orden: perfil o diccionario → IA → método de respaldo (la
// terminación del nombre), que solo se usa si la IA no está o falla. La respuesta de la IA se
// recuerda por nombre: el mismo nombre no se vuelve a consultar.
func (a *Analizador) determinarGenero(ctx context.Context, c *modelos.Candidato) (genero, origen string) {
	if g := a.clasif.GeneroSeguro(c.Nombre, c.TextoPerfil); g != "" {
		return g, ""
	}
	if ia, ok := a.ia.(GeneroIA); ok && ctx.Err() == nil {
		clave := strings.ToLower(strings.Join(strings.Fields(c.Nombre), " "))
		if g, ok := a.generosIA.Load(clave); ok {
			return g.(string), " (IA)"
		}
		g, err := ia.DeterminarGenero(ctx, c.Nombre)
		switch {
		case err != nil:
			fmt.Printf("[!] La IA no pudo determinar el género de %q: %v\n", c.Nombre, err)
		case g == "Hombre" || g == "Mujer":
			a.generosIA.Store(clave, g)
			return g, " (IA)"
		}
	}
	return a.clasif.DeterminarGenero(c.Nombre, c.TextoPerfil), ""
}

// ConEvaluadorIA activa el paso final del flujo: la IA califica a quien cumplió todas las validaciones.
func (a *Analizador) ConEvaluadorIA(e EvaluadorIA) *Analizador {
	a.ia = e
	return a
}

// evaluarConIA es la última etapa del flujo, solo para aptos sin errores de consulta: la IA califica la
// hoja de vida. Si falla, el candidato sigue apto y sin puntaje; el siguiente análisis lo reintenta.
// Devuelve true si dejó una calificación nueva en el candidato.
func (a *Analizador) evaluarConIA(ctx context.Context, c *modelos.Candidato) bool {
	if a.ia == nil || c.EvaluacionIA != nil || c.ErrorProceso != "" ||
		c.EvaluacionIdoneidad == nil || !c.EvaluacionIdoneidad.EsApto {
		return false
	}
	a.etapa(c, "ia", estadoIniciada, "", 0)
	t0 := time.Now()
	ev, err := a.ia.Calificar(ctx, modelos.RegistroApto{
		Nombre: c.Nombre, Cedula: c.Cedula, Genero: c.Genero,
		Adres: c.ResultadoAdres, Runt: c.ResultadoRunt, Simit: c.ResultadoSimit,
		Procuraduria: c.ResultadoProcuraduria, SarlaftONU: c.ResultadoSarlaft, Ofac: c.ResultadoOfac,
	}, textoHojaDeVida(c), a.oferta.Titulo)
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		a.etapa(c, "ia", estadoError, "La IA no pudo calificar: "+err.Error(), time.Since(t0))
		return false
	}
	c.EvaluacionIA = ev
	a.etapa(c, "ia", estadoOK, fmt.Sprintf("Score %d/100 · %s", ev.ScoreTotal, ev.Dictamen), time.Since(t0))
	return true
}

// guardarIA guarda la calificación en la base de datos (el candidato ya debe estar guardado).
func (a *Analizador) guardarIA(c *modelos.Candidato) {
	p, ok := a.persistencia.(PersistenciaIA)
	if !ok || c.EvaluacionIA == nil || c.PostulacionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), plazoPersistencia)
	defer cancel()
	if err := p.GuardarEvaluacionIA(ctx, c.PostulacionID, c.EvaluacionIA); err != nil {
		a.etapa(c, "guardado", estadoError, "no se pudo guardar la calificación de la IA: "+err.Error(), 0)
	}
}
