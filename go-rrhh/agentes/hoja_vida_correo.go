package agentes

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"go-rrhh/utilidades"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Funciones puras (sin red) que deciden qué correos son hojas de vida y si la persona busca ser
// conductor. Se mantienen aparte del cliente IMAP para poder probarlas con textos de ejemplo.

var quitarTildes = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

var reNoAlfanumerico = regexp.MustCompile(`[^a-z0-9]+`)

// normalizarTexto pasa a minúsculas, quita tildes y deja solo letras, números y espacios simples:
// "HOJA_DE_VIDA-Juan.pdf" → "hoja de vida juan pdf".
func normalizarTexto(s string) string {
	sin, _, err := transform.String(quitarTildes, strings.ToLower(s))
	if err != nil {
		sin = strings.ToLower(s)
	}
	return strings.TrimSpace(reNoAlfanumerico.ReplaceAllString(sin, " "))
}

// distancia es la distancia de Levenshtein entre dos cadenas ASCII (ya normalizadas).
func distancia(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		act := make([]int, len(b)+1)
		act[0] = i
		for j := 1; j <= len(b); j++ {
			coste := 1
			if a[i-1] == b[j-1] {
				coste = 0
			}
			act[j] = min(prev[j]+1, act[j-1]+1, prev[j-1]+coste)
		}
		prev = act
	}
	return prev[len(b)]
}

// Términos que delatan una hoja de vida. Los de una sola entrada exacta (hv, cv) son tan cortos que
// no admiten errores de digitación; los largos toleran hasta 2 letras mal escritas.
var (
	frasesHojaDeVida = []string{"hojadevida", "hojasdevida", "hojavida", "curriculum", "curriculumvitae", "curriculo", "resume"}
	palabrasExactas  = map[string]bool{"hv": true, "cv": true, "hdv": true}
	// Términos de búsqueda de empleo: solos no prueban que haya una hoja de vida, pero un correo que
	// los menciona merece revisarse (la gente escribe "adjunto mis datos para la vacante").
	// Se omiten palabras demasiado comunes en boletines y publicidad ("oportunidad", "oferta", "selección").
	palabrasEmpleo = []string{"postulacion", "postularme", "postulo", "postulante", "aplicar", "aplico", "vacante", "empleo", "trabajo", "solicito", "conductor", "operador", "chofer"}
)

// mencionaHojaDeVida dice si el texto nombra una hoja de vida o algo parecido, tolerando errores
// de digitación ("hoja de bida", "ojas de vida", "curiculum") y variantes pegadas o con guiones.
// Devuelve el término que coincidió.
func mencionaHojaDeVida(texto string) (string, bool) {
	tokens := strings.Fields(normalizarTexto(texto))
	for i := range tokens {
		// Ventanas de 1 a 4 palabras unidas: "hoja de vida" → "hojadevida", "H.V." → "hv".
		// Los números pegados al final ("hojadevida2025.pdf", "cv2") no cuentan.
		union := ""
		for n := 0; n < 4 && i+n < len(tokens); n++ {
			union += tokens[i+n]
			if len(union) > 20 {
				break
			}
			union := strings.TrimRight(union, "0123456789")
			if palabrasExactas[union] {
				return strings.Join(tokens[i:i+n+1], " "), true
			}
			for _, f := range frasesHojaDeVida {
				if abs(len(union)-len(f)) <= 2 && distancia(union, f) <= tolerancia(f) {
					return strings.Join(tokens[i:i+n+1], " "), true
				}
			}
		}
	}
	return "", false
}

