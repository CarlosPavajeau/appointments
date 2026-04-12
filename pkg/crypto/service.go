package crypto

import (
	"errors"
)

var errNilService = errors.New("crypto service is nil")

// Service holds the application AES key and exposes encrypt/decrypt without
// passing the key at every call site.
type Service struct {
	key []byte
}

// NewService validates key length (32 bytes for AES-256, matching production
// configuration) and returns a [Service] that wraps a defensive copy of key.
func NewService(key []byte) (*Service, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes")
	}
	k := make([]byte, len(key))
	copy(k, key)
	return &Service{key: k}, nil
}

// Encrypt encrypts plaintext using the service key. See [Encrypt].
func (s *Service) Encrypt(plaintext string) (string, error) {
	if s == nil {
		return "", errNilService
	}

	return Encrypt(plaintext, s.key)
}

// Decrypt decrypts encoded using the service key. See [Decrypt].
func (s *Service) Decrypt(encoded string) (string, error) {
	if s == nil {
		return "", errNilService
	}

	return Decrypt(encoded, s.key)
}
