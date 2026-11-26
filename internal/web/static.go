// Package web renders the pages and serves the embedded static files.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var embedded embed.FS

// Static is the static file tree: CSS, JS, fonts, and images.
var Static = func() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err) // The embed directive guarantees the directory.
	}

	return sub
}()

// versions maps each static file to a short hash of its contents. Asset puts
// it in the URL, so a deploy that changes a file changes its URL.
var versions = func() map[string]string {
	v := map[string]string{}

	_ = fs.WalkDir(Static, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		b, err := fs.ReadFile(Static, name)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(b)
		v[name] = hex.EncodeToString(sum[:5])

		return nil
	})

	return v
}()

// Cache lifetimes. A URL with the file's current version never changes, so it
// is cached for a year. Any other URL gets an hour.
const (
	cacheVersioned = "public, max-age=31536000, immutable"
	cacheOther     = "public, max-age=3600"
)

// Asset returns the URL of static file name, with its version.
func Asset(name string) string {
	return "/static/" + name + "?v=" + versions[name]
}

// HasAsset reports whether static file name exists.
func HasAsset(name string) bool {
	_, ok := versions[name]

	return ok
}

// StaticHandler serves the static files under /static/.
func StaticHandler() http.Handler {
	files := http.FileServerFS(Static)

	return http.StripPrefix(
		"/static/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

			v, ok := versions[name]
			if !ok {
				http.NotFound(w, r)

				return
			}

			if r.URL.Query().Get("v") == v {
				w.Header().Set("Cache-Control", cacheVersioned)
			} else {
				w.Header().Set("Cache-Control", cacheOther)
			}

			files.ServeHTTP(w, r)
		}),
	)
}
