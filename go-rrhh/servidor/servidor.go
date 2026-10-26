package servidor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-rrhh/configuracion"
	"go-rrhh/orquestador"
)

// ServidorWeb encapsula el servicio HTTP (solo API REST) de selección de RRHH.
type ServidorWeb struct {
	puerto       int
	controlador  *ControladorRRHH
	servidorHTTP *http.Server
}

// NuevoServidorWeb crea e inicializa el servidor HTTP.
func NuevoServidorWeb(cfg *configuracion.Configuracion, puerto int, modeloIA string) *ServidorWeb {
	if puerto <= 0 {
		puerto = 8080
	}

	repo := NuevoRepositorioDisco(cfg.DirectorioDescargas)
	controlador := NuevoControladorRRHH(cfg, repo, modeloIA)

	return &ServidorWeb{
		puerto:      puerto,
		controlador: controlador,
	}
}

// ConPersistencia activa el guardado de los análisis en la base de datos.
func (s *ServidorWeb) ConPersistencia(p orquestador.Persistencia) {
	s.controlador.ConPersistencia(p)
}

// ConBase activa la base de datos para guardar y para leer (reemplaza a los JSON en la API).
func (s *ServidorWeb) ConBase(b Base) {
	s.controlador.ConBase(b)
}

// rutas registra los endpoints. Los patrones con método (Go 1.22+) responden 405 solos y
// permiten parámetros de ruta (/api/vacantes/{slug}).
func (s *ServidorWeb) rutas() http.Handler {
	mux := http.NewServeMux()
	c := s.controlador

	// 1. API REST
	mux.HandleFunc("GET /api/salud", c.ManejarSalud)

	// Inicio de sesión: el resto de /api/ exige la cookie de sesión (ver conSesion).
	mux.HandleFunc("POST /api/auth/login", c.ManejarLogin)
	mux.HandleFunc("POST /api/auth/logout", c.ManejarLogout)
	mux.HandleFunc("GET /api/auth/yo", c.ManejarSesionActual)

	// Endpoints por oferta (id de carpeta)
	mux.HandleFunc("/api/ofertas", c.ManejarOfertas)
	mux.HandleFunc("/api/candidatos", c.ManejarCandidatos)
	mux.HandleFunc("/api/ranking", c.ManejarRanking)
	mux.HandleFunc("/api/evaluar-ia", c.ManejarEvaluarIA)
	mux.HandleFunc("GET /api/evaluar-ia/estado", c.ManejarEstadoEvaluacion)
	mux.HandleFunc("/api/pdf", c.ManejarServirPDF)
	mux.HandleFunc("/api/captura", c.ManejarServirPDF) // capturas de las validaciones; misma validación de ruta
	mux.HandleFunc("/api/candidato/preguntar", c.ManejarPreguntaCandidato)

	// Vacantes activas en Computrabajo (en vivo) cruzadas con lo procesado en local
	mux.HandleFunc("GET /api/computrabajo/vacantes", c.ManejarVacantesComputrabajo)
	mux.HandleFunc("GET /api/aptos/exportar", c.ManejarExportarAptos)
	mux.HandleFunc("GET /api/candidatos/exportar", c.ManejarExportarCandidato)

	// Análisis en vivo de una vacante (Server-Sent Events) y sus jobs
	mux.HandleFunc("GET /api/computrabajo/vacantes/{id}/analizar", c.ManejarAnalizarVacante)
	mux.HandleFunc("GET /api/computrabajo/vacantes/{id}/candidatos", c.ManejarCandidatosGuardadosVacante)
	// Contacto del candidato (solo personas de prueba): detectar en la hoja de vida y pedirle sus datos por correo
	mux.HandleFunc("GET /api/computrabajo/vacantes/{id}/candidatos/{candidato}/contacto", c.ManejarContactoCandidato)
	mux.HandleFunc("POST /api/computrabajo/vacantes/{id}/candidatos/{candidato}/solicitar-datos", c.ManejarSolicitarDatosFaltantes)
	// Datos faltantes: página pública del candidato (se entra con un token de un solo uso, no con
	// datos en la URL) y herramientas del reclutador.
	mux.HandleFunc("GET /api/validar-datos/{token}", c.ManejarVerSolicitudDatos)
	mux.HandleFunc("POST /api/validar-datos/{token}", c.ManejarEnviarDatosSolicitud)
	mux.HandleFunc("GET /api/postulaciones/{id}/historial", c.ManejarHistorialPostulacion)
	mux.HandleFunc("POST /api/postulaciones/{id}/solicitar-datos", c.ManejarSolicitarDatosPostulacion)
	mux.HandleFunc("GET /api/cambios", c.ManejarCambios)
	mux.HandleFunc("GET /api/postulaciones/{id}/solicitud", c.ManejarEstadoSolicitud)
	mux.HandleFunc("GET /api/jobs", c.ManejarListaJobs)
	mux.HandleFunc("GET /api/jobs/{id}", c.ManejarJob)
	mux.HandleFunc("GET /api/jobs/{id}/eventos", c.ManejarEventosJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", c.ManejarCancelarJob)

	// Endpoints por vacante (slug, p. ej. operador-buses-alimentadores-metro-medellin)
	mux.HandleFunc("GET /api/vacantes", c.ManejarVacantes)
	mux.HandleFunc("GET /api/vacantes/{slug}", c.ManejarVacante)
	mux.HandleFunc("GET /api/vacantes/{slug}/candidatos", c.ManejarVacanteCandidatos)
	mux.HandleFunc("GET /api/vacantes/{slug}/candidatos/{candidato}", c.ManejarVacanteCandidato)
	mux.HandleFunc("GET /api/vacantes/{slug}/ranking", c.ManejarVacanteRanking)

	// 2. Índice de la API en la raíz; cualquier otra ruta responde 404 en JSON
	mux.HandleFunc("GET /{$}", c.ManejarIndice)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ResponderJSON(w, http.StatusNotFound, map[string]string{"error": "ruta no encontrada"})
	})

	return cadenaMiddleware(c.conSesion(mux))
}

