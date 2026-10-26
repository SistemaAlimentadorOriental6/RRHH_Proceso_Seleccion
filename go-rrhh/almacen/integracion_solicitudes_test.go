package almacen

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go-rrhh/modelos"
)

// Corre solo con RRHH_TEST_MYSQL_DSN (ver integracion_test.go): una base vacía y exclusiva para pruebas.

var ofertaDePrueba = modelos.OfertaLaboral{ID: "ABC123", Titulo: "Conductor de Prueba"}

func candidatoPausado(nombre, url string) *modelos.Candidato {
	return &modelos.Candidato{
		Indice: 1, Nombre: nombre, Cedula: "No especificada", URLDetalle: url, Genero: "Hombre",
		TextoPerfil: "Perfil sin cédula", EstadoProceso: modelos.EstadoEsperandoDatos,
		IntentosCedula: []modelos.IntentoCedula{{Metodo: "ficha_html"}, {Metodo: "texto_pdf_cv"}},
		EvaluacionIdoneidad: &modelos.EvaluacionIdoneidad{
			Genero: "Hombre", CumpleGenero: true,
			DetalleEPS: modelos.DetalleNoConsultado, DetalleRunt: modelos.DetalleNoConsultado, DetalleSimit: modelos.DetalleNoConsultado,
			VerificacionPendiente: true,
			MotivosDescarte:       []string{"Verificación pendiente: cédula no encontrada; se le pedirá al candidato que complete sus datos"},
		},
	}
}

// pausar guarda al candidato en pausa y deja constancia, como lo hace el análisis. Devuelve su postulación.
func pausar(t *testing.T, a *Almacen, ctx context.Context, c *modelos.Candidato, correo, telefono string) string {
	t.Helper()
	if err := a.GuardarCandidato(ctx, ofertaDePrueba, c); err != nil {
		t.Fatal(err)
	}
	if err := a.PausarPorDatos(ctx, ofertaDePrueba, c, modelos.PausaDatos{
		Motivo: "cedula_no_encontrada", EtapaPausada: "adres", Correo: correo, Telefono: telefono,
	}); err != nil {
		t.Fatal(err)
	}
	if c.PostulacionID == "" {
		t.Fatal("GuardarCandidato debe dejar el id de la postulación en el candidato")
	}
	return c.PostulacionID
}

func tipos(h []EventoHistorial) []string {
	salida := make([]string, len(h))
	for i, e := range h {
		salida[i] = e.Tipo
	}
	return salida
}

func datosDePrueba(cedula, nacimiento string) DatosCandidato {
	n, _ := time.Parse("2006-01-02", nacimiento)
	return DatosCandidato{
		DatosValidados:  DatosValidados{Cedula: cedula, FechaNacimiento: n, Edad: edadEn(n, time.Now().UTC())},
		PoliticaVersion: "politica-v1",
	}
}

func TestIntegracionPausaGuardaContactoCifradoYCreaLaSolicitud(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "Carlos@Mail.com", "+57 311 234 5678")

	var etapa, dictamen string
	if err := a.db.QueryRow(`SELECT etapa_actual, dictamen FROM postulaciones`).Scan(&etapa, &dictamen); err != nil ||
		etapa != modelos.EstadoEsperandoDatos || dictamen != "pendiente" {
		t.Errorf("postulación: %q %q %v", etapa, dictamen, err)
	}
	var estado string
	var token []byte
	if err := a.db.QueryRow(`SELECT estado, token_hash FROM solicitudes_datos`).Scan(&estado, &token); err != nil || estado != "pendiente_envio" || token != nil {
		t.Errorf("la solicitud nace sin enviar y sin token: %q %v %v", estado, token, err)
	}

	contacto, err := a.ContactoDe(ctx, id)
	if err != nil || contacto.Correo != "carlos@mail.com" || contacto.Telefono != "3112345678" {
		t.Errorf("contacto normalizado: %+v %v", contacto, err)
	}
	var cifrado []byte
	if err := a.db.QueryRow(`SELECT valor_cifrado FROM contactos_candidato WHERE tipo = 'correo'`).Scan(&cifrado); err != nil ||
		strings.Contains(strings.ToLower(string(cifrado)), "carlos") {
		t.Errorf("el correo no debe estar en claro en la base")
	}

	// Pausar otra vez (un nuevo análisis) no duplica la solicitud ni el contacto.
	if err := a.PausarPorDatos(ctx, ofertaDePrueba, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), modelos.PausaDatos{
		Motivo: "cedula_no_encontrada", EtapaPausada: "adres", Correo: "carlos@mail.com", Telefono: "3112345678",
	}); err != nil {
		// Un candidato recién armado no tiene PostulacionID: se localiza por su identidad.
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM solicitudes_datos`); n != 1 {
		t.Errorf("solicitudes = %d; quería 1", n)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM contactos_candidato`); n != 2 {
		t.Errorf("contactos = %d; quería 2 (correo y celular)", n)
	}

	h, err := a.HistorialDe(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	antes := strings.Join(tipos(h)[:4], ",")
	if antes != "cedula.busqueda,cedula.busqueda,solicitud.creada,analisis.pausado" {
		t.Errorf("historial: %v", tipos(h))
	}

	lista, err := a.CandidatosDeOferta(ctx, "ABC123")
	if err != nil || len(lista) != 1 || lista[0].EstadoProceso != modelos.EstadoEsperandoDatos || lista[0].PostulacionID != id {
		t.Errorf("el candidato debe leerse en pausa: %+v %v", lista, err)
	}
}

