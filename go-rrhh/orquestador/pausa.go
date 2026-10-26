package orquestador

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-rrhh/agentes"
	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// Pausas es la parte de la base de datos que atiende a los candidatos a los que les falta un dato.
// Es opcional: sin ella (o sin base de datos) un candidato sin cédula se cierra con una verificación
// pendiente, pero no se le pide nada.
type Pausas interface {
	// PausarPorDatos deja constancia de la pausa: intentos de búsqueda, contacto hallado y la
	// solicitud (sin enviarla). El candidato ya debe estar guardado.
	PausarPorDatos(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato, p modelos.PausaDatos) error
	// CedulaConocida devuelve la cédula de una postulación anterior de la misma persona (mismo
	// correo y nombre), o "" si no hay una sola respuesta clara.
	CedulaConocida(ctx context.Context, nombre, correo string) (string, error)
	// RegistrarReanudacion deja en la línea de tiempo cómo terminó una reanudación.
	RegistrarReanudacion(ctx context.Context, c *modelos.Candidato, resultado string) error
}

// textoHojaDeVida junta el texto de la ficha con el de la hoja de vida y el perfil impreso, de
// donde se sacan el correo, el celular y, si hace falta, la cédula.
func textoHojaDeVida(c *modelos.Candidato) string {
	partes := []string{c.TextoPerfil}
	vistas := map[string]bool{}
	for _, ruta := range []string{c.RutaCVOriginal, c.RutaPDF, c.RutaPerfilPDF} {
		if ruta == "" || vistas[ruta] {
			continue
		}
		vistas[ruta] = true
		var texto string
		var err error
		switch strings.ToLower(filepath.Ext(ruta)) {
		case ".pdf":
			texto, err = utilidades.ExtraerTextoPDF(ruta)
		case ".docx":
			texto, err = utilidades.ExtraerTextoDocx(ruta)
		}
		if err == nil && texto != "" {
			partes = append(partes, texto)
		}
	}
	return strings.Join(partes, "\n")
}

// cedulaDePostulacionPrevia busca si la misma persona ya se postuló antes y dejó su cédula, y
// anota el intento. Devuelve "" si no hay base de datos o no hay una respuesta clara.
func (a *Analizador) cedulaDePostulacionPrevia(c *modelos.Candidato) string {
	pausas, ok := a.persistencia.(Pausas)
	if !ok {
		return ""
	}
	hallada := ""
	defer func() {
		c.IntentosCedula = append(c.IntentosCedula, modelos.IntentoCedula{Metodo: "postulacion_previa", Hallada: hallada != ""})
	}()

	correo := utilidades.ExtraerContacto(textoHojaDeVida(c), "").Correo
	if correo == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), plazoPersistencia)
	defer cancel()
	cedula, err := pausas.CedulaConocida(ctx, c.Nombre, correo)
	if err != nil {
		fmt.Printf("[!] No se pudo buscar la cédula en postulaciones anteriores: %v\n", err)
		return ""
	}
	hallada = cedula
	return cedula
}

// pausarPorDatos deja al candidato esperando los datos que solo él puede dar. No es un rechazo ni
// un error: queda "pendiente", con el motivo y los métodos que se probaron, y no se vuelve a leer
// de Computrabajo en los análisis siguientes. Si hay a quién escribirle se crea la solicitud.
func (a *Analizador) pausarPorDatos(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados) {
	contacto := utilidades.ExtraerContacto(textoHojaDeVida(c), "")
	motivo := "Verificación pendiente: cédula no encontrada; se le pedirá al candidato que complete sus datos"
	c.EstadoProceso = modelos.EstadoEsperandoDatos
	if contacto.Correo == "" && contacto.Telefono == "" {
		c.EstadoProceso = modelos.EstadoSinContacto
		motivo = "Verificación pendiente: cédula no encontrada y la hoja de vida no trae correo ni celular para pedírsela"
	}
	c.ErrorProceso = "" // no es un error: un error haría reprocesar al candidato en cada análisis

	a.cerrarSinAptitud(c, carpetaOferta, acum, nil, []string{motivo}, true)

	if pausas, ok := a.persistencia.(Pausas); ok {
		ctx, cancel := context.WithTimeout(context.Background(), plazoPersistencia)
		defer cancel()
		err := pausas.PausarPorDatos(ctx, a.oferta, c, modelos.PausaDatos{
			Motivo: "cedula_no_encontrada", EtapaPausada: "adres", Correo: contacto.Correo, Telefono: contacto.Telefono,
		})
		if err != nil {
			a.etapa(c, "guardado", estadoError, "no se pudo registrar la pausa en la base de datos: "+err.Error(), 0)
		}
	}
}

