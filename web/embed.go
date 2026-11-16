//go:build ui

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// dist is produced by `npm run build` in this directory.
//
//go:embed all:dist
var dist embed.FS

// Handler serves the embedded SPA.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("web: embedded dist is invalid: " + err.Error())
	}
	return spaHandler(sub)
}
