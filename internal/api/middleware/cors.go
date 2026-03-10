package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig controls Cross-Origin Resource Sharing headers.
type CORSConfig struct {
	AllowedOrigins   []string // e.g., ["https://console.example.com"]
	AllowedMethods   []string // e.g., ["GET","POST","PUT","DELETE"]
	AllowedHeaders   []string // e.g., ["Authorization","Content-Type"]
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds.
}

// DefaultCORSConfig returns a CORS config suitable for the SPA console.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours.
	}
}

// CORS returns middleware that handles CORS preflight and response headers.
// If no origins are configured, CORS headers are not sent (same-origin only).
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	if len(cfg.AllowedOrigins) == 0 {
		// No CORS configured — passthrough.
		return func(next http.Handler) http.Handler { return next }
	}

	originSet := make(map[string]bool, len(cfg.AllowedOrigins))
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originSet[o] = true
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := ""
	if cfg.MaxAge > 0 {
		maxAge = itoa(cfg.MaxAge)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if origin is allowed.
			if !allowAll && !originSet[origin] {
				next.ServeHTTP(w, r)
				return
			}

			// Set CORS headers.
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			if cfg.AllowCredentials && !allowAll {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight.
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				if maxAge != "" {
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// itoa converts a non-negative int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf) - 1
	for n > 0 {
		buf[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	return string(buf[i+1:])
}
