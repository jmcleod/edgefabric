// Package crypto provides key generation and encryption utilities.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// GenerateRandomBytes returns n cryptographically random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateRandomString returns a base64url-encoded random string of n bytes.
func GenerateRandomString(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// EncryptAESGCM encrypts plaintext with AES-256-GCM using the given key.
// Key must be 32 bytes (AES-256). Returns base64-encoded ciphertext.
// Uses unversioned format (no version prefix) for backward compatibility.
func EncryptAESGCM(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// VersionedEncryptAESGCM encrypts plaintext with AES-256-GCM using the given
// key, prepending a 1-byte version ID to the ciphertext before base64 encoding.
// This allows decryption to identify which key was used.
func VersionedEncryptAESGCM(keyVersion byte, key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Prepend version byte: [version][nonce][ciphertext+tag]
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	versioned := make([]byte, 1+len(sealed))
	versioned[0] = keyVersion
	copy(versioned[1:], sealed)

	return base64.StdEncoding.EncodeToString(versioned), nil
}

// DecryptAESGCM decrypts a base64-encoded AES-256-GCM ciphertext.
func DecryptAESGCM(key []byte, encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// ExtractKeyVersion extracts the key version byte from a base64-encoded
// versioned ciphertext. Returns the version and true if the ciphertext is
// versioned, or 0 and false if it's unversioned (legacy format).
//
// Heuristic: AES-GCM nonces start with random bytes. A version byte of 0
// indicates legacy (unversioned) ciphertext, since versioned ciphertext starts
// at version 1.
func ExtractKeyVersion(encoded string) (byte, bool) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(ciphertext) == 0 {
		return 0, false
	}
	// Version IDs start at 1. A leading 0 byte means unversioned.
	if ciphertext[0] == 0 {
		return 0, false
	}
	return ciphertext[0], true
}

// DecryptVersionedAESGCM decrypts a versioned ciphertext by stripping the
// 1-byte version prefix before standard AES-GCM decryption.
func DecryptVersionedAESGCM(key []byte, encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(ciphertext) < 2 {
		return nil, fmt.Errorf("versioned ciphertext too short")
	}
	// Strip version byte.
	raw := ciphertext[1:]
	reEncoded := base64.StdEncoding.EncodeToString(raw)
	return DecryptAESGCM(key, reEncoded)
}

// GenerateEncryptionKey creates a new random 32-byte AES-256 key, returned base64-encoded.
func GenerateEncryptionKey() (string, error) {
	key, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
