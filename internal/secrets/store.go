// Package secrets provides encrypted secret storage using AES-256-GCM.
package secrets

import (
	"encoding/base64"
	"fmt"

	"github.com/jmcleod/edgefabric/internal/crypto"
)

// Store encrypts and decrypts secrets using a master key.
// It supports versioned key rotation: new encryptions use the primary key,
// while decryption tries all keys in the ring for backward compatibility.
type Store struct {
	key     []byte // primary 32-byte AES-256 key (used for encryption)
	version byte   // version ID of the primary key (0 = unversioned/legacy)
	ring    []VersionedKey
}

// VersionedKey is a key with an associated version identifier.
type VersionedKey struct {
	Version byte
	Key     []byte // 32-byte AES-256 key
}

// NewStore creates a secret store from a base64-encoded encryption key.
// This uses unversioned (legacy) encryption for backward compatibility.
func NewStore(encodedKey string) (*Store, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Store{key: key, version: 0}, nil
}

// NewVersionedStore creates a secret store with a versioned key ring.
// The primary key (first in list) is used for encryption. All keys in the ring
// are tried during decryption, enabling key rotation without downtime.
func NewVersionedStore(keys []VersionedKey) (*Store, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one key is required")
	}
	for i, k := range keys {
		if len(k.Key) != 32 {
			return nil, fmt.Errorf("key %d: must be 32 bytes, got %d", i, len(k.Key))
		}
		if k.Version == 0 {
			return nil, fmt.Errorf("key %d: version must be >= 1", i)
		}
	}
	ring := make([]VersionedKey, len(keys))
	copy(ring, keys)
	return &Store{
		key:     keys[0].Key,
		version: keys[0].Version,
		ring:    ring,
	}, nil
}

// Encrypt encrypts a plaintext string and returns the encrypted value.
// If versioned keys are configured, the ciphertext is prefixed with a version byte.
func (s *Store) Encrypt(plaintext string) (string, error) {
	if s.version > 0 {
		return crypto.VersionedEncryptAESGCM(s.version, s.key, []byte(plaintext))
	}
	return crypto.EncryptAESGCM(s.key, []byte(plaintext))
}

// Decrypt decrypts an encrypted string and returns the plaintext.
// It tries versioned decryption first (if the version byte matches a known key),
// then falls back to unversioned (legacy) decryption with the primary key.
// This two-phase approach handles both legacy and versioned ciphertext reliably,
// even when a random nonce byte coincidentally matches a key version.
func (s *Store) Decrypt(encrypted string) (string, error) {
	// Try versioned decryption if we have a key ring.
	if ver, ok := crypto.ExtractKeyVersion(encrypted); ok && len(s.ring) > 0 {
		for _, k := range s.ring {
			if k.Version == ver {
				b, err := crypto.DecryptVersionedAESGCM(k.Key, encrypted)
				if err == nil {
					return string(b), nil
				}
				// Versioned decryption failed — could be unversioned ciphertext
				// whose nonce byte coincidentally matches a version. Fall through.
				break
			}
		}
	}

	// Unversioned (legacy) decryption — try primary key.
	b, err := crypto.DecryptAESGCM(s.key, encrypted)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReEncrypt decrypts with any available key and re-encrypts with the current
// primary key. Returns the original ciphertext if it's already using the
// current key version, or the new ciphertext if re-encrypted.
func (s *Store) ReEncrypt(encrypted string) (string, bool, error) {
	// Check if already on current version.
	if s.version > 0 {
		if ver, ok := crypto.ExtractKeyVersion(encrypted); ok && ver == s.version {
			return encrypted, false, nil
		}
	}

	plaintext, err := s.Decrypt(encrypted)
	if err != nil {
		return "", false, fmt.Errorf("decrypt for re-encryption: %w", err)
	}
	newCiphertext, err := s.Encrypt(plaintext)
	if err != nil {
		return "", false, fmt.Errorf("re-encrypt: %w", err)
	}
	return newCiphertext, true, nil
}

// PrimaryKeyVersion returns the version of the primary encryption key.
// Returns 0 for unversioned (legacy) stores.
func (s *Store) PrimaryKeyVersion() byte {
	return s.version
}
