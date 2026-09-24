package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

func (s *Server) registerStaticRoutes() {
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(subFS))

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
