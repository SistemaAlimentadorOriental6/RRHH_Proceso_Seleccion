package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-rrhh/almacen"
)

const cookieSesion = "rrhh_sesion"

// sesionesBase es lo que la autenticación necesita de la base de datos.
type sesionesBase interface {
	IniciarSesion(ctx context.Context, correo, clave string) (almacen.Usuario, string, error)
	UsuarioDeSesion(ctx context.Context, token string) (almacen.Usuario, bool, error)
	CerrarSesion(ctx context.Context, token string) error
}

// rutasPublicas son las rutas de /api/ que se atienden sin sesión: el estado del servicio, el
// propio inicio de sesión y la página que abre el candidato con su token de un solo uso.
func rutaPublica(ruta string) bool {
	return ruta == "/api/salud" || ruta == "/api/auth/login" || ruta == "/api/auth/logout" || ruta == "/api/auth/yo" ||
		strings.HasPrefix(ruta, "/api/validar-datos/")
}

// conSesion exige una sesión válida en toda la API salvo en las rutas públicas. Sin base de datos
// no hay con qué verificar usuarios, así que la API queda abierta como antes (se avisa al arrancar).
func (c *ControladorRRHH) conSesion(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.sesiones == nil || !strings.HasPrefix(r.URL.Path, "/api/") || rutaPublica(r.URL.Path) {
			siguiente.ServeHTTP(w, r)
			return
		}
		if _, ok := c.usuarioDe(r); !ok {
			ResponderJSON(w, http.StatusUnauthorized, map[string]string{"error": "sesión no iniciada o vencida"})
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

func (c *ControladorRRHH) usuarioDe(r *http.Request) (almacen.Usuario, bool) {
	ck, err := r.Cookie(cookieSesion)
	if err != nil {
		return almacen.Usuario{}, false
	}
	if u, ok := c.sesionesVistas.buscar(ck.Value); ok {
		return u, true
	}
	u, ok, err := c.sesiones.UsuarioDeSesion(r.Context(), ck.Value)
	if err != nil {
		log.Printf("[!] Error validando la sesión: %v", err)
		return almacen.Usuario{}, false
	}
	if ok {
		c.sesionesVistas.guardar(ck.Value, u)
	}
	return u, ok
}

// vigenciaSesionVista es cuánto se confía en una sesión ya validada sin volver a la base. Cerrar sesión
// la borra al instante; vencida o desactivada en otra parte deja de valer como mucho en este tiempo.
const vigenciaSesionVista = 30 * time.Second

// sesionesVistas recuerda por unos segundos las sesiones válidas: cada petición de la API (y las
// pantallas hacen muchas) consultaba la base solo para esto.
type sesionesVistas struct {
	mu     sync.Mutex
	vistas map[string]sesionVista
}

type sesionVista struct {
	u     almacen.Usuario
	hasta time.Time
}

func (s *sesionesVistas) buscar(token string) (almacen.Usuario, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vistas[token]
	if !ok || time.Now().After(v.hasta) {
		return almacen.Usuario{}, false
	}
	return v.u, true
}

func (s *sesionesVistas) guardar(token string, u almacen.Usuario) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ahora := time.Now()
	if s.vistas == nil || len(s.vistas) > 1000 {
		for k, v := range s.vistas { // no crece sin límite: se barren las vencidas
			if ahora.After(v.hasta) {
				delete(s.vistas, k)
			}
		}
		if s.vistas == nil {
			s.vistas = map[string]sesionVista{}
		}
	}
	s.vistas[token] = sesionVista{u: u, hasta: ahora.Add(vigenciaSesionVista)}
}

func (s *sesionesVistas) olvidar(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vistas, token)
}

func ponerCookieSesion(w http.ResponseWriter, r *http.Request, token string, duracion time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSesion,
		Value:    token,
		Path:     "/",
		MaxAge:   int(duracion.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}

func (c *ControladorRRHH) sinAutenticacion(w http.ResponseWriter) bool {
	if c.sesiones != nil {
		return false
	}
	ResponderJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "el inicio de sesión requiere la base de datos configurada (RRHH_MYSQL_*)"})
	return true
}

// ManejarLogin: POST /api/auth/login {correo, contrasena}.
func (c *ControladorRRHH) ManejarLogin(w http.ResponseWriter, r *http.Request) {
	if c.sinAutenticacion(w) {
		return
	}
	if !c.limiteLogin.permitir(ipDe(r)) {
		ResponderJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Demasiados intentos. Espera un minuto."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var cuerpo struct {
		Correo     string `json:"correo"`
		Contrasena string `json:"contrasena"`
	}
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		ResponderJSON(w, http.StatusBadRequest, map[string]string{"error": "solicitud inválida"})
		return
	}
	u, token, err := c.sesiones.IniciarSesion(r.Context(), cuerpo.Correo, cuerpo.Contrasena)
	switch {
	case errors.Is(err, almacen.ErrCredenciales):
		ResponderJSON(w, http.StatusUnauthorized, map[string]string{"error": "Correo o contraseña incorrectos."})
		return
	case errors.Is(err, almacen.ErrCuentaBloqueada):
		ResponderJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Cuenta bloqueada por intentos fallidos. Intenta de nuevo en 15 minutos."})
		return
	case err != nil:
		log.Printf("[!] Error iniciando sesión: %v", err)
		ResponderJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno al iniciar sesión"})
		return
	}
	ponerCookieSesion(w, r, token, almacen.DuracionSesion)
	ResponderJSON(w, http.StatusOK, u)
}

// ManejarSesionActual: GET /api/auth/yo devuelve el usuario de la sesión o 401.
func (c *ControladorRRHH) ManejarSesionActual(w http.ResponseWriter, r *http.Request) {
	if c.sinAutenticacion(w) {
		return
	}
	u, ok := c.usuarioDe(r)
	if !ok {
		ResponderJSON(w, http.StatusUnauthorized, map[string]string{"error": "sesión no iniciada o vencida"})
		return
	}
	ResponderJSON(w, http.StatusOK, u)
}

// ManejarLogout: POST /api/auth/logout invalida la sesión y borra la cookie.
func (c *ControladorRRHH) ManejarLogout(w http.ResponseWriter, r *http.Request) {
	if c.sinAutenticacion(w) {
		return
	}
	if ck, err := r.Cookie(cookieSesion); err == nil {
		c.sesionesVistas.olvidar(ck.Value)
		if err := c.sesiones.CerrarSesion(r.Context(), ck.Value); err != nil {
			log.Printf("[!] Error cerrando la sesión: %v", err)
		}
	}
	ponerCookieSesion(w, r, "", -time.Second)
	ResponderJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
