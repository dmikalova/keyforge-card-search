package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dmikalova/keyforge-data/card"

	"github.com/dmikalova/keyforge-card-search/internal/fixture"
)

func TestImageURL(t *testing.T) {
	if got := ImageURL("grimreminders/239.webp"); got != card.ImageBase+"grimreminders/239.webp" {
		t.Errorf("ImageURL = %q", got)
	}

	if got := ImageURL(
		"grimreminders/239.webp?v=17",
	); got != card.ImageBase+"grimreminders/239.webp?v=17" {
		t.Errorf("ImageURL drops the version: %q", got)
	}

	if got := ImageURL(""); got != "" {
		t.Errorf("ImageURL(\"\") = %q, want empty", got)
	}
}

// fakeBucket serves a dataset with an ETag. status, when set, replaces the
// response.
type fakeBucket struct {
	ds     *card.Dataset
	status atomic.Int32
	gets   atomic.Int32
}

func (b *fakeBucket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.gets.Add(1)

	if s := b.status.Load(); s != 0 {
		w.WriteHeader(int(s))

		return
	}

	etag := `"` + b.ds.Version + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	w.Header().Set("ETag", etag)
	_ = json.NewEncoder(w).Encode(b.ds)
}

func TestLoaderReadsAndKeepsTheDataset(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	b := &fakeBucket{ds: ds}
	srv := httptest.NewServer(b)
	defer srv.Close()

	l, err := NewLoader(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	if l.Engine().Len() != 0 {
		t.Fatal("a new loader is not empty")
	}

	ctx := context.Background()
	if err := l.Refresh(ctx); err != nil {
		t.Fatal(err)
	}

	if got := l.Engine().Len(); got != len(ds.Cards) || l.Engine().Version() != "fixture" {
		t.Fatalf("loaded %d cards, version %q", got, l.Engine().Version())
	}

	first := l.Engine()
	if err := l.Refresh(ctx); err != nil || l.Engine() != first {
		t.Errorf("an unchanged dataset: err %v, engine replaced %t", err, l.Engine() != first)
	}

	b.status.Store(http.StatusInternalServerError)

	if err := l.Refresh(ctx); err == nil || l.Engine() != first {
		t.Errorf("a failed read: err %v, engine kept %t", err, l.Engine() == first)
	}

	b.status.Store(http.StatusNotFound)

	if err := l.Refresh(ctx); err == nil || l.Missing() {
		t.Errorf(
			"a 404 after a good read: err %v, missing %t; want the old dataset",
			err,
			l.Missing(),
		)
	}

	b.status.Store(0)
	b.ds = &card.Dataset{Version: "v2", Cards: ds.Cards[:3], Sets: ds.Sets}

	if err := l.Refresh(ctx); err != nil || l.Engine().Version() != "v2" || l.Engine().Len() != 3 {
		t.Errorf(
			"a new dataset: err %v, version %q, %d cards",
			err,
			l.Engine().Version(),
			l.Engine().Len(),
		)
	}
}

func TestLoaderMarksAMissingDataset(t *testing.T) {
	b := &fakeBucket{}
	b.status.Store(http.StatusNotFound)

	srv := httptest.NewServer(b)
	defer srv.Close()

	l, err := NewLoader(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Refresh(context.Background()); err == nil || !l.Missing() || l.Engine().Len() != 0 {
		t.Errorf("err %v, missing %t, %d cards", err, l.Missing(), l.Engine().Len())
	}
}

// TestLoaderReadsThePointer checks that a loader of data/cards.json reads
// data/latest.json, reads the version file only for a new version, and
// falls back to data/cards.json when the pointer fails.
func TestLoaderReadsThePointer(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	var pointerStatus, versionGets, fileGets atomic.Int32

	version := atomic.Value{}
	version.Store("fixture")

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/"+card.LatestPath, func(w http.ResponseWriter, _ *http.Request) {
		if s := pointerStatus.Load(); s != 0 {
			w.WriteHeader(int(s))

			return
		}

		v, _ := version.Load().(string)
		_ = json.NewEncoder(w).
			Encode(card.Latest{Version: v, URL: srv.URL + "/" + card.VersionPath(v)})
	})
	mux.HandleFunc("/data/cards/", func(w http.ResponseWriter, _ *http.Request) {
		versionGets.Add(1)

		d := *ds
		d.Version, _ = version.Load().(string)
		_ = json.NewEncoder(w).Encode(&d)
	})
	mux.HandleFunc("/"+card.DatasetPath, func(w http.ResponseWriter, _ *http.Request) {
		fileGets.Add(1)

		d := *ds
		d.Version = "file"
		_ = json.NewEncoder(w).Encode(&d)
	})

	l, err := NewLoader(srv.URL+"/"+card.DatasetPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for range 2 {
		if err := l.Refresh(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if l.Engine().Version() != "fixture" || versionGets.Load() != 1 || fileGets.Load() != 0 {
		t.Errorf("version %q, %d version reads, %d file reads",
			l.Engine().Version(), versionGets.Load(), fileGets.Load())
	}

	version.Store("v2")

	if err := l.Refresh(
		ctx,
	); err != nil || l.Engine().Version() != "v2" ||
		versionGets.Load() != 2 {
		t.Errorf("a new version: err %v, version %q", err, l.Engine().Version())
	}

	pointerStatus.Store(http.StatusNotFound)

	if err := l.Refresh(ctx); err != nil || l.Engine().Version() != "file" {
		t.Errorf("a failed pointer: err %v, version %q", err, l.Engine().Version())
	}
}
