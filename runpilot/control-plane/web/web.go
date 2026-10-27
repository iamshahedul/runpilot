package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed index.html css js
var embeddedFS embed.FS

// AssetHandler returns an http.Handler that serves web dashboard assets.
// It checks candidate disk paths first, and seamlessly falls back to embedded assets.
func AssetHandler(preferredPath string) http.Handler {
	candidates := []string{
		preferredPath,
		"/opt/runpilot/web",
		"/opt/runpilot-repo/runpilot/control-plane/web",
		"control-plane/web",
		"web",
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(p, "index.html")); err == nil && !fi.IsDir() {
			return http.FileServer(http.Dir(p))
		}
	}

	sub, err := fs.Sub(embeddedFS, ".")
	if err == nil {
		return http.FileServer(http.FS(sub))
	}
	return http.FileServer(http.FS(embeddedFS))
}
