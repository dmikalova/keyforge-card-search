package search

import (
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/query"
)

// FilterState says how a query uses one facet value.
type FilterState int

// The filter states.
const (
	// StateNone: the query does not filter on the value.
	StateNone FilterState = iota
	// StateActive: a top-level term selects the value.
	StateActive
	// StateExcluded: a top-level negated term excludes the value.
	StateExcluded
)

// Filters are the top-level filters of a parsed query. The facets use them
// to show which values the query selects, and to remove a filter.
type Filters struct {
	e     *Engine
	nodes []query.Node
	dirs  []query.Directive
}

// Filters parses a query and returns its top-level filters. A query that
// does not parse has no filters.
func (e *Engine) Filters(src string) *Filters {
	f := &Filters{e: e}

	q, err := e.Parse(src)
	if err != nil {
		return f
	}

	f.nodes = topNodes(q.Root)
	f.dirs = q.Directives

	return f
}

// topNodes returns the children of the top AND, or the root alone.
func topNodes(root query.Node) []query.Node {
	switch r := root.(type) {
	case nil:
		return nil
	case *query.And:
		return r.Children
	default:
		return []query.Node{r}
	}
}

// State returns how the query uses the facet value, and the query without
// that filter. For a comma list, the query without the value keeps the
// other values of the list.
func (f *Filters) State(field, value string) (FilterState, string) {
	for i, n := range f.nodes {
		state := StateActive

		if not, ok := n.(*query.Not); ok {
			state = StateExcluded
			n = not.Child
		}

		switch n := n.(type) {
		case *query.Term:
			if f.e.matchFacet(n, field, value) {
				return state, f.without(i, -1)
			}
		case *query.Or:
			if !n.List {
				continue
			}

			for j, c := range n.Children {
				if t, ok := c.(*query.Term); ok && f.e.matchFacet(t, field, value) {
					return state, f.without(i, j)
				}
			}
		}
	}

	return StateNone, ""
}

// without returns the query without top-level node i, or without child j
// of the comma list at node i.
func (f *Filters) without(i, j int) string {
	var parts []string

	for k, n := range f.nodes {
		if k != i {
			parts = append(parts, nodeString(n))
			continue
		}

		if j < 0 {
			continue
		}

		if rest := withoutListValue(n, j); rest != nil {
			parts = append(parts, nodeString(rest))
		}
	}

	for _, d := range f.dirs {
		parts = append(parts, (&query.Query{Directives: []query.Directive{d}}).String())
	}

	return strings.Join(parts, " ")
}

// withoutListValue returns a comma list (maybe negated) without child j: a
// shorter list, one term, or nil.
func withoutListValue(n query.Node, j int) query.Node {
	not, negated := n.(*query.Not)
	if negated {
		n = not.Child
	}

	or, ok := n.(*query.Or)
	if !ok {
		return nil
	}

	var kids []query.Node

	for k, c := range or.Children {
		if k != j {
			kids = append(kids, c)
		}
	}

	var out query.Node

	switch len(kids) {
	case 0:
		return nil
	case 1:
		out = kids[0]
	default:
		out = &query.Or{Children: kids, List: true}
	}

	if negated {
		return &query.Not{Child: out}
	}

	return out
}

// nodeString writes one top-level node. An OR that is not a comma list
// keeps its parentheses, so that the joined parts keep their meaning.
func nodeString(n query.Node) string {
	n = plainQuotes(n)
	s := (&query.Query{Root: n}).String()
	if or, ok := n.(*query.Or); ok && (!or.List || strings.Contains(s, " or ")) {
		s = "(" + s + ")"
	}

	return s
}

// numberFields are the facet fields with numbers.
var numberFields = map[string]bool{"power": true, "armor": true, "aember": true}

// matchFacet reports whether a term filters on the facet value. Spellings
// of one value match: aliases, case, set codes, slugs, and names. A number
// term matches each bucket that it selects (a range matches each number in
// it). A keyword term matches its keyword with any comparison.
func (e *Engine) matchFacet(t *query.Term, field, value string) bool {
	if t.Field != field {
		return false
	}

	switch {
	case field == "keyword" || field == "enhance":
		return query.Normalize(t.Key) == query.Normalize(value)
	case field == "set":
		return e.setCode(t.Value.Text) == e.setCode(value)
	case field == "artist":
		// An artist term matches part of the name, so it marks each artist
		// whose name has it: art:fabio marks Fábio Perez.
		return t.Op == query.OpHas && strings.Contains(query.Fold(value), query.Fold(t.Value.Text))
	case numberFields[field]:
		return matchNumber(t, value)
	}

	return (t.Op == query.OpHas || t.Op == query.OpEq) &&
		query.Normalize(t.Value.Text) == query.Normalize(value)
}