// Iniciar arranca el servidor HTTP y escucha conexiones entrantes.
func (s *ServidorWeb) Iniciar(ctx context.Context) error {
	s.servidorHTTP = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.puerto),
		Handler: s.rutas(),
		// Plazos acotados contra clientes lentos. Los handlers que necesitan más (evaluación con IA,
		// pregunta a la IA, descarga de archivos) amplían su propio plazo con ResponseController.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	// Manejo de apagado elegante (graceful shutdown)
	go func() {
		<-ctx.Done()
		s.controlador.Detener() // cancela evaluaciones de IA en segundo plano
		ctxApagado, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.servidorHTTP.Shutdown(ctxApagado)
	}()

	log.SetFlags(log.Ldate | log.Ltime)
	s.controlador.PrecalentarSiConfigurado()
	s.controlador.IniciarAutomatico()
	fmt.Println("=================================================================")
	fmt.Printf("[★] SERVIDOR WEB RRHH INICIADO EXITOSAMENTE\n")
	fmt.Printf("[*] API disponible en: http://localhost:%d (índice de endpoints en /)\n", s.puerto)
	if s.controlador.sesiones == nil {
		fmt.Println("[!] SIN base de datos: no hay inicio de sesión y la API queda ABIERTA (configure RRHH_MYSQL_*)")
	}
	fmt.Printf("[*] CORS: %s (RRHH_CORS_ORIGINS para cambiarlos)\n", descripcionCORS())
	fmt.Printf("[*] Para detener el servidor presiona Ctrl + C\n")
	fmt.Println("=================================================================")

	err := s.servidorHTTP.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error en servidor HTTP: %w", err)
	}

	return nil
}
