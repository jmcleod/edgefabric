package middleware

import "net/http"

// SecurityHeaders returns middleware that sets common HTTP security headers
// on every response. These provide defense-in-depth against common web
// attacks (clickjacking, MIME sniffing, XSS reflection).
// FUTURE: Add configurable CORS and HSTS when TLS is enforced.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Modern recommendation: disable X-XSS-Protection to avoid
			// edge-case XSS vulnerabilities in older browsers.
			w.Header().Set("X-XSS-Protection", "0")
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBodySize returns middleware that limits request body size to the given
// number of bytes. This provides a global safety net in addition to the
// per-request limit enforced by apiutil.Decode.
func MaxBodySize(bytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, bytes)
			next.ServeHTTP(w, r)
		})
	}
}
