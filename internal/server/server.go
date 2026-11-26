// Package server holds the HTTP routes of the card search.
package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
	"github.com/dmikalova/keyforge-card-search/internal/search"
	"github.com/dmikalova/keyforge-card-search/internal/web"
)

// maxQuery is the longest query that the server reads.
const maxQuery = 2000

// Server serves the search pages.
type Server struct {
	src catalog.Source
	log *slog.Logger
	// seed gives the seed of a "sort:random" query without one.
	seed func() uint64
}

// New returns a Server for the source. A nil logger means slog.Default.
func New(src catalog.Source, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}

	return &Server{src: src, log: log, seed: rand.Uint64}
}

// Handler returns the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /batch", s.batch)
	mux.HandleFunc("GET /random", s.random)
	mux.HandleFunc("GET /syntax", func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, web.SyntaxPage())
	})
	mux.HandleFunc("GET /api/cards", s.export)
	mux.Handle("GET /static/", web.StaticHandler())
	mux.HandleFunc("GET /health", s.health)

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func queryOf(r *http.Request) string {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > maxQuery {
		q = q[:maxQuery]
	}

	return q
}

// page renders the search page with the first batch.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	q := queryOf(r)

	// The query form sends fz=1 and the Fuzzy box. Put the box's state in
	// the query, so the URL holds it.
	if r.URL.Query().Get("fz") == "1" {
		on := r.URL.Query().Get("fuzzy") == "on"
		http.Redirect(w, r, web.SearchURL(search.SetFuzzy(q, on)), http.StatusSeeOther)

		return
	}
	v := web.PageView{
		Query:   q,
		JSONURL: "/api/cards?q=" + web.QueryEscape(q),
		Houses:  web.NewHouseLinks(q),
		Missing: s.src.Missing(),
		Fuzzy:   !web.FuzzyOff(q),
	}

	res, err := s.src.Engine().Search(q)
	switch {
	case errors.Is(err, search.ErrNoSeed):
		http.Redirect(w, r, web.SearchURL(search.WithSeed(q, s.seed())), http.StatusSeeOther)

		return
	case err != nil:
		v.Err = errorView(q, err)
	default:
		at, _ := strconv.Atoi(r.URL.Query().Get("at"))
		first := res.Items[:web.FirstCount(at, len(res.Items))]
		v.Count = len(res.Items)
		v.Facets = web.NewFacets(q, res.Facets)
		v.Tiles = web.NewTiles(first, 0)
		v.Next = web.BatchURL(q, len(first), len(res.Items))
	}

	s.render(w, r, web.SearchPage(v))
}

// errorView splits the query at a parse error.
func errorView(q string, err error) *web.ErrorView {
	var qe *query.Error
	if !errors.As(err, &qe) {
		return &web.ErrorView{Msg: err.Error(), Before: q}
	}

	start := min(max(qe.Pos.Start, 0), len(q))
	end := min(max(qe.Pos.End, start), len(q))

	return &web.ErrorView{
		Msg:     qe.Msg,
		Suggest: qe.Suggest,
		Before:  q[:start],
		At:      q[start:end],
		After:   q[end:],
	}
}

// batch sends the batch at the offset: it appends the tiles to the grid
// and replaces the sentinel.
func (s *Server) batch(w http.ResponseWriter, r *http.Request) {
	q := queryOf(r)

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		http.Error(w, "bad offset", http.StatusBadRequest)

		return
	}

	res, err := s.src.Engine().Search(q)
	if err != nil {
		http.Error(w, "bad query", http.StatusBadRequest)

		return
	}

	offset = min(offset, len(res.Items))
	end := min(offset+web.BatchSize, len(res.Items))

	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(
		web.Tiles(web.NewTiles(res.Items[offset:end], offset)),
		datastar.WithSelectorID("grid"),
		datastar.WithModeAppend(),
	); err != nil {
		s.log.Warn("send a batch", "error", err)

		return
	}

	if err := sse.PatchElementTempl(
		web.Sentinel(web.BatchURL(q, end, len(res.Items))),
	); err != nil {
		s.log.Warn("send the sentinel", "error", err)
	}
}

