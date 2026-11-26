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
// terms (the chips), the draft that the user types, and the place of the
// draft among the chips.
type barSignals struct {
	Q     string `json:"q"`
	Draft string `json:"draft"`
	// Pos is the index of the chip after the draft, or -1 for the end. The
	// browser can send it as a number or as a string.
	Pos json.RawMessage `json:"pos"`
}

// pos returns the place of the draft, from 0 to the number of chips.
func (s barSignals) pos(chips int) int {
	n, err := strconv.Atoi(strings.Trim(string(s.Pos), `"`))
	if err != nil || n < 0 || n > chips {
		return chips
	}

	return n
}

// The operations of the live search endpoint.
const (
	opType   = "type"
	opCommit = "commit"
	opToggle = "toggle"
	opRemove = "remove"
	opEdit   = "edit"
	opPop    = "pop"
)

// barState is the search bar after an operation.
type barState struct {
	q, draft string
	// pos is the index of the chip after the draft, or -1 for the end.
	pos int
	// keep is true when the bar's signals must not change, for example
	// while the user types.
	keep bool
}

// query returns the whole query of the bar: the draft goes between the
// chips at pos.
func (b barState) query(e *search.Engine) string {
	d := strings.TrimSpace(b.draft)
	if d == "" {
		return strings.TrimSpace(b.q)
	}

	chips, err := e.Chips(b.q)
	if err != nil || b.pos < 0 || b.pos >= len(chips) {
		return search.AddTerm(b.q, d)
	}

	return strings.Join(slices.Concat(chips[:b.pos], []string{d}, chips[b.pos:]), " ")
}

// apply runs one operation on the bar.
func (s *Server) apply(op string, r *http.Request, sig barSignals) barState {
	e := s.src.Engine()
	chips, _ := e.Chips(sig.Q)
	pos := sig.pos(len(chips))

	at := pos
	if at == len(chips) {
		at = -1
	}

	typed := barState{q: sig.Q, draft: sig.Draft, pos: at, keep: true}
	full := typed.query(e)
	_, fullErr := e.Chips(full)
	index, _ := strconv.Atoi(r.URL.Query().Get("i"))
	inRange := index >= 0 && index < len(chips)

	switch op {
	case opCommit:
		if fullErr != nil {
			return typed
		}

		return barState{q: canonical(e, full), pos: -1}
	case opToggle:
		term := r.URL.Query().Get("term")
		if fullErr != nil {
			return barState{q: e.Toggle(sig.Q, term), draft: sig.Draft, pos: -1}
		}

		return barState{q: e.Toggle(full, term), pos: -1}
	case opRemove:
		if !inRange {
			return barState{q: sig.Q, draft: sig.Draft, pos: at}
		}

		if at > index {
			at--
		}

		return barState{
			q:     strings.Join(slices.Delete(chips, index, index+1), " "),
			draft: sig.Draft,
			pos:   at,
		}
	case opEdit:
		if !inRange {
			return barState{q: sig.Q, draft: sig.Draft, pos: at}
		}

		// The chip goes back to the draft, in its place among the chips. A
		// draft that the user typed elsewhere joins the chips first.
		rest := slices.Delete(slices.Clone(chips), index, index+1)
		if d := strings.TrimSpace(sig.Draft); d != "" {
			rest, _ = e.Chips(
				barState{
					q:     strings.Join(rest, " "),
					draft: d,
					pos:   placeAfterDelete(at, index),
				}.query(
					e,
				),
			)
		}

		return barState{q: strings.Join(rest, " "), draft: chips[index], pos: min(index, len(rest))}
	case opPop:
		if pos == 0 || strings.TrimSpace(sig.Draft) != "" {
			return typed
		}

		i := pos - 1

		return barState{
			q:     strings.Join(slices.Delete(slices.Clone(chips), i, i+1), " "),
			draft: chips[i],
			pos:   i,
		}
	default: // opType
		return typed
	}
}

// placeAfterDelete returns the place of the draft after the chip at index
// is removed.
func placeAfterDelete(at, index int) int {
	if at > index {
		return at - 1
	}

	return at
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
	q := b.query(e)

	res, err := e.Search(q)
	if errors.Is(err, search.ErrNoSeed) {
		b = barState{q: canonical(e, search.WithSeed(q, s.seed())), pos: -1}
		q = b.query(e)
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
		if !send("the signals", sse.PatchSignals([]byte(web.BarSignalsJSON(b.q, b.draft, b.pos)))) {
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
	)

	if r.URL.Query().Get("random") != "" {
		var sig barSignals
		_ = datastar.ReadSignals(r, &sig)

		chips, _ := e.Chips(sig.Q)
		res, err := e.Search(
			barState{q: sig.Q, draft: sig.Draft, pos: sig.pos(len(chips))}.query(e),
		)
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
	if err := sse.PatchElementTempl(
		web.Zoom(web.NewCardView(c, shown, setCode, e.Card)),
	); err != nil {
		s.log.Warn("send the card", "error", err)

		return
	}

	if err := sse.ExecuteScript(
		"window.kfsearch && kfsearch.opened(" + jsString(c.ID) + ")",
	); err != nil {
		s.log.Warn("send the card script", "error", err)
	}
}

// jsString returns s as a JavaScript string literal.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}

	return string(b)
}
