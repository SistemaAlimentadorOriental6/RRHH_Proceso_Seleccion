package utilidades

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// ExtraerTextoPDF extrae el contenido textual de una hoja de vida en PDF.
func ExtraerTextoPDF(rutaPDF string) (string, error) {
	if _, err := os.Stat(rutaPDF); os.IsNotExist(err) {
		return "", fmt.Errorf("archivo PDF no encontrado: %s", rutaPDF)
	}

	// 1. Extracción de alta fidelidad mediante PyMuPDF (fitz)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// La ruta viaja como argumento (sys.argv[1]) y no incrustada en el código: una ruta con
	// comillas o caracteres especiales ya no rompe el script.
	const cmdPython = `
import fitz, sys
sys.stdout.reconfigure(encoding='utf-8')
try:
    doc = fitz.open(sys.argv[1])
    texto = '\n'.join([p.get_text() for p in doc])
    print(texto)
except Exception as e:
    sys.exit(1)
`

	// En Linux suele existir solo «python3»; sin él la lectura caía al respaldo, que de un PDF comprimido
	// saca casi nada.
	for _, python := range []string{"python3", "python"} {
		if _, err := exec.LookPath(python); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, python, "-c", cmdPython, rutaPDF)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil && out.Len() > 50 {
			return limpiarTexto(out.String()), nil
		}
	}

	// 2. Respaldo en Go puro, para cuando no hay Python con PyMuPDF (p. ej. en el servidor). Antes aquí se
	// buscaba texto entre paréntesis en los bytes del archivo y de un PDF comprimido salía basura binaria.
	if texto, err := textoPDFNativo(rutaPDF); err == nil && textoPlausible(texto) {
		return limpiarTexto(texto), nil
	}

	return "", fmt.Errorf("no fue posible extraer texto del PDF: %s", rutaPDF)
}