func tolerancia(f string) int {
	if len(f) <= 6 {
		return 1
	}
	return 2
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// mencionaEmpleo dice si el texto habla de buscar trabajo (postulación, vacante, conductor…).
func mencionaEmpleo(texto string) (string, bool) {
	for _, tok := range strings.Fields(normalizarTexto(texto)) {
		for _, p := range palabrasEmpleo {
			// Solo las palabras largas toleran una letra de más o de menos ("postulasion"); en las
			// cortas eso confunde "solicita" con "solicito" y deja pasar publicidad.
			if tok == p || tok == p+"s" || (len(p) >= 10 && distancia(tok, p) <= 1) {
				return tok, true
			}
		}
	}
	return "", false
}

var marcadoresCV = []string{
	"experiencia laboral", "experiencia", "formacion academica", "formacion", "educacion", "estudios",
	"referencias", "perfil", "datos personales", "fecha de nacimiento", "cedula", "identificacion",
	"telefono", "celular", "habilidades", "idiomas", "objetivo", "competencias", "empresa", "cargo",
}

// datosPersonalesCV son secciones que solo una hoja de vida (no un boletín) trae.
var datosPersonalesCV = []string{"cedula", "identificacion", "fecha de nacimiento", "datos personales", "referencias", "estado civil", "lugar de nacimiento"}

// pareceHojaDeVida exige varias secciones típicas (mín. 4) y al menos un dato personal: un boletín
// de recursos humanos también habla de "experiencia", "perfil" y "cargo", pero no trae cédula ni referencias.
func pareceHojaDeVida(texto string) bool {
	n := normalizarTexto(texto)
	hallados := 0
	for _, m := range marcadoresCV {
		if strings.Contains(n, m) {
			hallados++
		}
	}
	if hallados < 4 {
		return false
	}
	for _, p := range datosPersonalesCV {
		if strings.Contains(n, p) {
			return true
		}
	}
	return false
}

var (
	reCedulaConPuntos = regexp.MustCompile(`\b\d{1,3}(?:\.\d{3}){2,3}\b`)
	reDatosPersonales = regexp.MustCompile(`(?i)datos\s+personales`)
	reNumeroSuelto    = regexp.MustCompile(`\d[\d.\s]{6,12}\d`)
	reDineroCerca     = regexp.MustCompile(`(?i)(\$|salario|sueldo|aspiraci|millon|pesos|cop)\s*$`)
)

// cedulaSinEtiqueta busca la cédula cuando la hoja de vida no la rotula ("C.C."). Dos pistas:
// el primer número de 8 a 10 dígitos que sigue al encabezado «Datos personales», o un número con
// puntos de miles (1.037.123.456) que no sea dinero. Es menos fiable que la rotulada y por eso solo
// se usa si esa no aparece. Los celulares no cuentan.
func cedulaSinEtiqueta(texto string) string {
	valido := func(d string) bool {
		if len(d) < 8 || len(d) > 10 {
			return false
		}
		celulares, fijos := utilidades.TelefonosEnTexto(d, "")
		return len(celulares)+len(fijos) == 0
	}
	if loc := reDatosPersonales.FindStringIndex(texto); loc != nil {
		resto := texto[loc[1]:min(loc[1]+200, len(texto))]
		if m := reNumeroSuelto.FindString(resto); m != "" && valido(soloDigitos(m)) {
			return soloDigitos(m)
		}
	}
	for _, loc := range reCedulaConPuntos.FindAllStringIndex(texto, -1) {
		antes := texto[max(0, loc[0]-20):loc[0]]
		if reDineroCerca.MatchString(antes) {
			continue
		}
		if d := soloDigitos(texto[loc[0]:loc[1]]); valido(d) {
			return d
		}
	}
	return ""
}

var reCampoFormulario = regexp.MustCompile(`(?im)^[ \t]*(nombres?|email|correo|e-mail|area|mensaje|telefono|tel[eé]fono|celular|c[eé]dula)[ \t]{2,}(\S.*?)[ \t]*$`)

// camposFormulario lee los avisos de formularios web ("nombre   Juan Pérez", "email   juan@…"): el
// remitente del correo es el formulario, no la persona, así que sus datos están en el cuerpo.
func camposFormulario(cuerpo string) map[string]string {
	campos := map[string]string{}
	for _, m := range reCampoFormulario.FindAllStringSubmatch(cuerpo, -1) {
		etiqueta := normalizarTexto(m[1])
		if _, ya := campos[etiqueta]; !ya {
			campos[etiqueta] = strings.TrimSpace(m[2])
		}
	}
	return campos
}

// necesitaLecturaConIA dice si el texto sacado de un PDF no sirve (escaneo o foto): casi vacío, o
// sin estructura de hoja de vida ni datos de contacto. La extracción de respaldo de PDF devuelve
// restos de código en lugar de nada, así que el tamaño solo no basta.
func necesitaLecturaConIA(texto string) bool {
	if len(strings.TrimSpace(texto)) < 80 {
		return true
	}
	c := utilidades.ExtraerContacto(texto, "")
	return !pareceHojaDeVida(texto) && c.Telefono == "" && c.Correo == "" && cedulaEnTexto(texto) == ""
}

// esDocumentoCV dice si un adjunto puede ser una hoja de vida por su extensión.
func esDocumentoCV(nombre string) bool {
	switch strings.ToLower(filepath.Ext(nombre)) {
	case ".pdf", ".doc", ".docx", ".odt", ".rtf", ".txt":
		return true
	}
	return false
}

// esImagen dice si un adjunto es una foto/escaneo (no se puede leer sin OCR).
func esImagen(nombre string) bool {
	switch strings.ToLower(filepath.Ext(nombre)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".bmp", ".tif", ".tiff":
		return true
	}
	return false
}

// Valores de InteresConductor.
const (
	ConductorSi     = "si"
	ConductorDudoso = "dudoso"
	ConductorNo     = "no"
)

// Señales de que la persona conduce o busca conducir un vehículo de transporte.
var (
	reIntencion = regexp.MustCompile(`\b(conductor|conductora|chofer|operador de bus|operadora de bus|operador de buses|operador alimentador|operador de vehiculo|operador de transporte|operador de flota|conduccion de bus|manejo de bus)`)
	reOficio    = regexp.MustCompile(`\b(conductor|conductora|chofer|operador de bus|operador de buses|operador alimentador|operador de vehiculo|operador de transporte|conductor profesional|conductor de bus|conductor de buses|conductor de carga|conductor de taxi)`)
	// Licencia de conducción de servicio público/pesado: C1, C2, C3, B2, B3 (también "categoria c2").
	reLicenciaPro = regexp.MustCompile(`\b(?:licencia[^.\n]{0,60}|categoria\s+)?(c1|c2|c3|b2|b3)\b`)
	reLicencia    = regexp.MustCompile(`\blicencia (?:de )?(?:conduccion|conducir|transito)\b`)
	reTransporte  = regexp.MustCompile(`\b(transporte de pasajeros|transporte publico|servicio publico|transporte masivo|sitp|alimentador|buseta|microbus|bus urbano|buses|ruta urbana|ruta escolar|flota|vehiculos pesados|tractomula|tractocamion)\b`)
	reConduccion  = regexp.MustCompile(`\b(conducir|conduccion|manejo de vehiculo|manejando|manejar bus|manejo de bus)\b`)
)

// EvaluarInteresConductor decide, con reglas, si quien envía la hoja de vida quiere (o puede) trabajar
// como conductor u operador de vehículos. asunto y cuerpo son lo que escribió la persona en el correo
// (la señal más directa de a qué cargo se postula); cv es el texto de la hoja de vida.
//
//	"si"     — lo dice en el correo ("postulación conductor") o su hoja de vida tiene al menos dos señales
//	           (cargo de conductor/operador, licencia profesional, experiencia en transporte).
//	"dudoso" — una sola señal débil (p. ej. solo "licencia de conducción", que casi todos tienen); una IA
//	           o una persona deben decidir.
//	"no"     — ninguna señal.
//
// Devuelve además la razón, en español, para mostrarla al reclutador.
func EvaluarInteresConductor(asunto, cuerpo, cv string) (veredicto, motivo string) {
	mensaje := normalizarTexto(asunto + " " + cuerpo)
	if m := reIntencion.FindString(mensaje); m != "" {
		return ConductorSi, "El correo menciona «" + m + "»"
	}

	texto := normalizarTexto(cv)
	var senales []string
	if m := reOficio.FindString(texto); m != "" {
		senales = append(senales, "cargo o experiencia como «"+m+"»")
	}
	if m := reLicenciaPro.FindStringSubmatch(texto); m != nil {
		senales = append(senales, "licencia de categoría "+strings.ToUpper(m[1]))
	}
	if m := reTransporte.FindString(texto); m != "" {
		senales = append(senales, "experiencia en transporte («"+m+"»)")
	}
	if m := reConduccion.FindString(texto); m != "" && len(senales) == 0 {
		senales = append(senales, "menciona «"+m+"»")
	}
	if m := reLicencia.FindString(texto); m != "" && len(senales) == 0 {
		senales = append(senales, "tiene «"+m+"»")
	}

	switch {
	case len(senales) >= 2:
		return ConductorSi, "Hoja de vida con: " + strings.Join(senales, "; ")
	case len(senales) == 1:
		return ConductorDudoso, "Solo una señal: " + senales[0]
	}
	return ConductorNo, "Ni el correo ni la hoja de vida hablan de conducir ni de operar vehículos"
}

// textoCrudoDoc rescata texto legible de un .doc antiguo (formato binario de Word) sin librerías:
// busca tramos largos de caracteres imprimibles, en Latin-1 y en UTF-16. Es tosco (puede traer
// basura entre líneas) pero conserva cédula, teléfono, correo y palabras clave, que es lo que se busca.
// shortcut: no interpreta la estructura del .doc; usar una librería de Word si se necesita el texto fiel.
func textoCrudoDoc(datos []byte) string {
	var b strings.Builder
	tramo := func(runas []rune) {
		if len(runas) >= 4 {
			b.WriteString(string(runas))
			b.WriteByte('\n')
		}
	}
	imprimible := func(r rune) bool {
		return r == ' ' || r == '\t' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsPunct(r) || r == '@' || r == '+'
	}

	var actual []rune
	for _, c := range datos { // Latin-1: cada byte es un carácter
		if r := rune(c); imprimible(r) {
			actual = append(actual, r)
		} else {
			tramo(actual)
			actual = actual[:0]
		}
	}
	tramo(actual)

	actual = actual[:0]
	for i := 0; i+1 < len(datos); i += 2 { // UTF-16 little endian
		if r := rune(datos[i]) | rune(datos[i+1])<<8; imprimible(r) && datos[i+1] < 0x05 {
			actual = append(actual, r)
		} else {
			tramo(actual)
			actual = actual[:0]
		}
	}
	tramo(actual)
	return b.String()
}
