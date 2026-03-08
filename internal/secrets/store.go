// Package secrets provides encrypted secret storage using AES-256-GCM.
package secrets

import (
	"encoding/base64"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/crypto"
)

// Store encrypts and decrypts secrets using a master key.
type Store struct {
	key []byte // 32-byte AES-256 key
}

// NewStore creates a secret store from a base64-encoded encryption key.
func NewStore(encodedKey string) (*Store, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Store{key: key}, nil
}

// Encrypt encrypts a plaintext string and returns the encrypted value.
func (s *Store) Encrypt(plaintext string) (string, error) {
	return crypto.EncryptAESGCM(s.key, []byte(plaintext))
}

// Decrypt decrypts an encrypted string and returns the plaintext.
func (s *Store) Decrypt(encrypted string) (string, error) {
	b, err := crypto.DecryptAESGCM(s.key, encrypted)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
