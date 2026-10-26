package almacen

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrCampos son los errores de validación de un formulario, por nombre de campo, para que la
// pantalla los muestre junto al campo que corresponde.
type ErrCampos map[string]string

func (e ErrCampos) Error() string {
	claves := make([]string, 0, len(e))
	for k := range e {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	partes := make([]string, len(claves))
	for i, k := range claves {
		partes[i] = k + ": " + e[k]
	}
	return "datos inválidos (" + strings.Join(partes, "; ") + ")"
}

// DatosValidados es lo que escribió el candidato, ya normalizado y comprobado.
type DatosValidados struct {
	Cedula          string    // solo dígitos, entre 6 y 12
	FechaNacimiento time.Time // fecha sin hora, en UTC
	Edad            int       // años cumplidos al momento de validar
}

// edadEn calcula los años cumplidos de quien nació en nacimiento, a la fecha ahora.
func edadEn(nacimiento, ahora time.Time) int {
	anios := ahora.Year() - nacimiento.Year()
	if ahora.Month() < nacimiento.Month() || (ahora.Month() == nacimiento.Month() && ahora.Day() < nacimiento.Day()) {
		anios--
	}
	return anios
}

// ValidarDatosCandidato normaliza y comprueba lo que escribe el candidato en el formulario:
// la cédula (6 a 12 dígitos; se aceptan puntos y espacios) y la fecha de nacimiento (AAAA-MM-DD),
// que debe ser pasada, plausible y de una persona con al menos edadMinima años.
func ValidarDatosCandidato(cedula, fechaNacimiento string, edadMinima int, ahora time.Time) (DatosValidados, error) {
	errores := ErrCampos{}
	var d DatosValidados

	limpia := strings.NewReplacer(".", "", ",", "", " ", "", "\t", "").Replace(cedula)
	solo := soloDigitos(limpia)
	if len(solo) != len(limpia) || len(solo) < 6 || len(solo) > 12 {
		errores["cedula"] = "Ingresa solo los números de tu cédula (entre 6 y 12 dígitos)"
	} else {
		d.Cedula = solo
	}

	nacimiento, err := time.Parse("2006-01-02", strings.TrimSpace(fechaNacimiento))
	switch {
	case err != nil:
		errores["fecha_nacimiento"] = "Ingresa una fecha de nacimiento válida"
	case nacimiento.After(ahora):
		errores["fecha_nacimiento"] = "La fecha de nacimiento no puede ser futura"
	default:
		d.FechaNacimiento = nacimiento
		d.Edad = edadEn(nacimiento, ahora.UTC())
		switch {
		case d.Edad > 100:
			errores["fecha_nacimiento"] = "Ingresa una fecha de nacimiento válida"
		case d.Edad < edadMinima:
			errores["fecha_nacimiento"] = fmt.Sprintf("Debes tener al menos %d años para continuar con el proceso", edadMinima)
		}
	}

	if len(errores) > 0 {
		return DatosValidados{}, errores
	}
	return d, nil
}

// enmascararCorreo oculta casi toda la parte local ("da***@gmail.com"). La página pública lo
// muestra al candidato sin exponer el correo completo a quien solo tenga el enlace.
func enmascararCorreo(correo string) string {
	arroba := strings.LastIndex(correo, "@")
	if arroba <= 0 {
		return ""
	}
	local, dominio := correo[:arroba], correo[arroba:]
	visibles := 2
	if len([]rune(local)) <= 2 {
		visibles = 1
	}
	return string([]rune(local)[:visibles]) + "***" + dominio
}
