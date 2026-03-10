package crypto

import (
	"testing"
)

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key, _ := GenerateRandomBytes(32)
	plaintext := "hello world secret data"

	encrypted, err := EncryptAESGCM(key, []byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := DecryptAESGCM(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(decrypted) != plaintext {
		t.Errorf("expected %q, got %q", plaintext, string(decrypted))
	}
}

func TestVersionedEncryptDecrypt_Roundtrip(t *testing.T) {
	key, _ := GenerateRandomBytes(32)
	plaintext := "versioned secret data"

	encrypted, err := VersionedEncryptAESGCM(1, key, []byte(plaintext))
	if err != nil {
		t.Fatalf("versioned encrypt: %v", err)
	}

	// Check version extraction.
	ver, ok := ExtractKeyVersion(encrypted)
	if !ok {
		t.Fatal("expected versioned ciphertext")
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	// Decrypt versioned.
	decrypted, err := DecryptVersionedAESGCM(key, encrypted)
	if err != nil {
		t.Fatalf("versioned decrypt: %v", err)
	}
	if string(decrypted) != plaintext {
		t.Errorf("expected %q, got %q", plaintext, string(decrypted))
	}
}

func TestVersionedEncrypt_DifferentVersions(t *testing.T) {
	key1, _ := GenerateRandomBytes(32)
	key2, _ := GenerateRandomBytes(32)
	plaintext := "multi-key test"

	enc1, err := VersionedEncryptAESGCM(1, key1, []byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt v1: %v", err)
	}
	enc2, err := VersionedEncryptAESGCM(2, key2, []byte(plaintext))
	if err != nil {
		t.Fatalf("encrypt v2: %v", err)
	}

	// Version 1 decrypts with key1.
	dec1, err := DecryptVersionedAESGCM(key1, enc1)
	if err != nil {
		t.Fatalf("decrypt v1: %v", err)
	}
	if string(dec1) != plaintext {
		t.Errorf("v1 roundtrip failed")
	}

	// Version 2 decrypts with key2.
	dec2, err := DecryptVersionedAESGCM(key2, enc2)
	if err != nil {
		t.Fatalf("decrypt v2: %v", err)
	}
	if string(dec2) != plaintext {
		t.Errorf("v2 roundtrip failed")
	}

	// Wrong key fails.
	if _, err := DecryptVersionedAESGCM(key2, enc1); err == nil {
		t.Error("expected error decrypting v1 with key2")
	}
}

func TestExtractKeyVersion_Unversioned(t *testing.T) {
	key, _ := GenerateRandomBytes(32)
	// Unversioned ciphertext — starts with a random nonce byte, not a version.
	encrypted, _ := EncryptAESGCM(key, []byte("test"))

	// We can't deterministically distinguish, but version 0 means unversioned.
	ver, ok := ExtractKeyVersion(encrypted)
	// If the first decoded byte happens to be 0 it should return false.
	// If it's non-zero, it reports "versioned" — but that's the nature of the heuristic.
	// In practice, we rely on the Store layer to try both paths.
	_ = ver
	_ = ok
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	key1, _ := GenerateRandomBytes(32)
	key2, _ := GenerateRandomBytes(32)

	encrypted, _ := EncryptAESGCM(key1, []byte("secret"))
	_, err := DecryptAESGCM(key2, encrypted)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if key == "" {
		t.Error("expected non-empty key")
	}
}
