package search

import (
	"slices"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
)

// Chips returns the top-level terms of a query in canonical form: each
// child of the top AND (a term, a negated term, a comma list, or a group in
// parentheses), then each directive. The search bar shows one chip for each.
// Joined with spaces, the chips are the canonical query.
func (e *Engine) Chips(src string) ([]string, error) {
	q, err := e.Parse(src)
	if err != nil {
		return nil, err
	}

	var out []string

	switch root := q.Root.(type) {
	case nil:
	case *query.And:
		for _, c := range root.Children {
			s := (&query.Query{Root: c}).String()
			// An OR inside the top AND needs its parentheses, so the joined
			// chips keep the meaning. A comma list does not.
			if or, ok := c.(*query.Or); ok && (!or.List || strings.Contains(s, " or ")) {
				s = "(" + s + ")"
			}

			out = append(out, s)
		}
	default:
		out = append(out, (&query.Query{Root: root}).String())
	}

	for _, d := range q.Directives {
		out = append(out, (&query.Query{Directives: []query.Directive{d}}).String())
	}

	return out, nil
}

// canonical returns the canonical form of one term, or the term as written
// if it does not parse.
func (e *Engine) canonical(term string) string {
	q, err := e.Parse(term)
	if err != nil {
		return strings.TrimSpace(term)
	}

	return q.String()
}

// HasChip reports whether the chips hold the term.
func (e *Engine) HasChip(chips []string, term string) bool {
	return slices.Contains(chips, e.canonical(term))
}

// Toggle returns the query without the term if the query has it at the top
// level, or with the term added at the end. A query with a parse error gets
// the term added.
func (e *Engine) Toggle(src, term string) string {
	chips, err := e.Chips(src)
	if err != nil {
		return AddTerm(src, term)
	}

	t := e.canonical(term)
	if i := slices.Index(chips, t); i >= 0 {
		return strings.Join(slices.Delete(chips, i, i+1), " ")
	}

	return strings.Join(append(chips, t), " ")
}

// Card returns the card with the ID, or nil.
func (e *Engine) Card(id string) *card.Card {
	for _, en := range e.cards {
		if en.Card.ID == id {
			return en.Card
		}
	}

	return nil
}
