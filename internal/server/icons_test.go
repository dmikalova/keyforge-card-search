package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestIconProxy(t *testing.T) {
	var hits atomic.Int32

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)

		if r.URL.Path != "/rarity/rare.svg" {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, "<svg/>")
	}))
	defer origin.Close()

	mux := http.NewServeMux()
	mux.Handle("GET /icons/{path...}", newIconProxy(origin.URL+"/"))

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		return rec
	}

	for range 2 {
		rec := get("/icons/rarity/rare.svg")
		if rec.Code != http.StatusOK || rec.Body.String() != "<svg/>" ||
			rec.Header().Get("Content-Type") != "image/svg+xml" {
			t.Fatalf("rare.svg: %d %q %q", rec.Code, rec.Body, rec.Header())
		}
	}

	if n := hits.Load(); n != 1 {
		t.Errorf("origin hits = %d, want 1 (cached)", n)
	}

	if rec := get("/icons/set/moremutation.svg"); rec.Code != http.StatusNotFound {
		t.Errorf("missing icon: %d", rec.Code)
	}

	// The mux cleans a path with "..", so it never serves it.
	if rec := get("/icons/../secret.svg"); rec.Code == http.StatusOK {
		t.Errorf("bad path: %d", rec.Code)
	}

	before := hits.Load()
	if rec := get("/icons/other/x.png"); rec.Code != http.StatusNotFound || hits.Load() != before {
		t.Errorf("a path outside the icon groups must not reach the origin")
	}
}
