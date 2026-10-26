package almacen

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// archivo es un fichero del candidato en disco. La base guarda la ruta (relativa a descargas/), su
// huella SHA-256 y su tamaño; el contenido se queda en el disco.
type archivo struct {
	id     []byte
	tipo   string         // cv_original | cv_pdf | perfil_pdf | foto | captura | texto_perfil
	fuente sql.NullString // solo capturas de una fuente consultada (clave foránea a fuentes)
	etapa  sql.NullString // solo capturas: la clave con la que el candidato las guarda (perfil, adres, runt...)
	ruta   string
	sha256 string
	bytes  int64
}

// fuentesConCaptura son las fuentes (tabla fuentes) cuya captura se puede asociar por clave foránea.
var fuentesConCaptura = map[string]bool{
	"adres": true, "runt": true, "simit": true, "procuraduria": true, "ofac": true, "sarlaft": true,
}

// carpetaCandidato es la carpeta donde viven los archivos del candidato. Si el análisis vino de la
// caché o de un JSON importado no se conoce, y se deduce de la ruta de alguno de sus archivos.
func carpetaCandidato(c *modelos.Candidato) string {
	if c.CarpetaCandidato != "" {
		return c.CarpetaCandidato
	}
	for _, ruta := range []string{c.RutaPDF, c.RutaCVOriginal, c.RutaPerfilPDF, c.RutaFoto} {
		if ruta != "" {
			return filepath.Dir(ruta)
		}
	}
	return ""
}

// rutaRelativa devuelve la ruta respecto a descargas/ con barras normales; si el archivo está
// fuera de descargas/ queda absoluta.
func rutaRelativa(descargas, ruta string) string {
	abs, err := filepath.Abs(ruta)
	if err != nil {
		return filepath.ToSlash(ruta)
	}
	base, err := filepath.Abs(descargas)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

func huella(ruta string) (string, int64, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// prepararArchivos junta los archivos del candidato que existen en disco. El texto del perfil, que
// solo vivía dentro del JSON, se escribe a perfil.txt para que también quede como archivo.
func (a *Almacen) prepararArchivos(c *modelos.Candidato) []archivo {
	type candidato struct {
		tipo, ruta, fuente string
	}
	var lista []candidato
	add := func(tipo, ruta, fuente string) {
		if strings.TrimSpace(ruta) != "" {
			lista = append(lista, candidato{tipo, ruta, fuente})
		}
	}
	add("cv_pdf", c.RutaPDF, "")
	add("cv_original", c.RutaCVOriginal, "")
	add("perfil_pdf", c.RutaPerfilPDF, "")
	add("foto", c.RutaFoto, "")
	for etapa, ruta := range c.Capturas {
		add("captura", ruta, etapa)
	}
	if carpeta := carpetaCandidato(c); carpeta != "" && strings.TrimSpace(c.TextoPerfil) != "" {
		destino := filepath.Join(carpeta, "perfil.txt")
		if err := utilidades.EscribirArchivoAtomico(destino, []byte(c.TextoPerfil), 0o644); err == nil {
			add("texto_perfil", destino, "")
		}
	}

	vistos := map[string]bool{}
	var salida []archivo
	for _, it := range lista {
		info, err := os.Stat(it.ruta)
		if err != nil || info.IsDir() {
			continue
		}
		rel := rutaRelativa(a.descargas, it.ruta)
		if len(rel) > 500 || vistos[it.tipo+"|"+rel] {
			continue
		}
		sha, n, err := huella(it.ruta)
		if err != nil {
			continue
		}
		id, err := nuevoID()
		if err != nil {
			continue
		}
		vistos[it.tipo+"|"+rel] = true
		var fuente, etapa sql.NullString
		if it.tipo == "captura" {
			etapa = texto(it.fuente, 30)
			if fuentesConCaptura[it.fuente] {
				fuente = sql.NullString{String: it.fuente, Valid: true}
			}
		}
		salida = append(salida, archivo{id: id, tipo: it.tipo, fuente: fuente, etapa: etapa, ruta: rel, sha256: sha, bytes: n})
	}
	return salida
}
