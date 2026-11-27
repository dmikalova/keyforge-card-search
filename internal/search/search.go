// Package search runs a query on the dataset in memory (ADR 0002). It wires
// the card fields into the keyforge-data query registry, picks the printing
// that each result shows, sorts the results, and counts the facets.
//
// A query matches cards, not printings: a term matches a card if one of the
// card's printings matches it. So "s:aoa s:wc" gives the cards that have a
// printing in AoA and a printing in WC.
package search

import (
	"cmp"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
	"github.com/dmikalova/keyforge-data/sets"
)

// Item is one result: a card with the printing that the tile shows.
type Item struct {
	Card     *card.Card
	Printing *card.Printing
	// Set is the set of the printing. Its slug is the printing's set slug if
	// the dataset does not list the set.
	Set sets.Set
	// SetIndex is the position of the set in the dataset's sets, or -1.
	SetIndex int
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

// Engine searches one dataset.
type Engine struct {
	reg *query.Registry[*Entry]
	// cards has one entry per card, with all its printings.
	cards []*Entry
	// prints has one entry per printing of each card, in the order of the
	// card's printings.
	prints  map[*card.Card][]*Entry
	sets    []sets.Set
	setRank map[string]int
	version string
	at      time.Time
	// artistAliases are the other spellings of each artist.
	artistAliases map[string][]string
	// labels are the names of the data values for people.
	labels Labels
	// flagTests are the "is:" flags in order, for the flag facet.
	flagTests []query.Flag[*Entry]
	// entryOf is the entry with all the printings of each card.
	entryOf map[*card.Card]*Entry
}

// New returns an engine for the dataset. The engine keeps the dataset.
func New(ds *card.Dataset) (*Engine, error) {
	setList := ds.Sets
	if len(setList) == 0 {
		setList = datasetSets(ds.Cards)
	}

	e := &Engine{
		sets:          setList,
		setRank:       map[string]int{},
		prints:        map[*card.Card][]*Entry{},
		entryOf:       map[*card.Card]*Entry{},
		version:       ds.Version,
		at:            ds.Generated,
		artistAliases: aliasesByArtist(ds.ArtistAliases),
		labels:        Labels(ds.Labels),
	}
	for i, s := range setList {
		e.setRank[s.Slug] = i
	}

	e.flagTests = e.flags(ds.Cards)

	reg, err := query.NewRegistry(e.fields(ds.Cards), e.flagTests)
	if err != nil {
		return nil, err
	}

	e.reg = reg

	for i := range ds.Cards {
		c := &ds.Cards[i]
		first := e.firstSet(c)
		all := &Entry{Card: c, FirstSet: first}

		for j := range c.Printings {
			p := &c.Printings[j]
			all.Printings = append(all.Printings, p)
			e.prints[c] = append(e.prints[c], &Entry{
				Card: c, Printings: []*card.Printing{p}, FirstSet: first,
			})
		}

		e.cards = append(e.cards, all)
		e.entryOf[c] = all
	}

	return e, nil
}

// Version returns the dataset version.
func (e *Engine) Version() string {
	return e.version
}

// Generated returns the time that the dataset was made.
func (e *Engine) Generated() time.Time {
	return e.at
}

// Len returns the number of cards.
func (e *Engine) Len() int {
	return len(e.cards)
}

// Sets returns the dataset's sets, oldest first.
func (e *Engine) Sets() []sets.Set {
	return e.sets
}

// Schema returns the query schema, for the syntax page and autocomplete.
func (e *Engine) Schema() query.Schema {
	return e.reg
}

// Parse parses a query.
func (e *Engine) Parse(src string) (*query.Query, error) {
	q, err := e.reg.Parse(src)
	if err != nil {
		return nil, err
	}

	if err := normalizeSort(q); err != nil {
		return nil, err
	}

	return q, nil
}

// Search parses and runs a query. A parse error is a *query.Error.
func (e *Engine) Search(src string) (*Result, error) {
	q, err := e.Parse(src)
	if err != nil {
		return nil, err
	}

	match := e.reg.Match(forMatch(q))
	show := e.printingFilter(forMatch(q))
	prints := isDirective(q, DirUnique, "prints")

	var items []Item

	for _, en := range e.cards {
		if !match(en) {
			continue
		}

		items = append(items, e.shown(en.Card, show, prints)...)
	}

	if err := e.sortItems(q, items); err != nil {
		return nil, err
	}

	return &Result{Query: q, Items: items, Facets: e.count(items, prints)}, nil
}

// shown returns the results of a matched card. The candidates are the
// printings that the first test of show selects that selects any; if no
// test selects a printing, every printing is a candidate. With prints, each candidate is a result. Otherwise the
// result is the best candidate: the newest set, and a printing that is not
// an anomaly before one that is.
func (e *Engine) shown(c *card.Card, show []func(*Entry) bool, prints bool) []Item {
	all := e.prints[c]

	var picked []*Entry

	for _, test := range show {
		for _, p := range all {
			if test(p) {
				picked = append(picked, p)
			}
		}

		if len(picked) > 0 {
			break
		}
	}

	if len(picked) == 0 {
		picked = all
	}

	if prints {
		out := make([]Item, len(picked))
		for i, p := range picked {
			out[i] = e.item(c, p.Printings[0])
		}

		return out
	}

	best := picked[0]
	for _, p := range picked[1:] {
		if e.better(p.Printings[0], best.Printings[0]) {
			best = p
		}
	}

	return []Item{e.item(c, best.Printings[0])}
}

func (e *Engine) item(c *card.Card, p *card.Printing) Item {
	i := e.setIndex(p.Set)
	s := sets.Set{Slug: p.Set, Code: p.Set, Name: p.Set}

	if i >= 0 {
		s = e.sets[i]
	}

	return Item{Card: c, Printing: p, Set: s, SetIndex: i}
}

// better reports whether a is a better default printing than b.
func (e *Engine) better(a, b *card.Printing) bool {
	if a.Anomaly != b.Anomaly {
		return !a.Anomaly
	}

	return e.setIndex(a.Set) > e.setIndex(b.Set)
}

// printingFilter returns the tests of one printing from the query's printing
// terms (set, rarity, number, flavor, and the printing flags) that are not
// inside a NOT. The first test is the printing part of the query. The second
// test matches a printing that matches any of the printing terms: with
// "s:aoa s:wc" no printing is in both sets, so the result shows the newest
// printing in AoA or WC. It returns nil if the query has no printing terms,
// so the result shows the default printing.
func (e *Engine) printingFilter(q *query.Query) []func(*Entry) bool {
	n, ok := printingPart(q.Root)
	if !ok {
		return nil
	}

	strict := e.reg.Match(&query.Query{Root: n, Directives: q.Directives})

	terms := printingTerms(n)
	if len(terms) < 2 {
		return []func(*Entry) bool{strict}
	}

	loose := e.reg.Match(&query.Query{
		Root: &query.Or{Children: terms, Pos: n.Span()}, Directives: q.Directives,
	})

	return []func(*Entry) bool{strict, loose}
}

// printingTerms returns the terms in the printing part of a query.
func printingTerms(n query.Node) []query.Node {
	switch n := n.(type) {
	case *query.Term:
		return []query.Node{n}
	case *query.And:
		return childTerms(n.Children)
	case *query.Or:
		return childTerms(n.Children)
	default:
		return nil
	}
}

func childTerms(nodes []query.Node) []query.Node {
	var out []query.Node
	for _, c := range nodes {
		out = append(out, printingTerms(c)...)
	}

	return out
}

// printingPart returns the part of the node that tests printings: the node
// itself if it is a printing term, the printing children of an AND, or an
// OR whose children all test printings.
func printingPart(n query.Node) (query.Node, bool) {
	switch n := n.(type) {
	case *query.Term:
		return n, printingTerm(n)
	case *query.And:
		var keep []query.Node

		for _, c := range n.Children {
			if p, ok := printingPart(c); ok {
				keep = append(keep, p)
			}
		}

		switch len(keep) {
		case 0:
			return nil, false
		case 1:
			return keep[0], true
		default:
			return &query.And{Children: keep, Pos: n.Pos}, true
		}
	case *query.Or:
		for _, c := range n.Children {
			if _, ok := printingPart(c); !ok {
				return nil, false
			}
		}

		return n, true
	default:
		return nil, false
	}
}

func printingTerm(t *query.Term) bool {
	if t.Op == query.OpNe {
		return false
	}

	if t.Field == query.FlagField {
		return slices.Contains(printingFlags, t.Value.Text)
	}

	return slices.Contains(printingFields, t.Field)
}

func isDirective(q *query.Query, name, value string) bool {
	v, ok := q.Directive(name)

	return ok && v == value
}

// sortItems puts the items in the order of the query. The sort directive is
// a comma list of keys (see sortKeys). Each key sorts the ties of the keys
// before it, and byDefault sorts the last ties. A "-" before a key makes it
// descending. "order:desc" reverses the whole order. Without a sort
// directive, a query with name terms or bare words sorts by name match
// quality, and other queries sort by byDefault.
func (e *Engine) sortItems(q *query.Query, items []Item) error {
	value, hasSort := q.Directive(DirSort)
	desc := isDirective(q, DirOrder, "desc")
	keys := sortList(value)

	if len(keys) == 1 && keys[0].name == SortRandom {
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
		if scores, ok := e.nameScores(q, items); ok {
			slices.SortStableFunc(items, func(a, b Item) int {
				return cmp.Or(cmp.Compare(scores[b.Card], scores[a.Card]), byDefault(a, b))
			})

			return nil
		}
	}

	slices.SortStableFunc(items, func(a, b Item) int {
		c := 0

		for _, k := range keys {
			if c = sorters[k.name](a, b); c != 0 {
				if k.desc {
					c = -c
				}

				break
			}
		}

		c = cmp.Or(c, byDefault(a, b))
		if desc {
			c = -c
		}

		return c
	})

	return nil
}

// byHouse orders the cards with a house by house, then the cards without a
// house in the order of their groups (card.HouselessGroups).
func byHouse(a, b Item) int {
	return cmp.Or(
		cmp.Compare(groupRank(a.Card), groupRank(b.Card)),
		cmp.Compare(a.Card.House, b.Card.House),
	)
}

// groupRank is 0 for a card with a house. A card without a house ranks
// after it, in the order of card.HouselessGroups. A card with an unknown
// group, or with no group, ranks as "houseless", or as "reference" if it is
// a reference card.
func groupRank(c *card.Card) int {
	if !lastGroup(c) {
		return 0
	}

	i := slices.Index(card.HouselessGroups, GroupOf(c))
	if i < 0 {
		i = slices.Index(card.HouselessGroups, "houseless")
	}

	return 1 + i
}

// GroupOf returns the sort group of a card without a house: the card's
// group from the dataset, or "reference" or "houseless".
func GroupOf(c *card.Card) string {
	switch {
	case c.Group != "":
		return c.Group
	case c.HasFlag(FlagReference):
		return "reference"
	default:
		return "houseless"
	}
}

// nameScores returns the name match quality of each item, and false if
// the query has no name terms or bare words.
func (e *Engine) nameScores(q *query.Query, items []Item) (map[*card.Card]int, bool) {
	scores := map[*card.Card]int{}

	for _, it := range items {
		s, ok := e.reg.NameRank(q, &Entry{Card: it.Card})
		if !ok {
			return nil, false
		}

		scores[it.Card] = s
	}

	return scores, len(items) > 0
}

// byDefault orders by house and name: the cards with a house first, then
// the cards without a house in the order of their groups
// (card.HouselessGroups), then by set.
func byDefault(a, b Item) int {
	return cmp.Or(
		byHouse(a, b),
		cmp.Compare(foldedOf(a.Card), foldedOf(b.Card)),
		cmp.Compare(a.Card.ID, b.Card.ID),
		cmp.Compare(a.SetIndex, b.SetIndex),
		cmp.Compare(a.Printing.Number, b.Printing.Number),
	)
}

var sorters = map[string]func(a, b Item) int{
	"name": func(a, b Item) int { return cmp.Compare(foldedOf(a.Card), foldedOf(b.Card)) },
	"set": func(a, b Item) int {
		return cmp.Or(
			cmp.Compare(a.SetIndex, b.SetIndex),
			cmp.Compare(a.Printing.Number, b.Printing.Number),
		)
	},
	"house": byHouse,
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
	// artist sorts by the shown printing's artist, folded. A printing
	// without an artist sorts last.
	"artist": func(a, b Item) int {
		x, y := a.Printing.Artist, b.Printing.Artist

		return cmp.Or(compareBool(x == "", y == ""), cmp.Compare(query.Fold(x), query.Fold(y)))
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

// setIndex returns the position of the set in the dataset's sets, or -1.
func (e *Engine) setIndex(slug string) int {
	if i, ok := e.setRank[slug]; ok {
		return i
	}

	return -1
}

// firstSet returns the slug of the card's oldest printing.
func (e *Engine) firstSet(c *card.Card) string {
	first := ""

	for _, p := range c.Printings {
		i := e.setIndex(p.Set)
		if first == "" || i >= 0 && i < e.setIndex(first) {
			first = p.Set
		}
	}

	return first
}

// SetOf returns the dataset's set of a slug.
func (e *Engine) SetOf(slug string) (sets.Set, bool) {
	i := e.setIndex(slug)
	if i < 0 {
		return sets.Set{}, false
	}

	return e.sets[i], true
}
