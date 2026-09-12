package http

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed webdist/placeholder
var placeholderWebDist embed.FS

// webDistFS is placeholderWebDist rooted at the placeholder directory
// itself, so "index.html" is directly accessible — the same shape the
// real web/dist build's fs.FS will have.
var webDistFS, webDistFSErr = fs.Sub(placeholderWebDist, "webdist/placeholder")

// StaticHandler serves fsys — the embedded web/dist build, or a fixture
// in tests: a request matching a
// real file is served with the Content-Type its extension implies
// (http.FileServerFS's own use of the standard mime.TypeByExtension
// registry, never a hardcoded map); a request matching no real file
// falls back to index.html (200, text/html) — the SPA-fallback pattern
// client-side routing needs, since a
// direct navigation to e.g. /library has no corresponding file in the
// build output.
func StaticHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "."
		}

		if info, err := fs.Stat(fsys, name); err != nil || info.IsDir() {
			// "/", not "/index.html": http.FileServerFS redirects an
			// explicit request for "/index.html" to "/" (its own
			// canonical-directory-index rule) rather than serving it
			// directly, which would turn this fallback into a redirect
			// loop from the client's perspective instead of a 200.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}

// DefaultStaticHandler serves the embedded placeholder web/dist.
// cmd/server registers this as the router's catch-all for everything
// that isn't /api/v1/... or /healthz/readyz.
func DefaultStaticHandler() http.Handler {
	if webDistFSErr != nil {
		// Can't actually happen: webdist/placeholder is a compile-time
		// //go:embed target, checked by the compiler itself. A panic
		// here would mean the embed directive above and this Sub call
		// disagree about the directory name — a build-time bug, not a
		// runtime condition to handle gracefully.
		panic("internal/transport/http: placeholder web/dist embed is broken: " + webDistFSErr.Error())
	}
	return StaticHandler(webDistFS)
}
