package main

import (
	"context"
	"fmt"
	"os"

	"go-rrhh/almacen"
	"go-rrhh/configuracion"
	"go-rrhh/modelos"
	"go-rrhh/orquestador"
	"go-rrhh/servidor"
)

// Comprobación en compilación: el almacén cumple lo que el orquestador y el servidor necesitan
// para guardar y para leer.
var (
	_ orquestador.Persistencia = (*almacen.Almacen)(nil)
	_ servidor.Base            = (*almacen.Almacen)(nil)
)

// abrirAlmacen conecta con la base de datos. Si está configurada y falla, el programa se detiene:
// seguir sin ella dejaría los datos solo en archivos sin que nadie se enterara.
func abrirAlmacen(ctx context.Context, cfg *configuracion.Configuracion) *almacen.Almacen {
	alm, err := almacen.Abrir(ctx, cfg, servidor.SlugVacante, servidor.SlugPersona)
	if err != nil {
		fmt.Printf("[X] Base de datos: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[+] Base de datos conectada (%s, base %q): cada candidato analizado se guarda también ahí.\n", cfg.MySQLHost, cfg.MySQLBase)
	return alm
}

// ejecutarImportacion lleva a la base los candidatos que ya estaban en descargas/*/candidatos.json.
// Es segura de repetir: los que ya están en la base se omiten. No imprime nombres ni cédulas.
func ejecutarImportacion(ctx context.Context, cfg *configuracion.Configuracion, alm *almacen.Almacen) {
	repo := servidor.NuevoRepositorioDisco(cfg.DirectorioDescargas)
	ofertas, err := repo.ListarOfertas()
	if err != nil {
		fmt.Printf("[X] No se pudieron listar las ofertas: %v\n", err)
		return
	}

	var nuevos, existentes, fallidos int
	for _, o := range ofertas {
		if o.Error != "" {
			fmt.Printf("[!] Oferta %q omitida: %s\n", o.Titulo, o.Error)
			continue
		}
		candidatos, err := repo.ObtenerCandidatosOferta(o.ID, "todos")
		if err != nil {
			fmt.Printf("[!] Oferta %q omitida: %v\n", o.Titulo, err)
			continue
		}
		// El id de Computrabajo no está en las carpetas: se importa con un id provisional que el
		// primer análisis en vivo de la misma vacante reemplaza por el real.
		oferta := modelos.OfertaLaboral{ID: almacen.IDCarpeta(o.ID), Titulo: o.Titulo}

		var nOferta, eOferta, fOferta int
		for i := range candidatos {
			if ctx.Err() != nil {
				fmt.Println("[!] Importación interrumpida.")
				return
			}
			creado, err := alm.ImportarCandidato(ctx, oferta, &candidatos[i], o.ActualizadoEn)
			switch {
			case err != nil:
				fOferta++
				fmt.Printf("[!] Candidato #%d de %q no se pudo importar: %v\n", candidatos[i].Indice, o.Titulo, err)
			case creado:
				nOferta++
			default:
				eOferta++
			}
		}
		fmt.Printf("[+] %s: %d importados o completados, %d ya estaban al día, %d con error\n", o.Titulo, nOferta, eOferta, fOferta)
		nuevos, existentes, fallidos = nuevos+nOferta, existentes+eOferta, fallidos+fOferta
	}
	fmt.Printf("\n[OK] Importación terminada: %d importados o completados, %d ya estaban al día, %d con error.\n", nuevos, existentes, fallidos)
}
