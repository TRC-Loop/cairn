// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var staticFS embed.FS

// StaticFS returns the embedded static file tree rooted at "static/" so it
// can be mounted under /static/ via http.FileServer.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("statuspage: static embed root missing: " + err.Error())
	}
	return sub
}

// StaticHandler serves embedded assets with sensible cache headers:
// fonts are immutable, CSS/JS are cached for an hour while we iterate.
// Mount under /static/ with http.StripPrefix.
func StaticHandler() http.Handler {
	fileServer := http.FileServer(http.FS(StaticFS()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		ext := strings.ToLower(path.Ext(p))
		switch ext {
		case ".woff2", ".woff", ".ttf":
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("Content-Type", "font/woff2")
		case ".css":
			if r.URL.Query().Get("v") != "" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=60")
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".js":
			if r.URL.Query().Get("v") != "" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=60")
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fileServer.ServeHTTP(w, r)
	})
}
