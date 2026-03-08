package domain

import "github.com/google/uuid"

// ID is the universal identifier type used across all domain entities.
type ID = uuid.UUID

// NewID generates a new random UUID.
func NewID() ID {
	return uuid.New()
}

// ParseID parses a UUID string.
func ParseID(s string) (ID, error) {
	return uuid.Parse(s)
}

// ZeroID is the zero-value UUID.
var ZeroID = uuid.UUID{}
