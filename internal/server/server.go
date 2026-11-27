// Package server holds the HTTP routes of the card gallery.
package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/dmikalova/gallery.keyforge.cards/internal/catalog"
	"github.com/dmikalova/gallery.keyforge.cards/internal/feedback"
	"github.com/dmikalova/gallery.keyforge.cards/internal/search"
	"github.com/dmikalova/gallery.keyforge.cards/internal/web"
)

// maxQuery is the longest query that the server reads.
const maxQuery = 2000

// Server serves the search pages.
type Server struct {
	src catalog.Source
	log *slog.Logger
	// seed gives the seed of a "sort:random" query without one.
	seed func() uint64
	// feedback sends the feedback form to Discord. Nil turns the form off.
	feedback *feedback.Handler
}

// New returns a Server for the source. A nil logger means slog.Default.
func New(src catalog.Source, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}

	return &Server{src: src, log: log, seed: rand.Uint64}
}

// WithFeedback turns on the feedback form, which posts to h.
func (s *Server) WithFeedback(h *feedback.Handler) *Server {
	s.feedback = h

	return s
}

// Handler returns the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.page)
	mux.HandleFunc("GET /batch", s.batch)
	mux.HandleFunc("GET /random", s.random)
	mux.HandleFunc("GET /results", s.live)
	mux.HandleFunc("GET /card", s.cardView)
	mux.HandleFunc("GET /card/{id}", s.cardPage)
	mux.HandleFunc("GET /syntax", func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, web.SyntaxPage(s.feedback.Enabled()))
	})
	// The syntax modal loads the same text from here.
	mux.HandleFunc("GET /syntax/content", func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, web.SyntaxContent())
	})
	mux.HandleFunc("GET /api/cards", s.export)
	mux.Handle("POST /api/feedback", s.feedback)
	mux.Handle("GET /static/", web.StaticHandler())
	mux.Handle("GET /icons/{path...}", newIconProxy(IconOrigin))
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

// page renders the search page with the first batch. card=ID opens the card
// view of the card, so a link to a card and a reload with the card view open
// work.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	s.searchPage(w, r, s.src.Engine().Card(r.URL.Query().Get("card")))
}

// cardPage renders the page of one card (/card/ID): the search page with the
// card view open, and a link preview of the card. The script moves the URL
// to /?card=ID, which has the same preview.
func (s *Server) cardPage(w http.ResponseWriter, r *http.Request) {
	c := s.src.Engine().Card(r.PathValue("id"))
	if c == nil {
		http.NotFound(w, r)

		return
	}

	s.searchPage(w, r, c)
}

// searchPage renders the search page, with the card view of c open if c is
// not nil.
func (s *Server) searchPage(w http.ResponseWriter, r *http.Request, c *card.Card) {
	q := queryOf(r)

	e := s.src.Engine()
	v := web.PageView{
		// The bar shows the query in canonical form.
		Query:    canonical(e, q),
		Missing:  s.src.Missing(),
		Complete: web.CompleteJSON(e.Completions()),
		Feedback: s.feedback.Enabled(),
	}

	if c != nil {
		cv := s.markedCard(c, defaultPrint(c), q)
		meta := web.CardMeta(cv, c)
		v.Card = &cv
		v.Meta = &meta
	}

	at, _ := strconv.Atoi(r.URL.Query().Get("at"))

	res, err := e.Search(q)
	if errors.Is(err, search.ErrNoSeed) {
		http.Redirect(w, r, web.SearchURL(search.WithSeed(q, s.seed())), http.StatusSeeOther)

		return
	}

	v.Results = s.results(q, res, err, at)
	s.render(w, r, web.SearchPage(v))
}

