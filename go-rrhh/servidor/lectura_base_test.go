package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"go-rrhh/almacen"
	"go-rrhh/modelos"
)

// ---- El análisis reutiliza lo que ya está en la base ----

func previoValido(nombre string, cedula string) *modelos.Candidato {
	url := "https://x/Detail/" + nombre
	return &modelos.Candidato{
		Indice: 1, Nombre: nombre, Cedula: cedula, URLDetalle: url, Genero: "Hombre",
		Capturas: map[string]string{"perfil": "perfil.jpg"}, EsApto: true,
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{
			EsApto: true, Genero: "Hombre", CumpleGenero: true, CumpleEPS: true, CumpleRunt: true, CumpleSimit: true,
		},
	}
}

func TestAnalisisReutilizaLoQueYaEstaEnLaBase(t *testing.T) {
	fuente := &fuenteAnalisisFalsa{}
	carlos := previoValido("Carlos Perez", "1111111")
	p := &persistenciaFalsa{previos: map[string]*modelos.Candidato{carlos.URLDetalle: carlos}}

	eventos := analizarConFuente(t, fuente, p)

	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.completado" {
		t.Fatalf("último evento = %s", ultimo)
	}
	if n := fuente.fichas.Load(); n != 2 {
		t.Errorf("se leyeron %d fichas; quería 2 (Carlos viene de la base y no se vuelve a consultar)", n)
	}
	cache := map[string]bool{}
	for _, ev := range eventos {
		if ev.tipo != "candidato.completo" {
			continue
		}
		var c struct {
			Nombre string `json:"nombre"`
			Cache  bool   `json:"cache"`
		}
		_ = json.Unmarshal([]byte(ev.datos), &c)
		cache[c.Nombre] = c.Cache
	}
	if !cache["Carlos Perez"] || cache["Maria Lopez"] || cache["Juan Gomez"] {
		t.Errorf("origen de cada candidato (true = caché): %v", cache)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.guardados) != 2 {
		t.Errorf("se guardaron %d candidatos; quería 2 (el reutilizado ya estaba guardado)", len(p.guardados))
	}
	if !p.vigentesLlamo || len(p.vigentes) != 3 {
		t.Errorf("MarcarVigentes: llamado=%v, urls=%d; quería 3 (las tres fichas vistas)", p.vigentesLlamo, len(p.vigentes))
	}
}

func TestAnalisisSeDetieneSiNoPuedeLeerLaBase(t *testing.T) {
	fuente := &fuenteAnalisisFalsa{}
	p := &persistenciaFalsa{errPrevios: errors.New("base caída")}

	eventos := analizarConFuente(t, fuente, p)

	if ultimo := eventos[len(eventos)-1].tipo; ultimo != "job.error" {
		t.Fatalf("último evento = %s; quería job.error", ultimo)
	}
	if n := fuente.fichas.Load(); n != 0 {
		t.Errorf("se leyeron %d fichas; no debe reprocesarse a nadie si la base no responde", n)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.guardados) != 0 {
		t.Errorf("no debía guardarse nada: %d", len(p.guardados))
	}
}

// ---- La API lee de la base ----

type lectorFalso struct {
	ofertas    []almacen.OfertaProcesada
	candidatos map[string][]modelos.Candidato
	err        error
}

func (l *lectorFalso) OfertasProcesadas(ctx context.Context) ([]almacen.OfertaProcesada, error) {
	return l.ofertas, l.err
}

func (l *lectorFalso) CandidatosDeOferta(ctx context.Context, id string) ([]modelos.Candidato, error) {
	return l.candidatos[id], l.err
}

func repositorioConBase(t *testing.T, l *lectorFalso) *RepositorioMySQL {
	t.Helper()
	dir := t.TempDir()
	return NuevoRepositorioMySQL(NuevoRepositorioDisco(dir), l, dir)
}

func lectorConDosVacantes() *lectorFalso {
	hoy := time.Now()
	return &lectorFalso{
		ofertas: []almacen.OfertaProcesada{
			{IDComputrabajo: "B", Titulo: "Operador(a) de buses - Metro", Slug: "operador-buses-metro", Total: 3, Aptos: 1, ActualizadoEn: hoy},
			{IDComputrabajo: "A", Titulo: "Conductor de camión", Slug: "conductor-camion", Total: 1, Aptos: 0, ActualizadoEn: hoy},
		},
		candidatos: map[string][]modelos.Candidato{
			"B": {
				{Indice: 1, Nombre: "Carlos Perez", Cedula: "1111111", EsApto: true},
				{Indice: 2, Nombre: "Maria Lopez", Cedula: "2222222"},
				{Indice: 3, Nombre: "Juan Gomez", Cedula: "3333333"},
			},
			"A": {{Indice: 1, Nombre: "Pedro Ruiz", Cedula: "4444444"}},
		},
	}
}

