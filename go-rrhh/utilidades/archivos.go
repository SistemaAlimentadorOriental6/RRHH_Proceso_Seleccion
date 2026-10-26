package utilidades

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// cerrojos guarda un mutex por archivo para serializar ciclos leer-modificar-escribir
// dentro del mismo proceso (el motor escribe desde muchas goroutines).
var cerrojos sync.Map

// cerrojosRenombrado serializa los reemplazos atómicos sobre un mismo destino. Es independiente de
// cerrojos para que ConBloqueoArchivo pueda llamar a EscribirArchivoAtomico sin interbloqueo.
var cerrojosRenombrado sync.Map

func cerrojoRenombrado(ruta string) *sync.Mutex {
	abs, err := filepath.Abs(ruta)
	if err != nil {
		abs = ruta
	}
	m, _ := cerrojosRenombrado.LoadOrStore(strings.ToLower(abs), &sync.Mutex{})
	return m.(*sync.Mutex)
}

func cerrojoPara(ruta string) *sync.Mutex {
	abs, err := filepath.Abs(ruta)
	if err != nil {
		abs = ruta
	}
	m, _ := cerrojos.LoadOrStore(strings.ToLower(abs), &sync.Mutex{})
	return m.(*sync.Mutex)
}

// ConBloqueoArchivo ejecuta fn con exclusión mutua sobre la ruta indicada.
func ConBloqueoArchivo(ruta string, fn func() error) error {
	m := cerrojoPara(ruta)
	m.Lock()
	defer m.Unlock()
	return fn()
}

// EscribirArchivoAtomico escribe en un temporal del mismo directorio y lo renombra sobre el
// destino, de modo que un lector nunca ve un archivo a medio escribir.
func EscribirArchivoAtomico(ruta string, datos []byte, perm os.FileMode) error {
	dir := filepath.Dir(ruta)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	rutaTmp := tmp.Name()
	limpiar := func() { _ = os.Remove(rutaTmp) }

	if _, err := tmp.Write(datos); err != nil {
		_ = tmp.Close()
		limpiar()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		limpiar()
		return err
	}
	if err := tmp.Close(); err != nil {
		limpiar()
		return err
	}
	if err := os.Chmod(rutaTmp, perm); err != nil {
		limpiar()
		return err
	}

	// En Windows el rename falla momentáneamente si un lector tiene el archivo abierto, y dos
	// renombrados simultáneos sobre el mismo destino se bloquean entre sí: se serializan.
	mu := cerrojoRenombrado(ruta)
	mu.Lock()
	defer mu.Unlock()

	var errRename error
	// (antivirus o lectores lentos pueden retener el archivo unos cientos de ms).
	for intento := 0; intento < 20; intento++ {
		if errRename = os.Rename(rutaTmp, ruta); errRename == nil {
			return nil
		}
		espera := time.Duration(intento+1) * 15 * time.Millisecond
		if espera > 120*time.Millisecond {
			espera = 120 * time.Millisecond
		}
		time.Sleep(espera)
	}
	limpiar()
	return fmt.Errorf("no se pudo reemplazar %s: %w", ruta, errRename)
}

// EscribirJSONAtomico serializa v y lo escribe de forma atómica.
func EscribirJSONAtomico(ruta string, v any, indentar bool) error {
	var (
		datos []byte
		err   error
	)
	if indentar {
		datos, err = json.MarshalIndent(v, "", "  ")
	} else {
		datos, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("error serializando %s: %w", filepath.Base(ruta), err)
	}
	return EscribirArchivoAtomico(ruta, datos, 0o644)
}

// leerArchivo lee el archivo reintentando brevemente: en Windows, abrir un archivo justo mientras
// otro proceso lo reemplaza (rename atómico) falla un instante con "being used by another process".
func leerArchivo(ruta string) ([]byte, error) {
	var (
		datos []byte
		err   error
	)
	for intento := 0; intento < 6; intento++ {
		if datos, err = os.ReadFile(ruta); err == nil || os.IsNotExist(err) {
			return datos, err
		}
		time.Sleep(time.Duration(intento+1) * 10 * time.Millisecond)
	}
	return nil, err
}

// LeerJSON lee y deserializa un archivo JSON. Preserva os.ErrNotExist para poder distinguir
// "no existe" de "está dañado".
func LeerJSON(ruta string, destino any) error {
	datos, err := leerArchivo(ruta)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(datos, destino); err != nil {
		return fmt.Errorf("JSON inválido en %s: %w", filepath.Base(ruta), err)
	}
	return nil
}

// RutaDentroDe devuelve la ruta absoluta y limpia de ruta (resolviendo enlaces simbólicos) y
// verifica que quede contenida dentro de base. Si ruta es relativa se resuelve contra base.
func RutaDentroDe(base, ruta string) (string, error) {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(baseAbs); err == nil {
		baseAbs = real
	}

	if !filepath.IsAbs(ruta) {
		ruta = filepath.Join(baseAbs, ruta)
	}
	rutaAbs, err := filepath.Abs(ruta)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(rutaAbs); err == nil {
		rutaAbs = real
	}

	rel, err := filepath.Rel(baseAbs, rutaAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("ruta fuera del directorio permitido")
	}
	return rutaAbs, nil
}