// textoPDFNativo lee el texto de un PDF sin programas externos, página por página y en líneas, como lo hace
// PyMuPDF. La librería puede entrar en pánico con archivos dañados: se trata como error.
func textoPDFNativo(rutaPDF string) (texto string, err error) {
	defer func() {
		if r := recover(); r != nil {
			texto, err = "", fmt.Errorf("PDF ilegible: %v", r)
		}
	}()
	f, lector, err := pdf.Open(rutaPDF)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	for i := 1; i <= lector.NumPage() && b.Len() < 2<<20; i++ {
		b.WriteString(textoDePagina(lector.Page(i)))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// textoDePagina arma el texto con las líneas de la página; si no se pudo (fuentes raras, contenido roto) usa
// el texto plano de la librería, que no conserva los saltos de línea pero rescata las palabras.
func textoDePagina(p pdf.Page) string {
	if t := lineasDePagina(p); strings.TrimSpace(t) != "" {
		return t
	}
	return textoPlanoDePagina(p)
}

func textoPlanoDePagina(p pdf.Page) (texto string) {
	defer func() {
		if recover() != nil {
			texto = ""
		}
	}()
	texto, _ = p.GetPlainText(nil)
	return texto
}

// lineasDePagina agrupa las letras por su posición vertical, las ordena de izquierda a derecha y pone un
// espacio donde hay hueco entre ellas. Un hueco grande (otra columna u otra celda de una tabla) abre una
// línea nueva, así «Empresa: X» de una columna no se mezcla con el texto de la columna vecina.
func lineasDePagina(p pdf.Page) (texto string) {
	defer func() {
		if recover() != nil {
			texto = ""
		}
	}()
	var letras []pdf.Text
	for _, l := range p.Content().Text {
		if l.S != "" && l.S != "\n" && l.S != "\r" {
			letras = append(letras, l)
		}
	}
	sort.SliceStable(letras, func(i, j int) bool { return letras[i].Y > letras[j].Y }) // de arriba abajo

	var lineas [][]pdf.Text
	for _, l := range letras {
		tolerancia := math.Max(2, 0.45*l.FontSize)
		if n := len(lineas); n > 0 && math.Abs(lineas[n-1][0].Y-l.Y) <= tolerancia {
			lineas[n-1] = append(lineas[n-1], l)
		} else {
			lineas = append(lineas, []pdf.Text{l})
		}
	}

	var b strings.Builder
	for _, linea := range lineas {
		sort.SliceStable(linea, func(i, j int) bool { return linea[i].X < linea[j].X })
		for i, l := range linea {
			if i > 0 {
				ant := linea[i-1]
				cuerpo := math.Max(ant.FontSize, 1)
				hueco := l.X - (ant.X + ant.W)
				switch {
				case hueco > 5*cuerpo:
					b.WriteByte('\n')
				case hueco > 0.15*cuerpo && !strings.HasSuffix(b.String(), " ") && !strings.HasPrefix(l.S, " "):
					b.WriteByte(' ')
				}
			}
			b.WriteString(l.S)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// textoPlausible dice si lo extraído parece texto y no basura: casi todo letras, números, espacios y signos.
func textoPlausible(texto string) bool {
	total, buenos := 0, 0
	for _, r := range texto {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		total++
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			buenos++
		}
	}
	return total > 20 && buenos*10 >= total*9
}

func limpiarTexto(texto string) string {
	// Reemplazar caracteres no imprimibles y espacios redundantes
	reEsp := regexp.MustCompile(`[ \t]+`)
	reSaltos := regexp.MustCompile(`\n{3,}`)
	limpio := reEsp.ReplaceAllString(texto, " ")
	limpio = reSaltos.ReplaceAllString(limpio, "\n\n")

	// Remover caracteres de iconos Unicode privados de fuentes web
	var b strings.Builder
	for _, r := range limpio {
		if r >= 0xE000 && r <= 0xF8FF {
			continue // Zona de uso privado (iconos de Computrabajo)
		}
		b.WriteRune(r)
	}

	return strings.TrimSpace(b.String())
}

// SintetizarTextoCV condensa la información clave del CV (experiencia, cargos, preguntas)
// eliminando textos de interfaz y reduciendo drásticamente los tokens de entrada para la CPU.
func SintetizarTextoCV(texto string) string {
	lineas := strings.Split(texto, "\n")
	var lineasUtiles []string

	descartar := []string{
		"computrabajo", "descargar cv", "imprimir cv", "aviso de privacidad",
		"términos y condiciones", "todos los derechos reservados", "postulado",
		"información confidencial", "cookie", "política de tratamiento",
		"mi cuenta", "cerrar sesión", "ver teléfono",
	}

	for _, l := range lineas {
		lTrim := strings.TrimSpace(l)
		if len(lTrim) < 3 {
			continue
		}
		lMin := strings.ToLower(lTrim)

		omitir := false
		for _, d := range descartar {
			if strings.Contains(lMin, d) {
				omitir = true
				break
			}
		}
		if omitir {
			continue
		}

		lineasUtiles = append(lineasUtiles, lTrim)
	}

	res := strings.Join(lineasUtiles, "\n")
	if len(res) > 2000 {
		return res[:2000] + "..."
	}
	return res
}

// SintesisTecnica resume indicadores objetivos de transporte extraídos del CV.
type SintesisTecnica struct {
	ExperienciaPasajeros bool
	ExperienciaPesada    bool
	TieneMecanica        bool
	EmpresasClave        []string
	ExtractoLaboral      string
	// Trabajos son los bloques «Empresa / Cargo / Tiempo laborado» del CV; ExperienciaConduccion, si algún
	// cargo es de conductor, chofer u operador (sin saber aún de qué vehículo: eso lo decide la IA).
	Trabajos              []TrabajoCV
	ExperienciaConduccion bool
}

// AnalizarExperienciaCV extrae de forma determinista y ultrarrápida evidencias laborales del CV.
func AnalizarExperienciaCV(textoCV string) SintesisTecnica {
	textoMin := strings.ToLower(textoCV)

	s := SintesisTecnica{}

	// Búsqueda de palabras de transporte masivo / pasajeros
	terminosPasajeros := []string{"bus", "buses", "alimentador", "alimentadores", "microbus", "microbús", "pasajeros", "servicio publico", "servicio público", "padron", "padrón", "metro"}
	for _, term := range terminosPasajeros {
		if strings.Contains(textoMin, term) {
			s.ExperienciaPasajeros = true
			break
		}
	}

	// Búsqueda de vehículos pesados y carga
	terminosPesados := []string{"camion", "camión", "c2", "c3", "tractocamion", "tractocamión", "vehiculos pesados", "vehículos pesados", "ditransa", "tcc", "carga pesada"}
	for _, term := range terminosPesados {
		if strings.Contains(textoMin, term) {
			s.ExperienciaPesada = true
			break
		}
	}

	// Búsqueda de formación técnica en mecánica
	if strings.Contains(textoMin, "mecanic") || strings.Contains(textoMin, "mecánic") || strings.Contains(textoMin, "automotriz") {
		s.TieneMecanica = true
	}

	// Detección de empresas relevantes
	empresasReferencia := []string{"Ditransa", "Metro", "Coonatra", "Bellanita", "Poblado", "TCC", "Servientrega", "Colectivos", "Flota", "Rápido Ochoa", "Sotrauraba"}
	for _, emp := range empresasReferencia {
		if strings.Contains(textoMin, strings.ToLower(emp)) {
			s.EmpresasClave = append(s.EmpresasClave, emp)
		}
	}

	s.Trabajos = TrabajosDeCV(textoCV)
	for _, t := range s.Trabajos {
		if EsCargoDeConduccion(t.Cargo) {
			s.ExperienciaConduccion = true
			break
		}
	}

	s.ExtractoLaboral = SintetizarTextoCV(textoCV)
	if len(s.ExtractoLaboral) > 1800 {
		s.ExtractoLaboral = s.ExtractoLaboral[:1800] + "..."
	}

	return s
}
