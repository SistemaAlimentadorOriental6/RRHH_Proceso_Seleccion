package agentes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
)

const (
	// vigenciaIndiceReintegro es cuánto se reutiliza la lista de empleados antes de volver a pedirla.
	vigenciaIndiceReintegro = 10 * time.Minute
	// maxPaginasReintegro frena un servicio que nunca deje de paginar.
	maxPaginasReintegro = 60
	tablaEmpleados      = "informacion_empleados"
)

// empleadoAdmin es lo que se conserva de cada fila de informacion_empleados: lo justo para el aviso.
type empleadoAdmin struct {
	Estado, Cargo, Contrato, Ingreso, Retiro, Sanciones string
}

// AgenteReintegro consulta el servicio web SOAP de Administremos y dice si una cédula ya figura como
// empleado. El servicio solo lista tablas por páginas (no filtra por cédula), así que se baja la tabla
// completa una vez y se reutiliza unos minutos en memoria, indexada por cédula.
type AgenteReintegro struct {
	cfg     *configuracion.Configuracion
	cliente *http.Client

	mu        sync.Mutex
	indice    map[string][]empleadoAdmin
	cargadoEn time.Time
}

func NuevoAgenteReintegro(cfg *configuracion.Configuracion) *AgenteReintegro {
	return &AgenteReintegro{cfg: cfg, cliente: &http.Client{Timeout: 45 * time.Second}}
}

// Disponible dice si hay credenciales de Administremos.
func (a *AgenteReintegro) Disponible() bool { return a.cfg != nil && a.cfg.AdministremosActivo() }

// Validar completa c.ResultadoReintegro con la cédula del candidato. Devuelve error solo si el servicio
// falló (el resultado queda como ERROR_CONSULTA).
func (a *AgenteReintegro) Validar(ctx context.Context, c *modelos.Candidato) error {
	r, err := a.ConsultarCedula(ctx, c.Cedula)
	c.ResultadoReintegro = r
	return err
}

// ConsultarCedula busca la cédula en la tabla de empleados de Administremos.
func (a *AgenteReintegro) ConsultarCedula(ctx context.Context, cedula string) (*modelos.ResultadoReintegro, error) {
	clave := normalizarCedula(cedula)
	if clave == "" {
		return &modelos.ResultadoReintegro{Estado: modelos.ReintegroErrorConsulta, Detalles: "Sin cédula: no se puede consultar Administremos"}, errors.New("sin cédula")
	}
	indice, err := a.indiceVigente(ctx)
	if err != nil {
		return &modelos.ResultadoReintegro{Estado: modelos.ReintegroErrorConsulta, Detalles: "No se pudo consultar Administremos: " + err.Error()}, err
	}
	return evaluarReintegro(indice[clave]), nil
}

// evaluarReintegro resume los registros de una cédula (más de uno si reingresó) en un aviso.
func evaluarReintegro(filas []empleadoAdmin) *modelos.ResultadoReintegro {
	if len(filas) == 0 {
		return &modelos.ResultadoReintegro{Estado: modelos.ReintegroSinRegistro, Detalles: "No registra como empleado en Administremos"}
	}
	// El registro más reciente (por fecha de ingreso, que viene como AAAA-MM-DD); uno activo manda.
	ultimo := filas[0]
	activo := false
	for _, f := range filas {
		if strings.EqualFold(f.Estado, "ACTIVO") {
			activo = true
			ultimo = f
			break
		}
		if f.Ingreso > ultimo.Ingreso {
			ultimo = f
		}
	}
	r := &modelos.ResultadoReintegro{
		Alerta: true, EstadoEmpleado: ultimo.Estado, Cargo: ultimo.Cargo, TipoContrato: ultimo.Contrato,
		FechaIngreso: ultimo.Ingreso, FechaRetiro: ultimo.Retiro, Sanciones: ultimo.Sanciones, Registros: len(filas),
	}
	datos := []string{}
	if ultimo.Cargo != "" {
		datos = append(datos, "cargo "+ultimo.Cargo)
	}
	if ultimo.Ingreso != "" {
		datos = append(datos, "ingreso "+ultimo.Ingreso)
	}
	if activo {
		r.Estado = modelos.ReintegroEmpleadoActivo
		r.Detalles = "Figura como empleado ACTIVO en Administremos"
	} else {
		r.Estado = modelos.ReintegroExEmpleado
		retiro := "sin fecha de retiro registrada"
		if ultimo.Retiro != "" {
			retiro = "retiro " + ultimo.Retiro
		}
		datos = append(datos, retiro)
		r.Detalles = "Reintegro: ya fue empleado"
	}
	if len(datos) > 0 {
		r.Detalles += " (" + strings.Join(datos, ", ") + ")"
	}
	if len(filas) > 1 {
		r.Detalles += fmt.Sprintf(" · %d registros", len(filas))
	}
	if ultimo.Sanciones != "" {
		r.Detalles += " · Con sanciones: " + recortar(ultimo.Sanciones, 160)
	}
	return r
}

