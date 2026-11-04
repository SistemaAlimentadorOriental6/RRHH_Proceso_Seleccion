package agentes

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

var reGeneroPerfil = regexp.MustCompile(`(?i)(?:género|genero|sexo)\s*[:\-]?\s*(hombre|mujer|masculino|femenino)`)

// AgenteClasificador evalua los requisitos de idoneidad y segrega candidatos aptos.
type AgenteClasificador struct {
	cfg               *configuracion.Configuracion
	nombresMasculinos map[string]struct{}
	nombresFemeninos  map[string]struct{}
}

// NuevoAgenteClasificador inicializa el evaluador con diccionarios foneticos y de nombres.
func NuevoAgenteClasificador(cfg *configuracion.Configuracion) *AgenteClasificador {
	masc := []string{
		"juan", "carlos", "victor", "jhon", "jairo", "yeferson", "yefferson", "julian", "jaider",
		"edwin", "andres", "jose", "pedro", "luis", "jorge", "miguel", "diego", "david",
		"fernando", "mateo", "santiago", "alejandro", "cristian", "daniel", "mario", "felipe",
		"oscar", "rodrigo", "camilo", "gabriel", "alvaro", "hernan", "fabio", "sebastian",
		"alexander", "brayan", "bryan", "kevin", "giovanny", "giovanni", "wilmar", "wilson", "rafael",
		"leonardo", "cesar", "mauricio", "wilkerson", "rufino", "johan", "arturo", "horacio",
		"manuel", "alberto", "hector", "gustavo", "guillermo", "hugo", "javier", "enrique",
		"eduardo", "ricardo", "sergio", "ivan", "raul", "ramon", "gonzalo", "alfonso",
		"jaime", "ignacio", "armando", "esteban", "federico", "antonio", "nelson", "ruben",
		"dario", "elkin", "edgar", "fredy", "freddy", "yeison", "jeison", "deimer", "duvan",
		"alex", "alexis", "walter", "frank", "miller", "didier", "elias", "emilio", "samuel", "nicolas",
		"lucas", "tomas", "martin", "joaquin", "maximiliano", "jeronimo", "matias", "simon",
		"jonathan", "jonatan", "jhonatan", "jhonathan", "danilo", "brian", "estiven", "stiven",
		"steven", "maicol", "michael", "albeiro", "alirio", "duber", "dumar", "yair", "harold",
		"henry", "william", "wilmer", "deivy", "deivid", "cristopher", "geovanny", "jhonny",
		"john", "norberto", "orlando", "oswaldo", "pablo", "patricio", "ramiro", "roberto",
		"rodolfo", "rogelio", "vicente", "wilfer", "wilfrido", "germán", "german", "misael",
	}

	fem := []string{
		"maria", "ana", "diana", "laura", "paola", "sandra", "claudia", "carolina", "daniela",
		"andrea", "valentina", "luisa", "natalia", "camila", "jessica", "marcela", "gloria",
		"patricia", "tatiana", "monica", "vanessa", "juliana", "angela", "adriana", "karen",
		"lorena", "liliana", "viviana", "leidy", "lady", "stefany", "stephanie", "catalina",
		"isabel", "sofia", "lucia", "clara", "elena", "carmen", "rosa", "teresa", "mercedes",
		"yuliana", "erika", "alejandra", "martha", "marta", "luz", "esperanza",
	}

	mapMasc := make(map[string]struct{})
	for _, n := range masc {
		mapMasc[n] = struct{}{}
	}

	mapFem := make(map[string]struct{})
	for _, n := range fem {
		mapFem[n] = struct{}{}
	}

	return &AgenteClasificador{
		cfg:               cfg,
		nombresMasculinos: mapMasc,
		nombresFemeninos:  mapFem,
	}
}

// GeneroSeguro devuelve el género solo si consta en el perfil o el nombre de pila está en el diccionario;
// "" si no se sabe. A diferencia de DeterminarGenero no adivina por la terminación del nombre: esos casos
// los decide la IA (AgenteIA.DeterminarGenero).
func (ac *AgenteClasificador) GeneroSeguro(nombre string, textoPerfil string) string {
	if m := reGeneroPerfil.FindStringSubmatch(textoPerfil); len(m) > 1 {
		if v := strings.ToLower(m[1]); v == "hombre" || v == "masculino" {
			return "Hombre"
		}
		return "Mujer"
	}
	palabras := strings.Fields(strings.ToLower(nombre))
	for i := 0; i < min(2, len(palabras)); i++ {
		if _, ok := ac.nombresMasculinos[palabras[i]]; ok {
			return "Hombre"
		}
		if _, ok := ac.nombresFemeninos[palabras[i]]; ok {
			return "Mujer"
		}
	}
	return ""
}

