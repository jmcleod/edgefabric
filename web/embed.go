// Package web provides the embedded static files for the SPA.
package web

import "embed"

// StaticFiles contains the embedded static web assets.
// The SPA's index.html and any other static files live under web/static/.
//
//go:embed static
var StaticFiles embed.FS
