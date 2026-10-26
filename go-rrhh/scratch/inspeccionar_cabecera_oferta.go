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

	urlOferta := "https://empresa.co.computrabajo.com/Company/Offers/Match?oi=A23B14B41E3853F561373E686DCF3405&cf=469814F59E4D6F04"

	var tabsHTML string
	var totalArticles int
	script := `
		(() => {
			const links = Array.from(document.querySelectorAll('nav a, ul.tabs a, ul.p0 a, header a, div.tabs a, a[href*="Candidates"], a[href*="Match"]')).map(a => ({
				text: a.innerText.trim(),
				href: a.href,
				className: a.className
			}));

			const articles = document.querySelectorAll('article.rowuser').length;
			return JSON.stringify({ links: links, articles: articles }, null, 2);
		})()
	`

	err = chromedp.Run(taskCtx,
		chromedp.Navigate(urlOferta),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(script, &tabsHTML),
		chromedp.Evaluate(`document.querySelectorAll('article.rowuser').length`, &totalArticles),
	)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Resultado en la página de la oferta:\n%s\n", tabsHTML)
}
