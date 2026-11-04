package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"go-rrhh/almacen"
)

// parametrosBase lo implementa almacen.Almacen: lee y guarda los parámetros de aceptación.
type parametrosBase interface {
	ParametrosVigentes(ctx context.Context) (almacen.ParametrosAceptacion, error)
	ParametrosVersion(ctx context.Context, version int) (almacen.ParametrosAceptacion, error)
	VersionesParametros(ctx context.Context) ([]almacen.VersionReglas, error)
	GuardarParametros(ctx context.Context, p almacen.ParametrosAceptacion, quien string) (almacen.ParametrosAceptacion, error)
}

// camposObligatorios son los que el PUT debe traer y no pueden ser null: un número ausente se leería como 0.
var camposObligatorios = []string{"edadMinima", "edadMaxima", "epsAceptadas", "aceptaReintegros", "generosAceptados",
	"licenciasAceptadas", "experienciaMinimaMeses", "runtAntiguedadMeses"}

// ManejarParametros atiende GET (lee los vigentes) y PUT (guarda una versión nueva) de /api/parametros-aceptacion.
func (c *ControladorRRHH) ManejarParametros(w http.ResponseWriter, r *http.Request) {
	if !c.hayParametros(w) {
		return
	}
	var (
		p   almacen.ParametrosAceptacion
		err error
	)
	if r.Method == http.MethodPut {
		if p, err = leerParametros(w, r); err != nil {
			ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		quien := ""
		if u, ok := c.usuarioDe(r); ok {
			quien = u.Correo
		}
		p, err = c.parametros.GuardarParametros(r.Context(), p, quien)
	} else {
		p, err = c.parametros.ParametrosVigentes(r.Context())
	}
	responderParametros(w, p, err)
}

// ManejarVersionesParametros lista las versiones de reglas (GET /api/parametros-aceptacion/versiones).
func (c *ControladorRRHH) ManejarVersionesParametros(w http.ResponseWriter, r *http.Request) {
	if !c.hayParametros(w) {
		return
	}
	lista, err := c.parametros.VersionesParametros(r.Context())
	if err != nil {
		log.Printf("[!] Error listando las versiones de reglas: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible leer las versiones de reglas"})
		return
	}
	ResponderJSON(w, http.StatusOK, lista)
}

// ManejarVersionParametros devuelve las reglas de una versión (GET /api/parametros-aceptacion/versiones/{version}):
// con ellas se evaluó a quien muestra ese número.
func (c *ControladorRRHH) ManejarVersionParametros(w http.ResponseWriter, r *http.Request) {
	if !c.hayParametros(w) {
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || version <= 0 {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "versión inválida"})
		return
	}
	p, err := c.parametros.ParametrosVersion(r.Context(), version)
	responderParametros(w, p, err)
}

func (c *ControladorRRHH) hayParametros(w http.ResponseWriter) bool {
	if c.parametros == nil {
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Los parámetros se guardan en la base de datos, que no está configurada"})
		return false
	}
	return true
}

// leerParametros decodifica el cuerpo del PUT y exige todos los campos: "nada puede ir vacío".
func leerParametros(w http.ResponseWriter, r *http.Request) (almacen.ParametrosAceptacion, error) {
	var p almacen.ParametrosAceptacion
	invalida := errors.New("solicitud inválida: falta algún campo o tiene un tipo incorrecto")
	cuerpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	if err != nil {
		return p, invalida
	}
	var crudo map[string]json.RawMessage
	if err := json.Unmarshal(cuerpo, &crudo); err != nil {
		return p, invalida
	}
	for _, campo := range camposObligatorios {
		if v, ok := crudo[campo]; !ok || string(v) == "null" {
			return p, errors.New("solicitud inválida: falta el campo " + campo)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(cuerpo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, invalida
	}
	return p, nil
}

func responderParametros(w http.ResponseWriter, p almacen.ParametrosAceptacion, err error) {
	switch {
	case errors.Is(err, almacen.ErrParametrosInvalidos):
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, almacen.ErrVersionInexistente):
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, almacen.ErrMigracionPendiente):
		ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	case err != nil:
		log.Printf("[!] Error con los parámetros de aceptación: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "No fue posible leer o guardar los parámetros en la base de datos"})
	default:
		ResponderJSON(w, http.StatusOK, p)
	}
}
