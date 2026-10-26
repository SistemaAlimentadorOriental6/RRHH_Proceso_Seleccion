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
		log.Fatalf("Error cargando configuración: %v", err)
	}

	agente := agentes.NuevoAgenteComputrabajo(cfg)
	defer agente.Cerrar()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := agente.AsegurarAutenticacion(ctx); err != nil {
		log.Fatalf("Error autenticación: %v", err)
	}

	ofertas, err := agente.ObtenerOfertasActivas(ctx)
	if err != nil {
		log.Fatalf("Error ofertas: %v", err)
	}

	if len(ofertas) == 0 {
		fmt.Println("No hay ofertas activas")
		return
	}

	fmt.Printf("Oferta encontrada: %s (%s)\n", ofertas[0].Titulo, ofertas[0].URLCandidatos)

	// Navegar a la URL de candidatos e inspeccionar estructura de paginación o scroll
	var infoPaginacion string
	var totalArticles int
	scriptInspeccionar := `
		(() => {
			const rows = document.querySelectorAll('article.rowuser').length;
			const paginacion = document.querySelector('.pagination, .paginator, .pages, .b_page, ul.pagination, .paginador, [class*="pagin"]');
			const botonesMas = Array.from(document.querySelectorAll('button, a')).filter(el => {
				const t = (el.innerText || '').toLowerCase();
				return t.includes('cargar más') || t.includes('ver más') || t.includes('más candidatos') || t.includes('siguiente');
			}).map(el => el.outerHTML.substring(0, 150));

			const linksP = Array.from(document.querySelectorAll('a[href*="p="], a[href*="page="]')).map(a => a.href);

			return JSON.stringify({
				total_rows: rows,
				paginacion_html: paginacion ? paginacion.outerHTML.substring(0, 300) : 'NO_PAGINACION',
				botones_mas: botonesMas,
				links_paginas: linksP,
				url_actual: window.location.href
			}, null, 2);
		})()
	`

	taskCtx, cancelTask := chromedp.NewContext(agente.SesionContexto())
	defer cancelTask()

	err = chromedp.Run(taskCtx,
		chromedp.Navigate(ofertas[0].URLCandidatos),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(scriptInspeccionar, &infoPaginacion),
		chromedp.Evaluate(`document.querySelectorAll('article.rowuser').length`, &totalArticles),
	)

	if err != nil {
		log.Fatalf("Error inspeccionando: %v", err)
	}

	fmt.Printf("Total articulos detectados: %d\n", totalArticles)
	fmt.Printf("Información de paginación:\n%s\n", infoPaginacion)
}
