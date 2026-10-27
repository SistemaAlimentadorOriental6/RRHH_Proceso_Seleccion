package servidor

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// origenesPorDefecto son los orígenes del frontend en desarrollo/preview. Se pueden reemplazar con
// RRHH_CORS_ORIGINS (lista separada por comas; "*" permite cualquiera).
var origenesPorDefecto = []string{
	"http://localhost:5173", "http://127.0.0.1:5173",
	"http://localhost:4173", "http://127.0.0.1:4173",
	"http://localhost:8080", "http://127.0.0.1:8080",
}

func origenesPermitidos() map[string]struct{} {
	lista := origenesPorDefecto
	if env := strings.TrimSpace(os.Getenv("RRHH_CORS_ORIGINS")); env != "" {
		lista = strings.Split(env, ",")
	}
	set := make(map[string]struct{}, len(lista))
	for _, o := range lista {
		if o = strings.TrimSpace(o); o != "" {
			set[o] = struct{}{}
		}
	}
	return set
}

// registroRespuesta captura el código de estado para el log. Unwrap permite que
// http.NewResponseController llegue al ResponseWriter real (plazos de escritura, Flush).
type registroRespuesta struct {
	http.ResponseWriter
	codigo int
}

func (r *registroRespuesta) WriteHeader(codigo int) {
	r.codigo = codigo
	r.ResponseWriter.WriteHeader(codigo)
}

func (r *registroRespuesta) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// conCORS agrega las cabeceras CORS solo a orígenes permitidos y responde los preflight (OPTIONS).
// Antes se enviaba "*" siempre y los POST con JSON desde otro origen fallaban en el preflight.
func conCORS(siguiente http.Handler) http.Handler {
	permitidos := origenesPermitidos()
	_, cualquiera := permitidos["*"]

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origen := r.Header.Get("Origin")
		if origen != "" {
			w.Header().Add("Vary", "Origin")
			if _, ok := permitidos[origen]; ok || cualquiera {
				w.Header().Set("Access-Control-Allow-Origin", origen)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, HEAD, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID, If-None-Match")
				w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, X-Job-ID, Content-Disposition, ETag, X-Vacantes-Fuente, X-Vacantes-Edad")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

// conRecuperacion evita que un panic en un handler tumbe el servidor y lo registra con su traza.
func conRecuperacion(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Printf("[PANIC] %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno del servidor"})
			}
		}()
		siguiente.ServeHTTP(w, r)
	})
}

// conRegistro escribe una línea por petición a la API y por cualquier respuesta de error.
func conRegistro(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rr := &registroRespuesta{ResponseWriter: w, codigo: http.StatusOK}
		siguiente.ServeHTTP(rr, r)

		if strings.HasPrefix(r.URL.Path, "/api/") || rr.codigo >= 400 {
			log.Printf("%s %s -> %d (%s)", r.Method, rutaParaLog(r), rr.codigo, time.Since(inicio).Round(time.Millisecond))
		}
	})
}

// rutaParaLog es la ruta de la petición lista para escribirse en el log: el token del enlace que
// recibe el candidato es una credencial y no debe quedar en ningún registro.
func rutaParaLog(r *http.Request) string {
	const prefijoToken = "/api/validar-datos/"
	if strings.HasPrefix(r.URL.Path, prefijoToken) {
		return prefijoToken + "<token>"
	}
	return r.URL.RequestURI()
}

// conCabecerasSeguras evita que el navegador "adivine" tipos de contenido y bloquea el embebido.
func conCabecerasSeguras(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		siguiente.ServeHTTP(w, r)
	})
}

// --- Compresión gzip (solo contenido textual; no PDFs ni descargas con Range) ---

var poolGzip = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

type respuestaGzip struct {
	http.ResponseWriter
	gz        *gzip.Writer
	decidido  bool
	comprimir bool
}

func (g *respuestaGzip) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func esContenidoComprimible(ct string) bool {
	ct = strings.ToLower(ct)
	if strings.HasPrefix(ct, "text/event-stream") {
		return false // SSE debe salir en cada flush; comprimirlo lo retendría en el búfer
	}
	return strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "text/") ||
		strings.HasPrefix(ct, "application/javascript") ||
		strings.HasPrefix(ct, "image/svg+xml")
}

// decidir define una sola vez, al escribir la cabecera, si la respuesta se comprime.
func (g *respuestaGzip) decidir(codigo int) {
	if g.decidido {
		return
	}
	g.decidido = true

	h := g.Header()
	if codigo == http.StatusNoContent || codigo == http.StatusNotModified || codigo < 200 ||
		h.Get("Content-Encoding") != "" || !esContenidoComprimible(h.Get("Content-Type")) {
		return
	}
	g.comprimir = true
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Del("Content-Length")
	g.gz = poolGzip.Get().(*gzip.Writer)
	g.gz.Reset(g.ResponseWriter)
}

func (g *respuestaGzip) WriteHeader(codigo int) {
	g.decidir(codigo)
	g.ResponseWriter.WriteHeader(codigo)
}

func (g *respuestaGzip) Write(p []byte) (int, error) {
	if !g.decidido {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(p))
		}
		g.decidir(http.StatusOK)
	}
	if g.comprimir {
		return g.gz.Write(p)
	}
	return g.ResponseWriter.Write(p)
}

func (g *respuestaGzip) cerrar() {
	if g.gz != nil {
		_ = g.gz.Close()
		poolGzip.Put(g.gz)
		g.gz = nil
	}
}

func conGzip(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			r.Method == http.MethodHead || r.Header.Get("Range") != "" {
			siguiente.ServeHTTP(w, r)
			return
		}
		g := &respuestaGzip{ResponseWriter: w}
		defer g.cerrar()
		siguiente.ServeHTTP(g, r)
	})
}

// cadenaMiddleware aplica los middlewares de afuera hacia adentro.
func cadenaMiddleware(h http.Handler) http.Handler {
	return conRegistro(conRecuperacion(conCORS(conCabecerasSeguras(conGzip(h)))))
}

// descripcion solo se usa en pruebas para depurar la configuración de CORS.
func descripcionCORS() string {
	return fmt.Sprintf("%d orígenes permitidos", len(origenesPermitidos()))
}
