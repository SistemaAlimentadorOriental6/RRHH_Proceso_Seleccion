//go:build ignore

// Comprueba contra el Chrome instalado que el bloqueo de imágenes/fuentes funciona
// y que scripts y estilos siguen cargando. Uso: go run scratch/probar_bloqueo.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func main() {
	var mu sync.Mutex
	visto := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		visto[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><link rel=stylesheet href=/e.css><script src=/s.js></script></head>
<body><img src=/foto.png><img src=/foto.jpg><div id=x>hola</div></body></html>`)
		case "/e.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, "body{color:red}")
		case "/s.js":
			w.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(w, "window.__js=true")
		default:
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("x"))
		}
	}))
	defer srv.Close()

	// Los patrones deben coincidir con los de agentes/comun.go
	patrones := []string{"*://*:*/*.png", "*://*:*/*.jpg", "*://*:*/*.jpeg", "*://*:*/*.gif", "*://*:*/*.webp", "*://*:*/*.svg"}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", true))...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var js bool
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(c context.Context) error {
			if err := network.Enable().Do(c); err != nil {
				return err
			}
			bp := make([]*network.BlockPattern, len(patrones))
			for i, p := range patrones {
				bp[i] = &network.BlockPattern{URLPattern: p, Block: true}
			}
			return network.SetBlockedURLs().WithURLPatterns(bp).Do(c)
		}),
		chromedp.Navigate(srv.URL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`window.__js === true`, &js),
	)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("peticiones: %v\n", visto)
	fmt.Printf("JS ejecutado: %v | png pedidas: %d | jpg pedidas: %d | css pedidas: %d\n", js, visto["/foto.png"], visto["/foto.jpg"], visto["/e.css"])
}
