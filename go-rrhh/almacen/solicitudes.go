package almacen

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-rrhh/modelos"
	"go-rrhh/utilidades"
)

// Errores del flujo de datos faltantes. Los de enlace los traduce la página pública a un mensaje
// para el candidato; el resto son del reclutador.
var (
	ErrEnlaceInvalido  = errors.New("el enlace no es válido")
	ErrEnlaceVencido   = errors.New("el enlace venció")
	ErrYaCompletada    = errors.New("la solicitud ya fue completada")
	ErrEstadoNoPermite = errors.New("el candidato no está esperando datos")
	ErrSinContacto     = errors.New("no hay correo ni celular del candidato a quien pedirle los datos")
)

// Estados de solicitudes_datos.
const (
	solPendienteEnvio = "pendiente_envio"
	solEnviada        = "enviada"
	solAbierta        = "abierta"
	solCompletada     = "completada"
	solVencida        = "vencida"
	solRevocada       = "revocada"

	motivoCedula    = "cedula_no_encontrada"
	etapaTrasCedula = "adres" // primera etapa que necesita la cédula

	// maxFallosToken son los datos rechazados que tolera un enlace antes de revocarse.
	maxFallosToken = 10

	plantillaSolicitud = "solicitud_datos"
)

// consultor es lo que comparten *sql.DB y *sql.Tx para leer.
type consultor interface {
	QueryContext(ctx context.Context, consulta string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, consulta string, args ...any) *sql.Row
}

// Contacto son el correo y el celular hallados en la hoja de vida del candidato.
type Contacto struct{ Correo, Telefono string }

