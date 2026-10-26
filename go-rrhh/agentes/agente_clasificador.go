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

// EvaluarEPS verifica que la EPS pertenezca a las autorizadas, sin importar si la afiliación figura activa o retirada.
func (ac *AgenteClasificador) EvaluarEPS(res *modelos.ResultadoAdres) (bool, string) {
	if res == nil {
		return false, "Sin información de ADRES"
	}

	if res.Estado == "PORTAL_EN_MANTENIMIENTO" {
		return true, "ADRES en mantenimiento oficial (Pendiente validación EPS)"
	}

	eps := strings.ToUpper(res.EPS)

	// El estado de la afiliación (activo, retirado...) no importa: solo que la EPS sea una de las autorizadas.
	autorizada := false
	for _, aut := range ac.cfg.EPSAutorizadas {
		if strings.Contains(eps, strings.ToUpper(aut)) {
			autorizada = true
			break
		}
	}

	if !autorizada {
		return false, fmt.Sprintf("EPS '%s' no autorizada (Solo Sura, Nueva EPS, Savia Salud, Sanitas)", res.EPS)
	}

	return true, fmt.Sprintf("EPS %s (%s)", res.EPS, res.EstadoAfiliacion)
}

// EvaluarRunt verifica antiguedad minima de 2 anos y licencia C2 o C3 activa.
func (ac *AgenteClasificador) EvaluarRunt(res *modelos.ResultadoRunt) (bool, string) {
	if res == nil {
		return false, "Sin información de RUNT"
	}

	if res.Estado != "INSCRITO_ACTIVO" || !res.Inscrito {
		return false, fmt.Sprintf("No inscrito o inactivo en RUNT (%s)", res.Estado)
	}

	// 1. Antigüedad en RUNT (mínimo 2 años)
	antiguedadAnios := 0.0
	if res.FechaInscripcion != "" {
		limpio := strings.Split(res.FechaInscripcion, ".")[0]
		limpio = strings.ReplaceAll(limpio, "T", " ")
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, limpio); err == nil {
				dias := time.Since(t).Hours() / 24
				antiguedadAnios = dias / 365.25
				break
			}
		}
	}

	// 2. Licencia activa con C2 o C3
	tieneCategoria := false
	for _, lic := range res.Licencias {
		if strings.ToUpper(lic.Estado) != "ACTIVA" {
			continue
		}
		for _, c := range lic.Categorias {
			cu := strings.ToUpper(c)
			if cu == "C2" || cu == "C3" {
				tieneCategoria = true
				break
			}
		}
		for _, det := range lic.DetallesCategorias {
			cu := strings.ToUpper(det.Categoria)
			if cu == "C2" || cu == "C3" {
				tieneCategoria = true
				break
			}
		}
		if tieneCategoria {
			break
		}
	}

	if !tieneCategoria {
		for _, catAct := range res.CategoriasActivas {
			if strings.Contains(catAct, "C2") || strings.Contains(catAct, "C3") {
				tieneCategoria = true
				break
			}
		}
	}

	if !tieneCategoria {
		return false, "No posee licencia activa en categoría C2 o C3"
	}

	if antiguedadAnios < 2.0 {
		return false, fmt.Sprintf("Antigüedad en RUNT insuficiente: %.1f años (Mínimo 2 años requeridos)", antiguedadAnios)
	}

	return true, fmt.Sprintf("RUNT OK: %.1f años de registro | Licencia C2/C3 ACTIVA", antiguedadAnios)
}

// EvaluarSimit valida 0 multas o comparendos hasta $1.000.000 con acuerdo de pago.
func (ac *AgenteClasificador) EvaluarSimit(res *modelos.ResultadoSimit) (bool, string) {
	if res == nil {
		return false, "Sin información de SIMIT"
	}

	// Una consulta fallida o sin cédula no es evidencia de "0 multas": hay que reintentarla.
	if strings.HasPrefix(res.Estado, "ERROR") || res.Estado == "NO_APLICA" || res.Estado == "SIN_CEDULA" {
		return false, fmt.Sprintf("SIMIT no verificado (%s): reintente la consulta", res.Estado)
	}

	multas := res.CantidadMultas
	comparendos := res.CantidadComparendos
	acuerdos := res.CantidadAcuerdos
	pazYSalvo := res.PazYSalvo

	deudaNum, _ := strconv.Atoi(soloDigitos(res.TotalDeuda))

	// Caso 1: 0 multas y paz y salvo (o sin comparendos pendientes)
	if multas == 0 && (comparendos == 0 || pazYSalvo || deudaNum == 0) {
		return true, "SIMIT OK: 0 multas y Paz y Salvo"
	}

	// Caso 2: 0 multas, comparendos hasta $1.000.000 con acuerdo de pago
	if multas == 0 && deudaNum <= 1000000 && acuerdos > 0 {
		return true, fmt.Sprintf("SIMIT OK: Comparendos en acuerdo de pago ($%d) <= $1.000.000", deudaNum)
	}

	var motivos []string
	if multas > 0 {
		motivos = append(motivos, fmt.Sprintf("%d multas de tránsito", multas))
	}
	if comparendos > 0 && acuerdos == 0 {
		motivos = append(motivos, fmt.Sprintf("%d comparendos sin acuerdo de pago ($%d)", comparendos, deudaNum))
	} else if deudaNum > 1000000 {
		motivos = append(motivos, fmt.Sprintf("Deuda ($%d) excede el tope de $1.000.000", deudaNum))
	}

	return false, fmt.Sprintf("SIMIT Rechazado: %s", strings.Join(motivos, ", "))
}

// Evaluar aplica los 4 filtros al candidato consolidado.
func (ac *AgenteClasificador) Evaluar(c *modelos.Candidato) *modelos.EvaluacionIdoneidad {
	// El género ya lo decidió el flujo (diccionario, perfil o IA): no se vuelve a adivinar aquí.
	genero := c.Genero
	if genero == "" || genero == "No especificado" {
		genero = ac.DeterminarGenero(c.Nombre, c.TextoPerfil)
	}
	cumpleGenero := (genero == "Hombre")

	cumpleEPS, detalleEPS := ac.EvaluarEPS(c.ResultadoAdres)
	cumpleRunt, detalleRunt := ac.EvaluarRunt(c.ResultadoRunt)
	cumpleSimit, detalleSimit := ac.EvaluarSimit(c.ResultadoSimit)

	var motivos []string
	if !cumpleGenero {
		motivos = append(motivos, fmt.Sprintf("Género (%s) no cumple requisito (requerido: Hombre)", genero))
	}
	if !cumpleEPS {
		motivos = append(motivos, detalleEPS)
	}
	if !cumpleRunt {
		motivos = append(motivos, detalleRunt)
	}
	if !cumpleSimit {
		motivos = append(motivos, detalleSimit)
	}

	esApto := len(motivos) == 0

	ev := &modelos.EvaluacionIdoneidad{
		EsApto:          esApto,
		Genero:          genero,
		CumpleGenero:    cumpleGenero,
		CumpleEPS:       cumpleEPS,
		DetalleEPS:      detalleEPS,
		CumpleRunt:      cumpleRunt,
		DetalleRunt:     detalleRunt,
		CumpleSimit:     cumpleSimit,
		DetalleSimit:    detalleSimit,
		MotivosDescarte: motivos,
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
