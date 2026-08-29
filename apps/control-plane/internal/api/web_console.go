package api

import (
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
)

// WithWebConsoleDirectory enables same-origin static SPA delivery. The
// directory is validated by config before the server is composed.
func WithWebConsoleDirectory(directory string) Option {
	return func(server *Server) { server.webConsoleDirectory = directory }
}

func (s *Server) handleWebConsole(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/v1" || strings.HasPrefix(request.URL.Path, "/v1/") {
		writeStatus(writer, http.StatusNotFound, "not_found")
		return
	}
	requested := strings.TrimPrefix(request.URL.Path, "/")
	if requested == "" {
		requested = "index.html"
	}
	if !fs.ValidPath(requested) {
		writeStatus(writer, http.StatusNotFound, "not_found")
		return
	}
	if s.serveWebConsoleFile(writer, request, requested) {
		return
	}
	if strings.HasPrefix(requested, "assets/") || path.Ext(requested) != "" {
		writeStatus(writer, http.StatusNotFound, "not_found")
		return
	}
	if !s.serveWebConsoleFile(writer, request, "index.html") {
		writeStatus(writer, http.StatusServiceUnavailable, "web_console_unavailable")
	}
}

func (s *Server) serveWebConsoleFile(writer http.ResponseWriter, request *http.Request, name string) bool {
	root, err := os.OpenRoot(s.webConsoleDirectory)
	if err != nil {
		return false
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if strings.HasPrefix(name, "assets/") {
		writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		writer.Header().Set("Cache-Control", "no-store")
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(writer, request, name, info.ModTime(), file)
	return true
}