func TestRepositorioBaseListaVacantesConLaFormaDeSiempre(t *testing.T) {
	r := repositorioConBase(t, lectorConDosVacantes())
	ofertas, err := r.ListarOfertas()
	if err != nil {
		t.Fatal(err)
	}
	if len(ofertas) != 2 || ofertas[0].Titulo != "Conductor de camión" {
		t.Fatalf("ofertas ordenadas por título: %+v", ofertas)
	}
	o := ofertas[1]
	if o.ID != "Operador(a)_de_buses_-_Metro" {
		t.Errorf("el id debe seguir siendo el nombre de carpeta de siempre, fue %q", o.ID)
	}
	if o.TotalCandidatos != 3 || o.TotalAptos != 1 || o.TotalNoAptos != 2 || o.Slug != "operador-buses-metro" {
		t.Errorf("totales o slug: %+v", o)
	}
}

// Dos vacantes con el mismo título (Computrabajo la republicó) no se mezclan: cada una tiene slug e id
// propios y fijos, aunque cambie cuál se actualizó de último. Antes la pantalla saltaba de una a otra.
func TestRepositorioBaseDistingueVacantesConElMismoTitulo(t *testing.T) {
	hoy := time.Now()
	l := &lectorFalso{
		ofertas: []almacen.OfertaProcesada{
			{IDComputrabajo: "X1", Titulo: "Conductor de buses", Slug: "conductor-buses", Total: 9, Aptos: 9, ActualizadoEn: hoy},
			{IDComputrabajo: "X2", Titulo: "Conductor de Buses", Slug: "conductor-buses", Total: 6, Aptos: 6, ActualizadoEn: hoy.Add(time.Minute)},
		},
		candidatos: map[string][]modelos.Candidato{
			"X1": {{Nombre: "Uno", Cedula: "1111111", EsApto: true}},
			"X2": {{Nombre: "Dos", Cedula: "2222222", EsApto: true}, {Nombre: "Tres", Cedula: "3333333", EsApto: true}},
		},
	}
	r := repositorioConBase(t, l)
	ofertas, err := r.ListarOfertas()
	if err != nil || len(ofertas) != 2 {
		t.Fatalf("ofertas = %+v, %v", ofertas, err)
	}
	if ofertas[0].Slug == ofertas[1].Slug || ofertas[0].ID == ofertas[1].ID {
		t.Fatalf("slug e id deben ser distintos: %+v", ofertas)
	}
	slugs := map[string]string{}
	for _, o := range ofertas {
		slugs[o.IDComputrabajo] = o.Slug
		got, err := r.ResolverOferta(o.Slug)
		if err != nil || got.IDComputrabajo != o.IDComputrabajo {
			t.Errorf("ResolverOferta(%q) = %+v, %v", o.Slug, got, err)
		}
		lista, err := r.ObtenerCandidatosOferta(o.ID, "aptos")
		if err != nil || len(lista) != len(l.candidatos[o.IDComputrabajo]) {
			t.Errorf("candidatos de %s: %d, %v", o.IDComputrabajo, len(lista), err)
		}
	}

	// Que la otra se actualice no cambia los slugs.
	l.ofertas[0].ActualizadoEn = hoy.Add(time.Hour)
	time.Sleep(vigenciaOfertasBase)
	otra, _ := r.ListarOfertas()
	for _, o := range otra {
		if slugs[o.IDComputrabajo] != o.Slug {
			t.Errorf("el slug de %s cambió: %q → %q", o.IDComputrabajo, slugs[o.IDComputrabajo], o.Slug)
		}
	}
	// Un enlace viejo con el slug compartido sigue abriendo una de ellas.
	if _, err := r.ResolverOferta("conductor-buses"); err != nil {
		t.Errorf("slug compartido: %v", err)
	}
}

func TestRepositorioBaseResuelveVacantePorSlugOId(t *testing.T) {
	r := repositorioConBase(t, lectorConDosVacantes())
	for _, clave := range []string{"operador-buses-metro", "Operador(a)_de_buses_-_Metro"} {
		o, err := r.ResolverOferta(clave)
		if err != nil || o.Titulo != "Operador(a) de buses - Metro" {
			t.Errorf("ResolverOferta(%q) = %+v, %v", clave, o, err)
		}
	}
	if _, err := r.ResolverOferta("no-existe"); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("vacante inexistente: %v", err)
	}
}

func TestRepositorioBaseFiltraCandidatos(t *testing.T) {
	r := repositorioConBase(t, lectorConDosVacantes())
	id := "Operador(a)_de_buses_-_Metro"
	for filtro, esperado := range map[string]int{"": 3, "todos": 3, "aptos": 1, "no_aptos": 2} {
		lista, err := r.ObtenerCandidatosOferta(id, filtro)
		if err != nil || len(lista) != esperado {
			t.Errorf("filtro %q: %d candidatos, %v; quería %d", filtro, len(lista), err, esperado)
		}
	}
	if _, err := r.ObtenerCandidatosOferta(id, "raro"); !errors.Is(err, ErrFiltroInvalid) {
		t.Errorf("filtro inválido: %v", err)
	}
	// Una vacante sin análisis previo es "no encontrada": el servidor la responde como lista vacía.
	if _, err := r.ObtenerCandidatosOferta("Otra_vacante", ""); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("vacante sin datos: %v", err)
	}
}