// random sends the user to one random card of the query.
func (s *Server) random(w http.ResponseWriter, r *http.Request) {
	res, err := s.src.Engine().Search(queryOf(r))
	if err != nil || len(res.Items) == 0 {
		res, err = s.src.Engine().Search("")
	}

	if err != nil || len(res.Items) == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)

		return
	}

	pick := res.Items[s.seed()%uint64(len(res.Items))].Card
	http.Redirect(w, r, web.SearchURL(CardQuery(pick)), http.StatusSeeOther)
}

// CardQuery returns a query that finds one card.
func CardQuery(c *card.Card) string {
	title := strings.ReplaceAll(c.Title, `"`, "")
	q := `n="` + title + `" h:` + search.HouseOf(c)

	if c.HasFlag(search.FlagEvilTwin) {
		q += " is:" + search.FlagEvilTwin
	}

	return q
}

// exportCard is one result of the JSON export.
type exportCard struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	House     string           `json:"house,omitempty"`
	Type      string           `json:"type"`
	Traits    []string         `json:"traits,omitempty"`
	Text      string           `json:"text,omitempty"`
	Power     *card.Power      `json:"power,omitempty"`
	Armor     int              `json:"armor,omitempty"`
	Aember    int              `json:"aember,omitempty"`
	Keywords  map[string]int   `json:"keywords,omitempty"`
	Enhance   map[string]int   `json:"enhance,omitempty"`
	Flags     []string         `json:"flags,omitempty"`
	Printing  exportPrinting   `json:"printing"`
	Printings []exportPrinting `json:"printings"`
}

type exportPrinting struct {
	Set     string `json:"set"`
	Number  string `json:"number"`
	Rarity  string `json:"rarity"`
	Anomaly bool   `json:"anomaly,omitempty"`
	Image   string `json:"image,omitempty"`
	Flavor  string `json:"flavor,omitempty"`
}

type exportResult struct {
	Query string       `json:"query"`
	Count int          `json:"count"`
	Cards []exportCard `json:"cards"`
}

func toExport(p *card.Printing) exportPrinting {
	return exportPrinting{
		Set:     p.Set,
		Number:  p.Number,
		Rarity:  p.Rarity,
		Anomaly: p.Anomaly,
		Image:   catalog.ImageURL(p.Image),
		Flavor:  p.Flavor,
	}
}

// export writes the results as JSON, with absolute image URLs.
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	q := queryOf(r)

	res, err := s.src.Engine().Search(q)
	if errors.Is(err, search.ErrNoSeed) {
		res, err = s.src.Engine().Search(search.WithSeed(q, s.seed()))
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})

		return
	}

	out := exportResult{Query: q, Count: len(res.Items), Cards: make([]exportCard, len(res.Items))}

	for i, it := range res.Items {
		c := it.Card
		e := exportCard{
			ID: c.ID, Title: c.Title, House: c.House, Type: c.Type, Traits: c.Traits,
			Text: c.Text, Power: c.Power, Armor: c.Armor, Aember: c.Aember,
			Keywords: c.Keywords, Enhance: c.Enhance, Flags: c.Flags,
			Printing: toExport(it.Printing),
		}

		for j := range c.Printings {
			e.Printings = append(e.Printings, toExport(&c.Printings[j]))
		}

		out.Cards[i] = e
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(out); err != nil {
		s.log.Warn("write the export", "error", err)
	}
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := c.Render(r.Context(), w); err != nil {
		s.log.Warn("render a page", "error", err)
	}
}

// health reports the dataset version and the number of cards.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	e := s.src.Engine()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"version":   e.Version(),
		"generated": e.Generated(),
		"cards":     e.Len(),
		"missing":   s.src.Missing(),
	}); err != nil {
		s.log.Warn("write the health", "error", err)
	}
}
