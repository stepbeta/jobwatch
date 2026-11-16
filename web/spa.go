// Package web serves the single-page application. The built assets live
// in web/dist and are embedded when the binary is built with `-tags ui`;
// without that tag a placeholder page explains how to build the UI.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves files from fsys and falls back to index.html for
// unknown paths, so client-side routes work on a full page reload.
func spaHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(fsys, name); err != nil {
			w.Header().Set("Cache-Control", "no-cache")
			fallback := r.Clone(r.Context())
			fallback.URL.Path = "/"
			files.ServeHTTP(w, fallback)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
