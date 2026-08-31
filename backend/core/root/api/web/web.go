// Package web serves the compiled frontend out of the binary itself, so a
// deployment is one artifact with no static directory to ship beside it.
package web

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// dist is written by `make web` (SvelteKit's adapter-static output). Only
// .gitkeep is committed, so a checkout without a frontend build still
// compiles — serveUnbuilt below covers that case at runtime.
//
//go:embed all:dist
var embedded embed.FS

// SvelteKit fingerprints everything under _app/immutable, so those may be
// cached forever. The SPA shell must never be, or a browser keeps booting
// last deploy's bundle against this deploy's API.
const (
	immutablePrefix = "_app/immutable"
	immutableCache  = "public, max-age=31536000, immutable"
	shellCache      = "no-cache"
)

var (
	ErrNotBuilt     = errors.New("frontend not built: run `make web` before building the binary")
	ErrBaseMismatch = errors.New("frontend built for a different path prefix")
)

// SvelteKit writes the prefix it compiled against into the shell. Comparing it
// to the prefix we serve at turns the silent version of this mistake — every
// asset 404s, blank page, no logs — into one line at startup.
var baseInShell = regexp.MustCompile(`base:\s*"([^"]*)"`)

// Handler serves the SPA under rootPath. Unknown paths fall back to the shell
// rather than 404ing: the client router owns every route below rootPath, and
// only it knows which of those are real.
func Handler(rootPath string) (http.Handler, error) {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	shell, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, ErrNotBuilt
	}
	files := http.FileServer(http.FS(dist))
	prefix := strings.TrimSuffix(rootPath, "/")

	if m := baseInShell.FindSubmatch(shell); m != nil && string(m[1]) != prefix {
		return nil, fmt.Errorf("%w: built for %q but serving at %q — rebuild with WEB_ROOT_PATH=%q",
			ErrBaseMismatch, m[1], prefix, prefix)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, prefix), "/")

		// A directory listing would expose the build layout, and any miss is a
		// client route, so both resolve to the shell.
		if rel == "" || !exists(dist, rel) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", shellCache)
			w.Write(shell) //nolint
			return
		}

		if strings.HasPrefix(rel, immutablePrefix) {
			w.Header().Set("Cache-Control", immutableCache)
		} else {
			w.Header().Set("Cache-Control", shellCache)
		}
		//FileServer resolves against the root of its FS, not the request path
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = "/" + rel
		files.ServeHTTP(w, r2)
	}), nil
}

// Notice stands in when the frontend is unusable, so the bot and API still boot
// and the operator gets the reason on the page instead of a blank 404.
func Notice(reason string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, reason+"\n") //nolint
	})
}

func exists(dist fs.FS, name string) bool {
	f, err := dist.Open(name)
	if err != nil {
		return false
	}
	defer f.Close() //nolint
	info, err := f.Stat()
	return err == nil && !info.IsDir()
}
