package utilidades

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEscrituraAtomicaConcurrenteNuncaDejaJSONRoto(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "datos.json")
	if err := EscribirJSONAtomico(ruta, []int{0}, false); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	parar := make(chan struct{})

	// Lector: el archivo siempre debe ser JSON válido.
	errLectura := make(chan error, 1)
	go func() {
		for {
			select {
			case <-parar:
				errLectura <- nil
				return
			default:
				var v []int
				if err := LeerJSON(ruta, &v); err != nil {
					errLectura <- err
					return
				}
				// Un lector real consulta de vez en cuando; en un bucle sin pausa Windows no puede
				// reemplazar el archivo nunca (los lectores lo mantienen abierto sin FILE_SHARE_DELETE).
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			datos := make([]int, 500)
			for k := range datos {
				datos[k] = n
			}
			for j := 0; j < 15; j++ {
				if err := EscribirJSONAtomico(ruta, datos, false); err != nil {
					t.Error(err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(parar)
	if err := <-errLectura; err != nil {
		t.Fatalf("un lector vio un JSON roto: %v", err)
	}

	// No deben quedar temporales huérfanos.
	entradas, _ := os.ReadDir(filepath.Dir(ruta))
	if len(entradas) != 1 {
		t.Errorf("quedaron archivos temporales: %v", entradas)
	}
}

func TestConBloqueoArchivoEvitaPerdidaDeActualizaciones(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "lista.json")
	_ = EscribirJSONAtomico(ruta, []int{}, false)

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = ConBloqueoArchivo(ruta, func() error {
				var lista []int
				_ = LeerJSON(ruta, &lista)
				return EscribirJSONAtomico(ruta, append(lista, n), false)
			})
		}(i)
	}
	wg.Wait()

	var lista []int
	datos, _ := os.ReadFile(ruta)
	_ = json.Unmarshal(datos, &lista)
	if len(lista) != 40 {
		t.Errorf("se perdieron actualizaciones: %d de 40", len(lista))
	}
}

func TestRutaDentroDe(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "ok.pdf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RutaDentroDe(base, filepath.Join(base, "ok.pdf")); err != nil {
		t.Errorf("ruta válida rechazada: %v", err)
	}
	if _, err := RutaDentroDe(base, "ok.pdf"); err != nil {
		t.Errorf("ruta relativa válida rechazada: %v", err)
	}
	for _, mala := range []string{"../x.pdf", filepath.Join(base, "..", "x.pdf"), filepath.Dir(base)} {
		if _, err := RutaDentroDe(base, mala); err == nil {
			t.Errorf("ruta %q debería rechazarse", mala)
		}
	}
}