// results makes the results part of the page: the error, or the count, the
// facets, and the tiles up to the place at.
func (s *Server) results(q string, res *search.Result, err error, at int) web.Results {
	if err != nil {
		return web.Results{Err: errorView(q, err)}
	}

	first := res.Items[:web.FirstCount(at, len(res.Items))]

	return web.Results{
		Count:  len(res.Items),
		Total:  res.Total,
		Facets: web.NewFacets(s.src.Engine(), q, res.Facets),
		Tiles:  web.NewTiles(first, 0),
		Next:   web.BatchURL(q, len(first), len(res.Items)),
	}
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
	Mentions  []card.Mention   `json:"mentions,omitempty"`
	Notes     []card.Note      `json:"notes,omitempty"`
	Related   []exportRelated  `json:"related,omitempty"`
	Printing  exportPrinting   `json:"printing"`
	Printings []exportPrinting `json:"printings"`
}

type exportPrinting struct {
	Set     string `json:"set"`
	Number  string `json:"number"`
	Rarity  string `json:"rarity"`
	Anomaly bool   `json:"anomaly,omitempty"`
	// Maverick is true for a printing that Master Vault has only as
	// mavericks. Its image is the card's newest normal image.
	Maverick bool   `json:"maverick,omitempty"`
	Image    string `json:"image,omitempty"`
	Flavor   string `json:"flavor,omitempty"`
	Artist   string `json:"artist,omitempty"`
	// House is the house of a printing in another house than the card
	// (Redemption).
	House string `json:"house,omitempty"`
}

type exportResult struct {
	Query string `json:"query"`
	Count int    `json:"count"`
	// Total is the number of results before limit:N.
	Total int          `json:"total"`
	Cards []exportCard `json:"cards"`
}

func toExport(p *card.Printing) exportPrinting {
	return exportPrinting{
		Set:      p.Set,
		Number:   p.Number,
		Rarity:   p.Rarity,
		Anomaly:  p.Anomaly,
		Maverick: p.Maverick,
		Image:    catalog.ImageURL(p.Image),
		Flavor:   p.Flavor,
		Artist:   p.Artist,
		House:    p.House,
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

	out := exportResult{
		Query: q, Count: len(res.Items), Total: res.Total,
		Cards: make([]exportCard, len(res.Items)),
	}

	for i, it := range res.Items {
		c := it.Card
		e := exportCard{
			ID: c.ID, Title: c.Title, House: c.House, Type: c.Type, Traits: c.Traits,
			Text: c.Text, Power: c.Power, Armor: c.Armor, Aember: c.Aember,
			Keywords: c.Keywords, Enhance: c.Enhance, Flags: c.Flags,
			Mentions: c.Mentions, Related: s.related(c),
			Notes:    c.Notes,
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

// defaultPrint returns the index of a card's default printing: the newest
// printing that is not an anomaly, or the newest printing.
func defaultPrint(c *card.Card) int {
	// A printing in the card's own house comes first: a Redemption printing
	// shows only when the query selects it.
	for i, v := range slices.Backward(c.Printings) {
		if !v.Anomaly && v.House == "" {
			return i
		}
	}

	for i, v := range slices.Backward(c.Printings) {
		if !v.Anomaly {
			return i
		}
	}

	return max(len(c.Printings)-1, 0)
}

// exportRelated is one thread of related cards, with the title of each card.
type exportRelated struct {
	Label string      `json:"label"`
	Cards []exportRef `json:"cards"`
}

// exportRef is a card that a thread names.
type exportRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// related returns the card's threads of related cards. A card ID that the
// dataset does not have keeps an empty title.
func (s *Server) related(c *card.Card) []exportRelated {
	out := make([]exportRelated, 0, len(c.Related))

	for _, r := range c.Related {
		t := exportRelated{Label: r.Label, Cards: make([]exportRef, len(r.Cards))}

		for i, id := range r.Cards {
			t.Cards[i].ID = id
			if rc := s.src.Engine().Card(id); rc != nil {
				t.Cards[i].Title = rc.Title
			}
		}

		out = append(out, t)
	}

	return out
}

// setNames returns the code and the full name of a set slug, or the slug
// twice.
func (s *Server) setNames(slug string) (string, string) {
	if set, ok := s.src.Engine().SetOf(slug); ok {
		return set.Code, set.Name
	}

	return slug, slug
}
