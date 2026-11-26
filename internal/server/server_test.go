package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dmikalova/keyforge-data/card"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
	"github.com/dmikalova/keyforge-card-search/internal/fixture"
)

func handler(t *testing.T) http.Handler {
	t.Helper()

	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	src, err := catalog.NewStatic(ds)
	if err != nil {
		t.Fatal(err)
	}

	s := New(src, nil)
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
		`id="q" name="q" type="search" value="house:brobnar"`,
		`data-signals="{&#34;q&#34;:&#34;house:brobnar&#34;}"`,
		`class="tile"`,
		`ageofascension/001.webp`,
		` House</h2>`,
		`Power</h2>`,
		`Æmber</h2>`,
		`stat/aember.webp`,
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

	for _, want := range []string{"selector #grid", "mode append", `class="tile"`, `data-i="30"`, `id="sentinel"`} {
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

	if img := out.Cards[0].Printings[0].Image; img != card.ImageBase+"callofthearchons/048.webp" {
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

	// The modal loads only the text, without the page around it.
	frag := get(t, h, "/syntax/content")
	if frag.Code != http.StatusOK ||
		!bytes.Contains(frag.Body.Bytes(), []byte(`id="syntax-title"`)) ||
		bytes.Contains(frag.Body.Bytes(), []byte("<html")) {
		t.Errorf("syntax content: %d %.80q", frag.Code, frag.Body.String())
	}

	page := get(t, h, "/").Body.String()
	// The card count and the size slider are at the top of the side panel.
	if f, s := strings.Index(
		page,
		`class="facets"`,
	), strings.Index(
		page,
		`class="summary"`,
	); f < 0 ||
		s < f {
		t.Error("the summary is not in the side panel")
	}

	if !strings.Contains(page, `id="syntax-modal"`) || !strings.Contains(page, `href="/syntax"`) {
		t.Error("the page has no syntax modal or no /syntax link")
	}

	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Cards   int    `json:"cards"`
	}

	w := get(t, h, "/health")
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil || health.Status != "ok" ||
		health.Version != "fixture" || health.Cards == 0 {
		t.Errorf("health: %q", w.Body.String())
	}
}

func TestSearchBarIsPlainText(t *testing.T) {
	body := get(t, handler(t), "/?q="+url.QueryEscape("n:troll h:brobnar")).Body.String()
	if !strings.Contains(body, `value="name:troll house:brobnar"`) {
		t.Error("the search bar does not hold the query text")
	}

	for _, gone := range []string{`id="fuzzy"`, `id="chips"`, `class="chip`, `name="d"`} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still has %s", gone)
		}
	}
}

func TestAnomalyTileHasNoOverlay(t *testing.T) {
	body := get(t, handler(t), "/?q="+url.QueryEscape("is:anomaly")).Body.String()
	if !strings.Contains(body, `class="tile"`) || strings.Contains(body, "rarity-mark") {
		t.Error("the anomaly tile has an overlay, or there is no tile")
	}
}

func TestCardParamOpensTheCardView(t *testing.T) {
	body := get(t, handler(t), "/?card=troll-brobnar").Body.String()
	for _, want := range []string{`id="zoom"`, `data-card="troll-brobnar"`, `id="acdata"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page has no %q", want)
		}
	}

	if strings.Contains(body, `id="zoom" class="zoom" hidden`) {
		t.Error("the card view is hidden")
	}
}

func TestMissingDatasetShowsABanner(t *testing.T) {
	src := missingSource{}

	empty, err := catalog.NewStatic(&card.Dataset{})
	if err != nil {
		t.Fatal(err)
	}

	src.Static = empty

	body := get(t, New(src, nil).Handler(), "/").Body.String()
	if !strings.Contains(body, "not published yet") {
		t.Error("no banner for a missing dataset")
	}
}

type missingSource struct{ *catalog.Static }

func (missingSource) Missing() bool { return true }
