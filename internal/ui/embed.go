package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var dist embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return spa(fileServer, sub)
}

func spa(fileServer http.Handler, assets fs.FS) http.Handler {
	indexHTML, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		indexHTML = []byte("<!DOCTYPE html><html><body>UI not built — run <code>make build</code></body></html>")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if path != "" && !strings.HasPrefix(path, "api/") {
			f, err := assets.Open(path)
			if err == nil {
				stat, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && stat.IsDir() {
					http.NotFound(w, r)
					return
				}
				if statErr == nil && !stat.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(indexHTML)
	})
}
