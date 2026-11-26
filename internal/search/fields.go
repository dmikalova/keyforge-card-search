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

// Entry is a card with some of its printings. The query runs on entries. A
// printing field (set, rarity, number, flavor) matches if one of the entry's
// printings matches. A search uses an entry with all the printings, so a
// term matches a card if one of its printings matches. The printing that a
// result shows comes from entries with one printing each.
type Entry struct {
	Card      *card.Card
	Printings []*card.Printing
	// FirstSet is the set slug of the card's oldest printing.
	FirstSet string
}

// printingFields are the fields that test a printing, not the card.
var printingFields = []string{"set", "rarity", query.NumberField, "flavor", FieldArtist}

// printingFlags are the flags that test a printing, not the card.
var printingFlags = []string{FlagAnomaly, FlagReprint}

// The flags that every dataset has. The houseless kind labels come from the
// card data.
const (
	FlagAnomaly       = "anomaly"
	FlagReprint       = "reprint"
	FlagHouseless     = "houseless"
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
	"name", "set", "house", "type", "rarity", "power", "armor", "aember", "enhance", "artist",
	"random",
}

// FieldGroup is the field of the groups of the cards without a house.
const FieldGroup = "group"

// HouseNone is the house value of a card without a house.
const HouseNone = "none"

// fields returns the card fields. The enum values come from the dataset, so
// a new house, type, rarity, or set in the data needs no code change.
func (e *Engine) fields(cards []card.Card) []query.Field[*Entry] {
	var houses, types, rarities []string

	for _, c := range cards {
		houses = appendNew(houses, HouseOf(&c))
		types = appendNew(types, c.Type)

		for _, p := range c.Printings {
			rarities = appendNew(rarities, p.Rarity)
		}
	}

	var setValues []string
	for _, s := range e.sets {
		setValues = append(setValues, s.Code, s.Slug, s.Name)
	}

	setNames := func(slug string) []string {
		if s, ok := e.SetOf(slug); ok {
			return []string{s.Code, s.Slug, s.Name}
		}

		return []string{slug}
	}

	return []query.Field[*Entry]{
		{
			Name: "name", Alias: "n", Kind: query.KindName, Bare: true,
			Strings: func(en *Entry) []string {
				return append([]string{en.Card.Title}, en.Card.Aliases...)
			},
		},
		{
			Name: "text", Alias: "x", Kind: query.KindText, Bare: true,
			Strings: func(en *Entry) []string { return []string{searchText(en.Card.Text)} },
		},
		{
			Name: "flavor", Alias: "fl", Kind: query.KindText, Bare: true,
			Strings: func(en *Entry) []string {
				return each(en, func(p *card.Printing) []string { return []string{p.Flavor} })
			},
		},
		{
			Name: "house", Alias: "h", Kind: query.KindEnum, Values: houses,
			Strings: func(en *Entry) []string { return []string{HouseOf(en.Card)} },
		},
		{
			// group is the sort group of a card without a house (GroupOf). A
			// card with a house has no group.
			Name: FieldGroup, Alias: "g", Kind: query.KindEnum, Values: card.HouselessGroups,
			Strings: func(en *Entry) []string {
				if !lastGroup(en.Card) {
					return nil
				}

				return []string{GroupOf(en.Card)}
			},
		},
		{
			Name: "set", Alias: "s", Kind: query.KindEnum, Values: setValues,
			Strings: func(en *Entry) []string {
				return each(en, func(p *card.Printing) []string { return setNames(p.Set) })
			},
		},
		{
			Name: "first", Kind: query.KindEnum, Values: setValues,
			Strings: func(en *Entry) []string { return setNames(en.FirstSet) },
		},
		{
			Name: "type", Alias: "t", Kind: query.KindEnum, Values: types,
			Strings: func(en *Entry) []string { return []string{en.Card.Type} },
		},
		{
			Name: "rarity", Alias: "r", Kind: query.KindEnum, Values: rarities,
			Strings: func(en *Entry) []string {
				return each(en, func(p *card.Printing) []string { return []string{p.Rarity} })
			},
		},
		{
			Name: "keyword", Alias: "k", Kind: query.KindKeyed, Values: derive.KeywordNames(),
			Keyed: func(en *Entry) map[string]int { return en.Card.Keywords },
		},
		{
			Name: "ability", Alias: "ab", Kind: query.KindEnum, Values: derive.Abilities,
			Strings: func(en *Entry) []string { return en.Card.Abilities },
		},
		{
			// Traits are open: any value is allowed.
			Name: "trait", Alias: "tr", Kind: query.KindEnum,
			Strings: func(en *Entry) []string { return en.Card.Traits },
		},
		{
			Name: query.NumberField, Kind: query.KindNumber,
			Numbers: func(en *Entry) []query.Num {
				out := make([]query.Num, len(en.Printings))
				for i, p := range en.Printings {
					n, _ := strconv.Atoi(p.Number)
					out[i] = query.Num{N: n, S: p.Number}
				}

				return out
			},
		},
		{
			Name: "power", Alias: "p", Kind: query.KindNumber,
			Numbers: func(en *Entry) []query.Num {
				if en.Card.Power == nil {
					return nil
				}

				return []query.Num{{N: en.Card.Power.N, X: en.Card.Power.X}}
			},
		},
		{
			Name: "armor", Alias: "ar", Kind: query.KindNumber,
			Numbers: func(en *Entry) []query.Num {
				if en.Card.Power == nil {
					return nil
				}

				return []query.Num{{N: en.Card.Armor}}
			},
		},
		{
			Name: "aember", Alias: "a", Kind: query.KindNumber,
			Numbers: func(en *Entry) []query.Num { return []query.Num{{N: en.Card.Aember}} },
		},
		{
			Name: "enhance", Alias: "en", Kind: query.KindKeyed,
			Keyed: func(en *Entry) map[string]int { return en.Card.Enhance },
		},
		{
			// The artist of each printing, folded for search: "artist:fabio"
			// finds Fábio Perez. "artist:none" finds a printing without an
			// artist (forMatch).
			Name: FieldArtist, Alias: "art", Kind: query.KindText,
			Strings: e.artists,
		},
		// The sort value is a comma list of sortKeys. Engine.Parse checks it.
		{Name: DirSort, Kind: query.KindDirective},
		{Name: DirOrder, Kind: query.KindDirective, Values: []string{"asc", "desc"}},
		{Name: DirUnique, Kind: query.KindDirective, Values: []string{"prints"}},
		{Name: DirSeed, Kind: query.KindDirective},
	}
}

