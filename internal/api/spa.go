package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves static files from an embed.FS and falls back to index.html
// for client-side routing. API routes (anything under /api/) are NOT handled here.
func SPAHandler(staticFS fs.FS) http.Handler {
	// Strip the "static" prefix from the embedded filesystem so file paths
	// resolve correctly (e.g., "index.html" rather than "static/index.html").
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("spa: failed to create sub-filesystem: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve API routes from the SPA handler.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the exact file. If it exists, the file server handles it.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if the file exists in the embedded FS.
		if _, err := fs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for SPA client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
