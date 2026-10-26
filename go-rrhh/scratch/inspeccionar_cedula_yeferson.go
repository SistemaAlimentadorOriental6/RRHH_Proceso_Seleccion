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

	urlYeferson := "https://empresa.co.computrabajo.com/Company/MatchCvDetail/MatchDetail?oi=A23B14B41E3853F561373E686DCF3405&ims=BD4866FB9D1B21C30478DC21ED06F675&cf=469814F59E4D6F04"

	var infoYeferson string
	script := `
		(() => {
			const lis = Array.from(document.querySelectorAll('li')).map(l => l.innerText.trim()).filter(t => t.length > 0 && t.length < 80);
			const spans = Array.from(document.querySelectorAll('span[title]')).map(s => s.getAttribute('title') + ' -> ' + s.parentElement.innerText.trim());
			const extra = document.querySelector('.b_detail')?.innerText || '';
			return JSON.stringify({ lis: lis.slice(0, 20), spans: spans, extra: extra.substring(0, 300) }, null, 2);
		})()
	`

	err = chromedp.Run(taskCtx,
		chromedp.Navigate(urlYeferson),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(script, &infoYeferson),
	)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Datos de Yeferson:\n%s\n", infoYeferson)
}
