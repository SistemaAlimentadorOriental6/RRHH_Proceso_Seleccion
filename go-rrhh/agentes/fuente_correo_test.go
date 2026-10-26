package agentes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

func TestFuenteCorreo(t *testing.T) {
	cfg := &configuracion.Configuracion{DirectorioDescargas: t.TempDir()}
	ag := NuevoAgenteCorreo(cfg, nil)
	t0 := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	hojas := []modelos.HojaVidaCorreo{
		{UID: 2, MessageID: "<b@x>", Fecha: t0.Add(time.Hour), Nombre: "Carlos Ruiz", Cedula: "1037123456", Correo: "c@x.com", Telefono: "3145515276", TextoCV: "Conductor categoría C2", InteresConductor: ConductorSi},
		{UID: 1, MessageID: "<a@x>", Fecha: t0, NombreRemitente: "Ana Gómez", Correo: "a@x.com", InteresConductor: ConductorDudoso},
		{UID: 3, MessageID: "<c@x>", Fecha: t0.Add(2 * time.Hour), Nombre: "Pedro Soto", InteresConductor: ConductorNo},
	}
	if err := utilidades.EscribirJSONAtomico(filepath.Join(cfg.DirectorioDescargas, "correo", "hojas_de_vida.json"), hojas, false); err != nil {
		t.Fatal(err)
	}

	var vistos []*modelos.Candidato
	if err := ag.ObtenerCandidatosStream(context.Background(), "", 0, func(c *modelos.Candidato) { vistos = append(vistos, c) }); err != nil {
		t.Fatal(err)
	}
	// Los «no» no entran; los demás salen del más antiguo al más nuevo con números de orden fijos.
	if len(vistos) != 2 || vistos[0].Nombre != "Ana Gómez" || vistos[0].Indice != 1 || vistos[1].Nombre != "Carlos Ruiz" || vistos[1].Indice != 2 {
		t.Fatalf("candidatos inesperados: %+v", vistos)
	}
	if vistos[0].URLDetalle == vistos[1].URLDetalle || vistos[0].URLDetalle[:7] != "correo:" {
		t.Errorf("las fichas deben ser únicas y empezar por correo: %q %q", vistos[0].URLDetalle, vistos[1].URLDetalle)
	}

	c := vistos[1]
	if err := ag.ExtraerDetalleCandidato(context.Background(), c, ""); err != nil {
		t.Fatal(err)
	}
	if c.Cedula != "1037123456" {
		t.Errorf("cédula = %q", c.Cedula)
	}
	// El contacto debe poder leerse del texto de perfil, que es de donde lo toma el flujo de datos faltantes.
	if ct := utilidades.ExtraerContacto(c.TextoPerfil, ""); ct.Correo != "c@x.com" || ct.Telefono != "3145515276" {
		t.Errorf("contacto no recuperable del perfil: %+v (%q)", ct, c.TextoPerfil)
	}
	if err := ag.ExtraerDetalleCandidato(context.Background(), &modelos.Candidato{URLDetalle: "correo:inexistente"}, ""); err == nil {
		t.Error("una hoja que ya no existe debe dar error")
	}

	if limitado := (func() int {
		n := 0
		_ = ag.ObtenerCandidatosStream(context.Background(), "", 1, func(*modelos.Candidato) { n++ })
		return n
	})(); limitado != 1 {
		t.Errorf("con límite 1 llegaron %d", limitado)
	}
	_ = os.Remove(filepath.Join(cfg.DirectorioDescargas, "correo", "hojas_de_vida.json"))
}
