package almacen

import (
	"context"
	"testing"
	"time"

	"go-rrhh/modelos"
)

// Corre solo con RRHH_TEST_MYSQL_DSN (ver integracion_test.go): una base vacía y exclusiva para pruebas.

func almacenDePrueba(t *testing.T) (*Almacen, context.Context) {
	t.Helper()
	db := baseDePrueba(t)
	k, err := nuevasClaves(claveA, claveB)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a, err := nuevo(ctx, db, k, t.TempDir(), slugDePrueba, slugDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	return a, ctx
}

// Guardar y volver a leer devuelve al candidato como lo produjo el análisis.
func TestIntegracionIdaYVuelta(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	oferta := modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}
	c := candidatoCompleto(t)
	c.EvaluacionIdoneidad.MotivosDescarte = nil
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}

	lista, err := a.CandidatosDeOferta(ctx, "ABC123")
	if err != nil || len(lista) != 1 {
		t.Fatalf("CandidatosDeOferta = %d candidatos, %v", len(lista), err)
	}
	g := lista[0]

	if g.Nombre != c.Nombre || g.Cedula != "1111111" || g.Indice != 1 || g.URLDetalle != c.URLDetalle || g.Genero != "Hombre" || !g.EsApto {
		t.Errorf("datos del candidato: %+v", g)
	}
	if g.TextoPerfil != c.TextoPerfil {
		t.Errorf("TextoPerfil = %q", g.TextoPerfil)
	}
	if g.RutaPDF != c.RutaPDF || g.Capturas["perfil"] != c.Capturas["perfil"] {
		t.Errorf("rutas: pdf=%q captura=%q; quería %q y %q", g.RutaPDF, g.Capturas["perfil"], c.RutaPDF, c.Capturas["perfil"])
	}

	if g.ResultadoAdres == nil || g.ResultadoAdres.EPS != "SURA" || g.ResultadoAdres.FechaAfiliacion != "15/03/2018" {
		t.Errorf("ADRES: %+v", g.ResultadoAdres)
	}
	if g.ResultadoRunt == nil || len(g.ResultadoRunt.Licencias) != 1 ||
		g.ResultadoRunt.Licencias[0].DetallesCategorias[0].Categoria != "C2" ||
		g.ResultadoRunt.FechaInscripcion != c.ResultadoRunt.FechaInscripcion {
		t.Errorf("RUNT: %+v", g.ResultadoRunt)
	}
	if g.ResultadoSimit == nil || g.ResultadoSimit.TotalDeuda != "$ 0" || !g.ResultadoSimit.PazYSalvo {
		t.Errorf("SIMIT: %+v", g.ResultadoSimit)
	}
	if g.ResultadoProcuraduria == nil || g.ResultadoSarlaft == nil || g.ResultadoOfac == nil {
		t.Errorf("informativas: %v %v %v", g.ResultadoProcuraduria, g.ResultadoSarlaft, g.ResultadoOfac)
	}

	ev := g.EvaluacionIdoneidad
	if ev == nil || !ev.EsApto || !ev.CumpleRunt || ev.DetalleRunt != c.EvaluacionIdoneidad.DetalleRunt || ev.DetalleEPS != "EPS SURA (ACTIVO)" {
		t.Errorf("evaluación: %+v", ev)
	}
}

// Los motivos de descarte vuelven en el orden en que se guardaron.
func TestIntegracionMotivosEnOrden(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	c := &modelos.Candidato{
		Indice: 1, Nombre: "Pedro Ruiz", Cedula: "4444444", URLDetalle: "https://x/Detail/4",
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{
			Genero: "Hombre", CumpleGenero: true,
			DetalleEPS: "EPS 'X' no autorizada", DetalleRunt: "No posee licencia activa en categoría C2 o C3", DetalleSimit: "SIMIT OK",
			MotivosDescarte: []string{"EPS 'X' no autorizada", "No posee licencia activa en categoría C2 o C3", "motivo tres", "motivo cuatro"},
		},
	}
	if err := a.GuardarCandidato(ctx, modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor"}, c); err != nil {
		t.Fatal(err)
	}
	lista, err := a.CandidatosDeOferta(ctx, "ABC123")
	if err != nil || len(lista) != 1 {
		t.Fatalf("%d candidatos, %v", len(lista), err)
	}
	got := lista[0].EvaluacionIdoneidad.MotivosDescarte
	for i, m := range c.EvaluacionIdoneidad.MotivosDescarte {
		if i >= len(got) || got[i] != m {
			t.Fatalf("motivos = %q; quería %q", got, c.EvaluacionIdoneidad.MotivosDescarte)
		}
	}
}