func recortar(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// normalizarCedula deja solo los dígitos y sin ceros a la izquierda, para comparar sin importar el formato.
func normalizarCedula(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return strings.TrimLeft(b.String(), "0")
}

// indiceVigente devuelve la tabla de empleados indexada por cédula, bajándola si hace falta.
func (a *AgenteReintegro) indiceVigente(ctx context.Context) (map[string][]empleadoAdmin, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.indice != nil && time.Since(a.cargadoEn) < vigenciaIndiceReintegro {
		return a.indice, nil
	}
	indice := map[string][]empleadoAdmin{}
	for pagina := 1; pagina <= maxPaginasReintegro; pagina++ {
		filas, totalPaginas, err := a.pagina(ctx, pagina)
		if err != nil {
			if a.indice != nil {
				return a.indice, nil // el servicio falló pero hay una copia anterior: se usa esa
			}
			return nil, err
		}
		for _, f := range filas {
			if clave := normalizarCedula(texto(f["nro_identificacion"])); clave != "" {
				indice[clave] = append(indice[clave], empleadoAdmin{
					Estado: texto(f["estado_empleado"]), Cargo: texto(f["cargo"]), Contrato: texto(f["tipo_contrato"]),
					Ingreso: texto(f["fecha_ingreso"]), Retiro: texto(f["fecha_retiro"]), Sanciones: texto(f["sanciones"]),
				})
			}
		}
		if pagina >= totalPaginas {
			break
		}
	}
	a.indice, a.cargadoEn = indice, time.Now()
	return indice, nil
}

// texto convierte un valor del JSON del servicio (cadena, número o nulo) en texto.
func texto(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// pagina pide una página de la tabla de empleados y devuelve sus filas y el total de páginas.
func (a *AgenteReintegro) pagina(ctx context.Context, n int) ([]map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.AdministremosURL, bytes.NewReader(a.sobre(tablaEmpleados, n)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "urn:AdministremosControllerwsdl#getInfoTablasBasicasPaginadas")
	resp, err := a.cliente.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("sin respuesta del servicio: %w", err)
	}
	defer resp.Body.Close()
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("el servicio respondió %d", resp.StatusCode)
	}
	return leerRespuestaAdministremos(cuerpo)
}

// sobre arma la petición SOAP (RPC) de getInfoTablasBasicasPaginadas, con las credenciales en el encabezado.
func (a *AgenteReintegro) sobre(tabla string, pagina int) []byte {
	esc := html.EscapeString
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:AdministremosControllerwsdl">
<soapenv:Header><Auth xmlns="http://cmd-it.dk/"><UserName>%s</UserName><Password>%s</Password><key>%s</key><token>%s</token></Auth></soapenv:Header>
<soapenv:Body><urn:getInfoTablasBasicasPaginadas soapenv:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<id_empresa xsi:type="xsd:int">%d</id_empresa><id_version_pesv xsi:type="xsd:int">1</id_version_pesv><tabla xsi:type="xsd:string">%s</tabla>
<paginacion xmlns="http://cmd-it.dk/"><paginaActual>%d</paginaActual></paginacion>
</urn:getInfoTablasBasicasPaginadas></soapenv:Body></soapenv:Envelope>`,
		esc(a.cfg.AdministremosUsuario), esc(a.cfg.AdministremosClave), esc(a.cfg.AdministremosKey), esc(a.cfg.AdministremosToken),
		a.cfg.AdministremosIDEmpresa, esc(tabla), pagina)
}

// leerRespuestaAdministremos saca el JSON de <return> ({"code":"00","response":[...]}). Se lee como texto y
// no con el decodificador XML porque el servicio a veces mezcla codificaciones y este rechazaría el cuerpo.
func leerRespuestaAdministremos(cuerpo []byte) ([]map[string]any, int, error) {
	if !utf8.Valid(cuerpo) {
		runas := make([]rune, len(cuerpo))
		for i, b := range cuerpo {
			runas[i] = rune(b) // Latin-1
		}
		cuerpo = []byte(string(runas))
	}
	s := string(cuerpo)
	ini := strings.Index(s, "<return")
	if ini < 0 {
		return nil, 0, errors.New("respuesta sin <return>")
	}
	ini = strings.Index(s[ini:], ">") + ini + 1
	fin := strings.Index(s[ini:], "</return>")
	if fin < 0 {
		return nil, 0, errors.New("respuesta truncada")
	}
	var r struct {
		Code     string          `json:"code"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(s[ini:ini+fin])), &r); err != nil {
		return nil, 0, fmt.Errorf("respuesta ilegible: %w", err)
	}
	if r.Code != "00" {
		var motivo string
		_ = json.Unmarshal(r.Response, &motivo)
		return nil, 0, fmt.Errorf("Administremos respondió código %s: %s", r.Code, recortar(motivo, 120))
	}
	var filas []map[string]any
	if err := json.Unmarshal(r.Response, &filas); err != nil {
		return nil, 0, fmt.Errorf("respuesta inesperada: %w", err)
	}
	total := 1
	if len(filas) > 0 {
		if n, err := strconv.Atoi(texto(filas[0]["total_paginas"])); err == nil && n > 0 {
			total = n
		}
	}
	return filas, total, nil
}
