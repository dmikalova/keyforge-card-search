package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dmikalova/keyforge-data/card"

	"github.com/dmikalova/gallery.keyforge.cards/internal/fixture"
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

// fakeBucket serves the pointer file data/latest.json and the version file
// that it names. status, when set, replaces the pointer response.
type fakeBucket struct {
	srv         *httptest.Server
	ds          atomic.Pointer[card.Dataset]
	status      atomic.Int32
	versionGets atomic.Int32
}

func newFakeBucket(t *testing.T, ds *card.Dataset) *fakeBucket {
	t.Helper()

	b := &fakeBucket{}
	b.ds.Store(ds)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+card.LatestPath, func(w http.ResponseWriter, _ *http.Request) {
		if s := b.status.Load(); s != 0 {
			w.WriteHeader(int(s))

			return
		}

		v := b.ds.Load().Version
		_ = json.NewEncoder(w).
			Encode(card.Latest{Version: v, URL: b.srv.URL + "/" + card.VersionPath(v)})
	})
	mux.HandleFunc("/data/cards/", func(w http.ResponseWriter, _ *http.Request) {
		b.versionGets.Add(1)
		_ = json.NewEncoder(w).Encode(b.ds.Load())
	})

	b.srv = httptest.NewServer(mux)
	t.Cleanup(b.srv.Close)

	return b
}

func (b *fakeBucket) pointer() string { return b.srv.URL + "/" + card.LatestPath }

// TestLoaderReadsAndKeepsTheDataset checks that the loader reads the version
// file only for a new version, and keeps the last good dataset on an error.
func TestLoaderReadsAndKeepsTheDataset(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	b := newFakeBucket(t, ds)

	l, err := NewLoader(b.pointer(), nil)
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
	if err := l.Refresh(ctx); err != nil || l.Engine() != first || b.versionGets.Load() != 1 {
		t.Errorf("an unchanged version: err %v, engine replaced %t, %d version reads",
			err, l.Engine() != first, b.versionGets.Load())
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
	b.ds.Store(&card.Dataset{Version: "v2", Cards: ds.Cards[:3], Sets: ds.Sets})

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
	b := newFakeBucket(t, &card.Dataset{})
	b.status.Store(http.StatusNotFound)

	l, err := NewLoader(b.pointer(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Refresh(context.Background()); err == nil || !l.Missing() || l.Engine().Len() != 0 {
		t.Errorf("err %v, missing %t, %d cards", err, l.Missing(), l.Engine().Len())
	}
}

func TestLoaderRefusesAPointerWithoutAVersion(t *testing.T) {
	b := newFakeBucket(t, &card.Dataset{})

	l, err := NewLoader(b.pointer(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Refresh(context.Background()); err == nil || b.versionGets.Load() != 0 {
		t.Errorf("err %v, %d version reads", err, b.versionGets.Load())
	}
}
