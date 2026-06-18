package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFiles embed.FS

// Handler returns an http.Handler that serves the embedded web assets.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