// Recorrido completo: enlace con token → el candidato lo abre → entrega sus datos → queda encolada la reanudación.
func TestIntegracionFlujoCompletoDelToken(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "3112345678")

	envio, err := a.PrepararEnvio(ctx, id, 7*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(envio.Token) != largoToken || envio.Contacto.Correo != "carlos@mail.com" || envio.Nombre != "Carlos Perez" || envio.Cargo != "Conductor de Prueba" {
		t.Fatalf("envío: %+v", envio)
	}
	// En la base solo queda la huella del token, nunca el token.
	var huella string
	if err := a.db.QueryRow(`SELECT token_hash FROM solicitudes_datos`).Scan(&huella); err != nil || huella != hashToken(envio.Token) || strings.Contains(huella, envio.Token) {
		t.Errorf("huella guardada: %q %v", huella, err)
	}

	// Antes de que el mensaje salga, el enlace no sirve.
	if _, err := a.AbrirSolicitud(ctx, envio.Token); !errors.Is(err, ErrEnlaceInvalido) {
		t.Errorf("enlace sin enviar: %v", err)
	}
	if err := a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}, {Canal: "whatsapp", Error: "falló el envío"}}); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM notificaciones WHERE canal = 'correo' AND estado = 'enviada'`); n != 1 {
		t.Errorf("notificación de correo enviada = %d", n)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM notificaciones WHERE canal = 'whatsapp' AND estado = 'fallida' AND ultimo_error IS NOT NULL`); n != 1 {
		t.Errorf("notificación de WhatsApp fallida = %d", n)
	}

	// Un enlace ajeno o mal formado no abre nada.
	otro, _, _ := nuevoToken()
	for _, malo := range []string{otro, "", "corto", strings.Repeat("a", largoToken+1)} {
		if _, err := a.AbrirSolicitud(ctx, malo); !errors.Is(err, ErrEnlaceInvalido) {
			t.Errorf("token %q: %v", malo, err)
		}
	}

	// El candidato abre su enlace: ve su nombre, el correo enmascarado y el cargo.
	info, err := a.AbrirSolicitud(ctx, envio.Token)
	if err != nil || info.Nombre != "Carlos Perez" || info.Correo != "ca***@mail.com" || info.Cargo != "Conductor de Prueba" || info.VenceEn.IsZero() {
		t.Fatalf("AbrirSolicitud: %+v %v", info, err)
	}
	if _, err := a.AbrirSolicitud(ctx, envio.Token); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT aperturas FROM solicitudes_datos`); n != 2 {
		t.Errorf("aperturas = %d", n)
	}
	h, _ := a.HistorialDe(ctx, id)
	if n := strings.Count(strings.Join(tipos(h), ","), "solicitud.abierta"); n != 1 {
		t.Errorf("la primera apertura se anota una sola vez, aparece %d veces", n)
	}

	// Entrega sus datos.
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); err != nil {
		t.Fatal(err)
	}
	var cedulaCif, nacimientoCif []byte
	var edad int
	var ipHuella string
	if err := a.db.QueryRow(`SELECT cedula_cifrada, fecha_nacimiento_cifrada, edad_al_declarar, COALESCE(ip_hmac, '') FROM datos_declarados`).
		Scan(&cedulaCif, &nacimientoCif, &edad, &ipHuella); err != nil {
		t.Fatal(err)
	}
	if ced, err := a.claves.Descifrar(cedulaCif); err != nil || ced != "1035126774" {
		t.Errorf("cédula declarada = %q %v", ced, err)
	}
	if nac, err := a.claves.Descifrar(nacimientoCif); err != nil || nac != "1990-05-17" {
		t.Errorf("fecha de nacimiento = %q %v", nac, err)
	}
	if strings.Contains(string(cedulaCif), "1035126774") || strings.Contains(string(nacimientoCif), "1990") {
		t.Error("los datos del candidato no deben estar en claro")
	}
	if edad < 35 {
		t.Errorf("edad al declarar = %d", edad)
	}

	// El token es de un solo uso.
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); !errors.Is(err, ErrYaCompletada) {
		t.Errorf("segundo uso: %v", err)
	}
	if _, err := a.AbrirSolicitud(ctx, envio.Token); !errors.Is(err, ErrYaCompletada) {
		t.Errorf("abrir un enlace ya usado: %v", err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM datos_declarados`); n != 1 {
		t.Errorf("datos_declarados = %d; el segundo uso no debe duplicar", n)
	}

	// La cédula declarada es ahora la identidad de la persona, sin dejar la provisional.
	if n := contar(t, a.db, `SELECT COUNT(*) FROM personas`); n != 1 {
		t.Errorf("personas = %d; la provisional debe desaparecer", n)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM personas WHERE cedula_hmac = ?`, a.claves.Pseudonimo("cc:1035126774")); n != 1 {
		t.Error("la persona debe quedar con la cédula declarada")
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'datos_recibidos'`); n != 1 {
		t.Error("el candidato debe quedar en datos_recibidos")
	}

	// Quedó encolada la reanudación, en la misma operación.
	trabajo, err := a.ReclamarReanudacion(ctx, "prueba", time.Minute)
	if err != nil || trabajo == nil || trabajo.PostulacionID != id || trabajo.Intentos != 1 {
		t.Fatalf("ReclamarReanudacion: %+v %v", trabajo, err)
	}
	if otroTrabajo, _ := a.ReclamarReanudacion(ctx, "otro", time.Minute); otroTrabajo != nil {
		t.Error("una reanudación reclamada no la puede tomar otro")
	}

	oferta, cand, err := a.CandidatoPorPostulacion(ctx, id)
	if err != nil || oferta.ID != "ABC123" || cand.Cedula != "1035126774" || cand.EstadoProceso != modelos.EstadoDatosRecibidos {
		t.Fatalf("CandidatoPorPostulacion: %+v %+v %v", oferta, cand, err)
	}
	nacimiento, _ := time.Parse("2006-01-02", "1990-05-17")
	if cand.Edad != edadEn(nacimiento, time.Now().UTC()) {
		t.Errorf("edad = %d", cand.Edad)
	}

	if err := a.TerminarReanudacion(ctx, trabajo, nil); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM trabajos WHERE estado = 'completado'`); n != 1 {
		t.Error("el trabajo debe quedar completado")
	}
}

func TestIntegracionReintentosDeLaReanudacion(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); err != nil {
		t.Fatal(err)
	}

	t1, err := a.ReclamarReanudacion(ctx, "w", time.Minute)
	if err != nil || t1 == nil {
		t.Fatalf("%v %v", t1, err)
	}
	// Falla: vuelve a pendiente pero no antes de la espera de reintento.
	if err := a.TerminarReanudacion(ctx, t1, errors.New("portal caído")); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM trabajos WHERE estado = 'pendiente' AND error = 'portal caído'`); n != 1 {
		t.Error("tras el primer fallo vuelve a pendiente con el motivo")
	}
	if pendiente, _ := a.ReclamarReanudacion(ctx, "w", time.Minute); pendiente != nil {
		t.Error("no se reintenta de inmediato")
	}
	if _, err := a.db.Exec(`UPDATE trabajos SET bloqueado_hasta = ?`, time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	t2, _ := a.ReclamarReanudacion(ctx, "w", time.Minute)
	if t2 == nil || t2.Intentos != 2 {
		t.Fatalf("segundo intento: %+v", t2)
	}

	// Tras el máximo de intentos queda en error definitivo.
	t2.Intentos = maxIntentosReanudar
	if err := a.TerminarReanudacion(ctx, t2, errors.New("sigue caído")); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM trabajos WHERE estado = 'error'`); n != 1 {
		t.Error("agotados los intentos queda en error")
	}

	// Un proceso que se cayó a mitad: el lease vencido lo devuelve a pendiente.
	if _, err := a.db.Exec(`UPDATE trabajos SET estado = 'en_curso', intentos = 1, bloqueado_hasta = ?`, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := a.RecuperarReanudaciones(ctx); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM trabajos WHERE estado = 'pendiente' AND worker_id IS NULL`); n != 1 {
		t.Error("el trabajo con lease vencido debe volver a pendiente")
	}
}

