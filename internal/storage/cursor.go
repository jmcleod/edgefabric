package storage

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// CursorData holds the position marker for cursor-based pagination.
// The cursor encodes the created_at timestamp and ID of the last item,
// enabling efficient keyset pagination with ORDER BY created_at, id.
type CursorData struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// EncodeCursor encodes a cursor into an opaque base64 string.
func EncodeCursor(createdAt time.Time, id string) string {
	data := CursorData{CreatedAt: createdAt, ID: id}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor decodes an opaque cursor string back into its components.
// Returns zero values and false if the cursor is invalid.
func DecodeCursor(cursor string) (CursorData, bool) {
	if cursor == "" {
		return CursorData{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return CursorData{}, false
	}
	var data CursorData
	if err := json.Unmarshal(b, &data); err != nil {
		return CursorData{}, false
	}
	if data.ID == "" {
		return CursorData{}, false
	}
	return data, true
}
