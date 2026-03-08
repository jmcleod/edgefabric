package middleware

import "net/http"

// Chain applies middleware in order: the first middleware is the outermost wrapper.
// Example: Chain(handler, logging, auth, rbac) results in:
//
//	logging → auth → rbac → handler
func Chain(handler http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse so the first middleware in the list wraps outermost.
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
