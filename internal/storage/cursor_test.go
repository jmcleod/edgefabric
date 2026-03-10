package storage

import (
	"testing"
	"time"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	id := "550e8400-e29b-41d4-a716-446655440000"

	encoded := EncodeCursor(ts, id)
	if encoded == "" {
		t.Fatal("expected non-empty cursor")
	}

	decoded, ok := DecodeCursor(encoded)
	if !ok {
		t.Fatal("expected valid cursor")
	}
	if !decoded.CreatedAt.Equal(ts) {
		t.Errorf("expected %v, got %v", ts, decoded.CreatedAt)
	}
	if decoded.ID != id {
		t.Errorf("expected %s, got %s", id, decoded.ID)
	}
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	_, ok := DecodeCursor("")
	if ok {
		t.Error("expected false for empty cursor")
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, ok := DecodeCursor("not-valid-base64!!!")
	if ok {
		t.Error("expected false for invalid base64")
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	_, ok := DecodeCursor("bm90LWpzb24") // "not-json" in base64
	if ok {
		t.Error("expected false for invalid JSON")
	}
}

func TestDecodeCursor_MissingID(t *testing.T) {
	// JSON without id field.
	encoded := EncodeCursor(time.Now(), "")
	_, ok := DecodeCursor(encoded)
	if ok {
		t.Error("expected false for missing id")
	}
}
