package orquestador

import (
	"context"
	"fmt"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
)

// FuenteReglas da las versiones de reglas de idoneidad guardadas desde Configuración > Parámetros (la
// implementa almacen.Almacen). Es opcional: sin ella se evalúa con las reglas de la configuración.
type FuenteReglas interface {
	ParametrosVigentes(ctx context.Context) (modelos.ReglasIdoneidad, error)
	ParametrosVersion(ctx context.Context, version int) (modelos.ReglasIdoneidad, error)
}

func (a *Analizador) fuenteReglas() FuenteReglas {
	f, _ := a.persistencia.(FuenteReglas)
	return f
}

// fijarReglas deja en el candidato las reglas con las que se le evalúa: las de version (las de su análisis
// anterior) o, con version 0, las vigentes en este momento. Quien empieza después de guardar parámetros
// nuevos los recibe; quien ya iba en curso termina con los que tenía.
func (a *Analizador) fijarReglas(ctx context.Context, c *modelos.Candidato, version int) error {
	f := a.fuenteReglas()
	if f == nil {
		r := agentes.ReglasDeConfiguracion(a.cfg)
		c.Reglas = &r
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, plazoPersistencia)
	defer cancel()
	var r modelos.ReglasIdoneidad
	var err error
	if version > 0 {
		r, err = f.ParametrosVersion(ctx, version)
	} else {
		r, err = f.ParametrosVigentes(ctx)
	}
	if err != nil {
		return fmt.Errorf("no se pudieron leer las reglas de idoneidad (versión %d): %w", version, err)
	}
	c.Reglas = &r
	return nil
}

// versionConservada es la versión de reglas que conserva quien se vuelve a analizar (por ejemplo, forzado):
// la de su dictamen anterior. Quien no tuvo dictamen firme (consulta pendiente o en pausa por datos) devuelve
// 0 y se evalúa con las reglas vigentes.
func versionConservada(previo *modelos.Candidato) int {
	if previo == nil || previo.EvaluacionIdoneidad == nil {
		return 0
	}
	ev := previo.EvaluacionIdoneidad
	if ev.VerificacionPendiente || previo.ErrorProceso != "" || modelos.EnPausa(previo.EstadoProceso) {
		return 0
	}
	return ev.ReglaVersion
}
