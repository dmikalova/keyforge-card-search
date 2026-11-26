// Package search runs a query on the catalog in memory (ADR 0002). It wires
// the card fields into the keyforge-data query registry, picks the default
// printing of each card, sorts the results, and counts the facets.
package search

import (
	"cmp"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
	"github.com/dmikalova/keyforge-data/sets"
)

// Item is one result: a card with the printing that the tile shows.
type Item struct {
	Card     *card.Card
	Printing *card.Printing
}

// Result is the outcome of a query.
type Result struct {
	Query *query.Query
	// Items are all the results, in order.
	Items []Item
	// Facets count the results.
	Facets Facets
}

// ErrNoSeed is the error of Search for "sort:random" without "seed:". The
// caller adds a seed to the query, so the URL holds the order.
var ErrNoSeed = errors.New("search: sort:random needs a seed")

// Engine searches one catalog.
type Engine struct {
	reg     *query.Registry[*Entry]
	entries []*Entry
}

// New returns an engine for the cards. The engine keeps the cards.
func New(cards []card.Card) (*Engine, error) {
	reg, err := query.NewRegistry(fields(cards), flags(cards))
	if err != nil {
		return nil, err
	}

	e := &Engine{reg: reg}

	for i := range cards {
		c := &cards[i]

		first := 0

		for j := range c.Printings {
			if setIndex(c.Printings[j].Set) < setIndex(c.Printings[first].Set) {
				first = j
			}
		}

		for j := range c.Printings {
			e.entries = append(e.entries, &Entry{
				Card:     c,
				Printing: &c.Printings[j],
				SetIndex: setIndex(c.Printings[j].Set),
				First:    j == first,
			})
		}
	}

	return e, nil
}

// Schema returns the query schema, for the syntax page and autocomplete.
func (e *Engine) Schema() query.Schema {
	return e.reg
}

// Parse parses a query.
func (e *Engine) Parse(src string) (*query.Query, error) {
	return e.reg.Parse(src)
}

// Search parses and runs a query. A parse error is a *query.Error.
func (e *Engine) Search(src string) (*Result, error) {
	q, err := e.reg.Parse(src)
	if err != nil {
		return nil, err
	}

	match := e.reg.Match(q)

	var matched []*Entry

	for _, en := range e.entries {
		if match(en) {
			matched = append(matched, en)
		}
	}

	items := group(matched, isDirective(q, DirUnique, "prints"))

	if err := e.sortItems(q, items); err != nil {
		return nil, err
	}

	return &Result{Query: q, Items: items, Facets: count(items)}, nil
}

// group makes the results. With prints, each entry is a result. Otherwise
// each card is one result with its default printing: the newest set, and a
// printing that is not an anomaly before one that is.
func group(entries []*Entry, prints bool) []Item {
	if prints {
		out := make([]Item, len(entries))
		for i, en := range entries {
			out[i] = Item{Card: en.Card, Printing: en.Printing}
		}

		return out
	}

	best := map[*card.Card]*Entry{}

	var order []*card.Card

	for _, en := range entries {
		cur, ok := best[en.Card]
		if !ok {
			order = append(order, en.Card)
			best[en.Card] = en

			continue
		}

		if better(en, cur) {
			best[en.Card] = en
		}
	}

	out := make([]Item, len(order))
	for i, c := range order {
		out[i] = Item{Card: c, Printing: best[c].Printing}
	}

	return out
}

// better reports whether a is a better default printing than b.
func better(a, b *Entry) bool {
	if a.Printing.Anomaly != b.Printing.Anomaly {
		return !a.Printing.Anomaly
	}

	return a.SetIndex > b.SetIndex
}

func isDirective(q *query.Query, name, value string) bool {
	v, ok := q.Directive(name)

	return ok && v == value
}

