package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDKey is the context key for the request ID.
type requestIDKey struct{}

// RequestIDFromContext extracts the request ID from the context.
// Returns an empty string if no request ID is present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// RequestID returns middleware that assigns a unique ID to each request.
//
// The ID is:
//  1. Read from the X-Request-ID header if present (to support upstream correlation).
//  2. Otherwise generated as a new UUID v4.
//
// The ID is set on the response header and stored in the request context
// for downstream handlers and middleware (logging, audit, etc.).
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.New().String()
			}

			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
