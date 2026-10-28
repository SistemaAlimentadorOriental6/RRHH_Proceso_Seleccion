package servidor

import (
	"sync"
	"testing"
)

// tokens se llama desde varias peticiones a la vez; antes compartía una cadena de transformación con estado.
func TestTokensConcurrente(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if got := SlugPersona("Julián Andrés Castañeda"); got != "julian-andres-castaneda" {
					t.Errorf("slug inesperado: %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