// DeterminarGenero clasifica el genero mediante perfil o diccionario de nombres.
func (ac *AgenteClasificador) DeterminarGenero(nombre string, textoPerfil string) string {
	if textoPerfil != "" {
		if m := reGeneroPerfil.FindStringSubmatch(textoPerfil); len(m) > 1 {
			val := strings.ToLower(m[1])
			if val == "hombre" || val == "masculino" {
				return "Hombre"
			}
			return "Mujer"
		}
	}

	palabras := strings.Fields(strings.ToLower(nombre))
	limite := 2
	if len(palabras) < limite {
		limite = len(palabras)
	}

	for i := 0; i < limite; i++ {
		p := palabras[i]
		if _, ok := ac.nombresMasculinos[p]; ok {
			return "Hombre"
		}
		if _, ok := ac.nombresFemeninos[p]; ok {
			return "Mujer"
		}
	}

	if len(palabras) > 0 && strings.HasSuffix(palabras[0], "o") {
		return "Hombre"
	}

	return "No especificado"
}

// ReglasDeConfiguracion son las reglas que se aplican cuando no hay base de datos de la que leerlas: las
// mismas de la versión 1 de reglas_idoneidad (las que el código aplicaba antes de los parámetros).
func ReglasDeConfiguracion(cfg *configuracion.Configuracion) modelos.ReglasIdoneidad {
	r := modelos.ReglasIdoneidad{
		EdadMinima: 22, EdadMaxima: 49,
		EpsAceptadas:        []string{"Sura", "Nueva EPS", "Savia Salud", "Sanitas"},
		GenerosAceptados:    []string{"masculino"},
		LicenciasAceptadas:  []string{"C2", "C3"},
		RuntAntiguedadMeses: 24,
	}
	if cfg != nil {
		r.EdadMinima, r.EdadMaxima = cfg.EdadMinimaContratacion, cfg.EdadMaximaContratacion
		if len(cfg.EPSAutorizadas) > 0 {
			r.EpsAceptadas = cfg.EPSAutorizadas
		}
	}
	return r
}

// ReglasDe son las reglas con las que se evalúa al candidato: las suyas o, si no tiene, las de la configuración.
func (ac *AgenteClasificador) ReglasDe(c *modelos.Candidato) *modelos.ReglasIdoneidad {
	if c != nil && c.Reglas != nil {
		return c.Reglas
	}
	r := ReglasDeConfiguracion(ac.cfg)
	return &r
}

// EvaluarEPS verifica que la EPS sea una de las aceptadas, sin importar si la afiliación figura activa o retirada.
func (ac *AgenteClasificador) EvaluarEPS(res *modelos.ResultadoAdres, reglas *modelos.ReglasIdoneidad) (bool, string) {
	if res == nil {
		return false, "Sin información de ADRES"
	}

	if res.Estado == "PORTAL_EN_MANTENIMIENTO" {
		return true, "ADRES en mantenimiento oficial (Pendiente validación EPS)"
	}

	if res.Estado == EstadoAdresNoAfiliado {
		return false, "No figura afiliado a ninguna EPS en ADRES (BDUA)"
	}

	// El estado de la afiliación (activo, retirado...) no importa: solo que la EPS sea una de las aceptadas.
	if !EPSAceptada(res.EPS, reglas.EpsAceptadas) {
		return false, fmt.Sprintf("EPS '%s' no autorizada (aceptadas: %s)", res.EPS, strings.Join(reglas.EpsAceptadas, ", "))
	}

	return true, fmt.Sprintf("EPS %s (%s)", res.EPS, res.EstadoAfiliacion)
}