func normalizarCorreo(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normalizarTelefono deja un celular colombiano de 10 dígitos sin el prefijo del país ("" si no lo es).
func normalizarTelefono(s string) string {
	d := strings.TrimPrefix(soloDigitos(s), "57")
	if len(d) != 10 {
		return ""
	}
	return d
}

// historialTx agrega un renglón a la línea de tiempo del candidato. detalle no debe llevar datos personales.
func historialTx(ctx context.Context, tx *sql.Tx, postulacion []byte, tipo string, detalle any) error {
	var datos sql.NullString
	if detalle != nil {
		if b, err := json.Marshal(detalle); err == nil {
			datos = sql.NullString{String: string(b), Valid: true}
		}
	}
	return exec(ctx, tx,
		`INSERT INTO postulacion_historial (postulacion_id, tipo, detalle, creado_en) VALUES (?,?,?,?)`,
		postulacion, tipo, datos, time.Now().UTC())
}

func (a *Almacen) guardarContacto(ctx context.Context, tx *sql.Tx, postulacion []byte, tipo, valor string, momento time.Time) error {
	if valor == "" {
		return nil
	}
	cifrado, err := a.claves.Cifrar(valor)
	if err != nil {
		return err
	}
	id, err := nuevoID()
	if err != nil {
		return err
	}
	return exec(ctx, tx,
		`INSERT INTO contactos_candidato (id, postulacion_id, tipo, valor_hmac, valor_cifrado, origen, detectado_en)
		 VALUES (?,?,?,?,?, 'cv', ?)
		 ON DUPLICATE KEY UPDATE detectado_en = VALUES(detectado_en)`,
		id, postulacion, tipo, a.claves.Pseudonimo(tipo+":"+valor), cifrado, momento)
}

// contacto devuelve el correo y el celular más recientes del candidato (descifrados).
func (a *Almacen) contacto(ctx context.Context, q consultor, postulacion []byte) (Contacto, error) {
	var c Contacto
	filas, err := q.QueryContext(ctx,
		`SELECT tipo, valor_cifrado FROM contactos_candidato WHERE postulacion_id = ? ORDER BY detectado_en DESC, id DESC`, postulacion)
	if err != nil {
		return c, err
	}
	defer filas.Close()
	for filas.Next() {
		var tipo string
		var cifrado []byte
		if err := filas.Scan(&tipo, &cifrado); err != nil {
			return c, err
		}
		valor, err := a.claves.Descifrar(cifrado)
		if err != nil {
			return c, fmt.Errorf("no se pudo descifrar un contacto (¿cambió RRHH_CLAVE_CIFRADO?): %w", err)
		}
		switch {
		case tipo == "correo" && c.Correo == "":
			c.Correo = valor
		case tipo == "telefono" && c.Telefono == "":
			c.Telefono = valor
		}
	}
	return c, filas.Err()
}

// ContactoDe devuelve el contacto guardado de una postulación.
func (a *Almacen) ContactoDe(ctx context.Context, postulacionID string) (Contacto, error) {
	id, err := idBytes(postulacionID)
	if err != nil {
		return Contacto{}, err
	}
	return a.contacto(ctx, a.db, id)
}

// postulacionDe localiza la postulación de un candidato ya guardado.
func (a *Almacen) postulacionDe(ctx context.Context, ofertaID []byte, c *modelos.Candidato) ([]byte, error) {
	if c.PostulacionID != "" {
		return idBytes(c.PostulacionID)
	}
	clave, _ := identidad(c)
	if clave == "" {
		return nil, errors.New("el candidato no tiene cédula ni URL de ficha: no se puede identificar")
	}
	var id []byte
	err := a.db.QueryRowContext(ctx,
		`SELECT po.id FROM postulaciones po JOIN personas pe ON pe.id = po.persona_id
		  WHERE pe.cedula_hmac = ? AND po.oferta_id = ?`, a.claves.Pseudonimo(clave), ofertaID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("la postulación todavía no está guardada")
	}
	return id, err
}

// GuardarContacto registra el correo y el celular hallados en la hoja de vida de una postulación
// que no pasó por la pausa (p. ej. se le pide datos a quien ya tiene cédula).
func (a *Almacen) GuardarContacto(ctx context.Context, postulacionID, correo, telefono string) error {
	postulacion, err := idBytes(postulacionID)
	if err != nil {
		return err
	}
	momento := time.Now().UTC()
	return a.enTx(ctx, func(tx *sql.Tx) error {
		if err := a.guardarContacto(ctx, tx, postulacion, "correo", normalizarCorreo(correo), momento); err != nil {
			return err
		}
		return a.guardarContacto(ctx, tx, postulacion, "telefono", normalizarTelefono(telefono), momento)
	})
}

// PausarPorDatos deja constancia de que el análisis del candidato quedó en pausa por un dato que
// falta: guarda qué métodos se probaron, el contacto hallado y crea la solicitud (sin enviarla).
// El candidato ya debe estar guardado (GuardarCandidato) con su EstadoProceso de pausa.
func (a *Almacen) PausarPorDatos(ctx context.Context, oferta modelos.OfertaLaboral, c *modelos.Candidato, p modelos.PausaDatos) error {
	ofertaID, err := a.asegurarOferta(ctx, oferta)
	if err != nil {
		return err
	}
	postulacion, err := a.postulacionDe(ctx, ofertaID, c)
	if err != nil {
		return err
	}
	correo, telefono := normalizarCorreo(p.Correo), normalizarTelefono(p.Telefono)
	solicitud, err := nuevoID()
	if err != nil {
		return err
	}
	momento := time.Now().UTC()

	return a.enTx(ctx, func(tx *sql.Tx) error {
		for _, i := range c.IntentosCedula {
			if err := historialTx(ctx, tx, postulacion, "cedula.busqueda", map[string]any{"metodo": i.Metodo, "hallada": i.Hallada}); err != nil {
				return err
			}
		}
		if err := a.guardarContacto(ctx, tx, postulacion, "correo", correo, momento); err != nil {
			return err
		}
		if err := a.guardarContacto(ctx, tx, postulacion, "telefono", telefono, momento); err != nil {
			return err
		}

		var abiertas int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM solicitudes_datos WHERE postulacion_id = ? AND estado IN ('pendiente_envio','enviada','abierta')`,
			postulacion).Scan(&abiertas); err != nil {
			return err
		}
		if abiertas == 0 {
			if err := exec(ctx, tx,
				`INSERT INTO solicitudes_datos (id, postulacion_id, motivo, etapa_pausada, estado, creada_en, actualizada_en)
				 VALUES (?,?,?,?, 'pendiente_envio', ?, ?)`, solicitud, postulacion, p.Motivo, p.EtapaPausada, momento, momento); err != nil {
				return err
			}
			if err := historialTx(ctx, tx, postulacion, "solicitud.creada", map[string]any{"motivo": p.Motivo}); err != nil {
				return err
			}
		}
		return historialTx(ctx, tx, postulacion, "analisis.pausado", map[string]any{
			"motivo": p.Motivo, "etapa_pausada": p.EtapaPausada, "con_contacto": correo != "" || telefono != "",
		})
	})
}

// Envio es lo necesario para escribirle al candidato: el token se genera aquí y no se vuelve a poder leer.
type Envio struct {
	SolicitudID   string
	PostulacionID string
	Token         string // solo existe en memoria durante el envío; en la base queda su huella
	Nombre        string
	Cargo         string
	Contacto      Contacto
	VenceEn       time.Time
}

// PrepararEnvio genera un token nuevo para la solicitud abierta del candidato (o crea la
// solicitud si no hay ninguna) e invalida el anterior: reenviar siempre da un enlace nuevo.
// Falla con ErrEstadoNoPermite si el candidato no está esperando datos (salvo cualquierEtapa, que
// solo se usa para probar el flujo con personas de prueba) y con ErrSinContacto si no hay a
// quién escribirle (en ese caso queda marcado sin_contacto).
func (a *Almacen) PrepararEnvio(ctx context.Context, postulacionID string, vigencia time.Duration, cualquierEtapa bool) (Envio, error) {
	postulacion, err := idBytes(postulacionID)
	if err != nil {
		return Envio{}, err
	}
	token, huella, err := nuevoToken()
	if err != nil {
		return Envio{}, err
	}
	nuevaSolicitud, err := nuevoID()
	if err != nil {
		return Envio{}, err
	}
	ahora := time.Now().UTC()
	vence := ahora.Add(vigencia)

	var envio Envio
	var sinContacto bool
	err = a.enTx(ctx, func(tx *sql.Tx) error {
		sinContacto = false
		var etapa, nombre, cargo string
		if err := tx.QueryRowContext(ctx,
			`SELECT po.etapa_actual, COALESCE(pf.nombre, ''), o.titulo
			   FROM postulaciones po
			   JOIN ofertas o ON o.id = po.oferta_id
			   LEFT JOIN perfiles pf ON pf.postulacion_id = po.id
			  WHERE po.id = ? FOR UPDATE`, postulacion).Scan(&etapa, &nombre, &cargo); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrEstadoNoPermite
			}
			return err
		}
		enPausaPorDatos := etapa == modelos.EstadoEsperandoDatos || etapa == modelos.EstadoSinContacto || etapa == modelos.EstadoSolicitudVencida
		if !enPausaPorDatos && !cualquierEtapa {
			return ErrEstadoNoPermite
		}
		contacto, err := a.contacto(ctx, tx, postulacion)
		if err != nil {
			return err
		}
		if contacto.Correo == "" && contacto.Telefono == "" {
			sinContacto = true
			if enPausaPorDatos && etapa != modelos.EstadoSinContacto {
				if err := exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ?`,
					modelos.EstadoSinContacto, ahora, postulacion); err != nil {
					return err
				}
				return historialTx(ctx, tx, postulacion, "solicitud.sin_contacto", nil)
			}
			return nil
		}

		var solicitud []byte
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM solicitudes_datos WHERE postulacion_id = ? AND estado IN ('pendiente_envio','enviada','abierta')
			  ORDER BY creada_en DESC LIMIT 1 FOR UPDATE`, postulacion).Scan(&solicitud)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			solicitud = nuevaSolicitud
			if err := exec(ctx, tx,
				`INSERT INTO solicitudes_datos (id, postulacion_id, motivo, etapa_pausada, estado, creada_en, actualizada_en)
				 VALUES (?,?,?,?, 'pendiente_envio', ?, ?)`, solicitud, postulacion, motivoCedula, etapaTrasCedula, ahora, ahora); err != nil {
				return err
			}
			if err := historialTx(ctx, tx, postulacion, "solicitud.creada", map[string]any{"motivo": motivoCedula}); err != nil {
				return err
			}
		case err != nil:
			return err
		}

		if err := exec(ctx, tx,
			`UPDATE solicitudes_datos SET token_hash = ?, vence_en = ?, envios = envios + 1, estado = 'pendiente_envio',
			   intentos_fallidos = 0, actualizada_en = ? WHERE id = ?`, huella, vence, ahora, solicitud); err != nil {
			return err
		}
		envio = Envio{
			SolicitudID: idTexto(solicitud), PostulacionID: postulacionID, Token: token,
			Nombre: nombre, Cargo: cargo, Contacto: contacto, VenceEn: vence,
		}
		return nil
	})
	if err != nil {
		return Envio{}, err
	}
	if sinContacto {
		return Envio{}, ErrSinContacto
	}
	return envio, nil
}

// ResultadoEnvio es lo que pasó al intentar escribirle al candidato por un canal.
type ResultadoEnvio struct {
	Canal string // correo | whatsapp
	OK    bool
	Error string // sin datos personales
}

// MarcarEnvio registra el resultado de cada canal. Con al menos uno exitoso la solicitud pasa a
// enviada y el candidato queda esperando; si todos fallan la solicitud sigue pendiente de envío.
func (a *Almacen) MarcarEnvio(ctx context.Context, solicitudID string, resultados []ResultadoEnvio) error {
	solicitud, err := idBytes(solicitudID)
	if err != nil {
		return err
	}
	ahora := time.Now().UTC()
	return a.enTx(ctx, func(tx *sql.Tx) error {
		var postulacion []byte
		if err := tx.QueryRowContext(ctx, `SELECT postulacion_id FROM solicitudes_datos WHERE id = ? FOR UPDATE`, solicitud).Scan(&postulacion); err != nil {
			return err
		}
		alguno := false
		for _, r := range resultados {
			tipoContacto := "correo"
			if r.Canal == "whatsapp" {
				tipoContacto = "telefono"
			}
			var contactoID []byte
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM contactos_candidato WHERE postulacion_id = ? AND tipo = ? ORDER BY detectado_en DESC, id DESC LIMIT 1`,
				postulacion, tipoContacto).Scan(&contactoID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			estado := "fallida"
			var enviada sql.NullTime
			if r.OK {
				estado, alguno = "enviada", true
				enviada = sql.NullTime{Time: ahora, Valid: true}
			}
			notificacion, err := nuevoID()
			if err != nil {
				return err
			}
			if err := exec(ctx, tx,
				`INSERT INTO notificaciones (id, postulacion_id, contacto_id, canal, plantilla, estado, intentos, ultimo_error, enviada_en, actualizado_en)
				 VALUES (?,?,?,?,?,?,1,?,?,?)
				 ON DUPLICATE KEY UPDATE contacto_id = VALUES(contacto_id), estado = VALUES(estado), intentos = intentos + 1,
				   ultimo_error = VALUES(ultimo_error), enviada_en = IF(VALUES(estado) = 'enviada', VALUES(enviada_en), enviada_en),
				   actualizado_en = VALUES(actualizado_en)`,
				notificacion, postulacion, contactoID, r.Canal, plantillaSolicitud, estado, texto(r.Error, 500), enviada, ahora); err != nil {
				return err
			}
			if err := historialTx(ctx, tx, postulacion, "solicitud.enviada", map[string]any{"canal": r.Canal, "ok": r.OK, "error": r.Error}); err != nil {
				return err
			}
		}
		if !alguno {
			return nil
		}
		if err := exec(ctx, tx, `UPDATE solicitudes_datos SET estado = 'enviada', enviada_en = ?, actualizada_en = ? WHERE id = ?`, ahora, ahora, solicitud); err != nil {
			return err
		}
		return exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ? AND etapa_actual IN (?, ?)`,
			modelos.EstadoEsperandoDatos, ahora, postulacion, modelos.EstadoSinContacto, modelos.EstadoSolicitudVencida)
	})
}

// EnvioPrevio dice por qué canales ya se le envió al candidato la solicitud vigente (enviada, abierta
// o completada). Una solicitud vencida o revocada no cuenta: ahí sí se puede volver a escribirle.
func (a *Almacen) EnvioPrevio(ctx context.Context, postulacionID string) (correo, whatsapp bool, err error) {
	id, err := idBytes(postulacionID)
	if err != nil {
		return false, false, err
	}
	filas, err := a.db.QueryContext(ctx,
		`SELECT n.canal FROM notificaciones n
		  WHERE n.postulacion_id = ? AND n.plantilla = ? AND n.estado = 'enviada'
		    AND EXISTS (SELECT 1 FROM solicitudes_datos s WHERE s.postulacion_id = n.postulacion_id
		                 AND s.estado IN ('enviada','abierta','completada'))`, id, plantillaSolicitud)
	if err != nil {
		return false, false, err
	}
	defer filas.Close()
	for filas.Next() {
		var canal string
		if err := filas.Scan(&canal); err != nil {
			return false, false, err
		}
		correo = correo || canal == "correo"
		whatsapp = whatsapp || canal == "whatsapp"
	}
	return correo, whatsapp, filas.Err()
}

// EnviosPendientes lista las postulaciones esperando datos cuya solicitud todavía no se ha enviado
// nunca (solo se usa cuando el envío automático está activo).
func (a *Almacen) EnviosPendientes(ctx context.Context) ([]string, error) {
	filas, err := a.db.QueryContext(ctx,
		`SELECT po.id FROM solicitudes_datos s JOIN postulaciones po ON po.id = s.postulacion_id
		  WHERE s.estado = 'pendiente_envio' AND s.envios = 0 AND po.etapa_actual = ?
		  ORDER BY s.creada_en LIMIT 500`, modelos.EstadoEsperandoDatos)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var ids []string
	for filas.Next() {
		var id []byte
		if err := filas.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, idTexto(id))
	}
	return ids, filas.Err()
}

// InfoSolicitud es lo que la página pública le muestra al candidato.
type InfoSolicitud struct {
	Nombre  string
	Correo  string // enmascarado
	Cargo   string
	VenceEn time.Time
}

// AbrirSolicitud valida el token y, si sirve, registra la apertura y devuelve lo que se muestra
// en el formulario. Los errores son los sentinelas del paquete.
func (a *Almacen) AbrirSolicitud(ctx context.Context, token string) (InfoSolicitud, error) {
	if !formaTokenValida(token) {
		return InfoSolicitud{}, ErrEnlaceInvalido
	}
	ahora := time.Now().UTC()
	var info InfoSolicitud
	var resultado error
	err := a.enTx(ctx, func(tx *sql.Tx) error {
		info, resultado = InfoSolicitud{}, nil
		var solicitud, postulacion []byte
		var estado, nombre, cargo string
		var vence sql.NullTime
		var aperturas int
		err := tx.QueryRowContext(ctx,
			`SELECT s.id, s.postulacion_id, s.estado, s.vence_en, s.aperturas, COALESCE(pf.nombre, ''), o.titulo
			   FROM solicitudes_datos s
			   JOIN postulaciones po ON po.id = s.postulacion_id
			   JOIN ofertas o ON o.id = po.oferta_id
			   LEFT JOIN perfiles pf ON pf.postulacion_id = po.id
			  WHERE s.token_hash = ? FOR UPDATE`, hashToken(token)).Scan(&solicitud, &postulacion, &estado, &vence, &aperturas, &nombre, &cargo)
		if errors.Is(err, sql.ErrNoRows) {
			resultado = ErrEnlaceInvalido
			return nil
		}
		if err != nil {
			return err
		}
		info = InfoSolicitud{Nombre: nombre, Cargo: cargo, VenceEn: vence.Time}

		switch estado {
		case solCompletada:
			resultado = ErrYaCompletada
			return nil
		case solVencida:
			resultado = ErrEnlaceVencido
			return nil
		case solEnviada, solAbierta:
		default: // pendiente_envio (nadie recibió ese enlace) o revocada
			resultado = ErrEnlaceInvalido
			return nil
		}
		if vence.Valid && vence.Time.Before(ahora) {
			resultado = ErrEnlaceVencido
			if err := exec(ctx, tx, `UPDATE solicitudes_datos SET estado = 'vencida', actualizada_en = ? WHERE id = ?`, ahora, solicitud); err != nil {
				return err
			}
			if err := exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ? AND etapa_actual = ?`,
				modelos.EstadoSolicitudVencida, ahora, postulacion, modelos.EstadoEsperandoDatos); err != nil {
				return err
			}
			return historialTx(ctx, tx, postulacion, "solicitud.vencida", nil)
		}

		if err := exec(ctx, tx,
			`UPDATE solicitudes_datos SET estado = 'abierta', aperturas = aperturas + 1, abierta_en = COALESCE(abierta_en, ?), actualizada_en = ?
			  WHERE id = ?`, ahora, ahora, solicitud); err != nil {
			return err
		}
		if aperturas == 0 {
			if err := historialTx(ctx, tx, postulacion, "solicitud.abierta", nil); err != nil {
				return err
			}
		}
		contacto, err := a.contacto(ctx, tx, postulacion)
		if err != nil {
			return err
		}
		info.Correo = enmascararCorreo(contacto.Correo)
		return nil
	})
	if err != nil {
		return InfoSolicitud{}, err
	}
	if resultado != nil {
		return info, resultado
	}
	return info, nil
}