// each returns the strings of each printing of the entry.
func each(en *Entry, get func(*card.Printing) []string) []string {
	var out []string
	for _, p := range en.Printings {
		out = append(out, get(p)...)
	}

	return out
}

// flags returns the "is:" flags: the fixed flags and every other flag in the
// card data (the houseless kind labels).
func (e *Engine) flags(cards []card.Card) []query.Flag[*Entry] {
	out := []query.Flag[*Entry]{
		{Name: FlagAnomaly, Test: func(en *Entry) bool {
			return slices.ContainsFunc(
				en.Printings,
				func(p *card.Printing) bool { return p.Anomaly },
			)
		}},
		{Name: FlagReprint, Test: func(en *Entry) bool {
			return slices.ContainsFunc(en.Printings, func(p *card.Printing) bool {
				return p.Set != en.FirstSet
			})
		}},
	}
	known := []string{FlagAnomaly, FlagReprint}

	for _, name := range []string{
		FlagHouseless, FlagReference, FlagTokenCreature, FlagGigantic, FlagEvilTwin,
	} {
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
	return query.Flag[*Entry]{
		Name: name,
		Test: func(en *Entry) bool { return en.Card.HasFlag(name) },
	}
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

// datasetSets returns the sets in sets.All that the cards use, for a
// dataset without a set list.
func datasetSets(cards []card.Card) []sets.Set {
	used := map[string]bool{}

	for _, c := range cards {
		for _, p := range c.Printings {
			used[p.Set] = true
		}
	}

	var out []sets.Set

	for _, s := range sets.All {
		if used[s.Slug] {
			out = append(out, s)
		}
	}

	return out
}

// textTokens are the icon tokens of the card text and the words that text
// search matches for them: "x:tide" finds the cards with the tide icon.
var textTokens = strings.NewReplacer("{tide}", "tide")

// searchText returns card text for text search, with the icon tokens as
// words.
func searchText(s string) string {
	if !strings.Contains(s, "{") {
		return s
	}

	return textTokens.Replace(s)
}
