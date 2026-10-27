package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/almacen"
	"go-rrhh/configuracion"
	"go-rrhh/orquestador"
	"go-rrhh/servidor"
)

func main() {
	// Definición de banderas de línea de comandos en paridad exacta con el sistema original
	concurrencia := flag.Int("concurrencia", 20, "Número de tareas concurrentes simultáneas (predeterminado: 20)")
	limite := flag.Int("limite", 0, "Límite opcional de candidatos a procesar por cada oferta laboral")
	validarAntecedentes := flag.Bool("antecedentes", false, "Valida antecedentes disciplinarios en tiempo real en la Procuraduría")
	validarSarlaft := flag.Bool("sarlaft", false, "Valida a cada candidato contra la Lista Consolidada de Sanciones ONU (SARLAFT)")
	validarOfac := flag.Bool("ofac", false, "Valida a cada candidato contra la lista de sanciones de la OFAC (EE. UU.)")
	validarAdres := flag.Bool("adres", false, "Valida afiliación a EPS en ADRES (BDUA) y cumplimiento de EPS autorizadas")
	validarRunt := flag.Bool("runt", false, "Valida información de ciudadanos, conductores, licencias y multas en el RUNT")
	validarSimit := flag.Bool("simit", false, "Valida multas y comparendos de tránsito en SIMIT")

	// Evaluación y ranking con Inteligencia Artificial (Gemini)
	evaluarIA := flag.Bool("ia", false, "Evalúa y califica a los candidatos aptos con el modelo de IA local")
	modeloIA := flag.String("modelo-ia", "", "Modelo de Gemini a utilizar (predeterminado: IA_MODEL del .env)")
	rankingAptos := flag.Bool("ranking-aptos", false, "Genera directamente el ranking y scoring con IA para los candidatos aptos ya descargados")
	cargoIA := flag.String("cargo", "Operador(a) de buses alimentadores - Metro de Medellín", "Título de la vacante o cargo para contextualizar la evaluación con IA")

	// Servidor HTTP (solo API REST)
	modoWeb := flag.Bool("web", false, "Inicia el servidor de la API REST (vacantes, candidatos, ranking con IA)")
	puertoWeb := flag.Int("puerto", 3002, "Puerto TCP de la API (predeterminado: 3002)")

	// Buzón de correo con hojas de vida (IMAP)
	leerCorreo := flag.Bool("correo", false, "Lee el buzón de hojas de vida (CORREO_* del .env) y guarda las halladas en descargas/correo/")
	correoUltimas := flag.Int("correo-ultimas", 20, "Con -correo: en la primera lectura solo guarda las N hojas de vida más recientes (0 = todo el buzón)")
	correoCada := flag.Duration("correo-cada", 0, "Con -correo: repetir la lectura cada este tiempo (ej. 5m) hasta Ctrl+C; 0 = una sola vez")

	// Base de datos
	importarJSON := flag.Bool("importar-json", false, "Importa a la base de datos los candidatos ya guardados en descargas/ (requiere RRHH_MYSQL_* en el .env)")

	// Usuarios de la plataforma: la contraseña se lee de RRHH_NUEVA_CLAVE para que no quede en el historial del shell
	crearUsuario := flag.Bool("crear-usuario", false, "Crea (o actualiza) un usuario con acceso a la plataforma; requiere -usuario-correo, -usuario-nombre y la variable RRHH_NUEVA_CLAVE")
	usuarioCorreo := flag.String("usuario-correo", "", "Con -crear-usuario: correo del usuario")
	usuarioNombre := flag.String("usuario-nombre", "", "Con -crear-usuario: nombre completo")
	usuarioCargo := flag.String("usuario-cargo", "", "Con -crear-usuario: cargo")

	// Consultas directas individuales
	cedulaProcuraduria := flag.String("validar-cedula", "", "Consulta directamente antecedentes de una cédula en Procuraduría")
	nombreSarlaft := flag.String("validar-sarlaft", "", "Consulta directamente un nombre en SARLAFT ONU")
	cedulaOfac := flag.String("validar-ofac", "", "Consulta directamente una cédula en la lista OFAC")
	cedulaAdres := flag.String("validar-adres", "", "Consulta directamente una cédula en la ADRES (BDUA)")
	cedulaRunt := flag.String("validar-runt", "", "Consulta directamente una cédula en el RUNT (requiere -apellido)")
	apellidoRunt := flag.String("apellido", "", "Primer apellido para la consulta en el RUNT")
	cedulaSimit := flag.String("validar-simit", "", "Consulta directamente multas y comparendos de una cédula en SIMIT")

	flag.Parse()

	// Inicializar configuración
	cfg, err := configuracion.Cargar()
	if err != nil {
		fmt.Printf("[X] Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *crearUsuario {
		if !cfg.MySQLActivo() {
			fmt.Println("[X] La base de datos no está configurada: complete RRHH_MYSQL_* y las claves en el .env")
			os.Exit(1)
		}
		alm := abrirAlmacen(ctx, cfg)
		defer alm.Cerrar()
		if err := alm.CrearUsuario(ctx, *usuarioCorreo, *usuarioNombre, *usuarioCargo, os.Getenv("RRHH_NUEVA_CLAVE")); err != nil {
			fmt.Printf("[X] No se pudo crear el usuario: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[+] Usuario %s listo para iniciar sesión\n", *usuarioCorreo)
		return
	}

	// 0.0. Importación de los JSON existentes a la base de datos
	if *importarJSON {
		if !cfg.MySQLActivo() {
			fmt.Println("[X] La base de datos no está configurada: complete RRHH_MYSQL_* y las claves en el .env")
			os.Exit(1)
		}
		alm := abrirAlmacen(ctx, cfg)
		defer alm.Cerrar()
		ejecutarImportacion(ctx, cfg, alm)
		return
	}

	// 0. Modo servidor web interactivo con frontend HTML/CSS/JS
	if *modoWeb {
		srv := servidor.NuevoServidorWeb(cfg, *puertoWeb, *modeloIA)
		if cfg.MySQLActivo() {
			alm := abrirAlmacen(ctx, cfg)
			defer alm.Cerrar()
			srv.ConBase(alm)
		}
		if err := srv.Iniciar(ctx); err != nil {
			fmt.Printf("[X] Error en servidor web: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 0.05. Lectura del buzón de hojas de vida
	if *leerCorreo {
		ejecutarCorreo(ctx, cfg, *modeloIA, *correoCada, *correoUltimas)
		return
	}

	// 0.1. Modo ranking directo de candidatos aptos con IA
	if *rankingAptos {
		ejecutarRankingAptosDirecto(ctx, cfg, *modeloIA, *cargoIA)
		return
	}

	// 1. Manejo de consultas individuales directas
	if *cedulaProcuraduria != "" {
		ejecutarConsultaProcuraduria(ctx, cfg, *cedulaProcuraduria)
		return
	}
	if *nombreSarlaft != "" {
		ejecutarConsultaSarlaft(cfg, *nombreSarlaft)
		return
	}
	if *cedulaOfac != "" {
		ejecutarConsultaOfac(ctx, cfg, *cedulaOfac)
		return
	}
	if *cedulaAdres != "" {
		ejecutarConsultaAdres(ctx, cfg, *cedulaAdres)
		return
	}
	if *cedulaRunt != "" {
		ejecutarConsultaRunt(ctx, cfg, *cedulaRunt, *apellidoRunt)
		return
	}
	if *cedulaSimit != "" {
		ejecutarConsultaSimit(ctx, cfg, *cedulaSimit)
		return
	}

	// 2. Ejecución del flujo principal concurrente
	opts := orquestador.OpcionesEjecucion{
		Concurrencia:        *concurrencia,
		LimiteCandidatos:    *limite,
		ValidarAntecedentes: *validarAntecedentes,
		ValidarSarlaft:      *validarSarlaft,
		ValidarOfac:         *validarOfac,
		ValidarAdres:        *validarAdres,
		ValidarRunt:         *validarRunt,
		ValidarSimit:        *validarSimit,
		EvaluarIA:           *evaluarIA,
		ModeloIA:            *modeloIA,
	}

	var alm *almacen.Almacen
	if cfg.MySQLActivo() {
		alm = abrirAlmacen(ctx, cfg)
		defer alm.Cerrar()
	}

	// Una cuenta de Computrabajo a la vez: cada una termina (y cierra su Chrome) antes de empezar la siguiente.
	fallo := false
	for i, cuenta := range cfg.CuentasComputrabajo() {
		if ctx.Err() != nil {
			break
		}
		fmt.Printf("\n[*] === Cuenta de Computrabajo %d: %s ===\n", i+1, cuenta.UsuarioComputrabajo)
		if err := ejecutarCuenta(ctx, cuenta, opts, alm); err != nil {
			fmt.Printf("\n[X] Error durante la ejecución del motor (cuenta %d): %v\n", i+1, err)
			fallo = true // se sigue con la siguiente cuenta: un fallo de login no debe perder la otra
		}
	}
	if fallo {
		os.Exit(1)
	}
}

func ejecutarCuenta(ctx context.Context, cfg *configuracion.Configuracion, opts orquestador.OpcionesEjecucion, alm *almacen.Almacen) error {
	motor := orquestador.NuevoMotorOrquestador(cfg, opts)
	defer motor.Cerrar()
	if alm != nil {
		motor.ConPersistencia(alm)
	}
	return motor.Ejecutar(ctx)
}

func ejecutarCorreo(ctx context.Context, cfg *configuracion.Configuracion, modeloIA string, cada time.Duration, ultimas int) {
	if !cfg.CorreoActivo() {
		fmt.Println("[X] Buzón no configurado: complete CORREO_USUARIO y CORREO_CLAVE en el .env")
		os.Exit(1)
	}
	ag := agentes.NuevoAgenteCorreo(cfg, agentes.NuevoAgenteIA(cfg, modeloIA))
	ag.UltimasEnPrimeraLectura = ultimas
	if cada > 0 {
		fmt.Printf("[*] Vigilando el buzón %s cada %s (Ctrl+C para detener)\n", cfg.CorreoUsuario, cada)
		ag.Vigilar(ctx, cada)
		return
	}
	revisados, guardados, err := ag.ProcesarNuevos(ctx)
	if err != nil {
		fmt.Printf("[X] Error leyendo el buzón: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[+] %d correos nuevos revisados, %d hojas de vida guardadas en %s\n", revisados, guardados, filepath.Join(cfg.DirectorioDescargas, "correo", "hojas_de_vida.json"))
}

func ejecutarRankingAptosDirecto(ctx context.Context, cfg *configuracion.Configuracion, modelo, cargo string) {
	rutaGlobalAptos := filepath.Join(cfg.DirectorioDescargas, "candidatos_aptos", "candidatos_aptos.json")
	if _, err := os.Stat(rutaGlobalAptos); os.IsNotExist(err) {
		fmt.Printf("[X] No se encontró el archivo de candidatos aptos en: %s\n", rutaGlobalAptos)
		return
	}

	agenteIA := agentes.NuevoAgenteIA(cfg, modelo)
	_, err := agenteIA.EvaluarCandidatosAptos(ctx, rutaGlobalAptos, cargo)
	if err != nil {
		fmt.Printf("[X] Error en evaluación con IA: %v\n", err)
		return
	}
}

func ejecutarConsultaProcuraduria(ctx context.Context, cfg *configuracion.Configuracion, cedula string) {
	fmt.Printf("[*] Consultando antecedentes en Procuraduría para documento: %s\n", cedula)
	ag := agentes.NuevoAgenteProcuraduria(cfg)
	defer ag.Cerrar()

	res, err := ag.ConsultarCedula(ctx, cedula, "")
	if err != nil {
		fmt.Printf("[X] Error en consulta: %v\n", err)
		return
	}
	fmt.Printf("[+] Resultado: %s | Alerta: %t | Detalles: %s\n", res.Estado, res.Alerta, res.Detalles)
}

func ejecutarConsultaSarlaft(cfg *configuracion.Configuracion, nombre string) {
	fmt.Printf("[*] Consultando SARLAFT ONU para nombre: %s\n", nombre)
	ag := agentes.NuevoAgenteSarlaft(cfg)
	res := ag.ValidarNombre(nombre)
	fmt.Printf("[+] Coincidencia: %t | Alerta: %t | Detalles: %s\n", res.Coincidencia, res.Alerta, res.Detalles)
}

func ejecutarConsultaOfac(ctx context.Context, cfg *configuracion.Configuracion, cedula string) {
	fmt.Printf("[*] Consultando lista OFAC (EE. UU.) para documento: %s\n", cedula)
	ag := agentes.NuevoAgenteOfac(cfg)
	defer ag.Cerrar()

	res, err := ag.ConsultarCedula(ctx, cedula, "")
	if err != nil {
		fmt.Printf("[X] Error en consulta: %v\n", err)
		return
	}
	fmt.Printf("[+] Estado: %s | Coincidencias: %d | Alerta: %t\n", res.Estado, res.TotalCoincidencias, res.Alerta)
}

func ejecutarConsultaAdres(ctx context.Context, cfg *configuracion.Configuracion, cedula string) {
	fmt.Printf("[*] Consultando BDUA de ADRES para documento: %s\n", cedula)
	ag := agentes.NuevoAgenteAdres(cfg)
	defer ag.Cerrar()

	res, err := ag.ConsultarCedula(ctx, cedula)
	if err != nil {
		fmt.Printf("[X] Error en consulta: %v\n", err)
		return
	}
	fmt.Printf("[+] EPS: %s | Estado: %s | Cumple: %t | Ciudadano: %s %s (%s, %s)\n",
		res.EPS, res.EstadoAfiliacion, res.CumpleEPS, res.Nombres, res.Apellidos, res.Municipio, res.Departamento)
}

func ejecutarConsultaRunt(ctx context.Context, cfg *configuracion.Configuracion, cedula, apellido string) {
	if apellido == "" {
		fmt.Println("[!] Debe proporcionar el primer apellido con la bandera -apellido")
		return
	}
	fmt.Printf("[*] Consultando RUNT para documento %s (%s)...\n", cedula, apellido)
	ag := agentes.NuevoAgenteRunt(cfg)
	res, err := ag.ConsultarCedulaConApellido(ctx, cedula, apellido)
	if err != nil {
		fmt.Printf("[X] Error en consulta RUNT: %v\n", err)
		return
	}
	fmt.Printf("[+] RUNT: %s | Inscrito: %t | Conductor: %t | Licencias: %d | Multas: %t\n",
		res.Estado, res.Inscrito, res.ConductorActivo, len(res.Licencias), res.TieneMultas)
}

func ejecutarConsultaSimit(ctx context.Context, cfg *configuracion.Configuracion, cedula string) {
	fmt.Printf("[*] Consultando multas y comparendos en SIMIT para documento: %s\n", cedula)
	ag := agentes.NuevoAgenteSimit(cfg)
	defer ag.Cerrar()

	res, err := ag.ConsultarCedula(ctx, cedula)
	if err != nil {
		fmt.Printf("[X] Error en consulta SIMIT: %v\n", err)
		return
	}
	fmt.Printf("[+] SIMIT: %s | Paz y Salvo: %t | Comparendos: %d | Multas: %d | Acuerdos: %d | Deuda: %s\n",
		res.Estado, res.PazYSalvo, res.CantidadComparendos, res.CantidadMultas, res.CantidadAcuerdos, res.TotalDeuda)
}