// EvaluarRunt verifica el tiempo mínimo inscrito en el RUNT y una licencia activa de las categorías aceptadas.
func (ac *AgenteClasificador) EvaluarRunt(res *modelos.ResultadoRunt, reglas *modelos.ReglasIdoneidad) (bool, string) {
	if res == nil {
		return false, "Sin información de RUNT"
	}

	if res.Estado != "INSCRITO_ACTIVO" || !res.Inscrito {
		return false, fmt.Sprintf("No inscrito o inactivo en RUNT (%s)", res.Estado)
	}

	// 1. Antigüedad en RUNT, en meses completos
	antiguedadMeses := 0
	if res.FechaInscripcion != "" {
		limpio := strings.Split(res.FechaInscripcion, ".")[0]
		limpio = strings.ReplaceAll(limpio, "T", " ")
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, limpio); err == nil {
				antiguedadMeses = mesesCompletos(t, time.Now())
				break
			}
		}
	}

	// 2. Licencia activa en alguna de las categorías aceptadas
	aceptada := func(cat string) bool {
		cat = strings.ToUpper(strings.TrimSpace(cat))
		for _, l := range reglas.LicenciasAceptadas {
			if cat == strings.ToUpper(l) {
				return true
			}
		}
		return false
	}
	tieneCategoria := false
	for _, lic := range res.Licencias {
		if strings.ToUpper(lic.Estado) != "ACTIVA" {
			continue
		}
		for _, c := range lic.Categorias {
			tieneCategoria = tieneCategoria || aceptada(c)
		}
		for _, det := range lic.DetallesCategorias {
			tieneCategoria = tieneCategoria || aceptada(det.Categoria)
		}
		if tieneCategoria {
			break
		}
	}

	if !tieneCategoria {
		// CategoriasActivas trae textos como "C2 - CAMIONES RÍGIDOS": se mira cada palabra.
		for _, catAct := range res.CategoriasActivas {
			for _, p := range palabrasDe(catAct) {
				tieneCategoria = tieneCategoria || aceptada(p)
			}
		}
	}

	licencias := listaO(reglas.LicenciasAceptadas)
	if !tieneCategoria {
		return false, fmt.Sprintf("No posee licencia activa en categoría %s", licencias)
	}

	if antiguedadMeses < reglas.RuntAntiguedadMeses {
		return false, fmt.Sprintf("Antigüedad en RUNT insuficiente: %s (mínimo %s requerido)",
			modelos.TextoMeses(antiguedadMeses), modelos.TextoMeses(reglas.RuntAntiguedadMeses))
	}

	return true, fmt.Sprintf("RUNT OK: %s de registro | Licencia %s ACTIVA", modelos.TextoMeses(antiguedadMeses), licencias)
}

// listaO junta valores como "C1, C2 o C3".
func listaO(l []string) string {
	if len(l) < 2 {
		return strings.Join(l, "")
	}
	return strings.Join(l[:len(l)-1], ", ") + " o " + l[len(l)-1]
}

// mesesCompletos cuenta los meses cumplidos entre desde y hasta (15/01 → 14/03 son 1 mes; → 15/03 son 2).
func mesesCompletos(desde, hasta time.Time) int {
	m := (hasta.Year()-desde.Year())*12 + int(hasta.Month()) - int(desde.Month())
	if hasta.Day() < desde.Day() {
		m--
	}
	return max(m, 0)
}

// Topes del SIMIT: con acuerdo de pago hasta $1.000.000 cumple; hasta $2.000.000 queda para semillero;
// por encima se descarta.
// shortcut: topes fijos en el código; pasarlos a Configuración > Parámetros si cambian a menudo.
const (
	simitTopeAceptable = 1_000_000
	simitTopeSemillero = 2_000_000
)

// EvaluarSimit dice si el SIMIT cumple (ver NivelSimit).
func (ac *AgenteClasificador) EvaluarSimit(res *modelos.ResultadoSimit) (bool, string) {
	nivel, detalle := NivelSimit(res)
	return nivel == modelos.NivelCumple, detalle
}

