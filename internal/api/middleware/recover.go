package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/jmcleod/edgefabric/internal/api/apiutil"
)

// Recover returns middleware that catches panics and returns a 500 JSON error.
// The panic value and stack trace are logged for debugging.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						slog.Any("error", err),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					apiutil.WriteError(w, http.StatusInternalServerError,
						"internal_error", "an unexpected error occurred")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