// DatosCandidato es lo que el candidato envía con el formulario, ya validado.
type DatosCandidato struct {
	DatosValidados
	PoliticaVersion string // política de datos personales que aceptó
	IPHuella        string // PseudonimoIP de su dirección (puede estar vacía)
}

// PseudonimoIP resume una dirección IP sin guardarla.
func (a *Almacen) PseudonimoIP(ip string) string {
	if ip == "" {
		return ""
	}
	return a.claves.Pseudonimo("ip:" + ip)
}

// CompletarSolicitud guarda lo que escribió el candidato y consume el token, todo en una
// transacción: datos y consentimiento cifrados, cédula vinculada a la persona, solicitud
// completada y, si no hay conflicto de identidad, la reanudación encolada para el trabajador.
func (a *Almacen) CompletarSolicitud(ctx context.Context, token string, d DatosCandidato) error {
	if !formaTokenValida(token) {
		return ErrEnlaceInvalido
	}
	cedulaCifrada, err := a.claves.Cifrar(d.Cedula)
	if err != nil {
		return err
	}
	nacimientoCifrado, err := a.claves.Cifrar(d.FechaNacimiento.Format("2006-01-02"))
	if err != nil {
		return err
	}
	hmacCedula := a.claves.Pseudonimo("cc:" + d.Cedula)
	declarado, err := nuevoID()
	if err != nil {
		return err
	}
	personaNueva, err := nuevoID()
	if err != nil {
		return err
	}
	trabajo, err := nuevoID()
	if err != nil {
		return err
	}
	ahora := time.Now().UTC()

	var resultado error
	err = a.enTx(ctx, func(tx *sql.Tx) error {
		resultado = nil
		var solicitud, postulacion, ofertaID, personaVieja []byte
		var estado string
		var vence sql.NullTime
		err := tx.QueryRowContext(ctx,
			`SELECT s.id, s.postulacion_id, s.estado, s.vence_en, po.oferta_id, po.persona_id
			   FROM solicitudes_datos s JOIN postulaciones po ON po.id = s.postulacion_id
			  WHERE s.token_hash = ? FOR UPDATE`, hashToken(token)).Scan(&solicitud, &postulacion, &estado, &vence, &ofertaID, &personaVieja)
		if errors.Is(err, sql.ErrNoRows) {
			resultado = ErrEnlaceInvalido
			return nil
		}
		if err != nil {
			return err
		}
		switch estado {
		case solCompletada:
			resultado = ErrYaCompletada
			return nil
		case solVencida:
			resultado = ErrEnlaceVencido
			return nil
		case solEnviada, solAbierta:
		default:
			resultado = ErrEnlaceInvalido
			return nil
		}
		if vence.Valid && vence.Time.Before(ahora) {
			resultado = ErrEnlaceVencido
			if err := exec(ctx, tx, `UPDATE solicitudes_datos SET estado = 'vencida', actualizada_en = ? WHERE id = ?`, ahora, solicitud); err != nil {
				return err
			}
			return exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ? AND etapa_actual = ?`,
				modelos.EstadoSolicitudVencida, ahora, postulacion, modelos.EstadoEsperandoDatos)
		}

		// La cédula declarada pasa a ser la identidad de la persona (antes era provisional).
		if err := exec(ctx, tx,
			`INSERT INTO personas (id, cedula_hmac, cedula_cifrada, creado_en, actualizado_en) VALUES (?,?,?,?,?)
			 ON DUPLICATE KEY UPDATE actualizado_en = VALUES(actualizado_en)`,
			personaNueva, hmacCedula, cedulaCifrada, ahora, ahora); err != nil {
			return err
		}
		var persona []byte
		if err := tx.QueryRowContext(ctx, `SELECT id FROM personas WHERE cedula_hmac = ?`, hmacCedula).Scan(&persona); err != nil {
			return err
		}
		// Si esa persona ya tiene otra postulación en la misma vacante no se mezclan: lo decide un reclutador.
		var otras int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM postulaciones WHERE persona_id = ? AND oferta_id = ? AND id <> ?`, persona, ofertaID, postulacion).Scan(&otras); err != nil {
			return err
		}
		conflicto := otras > 0
		if !conflicto && !bytes.Equal(persona, personaVieja) {
			if err := exec(ctx, tx, `UPDATE postulaciones SET persona_id = ? WHERE id = ?`, persona, postulacion); err != nil {
				return err
			}
			if err := exec(ctx, tx,
				`DELETE FROM personas WHERE id = ? AND NOT EXISTS (SELECT 1 FROM postulaciones po WHERE po.persona_id = ?)`,
				personaVieja, personaVieja); err != nil {
				return err
			}
		}

		if err := exec(ctx, tx,
			`INSERT INTO datos_declarados (id, solicitud_id, postulacion_id, cedula_hmac, cedula_cifrada, fecha_nacimiento_cifrada,
			   edad_al_declarar, politica_version, politica_aceptada_en, ip_hmac, creado_en)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			declarado, solicitud, postulacion, hmacCedula, cedulaCifrada, nacimientoCifrado,
			d.Edad, d.PoliticaVersion, ahora, texto(d.IPHuella, 64), ahora); err != nil {
			return err
		}
		if err := exec(ctx, tx, `UPDATE solicitudes_datos SET estado = 'completada', completada_en = ?, actualizada_en = ? WHERE id = ?`, ahora, ahora, solicitud); err != nil {
			return err
		}
		if err := historialTx(ctx, tx, postulacion, "solicitud.completada", map[string]any{"edad": d.Edad, "politica": d.PoliticaVersion}); err != nil {
			return err
		}

		if conflicto {
			if err := exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ?`,
				modelos.EstadoIdentidadEnRevision, ahora, postulacion); err != nil {
				return err
			}
			return historialTx(ctx, tx, postulacion, "persona.duplicada", map[string]any{"detalle": "la cédula declarada ya tiene otra postulación en esta vacante"})
		}
		if err := exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ?`,
			modelos.EstadoDatosRecibidos, ahora, postulacion); err != nil {
			return err
		}
		return a.encolarReanudacion(ctx, tx, trabajo, ofertaID, postulacion, ahora)
	})
	if err != nil {
		return err
	}
	return resultado
}

// RegistrarFallo cuenta un envío de datos rechazado con este token; al pasar de maxFallosToken
// el enlace se revoca (alguien está probando valores).
func (a *Almacen) RegistrarFallo(ctx context.Context, token string) error {
	if !formaTokenValida(token) {
		return nil
	}
	// El estado se asigna primero: usa el contador antes de incrementarlo.
	return a.enTx(ctx, func(tx *sql.Tx) error {
		return exec(ctx, tx,
			`UPDATE solicitudes_datos SET estado = IF(intentos_fallidos + 1 >= ?, 'revocada', estado),
			   intentos_fallidos = intentos_fallidos + 1, actualizada_en = ?
			  WHERE token_hash = ? AND estado IN ('enviada','abierta')`, maxFallosToken, time.Now().UTC(), hashToken(token))
	})
}

// VencerSolicitudes marca como vencidas las solicitudes cuyo enlace caducó sin respuesta y deja
// a esos candidatos como solicitud_vencida. Devuelve cuántas venció.
func (a *Almacen) VencerSolicitudes(ctx context.Context) (int, error) {
	ahora := time.Now().UTC()
	filas, err := a.db.QueryContext(ctx,
		`SELECT id, postulacion_id FROM solicitudes_datos WHERE estado IN ('enviada','abierta') AND vence_en < ?`, ahora)
	if err != nil {
		return 0, err
	}
	type par struct{ solicitud, postulacion []byte }
	var vencidas []par
	for filas.Next() {
		var p par
		if err := filas.Scan(&p.solicitud, &p.postulacion); err != nil {
			filas.Close()
			return 0, err
		}
		vencidas = append(vencidas, p)
	}
	filas.Close()

	for _, p := range vencidas {
		err := a.enTx(ctx, func(tx *sql.Tx) error {
			if err := exec(ctx, tx, `UPDATE solicitudes_datos SET estado = 'vencida', actualizada_en = ? WHERE id = ? AND estado IN ('enviada','abierta')`, ahora, p.solicitud); err != nil {
				return err
			}
			if err := exec(ctx, tx, `UPDATE postulaciones SET etapa_actual = ?, actualizado_en = ? WHERE id = ? AND etapa_actual = ?`,
				modelos.EstadoSolicitudVencida, ahora, p.postulacion, modelos.EstadoEsperandoDatos); err != nil {
				return err
			}
			return historialTx(ctx, tx, p.postulacion, "solicitud.vencida", nil)
		})
		if err != nil {
			return 0, err
		}
	}
	return len(vencidas), nil
}

// EventoHistorial es un renglón de la línea de tiempo de un candidato.
type EventoHistorial struct {
	Tipo    string          `json:"tipo"`
	Detalle json.RawMessage `json:"detalle,omitempty"`
	Fecha   time.Time       `json:"fecha"`
}

// HistorialDe devuelve la línea de tiempo del candidato, de la más antigua a la más reciente.
func (a *Almacen) HistorialDe(ctx context.Context, postulacionID string) ([]EventoHistorial, error) {
	id, err := idBytes(postulacionID)
	if err != nil {
		return nil, err
	}
	filas, err := a.db.QueryContext(ctx,
		`SELECT tipo, detalle, creado_en FROM postulacion_historial WHERE postulacion_id = ? ORDER BY id LIMIT 500`, id)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	eventos := []EventoHistorial{}
	for filas.Next() {
		var e EventoHistorial
		var detalle sql.NullString
		if err := filas.Scan(&e.Tipo, &detalle, &e.Fecha); err != nil {
			return nil, err
		}
		if detalle.Valid {
			e.Detalle = json.RawMessage(detalle.String)
		}
		eventos = append(eventos, e)
	}
	return eventos, filas.Err()
}

// CedulaConocida busca si la misma persona ya se postuló antes y dejó su cédula: coincide el
// correo y el nombre. No se usa el celular (se comparte con familiares) ni se adivina: si hay más
// de una cédula posible devuelve "".
func (a *Almacen) CedulaConocida(ctx context.Context, nombre, correo string) (string, error) {
	correo = normalizarCorreo(correo)
	if correo == "" {
		return "", nil
	}
	filas, err := a.db.QueryContext(ctx,
		`SELECT DISTINCT pe.cedula_cifrada, COALESCE(pf.nombre, '')
		   FROM contactos_candidato cc
		   JOIN postulaciones po ON po.id = cc.postulacion_id
		   JOIN personas pe ON pe.id = po.persona_id
		   LEFT JOIN perfiles pf ON pf.postulacion_id = po.id
		  WHERE cc.tipo = 'correo' AND cc.valor_hmac = ?`, a.claves.Pseudonimo("correo:"+correo))
	if err != nil {
		return "", err
	}
	defer filas.Close()
	candidatas := map[string]struct{}{}
	for filas.Next() {
		var cifrada []byte
		var nombreConocido string
		if err := filas.Scan(&cifrada, &nombreConocido); err != nil {
			return "", err
		}
		cedula, err := a.claves.Descifrar(cifrada)
		if err != nil || len(soloDigitos(cedula)) < 6 {
			continue // persona provisional (sin cédula) o dato ilegible
		}
		if utilidades.NombresCoinciden(nombre, nombreConocido, 2) {
			candidatas[cedula] = struct{}{}
		}
	}
	if err := filas.Err(); err != nil {
		return "", err
	}
	if len(candidatas) != 1 {
		return "", nil
	}
	for cedula := range candidatas {
		return cedula, nil
	}
	return "", nil
}

// CandidatoPorPostulacion carga a un solo candidato con la vacante a la que se postuló.
func (a *Almacen) CandidatoPorPostulacion(ctx context.Context, postulacionID string) (modelos.OfertaLaboral, *modelos.Candidato, error) {
	id, err := idBytes(postulacionID)
	if err != nil {
		return modelos.OfertaLaboral{}, nil, err
	}
	var oferta modelos.OfertaLaboral
	if err := a.db.QueryRowContext(ctx,
		`SELECT o.id_computrabajo, o.titulo FROM postulaciones po JOIN ofertas o ON o.id = po.oferta_id WHERE po.id = ?`, id).Scan(&oferta.ID, &oferta.Titulo); err != nil {
		return modelos.OfertaLaboral{}, nil, err
	}
	lista, err := a.candidatosDonde(ctx, true, "po.id = ?", id)
	if err != nil {
		return modelos.OfertaLaboral{}, nil, err
	}
	if len(lista) == 0 {
		return modelos.OfertaLaboral{}, nil, sql.ErrNoRows
	}
	return oferta, &lista[0], nil
}

// RegistrarReanudacion deja en la línea de tiempo del candidato cómo terminó la reanudación.
func (a *Almacen) RegistrarReanudacion(ctx context.Context, c *modelos.Candidato, resultado string) error {
	id, err := idBytes(c.PostulacionID)
	if err != nil {
		return err
	}
	return a.enTx(ctx, func(tx *sql.Tx) error {
		return historialTx(ctx, tx, id, "analisis.reanudado", map[string]any{"resultado": resultado})
	})
}

// EstadoSolicitudDatos es lo que el reclutador ve de la rama de contacto de un candidato: a quién se
// le puede escribir, qué pasó con cada canal y, si ya respondió, lo que declaró.
type EstadoSolicitudDatos struct {
	Existe     bool // el candidato ya pasó por la pausa y tiene solicitud
	Estado     string
	Contacto   Contacto
	Correo     string // enviada | fallida | pendiente ("" si no hay correo)
	Whatsapp   string
	Completada bool
	Cedula     string // la que declaró, en claro: es la que usan las validaciones
	Edad       int
}

// EstadoSolicitud lee el estado de la solicitud de datos de una postulación.
func (a *Almacen) EstadoSolicitud(ctx context.Context, postulacionID string) (EstadoSolicitudDatos, error) {
	var e EstadoSolicitudDatos
	id, err := idBytes(postulacionID)
	if err != nil {
		return e, err
	}
	err = a.db.QueryRowContext(ctx,
		`SELECT estado FROM solicitudes_datos WHERE postulacion_id = ? ORDER BY creada_en DESC, id DESC LIMIT 1`, id).Scan(&e.Estado)
	if errors.Is(err, sql.ErrNoRows) {
		return e, nil
	}
	if err != nil {
		return e, err
	}
	e.Existe = true
	if e.Contacto, err = a.contacto(ctx, a.db, id); err != nil {
		return e, err
	}

	filas, err := a.db.QueryContext(ctx,
		`SELECT canal, estado FROM notificaciones WHERE postulacion_id = ? AND plantilla = ?`, id, plantillaSolicitud)
	if err != nil {
		return e, err
	}
	defer filas.Close()
	for filas.Next() {
		var canal, estado string
		if err := filas.Scan(&canal, &estado); err != nil {
			return e, err
		}
		if canal == "correo" {
			e.Correo = estado
		} else {
			e.Whatsapp = estado
		}
	}
	if err := filas.Err(); err != nil {
		return e, err
	}
	if e.Contacto.Correo != "" && e.Correo == "" {
		e.Correo = "pendiente"
	}
	if e.Contacto.Telefono != "" && e.Whatsapp == "" {
		e.Whatsapp = "pendiente"
	}

	var cedula, nacimiento []byte
	err = a.db.QueryRowContext(ctx,
		`SELECT cedula_cifrada, fecha_nacimiento_cifrada FROM datos_declarados
		  WHERE postulacion_id = ? ORDER BY id DESC LIMIT 1`, id).Scan(&cedula, &nacimiento)
	if errors.Is(err, sql.ErrNoRows) {
		return e, nil
	}
	if err != nil {
		return e, err
	}
	e.Completada = true
	if e.Cedula, err = a.claves.Descifrar(cedula); err != nil {
		return e, fmt.Errorf("no se pudo descifrar la cédula declarada: %w", err)
	}
	if texto, err := a.claves.Descifrar(nacimiento); err == nil {
		if fecha, err := time.Parse("2006-01-02", texto); err == nil {
			e.Edad = edadEn(fecha, time.Now().UTC())
		}
	}
	return e, nil
}