// NivelSimit clasifica el SIMIT por la deuda y el acuerdo de pago:
//   - sin deuda (o paz y salvo): cumple
//   - con acuerdo de pago hasta $1.000.000: cumple
//   - con o sin acuerdo hasta $2.000.000: semillero
//   - más de $2.000.000: descarte
//
// Una consulta fallida da semillero con detalle "no verificado": el orquestador la deja pendiente.
func NivelSimit(res *modelos.ResultadoSimit) (nivel, detalle string) {
	if res == nil {
		return modelos.NivelSemillero, "Sin información de SIMIT"
	}

	// Una consulta fallida o sin cédula no es evidencia de "0 multas": hay que reintentarla.
	if strings.HasPrefix(res.Estado, "ERROR") || res.Estado == "NO_APLICA" || res.Estado == "SIN_CEDULA" {
		return modelos.NivelSemillero, fmt.Sprintf("SIMIT no verificado (%s): reintente la consulta", res.Estado)
	}

	multas, comparendos, acuerdos := res.CantidadMultas, res.CantidadComparendos, res.CantidadAcuerdos
	deuda := Pesos(res.TotalDeuda)

	if multas == 0 && (comparendos == 0 || res.PazYSalvo || deuda == 0) {
		return modelos.NivelCumple, "SIMIT OK: 0 multas y Paz y Salvo"
	}
	pendiente := fmt.Sprintf("%d comparendos y %d multas", comparendos, multas)
	switch {
	case deuda > simitTopeSemillero:
		return modelos.NivelDescarte, fmt.Sprintf("SIMIT: deuda de %s supera el tope de %s (%s)", PesosTexto(deuda), PesosTexto(simitTopeSemillero), pendiente)
	case acuerdos > 0 && deuda <= simitTopeAceptable:
		return modelos.NivelCumple, fmt.Sprintf("SIMIT OK: %s en acuerdo de pago (%s, hasta %s)", pendiente, PesosTexto(deuda), PesosTexto(simitTopeAceptable))
	case acuerdos > 0:
		return modelos.NivelSemillero, fmt.Sprintf("SIMIT: acuerdo de pago de %s, entre %s y %s (semillero)", PesosTexto(deuda), PesosTexto(simitTopeAceptable), PesosTexto(simitTopeSemillero))
	}
	return modelos.NivelSemillero, fmt.Sprintf("SIMIT: %s sin acuerdo de pago (%s)", pendiente, PesosTexto(deuda))
}

// Pesos lee un valor como "$ 1.234.567" o "$1.234.567,00" (los centavos se ignoran).
func Pesos(texto string) int {
	var limpio strings.Builder
	for _, r := range texto {
		if r >= '0' && r <= '9' || r == '.' || r == ',' {
			limpio.WriteRune(r)
		}
	}
	s := limpio.String()
	// ",00" o ".00" al final son centavos; con tres cifras es separador de miles.
	if n := len(s); n >= 3 && (s[n-3] == ',' || s[n-3] == '.') {
		s = s[:n-3]
	}
	v, _ := strconv.Atoi(soloDigitos(s))
	return v
}

// PesosTexto escribe 1234567 como "$1.234.567".
func PesosTexto(v int) string {
	d := strconv.Itoa(v)
	for i := len(d) - 3; i > 0; i -= 3 {
		d = d[:i] + "." + d[i:]
	}
	return "$" + d
}

// MotivoGenero es el motivo de descarte de quien no tiene un género aceptado.
func MotivoGenero(genero string, reglas *modelos.ReglasIdoneidad) string {
	return fmt.Sprintf("Género (%s) no cumple requisito (requerido: %s)", genero, reglas.GeneroRequerido())
}

// MotivoIncidentes devuelve los incidentes (antecedentes en la Procuraduría o coincidencias en listas ONU y
// OFAC) que descartan a la persona; vacío si no tiene.
func MotivoIncidentes(c *modelos.Candidato) []string {
	var l []string
	if r := c.ResultadoProcuraduria; r != nil && r.Alerta {
		l = append(l, "Incidente: antecedentes en la Procuraduría ("+primero(r.Detalles, r.Estado)+")")
	}
	if r := c.ResultadoSarlaft; r != nil && r.Alerta {
		l = append(l, "Incidente: coincidencia en la lista ONU ("+primero(r.Detalles, "SARLAFT")+")")
	}
	if r := c.ResultadoOfac; r != nil && r.Alerta {
		l = append(l, "Incidente: coincidencia en la lista OFAC ("+primero(r.Detalles, r.Estado)+")")
	}
	return l
}

// MotivoReintegro es el motivo de descarte de quien ya fue empleado si no se aceptan reintegros; vacío si no aplica.
func MotivoReintegro(c *modelos.Candidato, reglas *modelos.ReglasIdoneidad) string {
	r := c.ResultadoReintegro
	if r == nil || !r.Alerta || r.Estado == modelos.ReintegroErrorConsulta || reglas.AceptaReintegros {
		return ""
	}
	return "Reintegro: " + r.Detalles + ": no se admiten reintegros"
}

