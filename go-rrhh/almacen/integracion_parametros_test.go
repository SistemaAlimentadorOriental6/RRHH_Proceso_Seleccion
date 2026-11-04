package almacen

import (
	"context"
	"errors"
	"testing"

	"go-rrhh/modelos"
)

// Cada evaluación queda con la versión de reglas con la que se hizo, aunque después se guarden otras.
func TestIntegracionVersionDeReglasPorCandidato(t *testing.T) {
	db := baseDePrueba(t)
	ctx := context.Background()
	k, err := nuevasClaves(claveA, claveB)
	if err != nil {
		t.Fatal(err)
	}
	a, err := nuevo(ctx, db, k, t.TempDir(), slugDePrueba, slugDePrueba)
	if err != nil {
		t.Fatal(err)
	}

	v1, err := a.ParametrosVersion(ctx, 1)
	if err != nil || v1.Version != 1 || v1.RuntAntiguedadMeses != 24 || len(v1.EpsAceptadas) != 4 {
		t.Fatalf("versión 1 = %+v, %v", v1, err)
	}
	nuevos := v1
	nuevos.GenerosAceptados = []string{"masculino", "femenino"}
	nuevos.ExperienciaMinimaMeses = 18
	v2, err := a.GuardarParametros(ctx, nuevos, "prueba@x")
	if err != nil || v2.Version != 2 {
		t.Fatalf("guardar = %+v, %v", v2, err)
	}
	if v1otra, _ := a.ParametrosVersion(ctx, 1); len(v1otra.GenerosAceptados) != 1 {
		t.Errorf("la versión 1 no debe cambiar: %+v", v1otra)
	}
	if versiones, err := a.VersionesParametros(ctx); err != nil || len(versiones) != 2 || versiones[0].VigenteHasta != nil || versiones[1].VigenteHasta == nil {
		t.Errorf("versiones = %+v, %v", versiones, err)
	}
	if _, err := a.ParametrosVersion(ctx, 99); !errors.Is(err, ErrVersionInexistente) {
		t.Errorf("versión 99: %v", err)
	}

	// Evaluado con la versión 1 aunque la vigente ya es la 2.
	oferta := modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor"}
	c := candidatoCompleto(t)
	c.EvaluacionIdoneidad.ReglaVersion = 1
	c.EsApto, c.EvaluacionIdoneidad.EsApto = false, false
	c.EvaluacionIdoneidad.Clasificacion = modelos.ClasificacionSemillero
	c.EvaluacionIdoneidad.Semillero = []string{"runt"}
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}
	previos, err := a.CandidatosPrevios(ctx, oferta)
	if err != nil || previos[c.URLDetalle] == nil || previos[c.URLDetalle].EvaluacionIdoneidad.ReglaVersion != 1 {
		t.Fatalf("debe leerse con la versión 1: %+v, %v", previos[c.URLDetalle], err)
	}
	if ev := previos[c.URLDetalle].EvaluacionIdoneidad; ev.Clasificacion != modelos.ClasificacionSemillero || len(ev.Semillero) != 1 || ev.Semillero[0] != "runt" {
		t.Errorf("la clasificación debe leerse igual: %q %v", ev.Clasificacion, ev.Semillero)
	}
	if n := contar(t, db, `SELECT COUNT(*) FROM postulaciones WHERE clasificacion = 'semillero'`); n != 1 {
		t.Errorf("postulaciones.clasificacion: %d filas de semillero", n)
	}

	c.EvaluacionIdoneidad.ReglaVersion = 99
	if err := a.GuardarCandidato(ctx, oferta, c); !errors.Is(err, ErrVersionInexistente) {
		t.Errorf("una versión inexistente no debe guardarse: %v", err)
	}
}
