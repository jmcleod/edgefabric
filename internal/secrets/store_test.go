package secrets

import (
	"encoding/base64"
	"testing"

	"github.com/jmcleod/edgefabric/internal/crypto"
)

func makeKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func encodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

func TestStore_EncryptDecrypt_Legacy(t *testing.T) {
	key := makeKey(t)
	s, err := NewStore(encodeKey(key))
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "my secret value"
	encrypted, err := s.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := s.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestVersionedStore_EncryptDecrypt(t *testing.T) {
	key := makeKey(t)
	s, err := NewVersionedStore([]VersionedKey{
		{Version: 1, Key: key},
	})
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "versioned secret"
	encrypted, err := s.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := s.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}

	if s.PrimaryKeyVersion() != 1 {
		t.Errorf("expected primary key version 1, got %d", s.PrimaryKeyVersion())
	}
}

func TestVersionedStore_DecryptLegacy(t *testing.T) {
	key := makeKey(t)

	// Encrypt with legacy store (unversioned).
	legacyStore, err := NewStore(encodeKey(key))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := legacyStore.Encrypt("legacy data")
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt with versioned store (same key as primary, unversioned).
	versionedStore, err := NewVersionedStore([]VersionedKey{
		{Version: 1, Key: key},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Versioned store should fall back to primary key for unversioned data.
	decrypted, err := versionedStore.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt legacy: %v", err)
	}
	if decrypted != "legacy data" {
		t.Errorf("expected %q, got %q", "legacy data", decrypted)
	}
}

func TestVersionedStore_KeyRotation(t *testing.T) {
	key1 := makeKey(t)
	key2 := makeKey(t)

	// Create store with key1 as primary.
	storeV1, err := NewVersionedStore([]VersionedKey{
		{Version: 1, Key: key1},
	})
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := storeV1.Encrypt("data encrypted with v1")
	if err != nil {
		t.Fatal(err)
	}

	// Rotate: create new store with key2 as primary, key1 as old.
	storeV2, err := NewVersionedStore([]VersionedKey{
		{Version: 2, Key: key2},
		{Version: 1, Key: key1},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should still decrypt v1 ciphertext.
	decrypted, err := storeV2.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt v1 with v2 store: %v", err)
	}
	if decrypted != "data encrypted with v1" {
		t.Errorf("expected %q, got %q", "data encrypted with v1", decrypted)
	}

	// New encryption should use v2.
	enc2, err := storeV2.Encrypt("new data")
	if err != nil {
		t.Fatal(err)
	}
	dec2, err := storeV2.Decrypt(enc2)
	if err != nil {
		t.Fatal(err)
	}
	if dec2 != "new data" {
		t.Errorf("expected %q, got %q", "new data", dec2)
	}
}

func TestVersionedStore_ReEncrypt(t *testing.T) {
	key1 := makeKey(t)
	key2 := makeKey(t)

	// Encrypt with v1.
	storeV1, _ := NewVersionedStore([]VersionedKey{{Version: 1, Key: key1}})
	encrypted, _ := storeV1.Encrypt("migrate me")

	// Create store with v2 primary + v1 old.
	storeV2, _ := NewVersionedStore([]VersionedKey{
		{Version: 2, Key: key2},
		{Version: 1, Key: key1},
	})

	// Re-encrypt should produce new ciphertext.
	reEncrypted, changed, err := storeV2.ReEncrypt(encrypted)
	if err != nil {
		t.Fatalf("re-encrypt: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if reEncrypted == encrypted {
		t.Error("expected different ciphertext after re-encryption")
	}

	// Verify re-encrypted value decrypts correctly.
	dec, err := storeV2.Decrypt(reEncrypted)
	if err != nil {
		t.Fatalf("decrypt re-encrypted: %v", err)
	}
	if dec != "migrate me" {
		t.Errorf("expected %q, got %q", "migrate me", dec)
	}

	// Re-encrypting again should be a no-op.
	_, changed2, err := storeV2.ReEncrypt(reEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("expected changed=false for already-current ciphertext")
	}
}

func TestVersionedStore_ValidationErrors(t *testing.T) {
	_, err := NewVersionedStore(nil)
	if err == nil {
		t.Error("expected error for empty key list")
	}

	_, err = NewVersionedStore([]VersionedKey{{Version: 0, Key: makeKey(t)}})
	if err == nil {
		t.Error("expected error for version 0")
	}

	_, err = NewVersionedStore([]VersionedKey{{Version: 1, Key: []byte("short")}})
	if err == nil {
		t.Error("expected error for short key")
	}
}
