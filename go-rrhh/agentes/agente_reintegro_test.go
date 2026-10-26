package agentes

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

// servidorAdministremos responde como el servicio SOAP: el JSON va escapado dentro de <return>.
func servidorAdministremos(t *testing.T, paginas map[string]string) (*configuracion.Configuracion, *int) {
	t.Helper()
	llamadas := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas++
		cuerpo := new(strings.Builder)
		_, _ = fmt.Fprint(cuerpo, "")
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		peticion := string(buf[:n])
		if !strings.Contains(peticion, "<tabla xsi:type=\"xsd:string\">informacion_empleados</tabla>") || !strings.Contains(peticion, "<UserName>5</UserName>") {
			t.Errorf("petición inesperada: %s", peticion)
		}
		pagina := "1"
		if strings.Contains(peticion, "<paginaActual>2</paginaActual>") {
			pagina = "2"
		}
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><SOAP-ENV:Envelope><SOAP-ENV:Body><ns1:getInfoTablasBasicasPaginadasResponse><return xsi:type="xsd:string">%s</return></ns1:getInfoTablasBasicasPaginadasResponse></SOAP-ENV:Body></SOAP-ENV:Envelope>`,
			html.EscapeString(paginas[pagina]))
	}))
	t.Cleanup(srv.Close)
	return &configuracion.Configuracion{
		AdministremosURL: srv.URL, AdministremosUsuario: "5", AdministremosClave: "c", AdministremosKey: "k", AdministremosToken: "t", AdministremosIDEmpresa: 1,
	}, &llamadas
}

func TestReintegroRecorreLasPaginasYAvisa(t *testing.T) {
	cfg, llamadas := servidorAdministremos(t, map[string]string{
		"1": `{"code":"00","response":[
			{"nro_identificacion":111,"estado_empleado":"INACTIVO","cargo":"CONDUCTOR","fecha_ingreso":"2020-01-10","fecha_retiro":"2022-05-01","sanciones":"","total_paginas":"2"},
			{"nro_identificacion":222,"estado_empleado":"ACTIVO","cargo":"OPERADOR","fecha_ingreso":"2023-03-01","fecha_retiro":null,"total_paginas":"2"}]}`,
		"2": `{"code":"00","response":[
			{"nro_identificacion":"333","estado_empleado":"INACTIVO","cargo":"AUXILIAR","fecha_ingreso":"2019-01-01","fecha_retiro":"2020-01-01","sanciones":"Llamado de atención","total_paginas":"2"},
			{"nro_identificacion":333,"estado_empleado":"INACTIVO","cargo":"CONDUCTOR","fecha_ingreso":"2021-01-01","fecha_retiro":"2021-12-31","total_paginas":"2"}]}`,
	})
	a := NuevoAgenteReintegro(cfg)
	ctx := context.Background()

	casos := []struct {
		cedula, estado string
		alerta         bool
		contiene       string
	}{
		{"111", modelos.ReintegroExEmpleado, true, "retiro 2022-05-01"},
		{"1.1.1", modelos.ReintegroExEmpleado, true, "cargo CONDUCTOR"}, // formato con puntos
		{"222", modelos.ReintegroEmpleadoActivo, true, "ACTIVO"},
		{"333", modelos.ReintegroExEmpleado, true, "2 registros"}, // reingresó: gana el más reciente
		{"999", modelos.ReintegroSinRegistro, false, "No registra"},
	}
	for _, c := range casos {
		r, err := a.ConsultarCedula(ctx, c.cedula)
		if err != nil || r.Estado != c.estado || r.Alerta != c.alerta || !strings.Contains(r.Detalles, c.contiene) {
			t.Errorf("cédula %s: %+v (err %v), se esperaba %s con %q", c.cedula, r, err, c.estado, c.contiene)
		}
	}
	if *llamadas != 2 {
		t.Errorf("la tabla debe bajarse una sola vez (2 páginas); llamadas = %d", *llamadas)
	}
}

func TestReintegroErrorDelServicioNoEsSinRegistro(t *testing.T) {
	cfg, _ := servidorAdministremos(t, map[string]string{"1": `{"code":"90","response":"Firmas inválidas."}`})
	r, err := NuevoAgenteReintegro(cfg).ConsultarCedula(context.Background(), "111")
	if err == nil || r.Estado != modelos.ReintegroErrorConsulta {
		t.Fatalf("un fallo del servicio no puede decir que la persona no figura: %+v, %v", r, err)
	}
	if !strings.Contains(r.Detalles, "90") {
		t.Errorf("el detalle debe traer el código del servicio: %q", r.Detalles)
	}
}
