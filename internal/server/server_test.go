package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
	"github.com/dmikalova/keyforge-card-search/internal/search"
)

func handler(t *testing.T) http.Handler {
	t.Helper()

	c, err := catalog.Fixture()
	if err != nil {
		t.Fatal(err)
	}

	e, err := search.New(c.Cards())
	if err != nil {
		t.Fatal(err)
	}

	s := New(e, nil)
	s.seed = func() uint64 { return 42 }

	return s.Handler()
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

	return w
}

func TestPageShowsResultsAndFacets(t *testing.T) {
	w := get(t, handler(t), "/?q="+url.QueryEscape("h:brobnar"))
	body := w.Body.String()

	for _, want := range []string{
		`value="h:brobnar"`,
		`class="tile"`,
		`ageofascension/001.webp`,
		`<h2>House</h2>`,
		`<h2>Power</h2>`,
		`id="sentinel"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page has no %q", want)
		}
	}

	if strings.Contains(body, `data-on-intersect`) {
		t.Error("a query with one batch has a live sentinel")
	}
}

func TestPageShowsParseError(t *testing.T) {
	body := get(t, handler(t), "/?q="+url.QueryEscape("t:creature hose:dis")).Body.String()

	for _, want := range []string{`role="alert"`, `<mark>hose</mark>:dis`, `<code>house</code>`} {
		if !strings.Contains(body, want) {
			t.Errorf("page has no %q\n%s", want, body)
		}
	}
}

func TestRandomSortGetsASeed(t *testing.T) {
	w := get(t, handler(t), "/?q="+url.QueryEscape("sort:random"))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", w.Code)
	}

	if loc := w.Header().Get("Location"); loc != "/?q="+url.QueryEscape("sort:random seed:42") {
		t.Errorf("Location = %s", loc)
	}
}

func TestBatchAppendsTiles(t *testing.T) {
	w := get(t, handler(t), "/batch?q=&offset=30")
	body := w.Body.String()

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %s", ct)
	}

	for _, want := range []string{"selector #grid", "mode append", `class="tile"`, `id="sentinel"`} {
		if !strings.Contains(body, want) {
			t.Errorf("batch has no %q\n%s", want, body)
		}
	}
}

func TestRandomRedirectsToOneCard(t *testing.T) {
	w := get(t, handler(t), "/random?q="+url.QueryEscape("is:eviltwin"))
	want := "/?q=" + url.QueryEscape(`n="Krump" h:brobnar is:eviltwin`)

	if loc := w.Header().Get("Location"); loc != want {
		t.Errorf("Location = %s, want %s", loc, want)
	}
}

func TestExportHasImageURLs(t *testing.T) {
	w := get(t, handler(t), "/api/cards?q="+url.QueryEscape("n=troll"))

	var out exportResult
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}

	if out.Count != 1 || len(out.Cards[0].Printings) != 2 {
		t.Fatalf("export = %+v", out)
	}

	if img := out.Cards[0].Printing.Image; img != "" {
		t.Errorf("image = %s, want empty (the fixture has no image for this printing)", img)
	}

	if img := out.Cards[0].Printings[0].Image; img != catalog.ImageBaseURL+"callofthearchons/048.webp" {
		t.Errorf("first printing image = %s", img)
	}

	if w := get(t, handler(t), "/api/cards?q=hose:dis"); w.Code != http.StatusBadRequest {
		t.Errorf("bad query status %d, want 400", w.Code)
	}
}

func TestSyntaxAndHealth(t *testing.T) {
	h := handler(t)
	if w := get(
		t,
		h,
		"/syntax",
	); w.Code != http.StatusOK ||
		!bytes.Contains(w.Body.Bytes(), []byte("Query syntax")) {
		t.Errorf("syntax: %d", w.Code)
	}

	if w := get(t, h, "/health"); w.Body.String() != "ok" {
		t.Errorf("health: %q", w.Body.String())
	}
}
