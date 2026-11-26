package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/dmikalova/keyforge-card-search/internal/search"
	"github.com/dmikalova/keyforge-card-search/internal/web"
)

// barSignals are the Datastar signals of the search bar: the committed
// terms (the chips) and the draft that the user types after them.
type barSignals struct {
	Q     string `json:"q"`
	Draft string `json:"draft"`
}

// The operations of the live search endpoint.
const (
	opType   = "type"
	opCommit = "commit"
	opToggle = "toggle"
	opRemove = "remove"
	opEdit   = "edit"
	opPop    = "pop"
	opFuzzy  = "fuzzy"
)

// barState is the search bar after an operation.
type barState struct {
	q, draft string
	// keep is true when the bar's signals must not change, for example
	// while the user types.
	keep bool
}

// query returns the whole query of the bar.
func (b barState) query() string {
	d := strings.TrimSpace(b.draft)
	if d == "" {
		return strings.TrimSpace(b.q)
	}

	return search.AddTerm(b.q, d)
}

// apply runs one operation on the bar.
func (s *Server) apply(op string, r *http.Request, sig barSignals) barState {
	e := s.src.Engine()
	full := barState{q: sig.Q, draft: sig.Draft}.query()
	chips, _ := e.Chips(sig.Q)
	_, fullErr := e.Chips(full)
	index, _ := strconv.Atoi(r.URL.Query().Get("i"))
	inRange := index >= 0 && index < len(chips)

	switch op {
	case opCommit:
		if fullErr != nil {
			return barState{q: sig.Q, draft: sig.Draft, keep: true}
		}

		return barState{q: canonical(e, full)}
	case opToggle:
		term := r.URL.Query().Get("term")
		if fullErr != nil {
			return barState{q: e.Toggle(sig.Q, term), draft: sig.Draft}
		}

		return barState{q: e.Toggle(full, term)}
	case opRemove:
		if inRange {
			chips = slices.Delete(chips, index, index+1)
		}

		return barState{q: strings.Join(chips, " "), draft: sig.Draft}
	case opEdit:
		if !inRange {
			return barState{q: sig.Q, draft: sig.Draft}
		}

		draft := barState{q: chips[index], draft: sig.Draft}.query()

		return barState{q: strings.Join(slices.Delete(chips, index, index+1), " "), draft: draft}
	case opPop:
		if len(chips) == 0 || strings.TrimSpace(sig.Draft) != "" {
			return barState{q: sig.Q, draft: sig.Draft, keep: true}
		}

		last := chips[len(chips)-1]

		return barState{q: strings.Join(chips[:len(chips)-1], " "), draft: last}
	case opFuzzy:
		on := r.URL.Query().Get("on") == "on"
		if fullErr != nil {
			return barState{q: search.SetFuzzy(sig.Q, on), draft: sig.Draft}
		}

		return barState{q: canonical(e, search.SetFuzzy(full, on))}
	default: // opType
		return barState{q: sig.Q, draft: sig.Draft, keep: true}
	}
}

// canonical returns the joined chips of the query, or the query if it does
// not parse.
func canonical(e *search.Engine, q string) string {
	chips, err := e.Chips(q)
	if err != nil {
		return q
	}

	return strings.Join(chips, " ")
}

// live runs one search bar operation and sends the new results, chips,
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
	q := b.query()

	res, err := e.Search(q)
	if errors.Is(err, search.ErrNoSeed) {
		b = barState{q: canonical(e, search.WithSeed(q, s.seed()))}
		q = b.query()
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

	chips, _ := e.Chips(b.q)
	if !send("the chips", sse.PatchElementTempl(web.Chips(web.NewChips(chips), true))) ||
		!send("the download link", sse.PatchElementTempl(web.DownloadLink(q))) {
		return
	}

	if !b.keep {
		if !send("the signals", sse.PatchSignals([]byte(web.SignalsJSON(b.q, b.draft)))) {
			return
		}
	}

	update, _ := json.Marshal(map[string]any{
		"url":   web.SearchURL(q),
		"title": web.PageTitle(q),
		"fuzzy": !search.FuzzyIsOff(q),
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
	)

	if r.URL.Query().Get("random") != "" {
		var sig barSignals
		_ = datastar.ReadSignals(r, &sig)

		res, err := e.Search(barState{q: sig.Q, draft: sig.Draft}.query())
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

	setCode := func(slug string) string {
		if set, ok := e.SetOf(slug); ok {
			return set.Code
		}

		return slug
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(web.Zoom(web.NewCardView(c, shown, setCode))); err != nil {
		s.log.Warn("send the card", "error", err)

		return
	}

	if err := sse.ExecuteScript("window.kfsearch && kfsearch.opened()"); err != nil {
		s.log.Warn("send the card script", "error", err)
	}
}
