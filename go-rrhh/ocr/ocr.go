package ocr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	procesoMu    sync.Mutex
	puertoSolver = 9095
	clienteHTTP  = &http.Client{Timeout: 5 * time.Second}
)

// SolucionadorCaptcha gestiona la resolucion de LibreCaptcha para el RUNT.
type SolucionadorCaptcha struct {
	RutaSolver string
}

// NuevoSolucionador inicializa y asegura que el servicio OCR este operativo.
func NuevoSolucionador(rutaSolver string) *SolucionadorCaptcha {
	if rutaSolver == "" {
		// Buscar en ../bin/solve.exe o en C:\Users\daniel.ochoa\Proyectos\PruebaRRHH\bin\solve.exe
		candidatas := []string{
			filepath.Join("..", "bin", "solve.exe"),
			filepath.Join(".", "bin", "solve.exe"),
			`C:\Users\daniel.ochoa\Proyectos\PruebaRRHH\bin\solve.exe`,
			`C:\Users\daniel.ochoa\Proyectos\RuntAutomatizado\solve.exe`,
		}
		for _, c := range candidatas {
			if _, err := os.Stat(c); err == nil {
				rutaSolver = c
				break
			}
		}
	}

	s := &SolucionadorCaptcha{RutaSolver: rutaSolver}
	s.asegurarServicio()
	return s
}

// asegurarServicio verifica el endpoint /ping o arranca solve.exe en modo server.
func (s *SolucionadorCaptcha) asegurarServicio() {
	procesoMu.Lock()
	defer procesoMu.Unlock()

	// Probar si ya responde
	resp, err := clienteHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", puertoSolver))
	if err == nil && resp.StatusCode == 200 {
		_ = resp.Body.Close()
		return
	}

	if s.RutaSolver == "" {
		return
	}

	// Iniciar servidor solve.exe en background
	cmd := exec.Command(s.RutaSolver, "--server", fmt.Sprintf("%d", puertoSolver))
	cmd.Dir = filepath.Dir(s.RutaSolver)
	_ = cmd.Start()

	// Esperar arranque rapido (max 2 segundos)
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		r, err := clienteHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", puertoSolver))
		if err == nil && r.StatusCode == 200 {
			_ = r.Body.Close()
			return
		}
	}
}

// Resolver envia la imagen en base64 al microservicio OCR y obtiene el texto de 5 caracteres.
func (s *SolucionadorCaptcha) Resolver(imagenBase64 string) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/solve", puertoSolver)
	resp, err := clienteHTTP.Post(url, "text/plain", strings.NewReader(imagenBase64))
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		cuerpo, _ := io.ReadAll(resp.Body)
		var resultado struct {
			Texto string `json:"text"`
		}
		if err := json.Unmarshal(cuerpo, &resultado); err == nil && len(resultado.Texto) > 0 {
			return strings.TrimSpace(resultado.Texto), nil
		}
	}

	// Fallback via CLI si el servicio HTTP local no responde
	if s.RutaSolver != "" {
		procesoMu.Lock()
		defer procesoMu.Unlock()

		cmd := exec.Command(s.RutaSolver, imagenBase64)
		cmd.Dir = filepath.Dir(s.RutaSolver)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			txt := strings.TrimSpace(out.String())
			if len(txt) == 5 {
				return txt, nil
			}
		}
	}

	return "", fmt.Errorf("no fue posible resolver el captcha en OCR")
}
