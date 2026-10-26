package almacen

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const (
	bytesToken = 32 // 256 bits de azar
	largoToken = 43 // 32 bytes en base64url sin relleno
)

// nuevoToken genera el token del enlace que se envía al candidato y su huella. En la base solo se
// guarda la huella: quien lea la base no puede reconstruir un enlace válido.
func nuevoToken() (token, huella string, err error) {
	b := make([]byte, bytesToken)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

// hashToken devuelve el SHA-256 del token en hexadecimal (64 caracteres).
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// formaTokenValida descarta de entrada lo que no puede ser un token nuestro, sin tocar la base.
func formaTokenValida(t string) bool {
	if len(t) != largoToken {
		return false
	}
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
