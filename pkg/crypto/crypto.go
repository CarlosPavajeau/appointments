// Package crypto provides symmetric authenticated encryption helpers built on
// AES-GCM (Galois/Counter Mode).
//
// Every encrypted value is self-contained: the random nonce is prepended to
// the ciphertext before base64 encoding, so [Decrypt] needs only the encoded
// string and the same key — no separate nonce storage is required.
//
// Key requirements: AES-GCM accepts 16, 24, or 32-byte keys (AES-128,
// AES-192, or AES-256). Passing any other length returns an error from
// [aes.NewCipher].
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt encrypts plaintext using AES-GCM with the provided key and returns
// a base64-encoded string containing the nonce prepended to the ciphertext.
//
// A new random nonce is generated for every call, so encrypting the same
// plaintext twice produces different outputs. The result can be safely stored
// or transmitted as an opaque string and later recovered with [Decrypt].
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses [Encrypt]: it base64-decodes encoded, splits off the
// leading nonce, and authenticates + decrypts the remaining ciphertext with
// AES-GCM using key.
//
// Returns an error if the input is not valid base64, is too short to contain a
// nonce, or if authentication fails (tampered or corrupted data).
func Decrypt(encoded string, key []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