func primero(textos ...string) string {
	for _, t := range textos {
		if strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

// Evaluar aplica todas las reglas al candidato consolidado, con sus reglas, y lo clasifica:
//   - descartado: género no aceptado, mayor a la edad tope, reintegro no aceptado, incidentes o SIMIT > $2.000.000
//   - semillero: no cumple la edad mínima, la EPS, el RUNT o el SIMIT (hasta $2.000.000), sin nada que lo descarte
//   - reclutado P1: cumple todo
func (ac *AgenteClasificador) Evaluar(c *modelos.Candidato) *modelos.EvaluacionIdoneidad {
	// El género ya lo decidió el flujo (diccionario, perfil o IA): no se vuelve a adivinar aquí.
	genero := c.Genero
	if genero == "" || genero == "No especificado" {
		genero = ac.DeterminarGenero(c.Nombre, c.TextoPerfil)
	}
	reglas := ac.ReglasDe(c)
	cumpleGenero := reglas.AceptaGenero(genero)

	cumpleEPS, detalleEPS := ac.EvaluarEPS(c.ResultadoAdres, reglas)
	cumpleRunt, detalleRunt := ac.EvaluarRunt(c.ResultadoRunt, reglas)
	nivelSimit, detalleSimit := NivelSimit(c.ResultadoSimit)

	var descartes, semilleros, semillero []string
	if !cumpleGenero {
		descartes = append(descartes, MotivoGenero(genero, reglas))
	}
	switch nivel, motivo := reglas.NivelEdad(c.Edad); nivel {
	case modelos.NivelDescarte:
		descartes = append(descartes, motivo)
	case modelos.NivelSemillero:
		semilleros, semillero = append(semilleros, motivo), append(semillero, "edad")
	}
	if m := MotivoReintegro(c, reglas); m != "" {
		descartes = append(descartes, m)
	}
	if !cumpleEPS {
		semilleros, semillero = append(semilleros, detalleEPS), append(semillero, "adres")
	}
	if !cumpleRunt {
		semilleros, semillero = append(semilleros, detalleRunt), append(semillero, "runt")
	}
	switch nivelSimit {
	case modelos.NivelDescarte:
		descartes = append(descartes, detalleSimit)
	case modelos.NivelSemillero:
		semilleros, semillero = append(semilleros, detalleSimit), append(semillero, "simit")
	}
	descartes = append(descartes, MotivoIncidentes(c)...)

	clasificacion := modelos.ClasificacionReclutadoP1
	switch {
	case len(descartes) > 0:
		clasificacion = modelos.ClasificacionDescartado
	case len(semilleros) > 0:
		clasificacion = modelos.ClasificacionSemillero
	}
	esApto := clasificacion == modelos.ClasificacionReclutadoP1

	ev := &modelos.EvaluacionIdoneidad{
		EsApto:          esApto,
		Genero:          genero,
		CumpleGenero:    cumpleGenero,
		CumpleEPS:       cumpleEPS,
		DetalleEPS:      detalleEPS,
		CumpleRunt:      cumpleRunt,
		DetalleRunt:     detalleRunt,
		CumpleSimit:     nivelSimit == modelos.NivelCumple,
		DetalleSimit:    detalleSimit,
		MotivosDescarte: append(descartes, semilleros...), // primero lo que descarta
		ReglaVersion:    reglas.Version,
		Clasificacion:   clasificacion,
		Semillero:       semillero,
	}

	c.Genero = genero
	c.EsApto = esApto
	c.EvaluacionIdoneidad = ev

	return ev
}

// GuardarApto copia la hoja de vida y actualiza candidatos_aptos.json.
func (ac *AgenteClasificador) GuardarApto(c *modelos.Candidato, ev *modelos.EvaluacionIdoneidad, carpetaOferta string) error {
	if !ev.EsApto {
		return nil
	}

	carpetasDestino := []string{
		filepath.Join(carpetaOferta, "candidatos_aptos"),
		filepath.Join(ac.cfg.DirectorioDescargas, "candidatos_aptos"),
	}

	for _, carp := range carpetasDestino {
		_ = os.MkdirAll(carp, 0755)

		// Copiar hoja de vida principal o PDF
		destinoPDF := c.RutaPDF
		if c.RutaPDF != "" {
			if _, err := os.Stat(c.RutaPDF); err == nil {
				destino := filepath.Join(carp, filepath.Base(c.RutaPDF))
				if err := copiarArchivo(c.RutaPDF, destino); err == nil {
					destinoPDF = destino
				}
			}
		}

		// Copiar hoja de vida original (PDF o DOCX) si existe
		destinoCVOriginal := c.RutaCVOriginal
		if c.RutaCVOriginal != "" {
			if _, err := os.Stat(c.RutaCVOriginal); err == nil {
				destino := filepath.Join(carp, filepath.Base(c.RutaCVOriginal))
				if err := copiarArchivo(c.RutaCVOriginal, destino); err == nil {
					destinoCVOriginal = destino
				}
			}
		}

		// Copiar perfil impreso de Computrabajo si existe
		destinoPerfilPDF := c.RutaPerfilPDF
		if c.RutaPerfilPDF != "" {
			if _, err := os.Stat(c.RutaPerfilPDF); err == nil {
				destino := filepath.Join(carp, filepath.Base(c.RutaPerfilPDF))
				if err := copiarArchivo(c.RutaPerfilPDF, destino); err == nil {
					destinoPerfilPDF = destino
				}
			}
		}

		// Actualizar candidatos_aptos.json: ciclo leer-modificar-escribir bajo cerrojo (el motor llama
		// aquí desde muchas goroutines; sin él se pierden candidatos) y escritura atómica.
		registro := modelos.RegistroApto{
			Nombre: c.Nombre,
			Cedula: c.Cedula,
			Genero: ev.Genero,
			Evaluacion: map[string]string{
				"eps":   ev.DetalleEPS,
				"runt":  ev.DetalleRunt,
				"simit": ev.DetalleSimit,
			},
			TextoPerfil:       c.TextoPerfil,
			RutaPDF:           destinoPDF,
			RutaCVOriginal:    destinoCVOriginal,
			RutaPerfilPDF:     destinoPerfilPDF,
			Adres:             c.ResultadoAdres,
			Runt:              c.ResultadoRunt,
			Simit:             c.ResultadoSimit,
			Procuraduria:      c.ResultadoProcuraduria,
			SarlaftONU:        c.ResultadoSarlaft,
			Ofac:              c.ResultadoOfac,
			FechaCalificacion: time.Now(),
		}

		rutaJSON := filepath.Join(carp, "candidatos_aptos.json")
		errJSON := utilidades.ConBloqueoArchivo(rutaJSON, func() error {
			var lista []modelos.RegistroApto
			if err := utilidades.LeerJSON(rutaJSON, &lista); err != nil && !os.IsNotExist(err) {
				// Archivo dañado: conservarlo para diagnóstico en vez de sobrescribirlo a ciegas.
				_ = os.Rename(rutaJSON, rutaJSON+".corrupto-"+time.Now().Format("20060102-150405"))
				lista = nil
			}

			filtrada := lista[:0]
			for _, r := range lista {
				if r.Cedula != c.Cedula {
					filtrada = append(filtrada, r)
				}
			}
			filtrada = append(filtrada, registro)
			return utilidades.EscribirJSONAtomico(rutaJSON, filtrada, true)
		})
		if errJSON != nil {
			return fmt.Errorf("error actualizando %s: %w", rutaJSON, errJSON)
		}
	}

	return nil
}

// EvaluarCandidato alias para evaluar idoneidad del candidato.
func (ac *AgenteClasificador) EvaluarCandidato(c *modelos.Candidato) *modelos.EvaluacionIdoneidad {
	return ac.Evaluar(c)
}

// GuardarCandidatoApto alias para persistir candidato apto.
func (ac *AgenteClasificador) GuardarCandidatoApto(c *modelos.Candidato, carpetaOferta string) error {
	if c.EvaluacionIdoneidad == nil {
		c.EvaluacionIdoneidad = ac.Evaluar(c)
	}
	return ac.GuardarApto(c, c.EvaluacionIdoneidad, carpetaOferta)
}

func copiarArchivo(origen, destino string) error {
	in, err := os.Open(origen)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destino)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