// matchNumber reports whether a number term selects the bucket value.
func matchNumber(t *query.Term, value string) bool {
	if value == "X" {
		return t.Value.X && (t.Op == query.OpHas || t.Op == query.OpEq)
	}

	n, err := strconv.Atoi(value)
	if err != nil || t.Value.X {
		return false
	}

	v := t.Value

	switch {
	case v.Range:
		return n >= v.Num && n <= v.Hi
	case t.Op == query.OpHas || t.Op == query.OpEq:
		return n == v.Num
	case t.Op == query.OpLt:
		return n < v.Num
	case t.Op == query.OpLe:
		return n <= v.Num
	case t.Op == query.OpGt:
		return n > v.Num
	case t.Op == query.OpGe:
		return n >= v.Num
	}

	return false
}

// setCode returns the set code of a set code, slug, or name in any case,
// or the text.
func (e *Engine) setCode(text string) string {
	norm := query.Normalize(text)

	for _, s := range e.sets {
		if strings.EqualFold(s.Code, text) || s.Slug == norm || query.Normalize(s.Name) == norm ||
			query.Normalize(s.Code) == norm {
			return s.Code
		}
	}

	return text
}

// facetRef returns the facet field and value of a one-term query such as
// "h:dis" or "s:wc".
func (e *Engine) facetRef(term string) (string, string, bool) {
	q, err := e.Parse(term)
	if err != nil {
		return "", "", false
	}

	t, ok := q.Root.(*query.Term)
	if !ok {
		return "", "", false
	}

	switch {
	case t.Field == "keyword" || t.Field == "enhance":
		return t.Field, t.Key, true
	case t.Field == "set":
		return t.Field, e.setCode(t.Value.Text), true
	case t.Value.X:
		return t.Field, "X", true
	}

	return t.Field, t.Value.Text, true
}

// FacetRef names one facet value.
type FacetRef struct {
	Field, Value string
}

// Excluded returns the facet values that top-level negated filters exclude.
// The results have none of them, so the facets show them with a count of 0,
// and a click removes the exclusion.
func (f *Filters) Excluded() []FacetRef {
	var out []FacetRef

	for _, n := range f.nodes {
		not, ok := n.(*query.Not)
		if !ok {
			continue
		}

		var terms []*query.Term

		switch c := not.Child.(type) {
		case *query.Term:
			terms = append(terms, c)
		case *query.Or:
			if !c.List {
				continue
			}

			for _, k := range c.Children {
				if t, ok := k.(*query.Term); ok {
					terms = append(terms, t)
				}
			}
		}

		for _, t := range terms {
			if v, ok := f.e.termValue(t); ok {
				out = append(out, FacetRef{Field: t.Field, Value: v})
			}
		}
	}

	return out
}

// termValue returns the facet value of a term: the value as the facets hold
// it.
func (e *Engine) termValue(t *query.Term) (string, bool) {
	switch {
	case t.Field == "keyword" || t.Field == "enhance":
		return t.Key, true
	case t.Field == "set":
		return e.setCode(t.Value.Text), true
	case t.Value.X:
		return "X", true
	case t.Value.Range:
		return "", false
	}

	return t.Value.Text, true
}

// ZeroCount returns the facet value with a count of 0, for a value that the
// query excludes.
func (e *Engine) ZeroCount(field, value string) Count {
	alias := field
	for a, f := range facetFields {
		if f == field {
			alias = a
		}
	}

	c := Count{
		Value: value,
		Label: e.facetLabel(field, value),
		Field: field,
		Term:  alias + ":" + quote(value),
	}

	switch field {
	case "house", "type", "rarity":
		c.Key = value
	case "set":
		if j := e.codeIndex(value); j < len(e.sets) {
			c.Key = e.sets[j].Slug
		}
	}

	return c
}

// facetLabel returns the label of a facet value, as the facets show it.
func (e *Engine) facetLabel(field, value string) string {
	switch field {
	case "house":
		return e.labels.House(value)
	case "type":
		return e.labels.Type(value)
	case "set":
		return e.setLabel(value)
	case "keyword":
		return keywordLabel(value)
	case "ability":
		return e.labels.Ability(value)
	case "trait":
		return traitLabel(value)
	}

	return value
}

// plainQuotes returns the node with the quotes dropped from values that do
// not need them: set:"Worlds Collide" parses to the slug worldscollide, and
// a rewritten query writes set:worldscollide.
func plainQuotes(n query.Node) query.Node {
	switch n := n.(type) {
	case *query.Term:
		if !n.Value.Quoted || strings.ContainsAny(n.Value.Text, " ,()\"") {
			return n
		}

		t := *n
		t.Value.Quoted = false

		return &t
	case *query.And:
		return &query.And{Children: plainAll(n.Children), Pos: n.Pos}
	case *query.Or:
		return &query.Or{Children: plainAll(n.Children), Pos: n.Pos, List: n.List}
	case *query.Not:
		return &query.Not{Child: plainQuotes(n.Child), Pos: n.Pos}
	}

	return n
}

func plainAll(nodes []query.Node) []query.Node {
	out := make([]query.Node, len(nodes))
	for i, n := range nodes {
		out[i] = plainQuotes(n)
	}

	return out
}
