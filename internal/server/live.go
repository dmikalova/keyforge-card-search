package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/dmikalova/keyforge-card-search/internal/search"
	"github.com/dmikalova/keyforge-card-search/internal/web"
)

// barSignals are the Datastar signals of the search bar: the query text.
type barSignals struct {
	Q string `json:"q"`
}

// The operations of the live search endpoint.
const (
	opType   = "type"
	opCommit = "commit"
	opToggle = "toggle"
)

// barState is the search bar after an operation.
type barState struct {
	q string
	// keep is true when the bar's text must not change, for example while
	// the user types.
	keep bool
}

// apply runs one operation on the bar.
func (s *Server) apply(op string, r *http.Request, sig barSignals) barState {
	e := s.src.Engine()

	switch op {
	case opCommit:
		// Enter writes the query in canonical form. A query with an error
		// stays as the user typed it.
		return barState{q: canonical(e, sig.Q)}
	case opToggle:
		return barState{q: e.Toggle(sig.Q, r.URL.Query().Get("term"))}
	default: // opType
		return barState{q: sig.Q, keep: true}
	}
}

// canonical returns the canonical form of the query, or the query if it
// does not parse.
func canonical(e *search.Engine, q string) string {
	terms, err := e.Terms(q)
	if err != nil {
		return strings.TrimSpace(q)
	}

	return strings.Join(terms, " ")
}

// live runs one search bar operation and sends the new results, the
// download link, and URL. While the user types, a query with an error only
// shows the error line, so the results do not flash.
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	var sig barSignals
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad signals", http.StatusBadRequest)

		return
	}

	op := r.URL.Query().Get("op")
	b := s.apply(op, r, sig)
	e := s.src.Engine()
	q := b.q

	res, err := e.Search(q)
	if errors.Is(err, search.ErrNoSeed) {
		b = barState{q: canonical(e, search.WithSeed(q, s.seed()))}
		q = b.q
		res, err = e.Search(q)
	}

	sse := datastar.NewSSE(w, r)
	send := func(what string, err error) bool {
		if err != nil {
			s.log.Warn("send "+what, "error", err)

			return false
		}

		return true
	}

	if err != nil && (op == "" || op == opType || op == opCommit) {
		send("the error line", sse.PatchElementTempl(web.QueryError(errorLine(q, err))))

		return
	}

	if !send("the error line", sse.PatchElementTempl(web.QueryError(""))) ||
		!send("the results", sse.PatchElementTempl(web.ResultsView(s.results(q, res, err, 0)))) {
		return
	}

	if !send("the download link", sse.PatchElementTempl(web.DownloadLink(q))) {
		return
	}

	// A filter click in the card view (card=ID) keeps the view open with
	// the new marks.
	if c := e.Card(r.URL.Query().Get("card")); c != nil {
		shown, _ := strconv.Atoi(r.URL.Query().Get("p"))
		if !send("the card", sse.PatchElementTempl(web.Zoom(s.markedCard(c, shown, q)))) {
			return
		}
	}

	if !b.keep {
		if !send("the signals", sse.PatchSignals([]byte(web.SignalsJSON(b.q)))) {
			return
		}
	}

	update, _ := json.Marshal(map[string]any{
		"url":   web.SearchURL(q),
		"title": web.PageTitle(q),
	})
	send("the URL", sse.ExecuteScript("window.kfsearch && kfsearch.update("+string(update)+")"))
}

// errorLine is the short error text under the search bar.
func errorLine(q string, err error) string {
	ev := errorView(q, err)
	msg := ev.Msg

	if ev.Suggest != "" {
		msg += ". Did you mean " + ev.Suggest + "?"
	}

	return msg
}

// cardView opens the card view: one card by ID and printing index, or a
// random card of the bar's query.
func (s *Server) cardView(w http.ResponseWriter, r *http.Request) {
	e := s.src.Engine()

	var (
		c     *card.Card
		shown int
		sig   barSignals
	)

	// The bar's query marks the values that it filters on.
	_ = datastar.ReadSignals(r, &sig)

	if r.URL.Query().Get("random") != "" {
		res, err := e.Search(sig.Q)
		if err != nil || len(res.Items) == 0 {
			res, err = e.Search("")
		}

		if err != nil || len(res.Items) == 0 {
			http.Error(w, "no cards", http.StatusNotFound)

			return
		}

		it := res.Items[s.seed()%uint64(len(res.Items))]
		c = it.Card
		for i := range c.Printings {
			if &c.Printings[i] == it.Printing {
				shown = i
			}
		}
	} else {
		c = e.Card(r.URL.Query().Get("id"))
		shown, _ = strconv.Atoi(r.URL.Query().Get("p"))
	}

	if c == nil {
		http.Error(w, "no card", http.StatusNotFound)

		return
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(web.Zoom(s.markedCard(c, shown, sig.Q))); err != nil {
		s.log.Warn("send the card", "error", err)

		return
	}

	if err := sse.ExecuteScript(
		"window.kfsearch && kfsearch.opened(" + jsString(c.ID) + ")",
	); err != nil {
		s.log.Warn("send the card script", "error", err)
	}
}

// markedCard returns the card view of a card with the values that the query
// filters on marked.
func (s *Server) markedCard(c *card.Card, shown int, q string) web.CardView {
	e := s.src.Engine()
	cv := web.NewCardView(c, shown, e.Labels(), s.setName, e.Card)
	cv.Mark(e.Filters(q))

	return cv
}

// jsString returns s as a JavaScript string literal.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}

	return string(b)
}