// sortItems puts the items in the order of the query. Without a sort
// directive, a query of only name terms sorts by match quality. Otherwise
// the order is house and name, with reference and houseless cards last.
func (e *Engine) sortItems(q *query.Query, items []Item) error {
	key, hasSort := q.Directive(DirSort)
	desc := isDirective(q, DirOrder, "desc")

	if key == "random" {
		v, ok := q.Directive(DirSeed)
		if !ok {
			return ErrNoSeed
		}

		seed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return &query.Error{Msg: "seed must be a positive number"}
		}

		slices.SortFunc(items, byDefault)
		r := rand.New(rand.NewPCG(seed, seed))
		r.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })

		return nil
	}

	if !hasSort {
		if scores, ok := e.fuzzyScores(q, items); ok {
			slices.SortStableFunc(items, func(a, b Item) int {
				return cmp.Or(cmp.Compare(scores[b.Card], scores[a.Card]), byDefault(a, b))
			})

			return nil
		}
	}

	by := sorters[key]
	if by == nil {
		by = byDefault
	}

	slices.SortStableFunc(items, func(a, b Item) int {
		c := by(a, b)
		if desc {
			c = -c
		}

		return cmp.Or(c, byDefault(a, b))
	})

	return nil
}

// fuzzyScores returns the name match quality of each item, and false if
// the query is not only name terms.
func (e *Engine) fuzzyScores(q *query.Query, items []Item) (map[*card.Card]int, bool) {
	scores := map[*card.Card]int{}

	for _, it := range items {
		en := &Entry{Card: it.Card, Printing: it.Printing}

		s, ok := e.reg.FuzzyRank(q, en)
		if !ok {
			return nil, false
		}

		scores[it.Card] = s
	}

	return scores, len(items) > 0
}

// byDefault orders by house and name, with reference and houseless cards
// last, then by set.
func byDefault(a, b Item) int {
	return cmp.Or(
		compareBool(lastGroup(a.Card), lastGroup(b.Card)),
		cmp.Compare(a.Card.House, b.Card.House),
		cmp.Compare(foldedOf(a.Card), foldedOf(b.Card)),
		cmp.Compare(a.Card.ID, b.Card.ID),
		cmp.Compare(setIndex(a.Printing.Set), setIndex(b.Printing.Set)),
		cmp.Compare(a.Printing.Number, b.Printing.Number),
	)
}

var sorters = map[string]func(a, b Item) int{
	"name": func(a, b Item) int { return cmp.Compare(foldedOf(a.Card), foldedOf(b.Card)) },
	"set": func(a, b Item) int {
		return cmp.Or(
			cmp.Compare(setIndex(a.Printing.Set), setIndex(b.Printing.Set)),
			cmp.Compare(a.Printing.Number, b.Printing.Number),
		)
	},
	"house": func(a, b Item) int { return cmp.Compare(a.Card.House, b.Card.House) },
	"type":  func(a, b Item) int { return cmp.Compare(a.Card.Type, b.Card.Type) },
	"rarity": func(a, b Item) int {
		return cmp.Compare(rarityRank(a.Printing.Rarity), rarityRank(b.Printing.Rarity))
	},
	"power":  func(a, b Item) int { return cmp.Compare(powerRank(a.Card), powerRank(b.Card)) },
	"armor":  func(a, b Item) int { return cmp.Compare(a.Card.Armor, b.Card.Armor) },
	"aember": func(a, b Item) int { return cmp.Compare(a.Card.Aember, b.Card.Aember) },
	"enhance": func(a, b Item) int {
		return cmp.Compare(total(a.Card.Enhance), total(b.Card.Enhance))
	},
}

// rarityOrder is the order of the printed rarities. Other rarities sort
// after these.
var rarityOrder = []string{"common", "uncommon", "rare", "special", "variant"}

func rarityRank(r string) int {
	i := slices.Index(rarityOrder, strings.ToLower(r))
	if i < 0 {
		return len(rarityOrder)
	}

	return i
}

// powerRank orders cards without power first, then by power, then X.
func powerRank(c *card.Card) int {
	switch {
	case c.Power == nil:
		return -1
	case c.Power.X:
		return 1 << 20
	default:
		return c.Power.N
	}
}

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}

	return n
}

func foldedOf(c *card.Card) string {
	if c.Folded != "" {
		return c.Folded
	}

	return query.Fold(c.Title)
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// setIndex returns the position of the set in sets.All, or -1.
func setIndex(slug string) int {
	return slices.IndexFunc(sets.All, func(s sets.Set) bool { return s.Slug == slug })
}

// SetOf returns the set of a slug.
func SetOf(slug string) (sets.Set, bool) {
	i := setIndex(slug)
	if i < 0 {
		return sets.Set{}, false
	}

	return sets.All[i], true
}
