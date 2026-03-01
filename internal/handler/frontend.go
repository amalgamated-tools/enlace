package handler

import (
	"io/fs"
	"net/http"
	"strings"
)

// FrontendHandler serves the embedded frontend files.
// It handles SPA routing by serving index.html for non-file requests.
type FrontendHandler struct {
	fileServer http.Handler
	indexHTML  []byte
}

// NewFrontendHandler creates a new frontend handler from the provided filesystem.
// The fs should be the root of the frontend dist directory.
func NewFrontendHandler(distFS fs.FS) (*FrontendHandler, error) {
	// Read index.html for SPA fallback
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return nil, err
	}

	return &FrontendHandler{
		fileServer: http.FileServer(http.FS(distFS)),
		indexHTML:  indexHTML,
	}, nil
}

// ServeHTTP serves frontend files or falls back to index.html for SPA routing.
func (h *FrontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Clean the path
	if path == "" || path == "/" {
		h.serveIndex(w)
		return
	}

	// Check if the path has a file extension (static asset)
	if strings.Contains(path, ".") {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// For all other paths (SPA routes), serve index.html
	h.serveIndex(w)
}

// serveIndex serves the index.html file.
func (h *FrontendHandler) serveIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.indexHTML)
}
