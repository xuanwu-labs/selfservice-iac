// Package web embeds the built frontend assets (Vite build output) into the Go
// binary. The frontend is built via `cd web && npm run build` which outputs to
// `server/internal/web/dist/`. This package serves the SPA with fallback to
// index.html for client-side routing.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var Assets embed.FS

// Handler returns an http.Handler that serves the embedded frontend.
// SPA fallback: any path not matching a static file returns index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(Assets, "dist")
	if err != nil {
		panic("web: failed to create sub FS: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Check if the file exists in the embedded FS.
		if path != "" {
			if _, err := fs.Stat(sub, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