func TestRepositorioBaseNoOcultaLosErroresDeLaBase(t *testing.T) {
	r := repositorioConBase(t, &lectorFalso{err: errors.New("base caída")})
	if _, err := r.ListarOfertas(); err == nil {
		t.Error("ListarOfertas debe fallar si la base falla")
	}
	if _, err := r.ObtenerCandidatosOferta("x", ""); err == nil || errors.Is(err, ErrNoEncontrado) {
		t.Errorf("un fallo de la base no debe parecer 'vacante sin datos': %v", err)
	}
}

// La API completa (rutas y JSON) funciona igual cuando el repositorio es el de la base.
func TestAPILeeLosCandidatosDeLaBase(t *testing.T) {
	cfg, _ := montarDescargas(t)
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	ctrl.repositorio = NuevoRepositorioMySQL(NuevoRepositorioDisco(cfg.DirectorioDescargas), lectorConDosVacantes(), cfg.DirectorioDescargas)
	t.Cleanup(ctrl.Detener)
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	rr := pedir(h, "GET", "/api/vacantes/operador-buses-metro/candidatos?filtro=aptos", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rr.Code, rr.Body.String())
	}
	var lista []struct {
		Nombre string `json:"nombre"`
		Slug   string `json:"slug"`
		EsApto bool   `json:"es_apto"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &lista); err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].Nombre != "Carlos Perez" || lista[0].Slug != "carlos-perez" || !lista[0].EsApto {
		t.Errorf("candidatos aptos = %+v", lista)
	}
	if rr.Header().Get("X-Total-Count") != "1" {
		t.Errorf("X-Total-Count = %q", rr.Header().Get("X-Total-Count"))
	}

	rr = pedir(h, "GET", "/api/vacantes/operador-buses-metro/candidatos/carlos-perez", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("detalle: código %d: %s", rr.Code, rr.Body.String())
	}
	rr = pedir(h, "GET", "/api/vacantes/no-existe", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("vacante inexistente: código %d", rr.Code)
	}
}

// lectorConSinTexto cuenta qué lectura usa la API: las pantallas no deben leer el texto de los perfiles.
type lectorConSinTexto struct {
	*lectorFalso
	conTexto, sinTexto int
}

func (l *lectorConSinTexto) CandidatosDeOferta(ctx context.Context, id string) ([]modelos.Candidato, error) {
	l.conTexto++
	return l.lectorFalso.CandidatosDeOferta(ctx, id)
}

func (l *lectorConSinTexto) CandidatosDeOfertaSinTexto(ctx context.Context, id string) ([]modelos.Candidato, error) {
	l.sinTexto++
	return l.lectorFalso.CandidatosDeOferta(ctx, id)
}

func TestPantallasLeenCandidatosSinTextoDelPerfil(t *testing.T) {
	cfg, _ := montarDescargas(t)
	ctrl := NuevoControladorRRHH(cfg, NuevoRepositorioDisco(cfg.DirectorioDescargas), "")
	lector := &lectorConSinTexto{lectorFalso: lectorConDosVacantes()}
	ctrl.repositorio = NuevoRepositorioMySQL(NuevoRepositorioDisco(cfg.DirectorioDescargas), lector, cfg.DirectorioDescargas)
	t.Cleanup(ctrl.Detener)
	h := (&ServidorWeb{controlador: ctrl}).rutas()

	if rr := pedir(h, "GET", "/api/vacantes/operador-buses-metro/candidatos", nil); rr.Code != http.StatusOK {
		t.Fatalf("código %d", rr.Code)
	}
	if lector.sinTexto != 1 || lector.conTexto != 0 {
		t.Errorf("la lista debe leerse sin texto: sin=%d con=%d", lector.sinTexto, lector.conTexto)
	}
	pedir(h, "GET", "/api/vacantes/operador-buses-metro/candidatos?incluir_cv=1", nil)
	if lector.conTexto != 1 {
		t.Errorf("con incluir_cv=1 sí se lee el texto: con=%d", lector.conTexto)
	}
}

// Una sesión validada no vuelve a la base hasta vencer, y cerrar sesión la olvida al instante.
func TestSesionesVistas(t *testing.T) {
	var s sesionesVistas
	if _, ok := s.buscar("t"); ok {
		t.Fatal("vacía no debe encontrar nada")
	}
	s.guardar("t", almacen.Usuario{Correo: "a@b.co"})
	if u, ok := s.buscar("t"); !ok || u.Correo != "a@b.co" {
		t.Errorf("recién guardada: %+v %v", u, ok)
	}
	s.olvidar("t")
	if _, ok := s.buscar("t"); ok {
		t.Error("tras olvidarla no debe encontrarse")
	}
	s.vistas["v"] = sesionVista{hasta: time.Now().Add(-time.Second)}
	if _, ok := s.buscar("v"); ok {
		t.Error("una vencida no vale")
	}
}
