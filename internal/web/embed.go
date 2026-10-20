// Package web serves the panel's single page application.
//
// The build output of the frontend is embedded into the binary so the panel
// ships as one file. When the frontend has not been built the package
// serves a short placeholder page instead of failing to start.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var assets embed.FS

// FS returns the embedded frontend assets.
func FS() fs.FS {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return assets
	}
	return sub
}

// Handler serves the frontend. Unknown paths fall back to index.html so the
// client side router can handle them.
func Handler() http.Handler {
	files := FS()
	fileServer := http.FileServer(http.FS(files))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(files, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