func TestIntegracionEnlaceVencido(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})

	// Pasa el tiempo: el enlace deja de servir y el candidato queda como solicitud vencida.
	if _, err := a.db.Exec(`UPDATE solicitudes_datos SET vence_en = ?`, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); !errors.Is(err, ErrEnlaceVencido) {
		t.Fatalf("completar con el enlace vencido: %v", err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM datos_declarados`); n != 0 {
		t.Error("con el enlace vencido no se guarda nada")
	}
	if _, err := a.AbrirSolicitud(ctx, envio.Token); !errors.Is(err, ErrEnlaceVencido) {
		t.Errorf("abrir vencido: %v", err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'solicitud_vencida'`); n != 1 {
		t.Error("el candidato debe quedar en solicitud_vencida")
	}

	// El reclutador puede reenviar: nace un enlace nuevo y el viejo sigue sin servir.
	nuevo, err := a.PrepararEnvio(ctx, id, time.Hour, false)
	if err != nil || nuevo.Token == envio.Token {
		t.Fatalf("reenvío: %+v %v", nuevo, err)
	}
	_ = a.MarcarEnvio(ctx, nuevo.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})
	if _, err := a.AbrirSolicitud(ctx, envio.Token); err == nil {
		t.Error("el enlace viejo no debe servir")
	}
	if _, err := a.AbrirSolicitud(ctx, nuevo.Token); err != nil {
		t.Errorf("el enlace nuevo debe servir: %v", err)
	}
}

