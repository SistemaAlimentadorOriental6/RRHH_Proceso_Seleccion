package almacen

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// claves agrupa las dos claves de privacidad: una pseudonimiza para poder buscar (HMAC-SHA256) y
// la otra cifra para poder recuperar el valor original (AES-256-GCM). Son distintas a propósito.
type claves struct {
	hmac []byte
	gcm  cipher.AEAD
}

func decodificarClave(nombre, hexa string) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimSpace(hexa))
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("%s debe tener 64 caracteres hexadecimales (32 bytes)", nombre)
	}
	return b, nil
}

func nuevasClaves(pseudonimos, cifrado string) (*claves, error) {
	kp, err := decodificarClave("RRHH_CLAVE_PSEUDONIMOS", pseudonimos)
	if err != nil {
		return nil, err
	}
	kc, err := decodificarClave("RRHH_CLAVE_CIFRADO", cifrado)
	if err != nil {
		return nil, err
	}
	if hmac.Equal(kp, kc) {
		return nil, errors.New("RRHH_CLAVE_PSEUDONIMOS y RRHH_CLAVE_CIFRADO deben ser distintas")
	}
	bloque, err := aes.NewCipher(kc)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(bloque)
	if err != nil {
		return nil, err
	}
	return &claves{hmac: kp, gcm: gcm}, nil
}

// Pseudonimo devuelve el HMAC-SHA256 en hexadecimal (64 caracteres). Sin la clave no se puede
// calcular, así que una cédula (pocos dígitos) no se adivina probando todos los valores.
func (k *claves) Pseudonimo(valor string) string {
	m := hmac.New(sha256.New, k.hmac)
	m.Write([]byte(valor))
	return hex.EncodeToString(m.Sum(nil))
}

// Cifrar devuelve nonce || texto cifrado || etiqueta de autenticación.
func (k *claves) Cifrar(texto string) ([]byte, error) {
	nonce := make([]byte, k.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return k.gcm.Seal(nonce, nonce, []byte(texto), nil), nil
}

// Descifrar revierte Cifrar. Falla si la clave es otra o el dato fue alterado.
func (k *claves) Descifrar(datos []byte) (string, error) {
	n := k.gcm.NonceSize()
	if len(datos) < n+k.gcm.Overhead() {
		return "", errors.New("dato cifrado demasiado corto")
	}
	claro, err := k.gcm.Open(nil, datos[:n], datos[n:], nil)
	if err != nil {
		return "", err
	}
	return string(claro), nil
}
