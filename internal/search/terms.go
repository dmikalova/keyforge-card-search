package search

import (
	"slices"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
)

// Terms returns the top-level terms of a query in canonical form: each
// child of the top AND (a term, a negated term, a comma list, or a group in
// parentheses), then each directive. Joined with spaces, the terms are the
// canonical query. The facets use them to add and remove a term.
func (e *Engine) Terms(src string) ([]string, error) {
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
			// terms keep the meaning. A comma list does not.
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

// HasTerm reports whether the terms hold the term.
func (e *Engine) HasTerm(terms []string, term string) bool {
	return slices.Contains(terms, e.canonical(term))
}

// Toggle returns the query without the facet filter of the term, if the
// query has it at the top level in any spelling (Filters.State), or with the
// term added at the end. A query with a parse error gets the term added.
func (e *Engine) Toggle(src, term string) string {
	if field, value, ok := e.facetRef(term); ok {
		if state, without := e.Filters(src).State(field, value); state != StateNone {
			return without
		}
	}

	terms, err := e.Terms(src)
	if err != nil {
		return AddTerm(src, term)
	}

	t := e.canonical(term)
	if i := slices.Index(terms, t); i >= 0 {
		return strings.Join(slices.Delete(terms, i, i+1), " ")
	}

	return strings.Join(append(terms, t), " ")
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