func TestIntegracionVencerSolicitudesCaducadas(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})

	if n, err := a.VencerSolicitudes(ctx); err != nil || n != 0 {
		t.Fatalf("aún vigente: %d %v", n, err)
	}
	if _, err := a.db.Exec(`UPDATE solicitudes_datos SET vence_en = ?`, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n, err := a.VencerSolicitudes(ctx); err != nil || n != 1 {
		t.Fatalf("vencida: %d %v", n, err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM solicitudes_datos WHERE estado = 'vencida'`); n != 1 {
		t.Error("la solicitud debe quedar vencida")
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'solicitud_vencida'`); n != 1 {
		t.Error("el candidato debe quedar en solicitud_vencida")
	}
}

func TestIntegracionRevocaElEnlaceTrasMuchosFallos(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})

	for i := 0; i < maxFallosToken-1; i++ {
		if err := a.RegistrarFallo(ctx, envio.Token); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.AbrirSolicitud(ctx, envio.Token); err != nil {
		t.Fatalf("con %d fallos el enlace aún sirve: %v", maxFallosToken-1, err)
	}
	_ = a.RegistrarFallo(ctx, envio.Token)
	if _, err := a.AbrirSolicitud(ctx, envio.Token); !errors.Is(err, ErrEnlaceInvalido) {
		t.Errorf("al décimo fallo el enlace se revoca: %v", err)
	}
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); !errors.Is(err, ErrEnlaceInvalido) {
		t.Errorf("un enlace revocado no recibe datos: %v", err)
	}
}

