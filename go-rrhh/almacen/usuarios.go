package almacen

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	iteracionesClave    = 600_000 // recomendación de OWASP para PBKDF2-HMAC-SHA256
	maxIntentosFallidos = 5
	bloqueoTrasFallos   = 15 * time.Minute
	// DuracionSesion es lo que dura una sesión abierta antes de pedir la contraseña de nuevo.
	DuracionSesion = 12 * time.Hour
)

// ErrCredenciales cubre tanto "no existe" como "contraseña incorrecta": no se distingue a propósito.
var ErrCredenciales = errors.New("correo o contraseña incorrectos")

// ErrCuentaBloqueada indica demasiados intentos fallidos seguidos.
var ErrCuentaBloqueada = errors.New("cuenta bloqueada temporalmente por intentos fallidos")

// Usuario es quien tiene acceso a la plataforma.
type Usuario struct {
	ID     string `json:"id"`
	Correo string `json:"correo"`
	Nombre string `json:"nombre"`
	Cargo  string `json:"cargo"`
}

// HashClave devuelve "pbkdf2-sha256$iteraciones$sal$hash" con una sal aleatoria nueva.
func HashClave(clave string) (string, error) {
	sal := make([]byte, 16)
	if _, err := rand.Read(sal); err != nil {
		return "", err
	}
	return hashConSal(clave, sal, iteracionesClave)
}

func hashConSal(clave string, sal []byte, iter int) (string, error) {
	h, err := pbkdf2.Key(sha256.New, clave, sal, iter, 32)
	if err != nil {
		return "", err
	}
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iter, b64.EncodeToString(sal), b64.EncodeToString(h)), nil
}

// ClaveCoincide compara en tiempo constante la contraseña con un hash guardado.
func ClaveCoincide(clave, guardado string) bool {
	partes := strings.Split(guardado, "$")
	if len(partes) != 4 || partes[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(partes[1])
	if err != nil || iter < 1 || iter > 10_000_000 {
		return false
	}
	sal, err := base64.RawStdEncoding.DecodeString(partes[2])
	if err != nil {
		return false
	}
	esperado, err := hashConSal(clave, sal, iter)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(esperado), []byte(guardado)) == 1
}

// hashFicticio se compara cuando el correo no existe, para que responder tarde lo mismo.
var hashFicticio, _ = HashClave("sin-usuario")

func huellaToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}

// CrearUsuario da de alta al usuario o, si el correo ya existe, actualiza su nombre, cargo y
// contraseña (y lo reactiva y desbloquea).
func (a *Almacen) CrearUsuario(ctx context.Context, correo, nombre, cargo, clave string) error {
	correo = strings.ToLower(strings.TrimSpace(correo))
	if correo == "" || strings.TrimSpace(nombre) == "" || len(clave) < 8 {
		return errors.New("correo, nombre y una contraseña de al menos 8 caracteres son obligatorios")
	}
	hash, err := HashClave(clave)
	if err != nil {
		return err
	}
	id, err := nuevoID()
	if err != nil {
		return err
	}
	ahora := time.Now().UTC()
	_, err = a.db.ExecContext(ctx,
		`INSERT INTO usuarios (id, correo, nombre, cargo, clave_hash, creado_en, actualizado_en)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE nombre = VALUES(nombre), cargo = VALUES(cargo), clave_hash = VALUES(clave_hash),
		   activo = 1, intentos_fallidos = 0, bloqueado_hasta = NULL, actualizado_en = VALUES(actualizado_en)`,
		id, correo, strings.TrimSpace(nombre), strings.TrimSpace(cargo), hash, ahora, ahora)
	return err
}

// IniciarSesion valida las credenciales y abre una sesión. Devuelve el token en claro (para la
// cookie); en la base solo queda su huella.
func (a *Almacen) IniciarSesion(ctx context.Context, correo, clave string) (Usuario, string, error) {
	correo = strings.ToLower(strings.TrimSpace(correo))
	var (
		id        []byte
		u         Usuario
		hash      string
		activo    bool
		fallidos  int
		bloqueado sql.NullTime
	)
	err := a.db.QueryRowContext(ctx,
		`SELECT id, correo, nombre, cargo, clave_hash, activo, intentos_fallidos, bloqueado_hasta FROM usuarios WHERE correo = ?`, correo).
		Scan(&id, &u.Correo, &u.Nombre, &u.Cargo, &hash, &activo, &fallidos, &bloqueado)
	if errors.Is(err, sql.ErrNoRows) {
		ClaveCoincide(clave, hashFicticio)
		return Usuario{}, "", ErrCredenciales
	}
	if err != nil {
		return Usuario{}, "", err
	}
	ahora := time.Now().UTC()
	if bloqueado.Valid && bloqueado.Time.After(ahora) {
		return Usuario{}, "", ErrCuentaBloqueada
	}
	if !ClaveCoincide(clave, hash) || !activo {
		if fallidos+1 >= maxIntentosFallidos {
			_, err = a.db.ExecContext(ctx, `UPDATE usuarios SET intentos_fallidos = 0, bloqueado_hasta = ? WHERE id = ?`, ahora.Add(bloqueoTrasFallos), id)
		} else {
			_, err = a.db.ExecContext(ctx, `UPDATE usuarios SET intentos_fallidos = intentos_fallidos + 1 WHERE id = ?`, id)
		}
		if err != nil {
			return Usuario{}, "", err
		}
		return Usuario{}, "", ErrCredenciales
	}

	crudo := make([]byte, 32)
	if _, err := rand.Read(crudo); err != nil {
		return Usuario{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(crudo)
	if _, err := a.db.ExecContext(ctx, `DELETE FROM sesiones WHERE expira_en < ?`, ahora); err != nil {
		return Usuario{}, "", err
	}
	if _, err := a.db.ExecContext(ctx, `INSERT INTO sesiones (token_hash, usuario_id, creada_en, expira_en) VALUES (?, ?, ?, ?)`,
		huellaToken(token), id, ahora, ahora.Add(DuracionSesion)); err != nil {
		return Usuario{}, "", err
	}
	if _, err := a.db.ExecContext(ctx, `UPDATE usuarios SET intentos_fallidos = 0, bloqueado_hasta = NULL, ultimo_acceso = ? WHERE id = ?`, ahora, id); err != nil {
		return Usuario{}, "", err
	}
	u.ID = idTexto(id)
	return u, token, nil
}

// UsuarioDeSesion devuelve el usuario dueño de un token de sesión vigente.
func (a *Almacen) UsuarioDeSesion(ctx context.Context, token string) (Usuario, bool, error) {
	if token == "" {
		return Usuario{}, false, nil
	}
	var (
		id []byte
		u  Usuario
	)
	err := a.db.QueryRowContext(ctx,
		`SELECT u.id, u.correo, u.nombre, u.cargo FROM sesiones s JOIN usuarios u ON u.id = s.usuario_id
		  WHERE s.token_hash = ? AND s.expira_en > ? AND u.activo = 1`, huellaToken(token), time.Now().UTC()).
		Scan(&id, &u.Correo, &u.Nombre, &u.Cargo)
	if errors.Is(err, sql.ErrNoRows) {
		return Usuario{}, false, nil
	}
	if err != nil {
		return Usuario{}, false, err
	}
	u.ID = idTexto(id)
	return u, true, nil
}

// CerrarSesion invalida el token.
func (a *Almacen) CerrarSesion(ctx context.Context, token string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM sesiones WHERE token_hash = ?`, huellaToken(token))
	return err
}
