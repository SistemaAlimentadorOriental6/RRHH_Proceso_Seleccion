//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"go-rrhh/configuracion"
	"go-rrhh/agentes"
)

func main() {
	cfg, err := configuracion.Cargar()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	agente := agentes.NuevoAgenteComputrabajo(cfg)
	defer agente.Cerrar()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := agente.AsegurarAutenticacion(ctx); err != nil {
		log.Fatalf("Error autenticación: %v", err)
	}

	taskCtx, cancelTask := chromedp.NewContext(agente.SesionContexto())
	defer cancelTask()

	var enlacesOfertas []string
	script := `
		(() => {
			const res = [];
			const cards = document.querySelectorAll('article.listacheckbox');
			for (const c of cards) {
				const links = Array.from(c.querySelectorAll('a')).map(a => ({
					text: a.innerText.trim(),
					href: a.href,
					html: a.outerHTML.substring(0, 100)
				}));
				res.push(JSON.stringify(links, null, 2));
			}
			return res;
		})()
	`

	err = chromedp.Run(taskCtx,
		chromedp.Navigate(cfg.URLOfertas),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(script, &enlacesOfertas),
	)
	if err != nil {
		log.Fatalf("Error leyendo ofertas: %v", err)
	}

	for i, o := range enlacesOfertas {
		fmt.Printf("=== OFERTA #%d ===\n%s\n", i+1, o)
	}
}