func TestIntegracionSoloSePideDatosAQuienEstaEnPausa(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	c := candidatoCompleto(t)
	if err := a.GuardarCandidato(ctx, ofertaDePrueba, c); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PrepararEnvio(ctx, c.PostulacionID, time.Hour, false); !errors.Is(err, ErrEstadoNoPermite) {
		t.Errorf("a quien ya fue evaluado no se le piden datos: %v", err)
	}
	// Para probar el flujo con personas de prueba sí se permite, pero sin contacto no hay a quién escribirle.
	if _, err := a.PrepararEnvio(ctx, c.PostulacionID, time.Hour, true); !errors.Is(err, ErrSinContacto) {
		t.Errorf("sin contacto: %v", err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'completado'`); n != 1 {
		t.Error("fallar por falta de contacto no debe cambiar el estado de quien ya fue evaluado")
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM solicitudes_datos`); n != 0 {
		t.Error("no debe crearse una solicitud sin a quién enviarla")
	}
}

func TestIntegracionSinContactoQuedaMarcado(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	c := candidatoPausado("Pedro Ruiz", "https://x/Detail/pedro")
	c.EstadoProceso = modelos.EstadoSinContacto
	id := pausar(t, a, ctx, c, "", "")
	if _, err := a.PrepararEnvio(ctx, id, time.Hour, false); !errors.Is(err, ErrSinContacto) {
		t.Fatalf("sin contacto: %v", err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'sin_contacto'`); n != 1 {
		t.Error("debe quedar sin_contacto")
	}
	if pendientes, _ := a.EnviosPendientes(ctx); len(pendientes) != 0 {
		t.Errorf("quien no tiene contacto no es un envío pendiente: %v", pendientes)
	}
}

func TestIntegracionEnviosPendientes(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Perez", "https://x/Detail/carlos"), "carlos@mail.com", "")
	if pendientes, err := a.EnviosPendientes(ctx); err != nil || len(pendientes) != 1 || pendientes[0] != id {
		t.Fatalf("pendientes: %v %v", pendientes, err)
	}
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})
	if pendientes, _ := a.EnviosPendientes(ctx); len(pendientes) != 0 {
		t.Errorf("tras enviarse deja de estar pendiente: %v", pendientes)
	}
}

// Si la cédula declarada ya pertenece a otra postulación de la misma vacante no se mezclan: lo revisa una persona.
func TestIntegracionConflictoDeIdentidad(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	existente := candidatoCompleto(t) // cédula 1111111
	if err := a.GuardarCandidato(ctx, ofertaDePrueba, existente); err != nil {
		t.Fatal(err)
	}
	id := pausar(t, a, ctx, candidatoPausado("Otro Perez", "https://x/Detail/otro"), "otro@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})

	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1111111", "1990-05-17")); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'identidad_en_revision'`); n != 1 {
		t.Error("el conflicto debe quedar en revisión")
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM trabajos`); n != 0 {
		t.Error("no se reanuda automáticamente un conflicto de identidad")
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones`); n != 2 {
		t.Errorf("las dos postulaciones se conservan: %d", n)
	}
	h, _ := a.HistorialDe(ctx, id)
	if !strings.Contains(strings.Join(tipos(h), ","), "persona.duplicada") {
		t.Errorf("debe quedar constancia del conflicto: %v", tipos(h))
	}
}

// Si luego se halla la cédula por otra vía, la ficha pasa a la persona con cédula (no se duplica)
// y la solicitud abierta deja de servir.
func TestIntegracionLaFichaPasaALaPersonaConCedula(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	c := candidatoPausado("Carlos Perez", "https://x/Detail/carlos")
	id := pausar(t, a, ctx, c, "carlos@mail.com", "")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})

	// Un análisis posterior sí encuentra la cédula.
	hallado := candidatoCompleto(t)
	hallado.Nombre, hallado.Cedula, hallado.URLDetalle = "Carlos Perez", "1035126774", "https://x/Detail/carlos"
	if err := a.GuardarCandidato(ctx, ofertaDePrueba, hallado); err != nil {
		t.Fatal(err)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones`); n != 1 {
		t.Fatalf("postulaciones = %d; la misma ficha no debe duplicarse", n)
	}
	if hallado.PostulacionID != id {
		t.Errorf("debe ser la misma postulación: %q vs %q", hallado.PostulacionID, id)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM personas`); n != 1 {
		t.Errorf("personas = %d; la provisional debe desaparecer", n)
	}
	if n := contar(t, a.db, `SELECT COUNT(*) FROM postulaciones WHERE etapa_actual = 'completado'`); n != 1 {
		t.Error("la postulación sale de la pausa")
	}
	if _, err := a.AbrirSolicitud(ctx, envio.Token); !errors.Is(err, ErrEnlaceInvalido) {
		t.Errorf("la solicitud abierta debe revocarse: %v", err)
	}
}

func TestIntegracionCedulaConocidaPorCorreoYNombre(t *testing.T) {
	a, ctx := almacenDePrueba(t)
	id := pausar(t, a, ctx, candidatoPausado("Carlos Andres Perez", "https://x/Detail/carlos"), "carlos@mail.com", "3112345678")
	envio, _ := a.PrepararEnvio(ctx, id, time.Hour, false)
	_ = a.MarcarEnvio(ctx, envio.SolicitudID, []ResultadoEnvio{{Canal: "correo", OK: true}})
	if err := a.CompletarSolicitud(ctx, envio.Token, datosDePrueba("1035126774", "1990-05-17")); err != nil {
		t.Fatal(err)
	}

	if ced, err := a.CedulaConocida(ctx, "Carlos Perez", "Carlos@Mail.com"); err != nil || ced != "1035126774" {
		t.Errorf("mismo correo y nombre: %q %v", ced, err)
	}
	if ced, _ := a.CedulaConocida(ctx, "Pedro Ruiz", "carlos@mail.com"); ced != "" {
		t.Errorf("mismo correo pero otro nombre no es la misma persona: %q", ced)
	}
	if ced, _ := a.CedulaConocida(ctx, "Carlos Perez", "otro@mail.com"); ced != "" {
		t.Errorf("otro correo: %q", ced)
	}
	if ced, _ := a.CedulaConocida(ctx, "Carlos Perez", ""); ced != "" {
		t.Errorf("sin correo no se busca: %q", ced)
	}
}