func TestIntegracionVacantesYVigentes(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	oferta := modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}
	c := candidatoCompleto(t)
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}

	ofertas, err := a.OfertasProcesadas(ctx)
	if err != nil || len(ofertas) != 1 || ofertas[0].Total != 1 || ofertas[0].Aptos != 1 ||
		ofertas[0].IDComputrabajo != "ABC123" || ofertas[0].ActualizadoEn.IsZero() {
		t.Fatalf("OfertasProcesadas = %+v, %v", ofertas, err)
	}

	// Cuando el postulante ya no aparece se retira de la vacante, pero se conserva para reutilizarlo.
	if err := a.MarcarVigentes(ctx, oferta, []string{}); err != nil {
		t.Fatal(err)
	}
	if lista, _ := a.CandidatosDeOferta(ctx, "ABC123"); len(lista) != 0 {
		t.Errorf("tras retirarlo quedan %d candidatos vigentes", len(lista))
	}
	if ofertas, _ := a.OfertasProcesadas(ctx); len(ofertas) != 0 {
		t.Errorf("una vacante sin postulantes vigentes no debe listarse: %+v", ofertas)
	}
	previos, err := a.CandidatosPrevios(ctx, oferta)
	if err != nil || previos[c.URLDetalle] == nil {
		t.Fatalf("CandidatosPrevios debe conservarlo: %v, %v", previos, err)
	}

	// Si vuelve a aparecer queda vigente otra vez.
	if err := a.MarcarVigentes(ctx, oferta, []string{c.URLDetalle, "https://x/otro"}); err != nil {
		t.Fatal(err)
	}
	if lista, _ := a.CandidatosDeOferta(ctx, "ABC123"); len(lista) != 1 {
		t.Errorf("tras reaparecer hay %d candidatos vigentes; quería 1", len(lista))
	}

	// Guardar de nuevo a un retirado lo reactiva.
	_ = a.MarcarVigentes(ctx, oferta, []string{})
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}
	if lista, _ := a.CandidatosDeOferta(ctx, "ABC123"); len(lista) != 1 {
		t.Errorf("al analizarlo de nuevo debe quedar vigente: %d", len(lista))
	}
}

// Lo importado antes de existir validaciones.datos se vuelve a guardar completo, una sola vez.
func TestIntegracionImportarMejoraLoGuardadoSinDatos(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	oferta := modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}
	c := candidatoCompleto(t)
	if err := a.GuardarCandidato(ctx, oferta, c); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE validaciones SET datos = NULL`); err != nil {
		t.Fatal(err)
	}

	escribio, err := a.ImportarCandidato(ctx, oferta, c, time.Now())
	if err != nil || !escribio {
		t.Fatalf("la primera importación debía completar lo guardado: %v, %v", escribio, err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM validaciones WHERE datos IS NULL`); n != 0 {
		t.Errorf("quedan %d validaciones sin datos", n)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM validaciones`); n != 6 {
		t.Errorf("validaciones = %d; quería 6 (las viejas se reemplazan, no se duplican)", n)
	}
	if lista, _ := a.CandidatosDeOferta(ctx, "ABC123"); len(lista) != 1 || lista[0].ResultadoRunt == nil {
		t.Errorf("tras mejorarlo debe poder leerse completo: %+v", lista)
	}

	escribio, err = a.ImportarCandidato(ctx, oferta, c, time.Now())
	if err != nil || escribio {
		t.Errorf("la segunda importación no debe hacer nada: %v, %v", escribio, err)
	}
}