// identidadDe compara el nombre del perfil con el titular que devolvió ADRES. hayTitular es falso
// si ADRES no devolvió a nadie (no se puede confirmar); confirmada solo es verdadera si el titular
// coincide con el nombre del perfil.
func identidadDe(c *modelos.Candidato) (confirmada, hayTitular bool) {
	r := c.ResultadoAdres
	if r == nil {
		return false, false
	}
	titular := strings.TrimSpace(r.Nombres + " " + r.Apellidos)
	if titular == "" {
		return false, false
	}
	return utilidades.NombresCoinciden(c.Nombre, titular, 2), true
}

// identidadEnRevision deja al candidato pendiente de una revisión manual: la cédula que se
// obtuvo sin verlo en su perfil no se pudo confirmar. No se rechaza: puede ser un error al digitar.
func (a *Analizador) identidadEnRevision(c *modelos.Candidato, carpetaOferta string, acum *acumuladorResultados, hechas map[string]resultadoEtapa, hayTitular bool) {
	motivo := "Verificación pendiente: ADRES no encontró a nadie con la cédula declarada; revisar si se digitó bien"
	if hayTitular {
		motivo = "Verificación pendiente: el titular de la cédula declarada no coincide con el nombre del perfil; revisar"
	}
	c.EstadoProceso = modelos.EstadoIdentidadEnRevision
	c.ErrorProceso = ""
	a.cerrarSinAptitud(c, carpetaOferta, acum, hechas, []string{motivo}, true)
}

// Reanudar continúa el análisis de un candidato que estaba en pausa y ya entregó su cédula. Retoma
// justo después de la lectura del perfil y del género (ya hechos): consulta ADRES, RUNT y SIMIT,
// comprueba la identidad y guarda la evaluación. No vuelve a abrir Computrabajo.
func (a *Analizador) Reanudar(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato) error {
	if a.val.Runt == nil || a.val.Simit == nil || a.val.Adres == nil {
		return errors.New("faltan las consultas RUNT, SIMIT y ADRES: son las que definen la idoneidad")
	}
	if len(soloDigitos(c.Cedula)) < 6 {
		return errors.New("el candidato no tiene cédula: no hay nada que reanudar")
	}
	a.oferta = oferta

	carpetaOferta := filepath.Join(a.cfg.DirectorioDescargas, agentes.LimpiarNombre(oferta.Titulo))
	c.CarpetaCandidato = carpetaDe(c, carpetaOferta)
	_ = os.MkdirAll(c.CarpetaCandidato, 0o755)

	c.EstadoProceso, c.ErrorProceso, c.IntentosCedula = "", "", nil
	c.Genero, _ = a.determinarGenero(ctx, c)

	// El resultado se guarda en la base; no hay candidatos.json de la vacante que reescribir.
	a.continuarDesdeCedula(ctx, c, carpetaOferta, &acumuladorResultados{}, true)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if pausas, ok := a.persistencia.(Pausas); ok {
		ctxBase, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := pausas.RegistrarReanudacion(ctxBase, c, resultadoDe(c)); err != nil {
			return fmt.Errorf("no se pudo registrar la reanudación: %w", err)
		}
	}
	return nil
}

// resultadoDe resume cómo quedó el candidato después de una reanudación.
func resultadoDe(c *modelos.Candidato) string {
	switch {
	case c.EstadoProceso == modelos.EstadoIdentidadEnRevision:
		return "identidad_en_revision"
	case c.EvaluacionIdoneidad != nil && c.EvaluacionIdoneidad.VerificacionPendiente:
		return "pendiente"
	case c.EsApto:
		return "apto"
	}
	return "no_apto"
}

// carpetaDe es la carpeta donde viven los archivos del candidato: la de los que ya tiene o, si no
// tiene ninguno, una nueva dentro de la carpeta de la vacante.
func carpetaDe(c *modelos.Candidato, carpetaOferta string) string {
	for _, ruta := range []string{c.RutaPDF, c.RutaCVOriginal, c.RutaPerfilPDF, c.RutaFoto} {
		if ruta != "" {
			return filepath.Dir(ruta)
		}
	}
	return filepath.Join(carpetaOferta, fmt.Sprintf("%d_%s", c.Indice, agentes.LimpiarNombre(c.Nombre)))
}
