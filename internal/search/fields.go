package search

import (
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/derive"
	"github.com/dmikalova/keyforge-data/query"
	"github.com/dmikalova/keyforge-data/sets"
)

// Entry is one printing of one card. The query runs on entries, so the
// printing fields (set, number, rarity, anomaly, flavor) and the card fields
// apply to the same printing.
type Entry struct {
	Card     *card.Card
	Printing *card.Printing
	// SetIndex is the position of the printing's set in sets.All. A higher
	// index is a newer set.
	SetIndex int
	// First is true for the card's printing in its oldest set.
	First bool
}

// The flags that every catalog has. The houseless kind labels come from the
// card data.
const (
	FlagAnomaly       = "anomaly"
	FlagReprint       = "reprint"
	FlagNew           = "new"
	FlagReference     = "reference"
	FlagTokenCreature = "tokencreature"
	FlagGigantic      = "gigantic"
	FlagEvilTwin      = "eviltwin"
)

// The directives.
const (
	DirSort   = "sort"
	DirOrder  = "order"
	DirUnique = "unique"
	DirSeed   = "seed"
)

// The sort keys of the sort directive.
var sortKeys = []string{
	"name", "set", "house", "type", "rarity", "power", "armor", "aember", "enhance", "random",
}

// HouseNone is the house value of a card without a house.
const HouseNone = "none"

// fields returns the card fields. The enum values come from the cards, so
// a new house, type, or rarity in the data needs no code change.
func fields(cards []card.Card) []query.Field[*Entry] {
	var houses, types, rarities []string

	for _, c := range cards {
		houses = appendNew(houses, HouseOf(&c))
		types = appendNew(types, c.Type)

		for _, p := range c.Printings {
			rarities = appendNew(rarities, p.Rarity)
		}
	}

	var setValues []string
	for _, s := range sets.All {
		setValues = append(setValues, s.Code, s.Slug, s.Name)
	}

	return []query.Field[*Entry]{
		{
			Name: "name", Alias: "n", Kind: query.KindFuzzy, Bare: true,
			Strings: func(e *Entry) []string {
				return append([]string{e.Card.Title}, e.Card.Aliases...)
			},
		},
		{
			Name: "text", Alias: "x", Kind: query.KindText, Bare: true,
			Strings: func(e *Entry) []string { return []string{e.Card.Text} },
		},
		{
			Name: "flavor", Alias: "fl", Kind: query.KindText, Bare: true,
			Strings: func(e *Entry) []string { return []string{e.Printing.Flavor} },
		},
		{
			Name: "house", Alias: "h", Kind: query.KindEnum, Values: houses,
			Strings: func(e *Entry) []string { return []string{HouseOf(e.Card)} },
		},
		{
			Name: "set", Alias: "s", Kind: query.KindEnum, Values: setValues,
			Strings: func(e *Entry) []string {
				if e.SetIndex < 0 {
					return nil
				}

				s := sets.All[e.SetIndex]

				return []string{s.Code, s.Slug, s.Name}
			},
		},
		{
			Name: "type", Alias: "t", Kind: query.KindEnum, Values: types,
			Strings: func(e *Entry) []string { return []string{e.Card.Type} },
		},
		{
			Name: "rarity", Alias: "r", Kind: query.KindEnum, Values: rarities,
			Strings: func(e *Entry) []string { return []string{e.Printing.Rarity} },
		},
		{
			Name: "keyword", Alias: "k", Kind: query.KindKeyed, Values: derive.KeywordNames(),
			Keyed: func(e *Entry) map[string]int { return e.Card.Keywords },
		},
		{
			// Traits are open: any value is allowed.
			Name: "trait", Alias: "tr", Kind: query.KindEnum,
			Strings: func(e *Entry) []string { return e.Card.Traits },
		},
		{
			Name: query.NumberField, Kind: query.KindNumber,
			Numbers: func(e *Entry) []query.Num {
				n, _ := strconv.Atoi(e.Printing.Number)

				return []query.Num{{N: n, S: e.Printing.Number}}
			},
		},
		{
			Name: "power", Alias: "p", Kind: query.KindNumber,
			Numbers: func(e *Entry) []query.Num {
				if e.Card.Power == nil {
					return nil
				}

				return []query.Num{{N: e.Card.Power.N, X: e.Card.Power.X}}
			},
		},
		{
			Name: "armor", Alias: "ar", Kind: query.KindNumber,
			Numbers: func(e *Entry) []query.Num {
				if e.Card.Power == nil {
					return nil
				}

				return []query.Num{{N: e.Card.Armor}}
			},
		},
		{
			Name: "aember", Alias: "a", Kind: query.KindNumber,
			Numbers: func(e *Entry) []query.Num { return []query.Num{{N: e.Card.Aember}} },
		},
		{
			Name: "enhance", Alias: "en", Kind: query.KindKeyed,
			Keyed: func(e *Entry) map[string]int { return e.Card.Enhance },
		},
		{Name: DirSort, Kind: query.KindDirective, Values: sortKeys},
		{
			Name: DirOrder, Kind: query.KindDirective, Values: []string{"asc", "desc"}},
		{
			Name: DirUnique, Kind: query.KindDirective, Values: []string{"prints"}},
		{Name: DirSeed, Kind: query.KindDirective},
	}
}

// flags returns the "is:" flags: the fixed flags and every other flag in the
// card data (the houseless kind labels).
func flags(cards []card.Card) []query.Flag[*Entry] {
	out := []query.Flag[*Entry]{
		{Name: FlagAnomaly, Test: func(e *Entry) bool { return e.Printing.Anomaly }},
		{Name: FlagReprint, Test: func(e *Entry) bool { return !e.First }},
		{Name: FlagNew, Test: func(e *Entry) bool { return e.First }},
	}
	known := []string{FlagAnomaly, FlagReprint, FlagNew}

	for _, name := range []string{FlagReference, FlagTokenCreature, FlagGigantic, FlagEvilTwin} {
		known = append(known, name)
		out = append(out, cardFlag(name))
	}

	for _, c := range cards {
		for _, f := range c.Flags {
			if !slices.Contains(known, f) {
				known = append(known, f)
				out = append(out, cardFlag(f))
			}
		}
	}

	return out
}

func cardFlag(name string) query.Flag[*Entry] {
	return query.Flag[*Entry]{Name: name, Test: func(e *Entry) bool { return e.Card.HasFlag(name) }}
}

// HouseOf returns the card's house, or HouseNone.
func HouseOf(c *card.Card) string {
	if c.House == "" {
		return HouseNone
	}

	return c.House
}

// lastGroup reports whether a card sorts after the other cards by default:
// a reference card or a card without a house.
func lastGroup(c *card.Card) bool {
	return c.House == "" || c.HasFlag(FlagReference)
}

func appendNew(list []string, v string) []string {
	if v == "" ||
		slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, v) }) {
		return list
	}

	return append(list, v)
}
